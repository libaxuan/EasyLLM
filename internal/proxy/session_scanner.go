package proxy

import (
	"bufio"
	"easyllm/internal/models"
	"easyllm/internal/storage"
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// SessionScanner monitors ~/.codex/sessions/ for new Codex CLI session files
// and imports usage data into the dashboard log database.
type SessionScanner struct {
	codexDB  *storage.CodexStorage
	baseDir  string
	seen     map[string]bool
	mu       sync.Mutex
	stopCh   chan struct{}
}

func NewSessionScanner(codexDB *storage.CodexStorage) *SessionScanner {
	home, _ := os.UserHomeDir()
	return &SessionScanner{
		codexDB: codexDB,
		baseDir: filepath.Join(home, ".codex", "sessions"),
		seen:    make(map[string]bool),
		stopCh:  make(chan struct{}),
	}
}

func (s *SessionScanner) Start() {
	s.loadSeen()
	go s.pollLoop()
}

func (s *SessionScanner) Stop() {
	close(s.stopCh)
}

func (s *SessionScanner) loadSeen() {
	if s.codexDB == nil {
		return
	}
	if n := s.codexDB.BackfillPlatform(localPlatform()); n > 0 {
		log.Printf("[session-scanner] backfilled platform for %d existing logs", n)
	}
	paths := s.codexDB.GetSessionLogPaths()
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, p := range paths {
		s.seen[p] = true
	}
}

func (s *SessionScanner) pollLoop() {
	s.scan()
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			s.scan()
		case <-s.stopCh:
			return
		}
	}
}

func (s *SessionScanner) scan() {
	now := time.Now()
	// Scan today and yesterday
	for _, d := range []time.Time{now, now.AddDate(0, 0, -1)} {
		dir := filepath.Join(s.baseDir, d.Format("2006"), d.Format("01"), d.Format("02"))
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".jsonl") {
				continue
			}
			key := "session:" + e.Name()
			s.mu.Lock()
			alreadySeen := s.seen[key]
			s.mu.Unlock()
			if alreadySeen {
				continue
			}

			info, err := e.Info()
			if err != nil {
				continue
			}
			// Skip files still being written (modified in last 5 seconds)
			if time.Since(info.ModTime()) < 5*time.Second {
				continue
			}

			fullPath := filepath.Join(dir, e.Name())
			if err := s.importSession(fullPath, key); err != nil {
				log.Printf("[session-scanner] error importing %s: %v", e.Name(), err)
			}
		}
	}
}

type sessionMeta struct {
	ID            string `json:"id"`
	Originator    string `json:"originator"`
	ModelProvider string `json:"model_provider"`
	CLIVersion    string `json:"cli_version"`
}

type tokenCountInfo struct {
	TotalTokenUsage struct {
		InputTokens  int64 `json:"input_tokens"`
		OutputTokens int64 `json:"output_tokens"`
	} `json:"total_token_usage"`
}

func (s *SessionScanner) importSession(path, key string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	var (
		model        string
		sessionID    string
		inputTokens  int64
		outputTokens int64
		sessionStart time.Time
		sessionEnd   time.Time
	)

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)
	for scanner.Scan() {
		var raw map[string]interface{}
		if err := json.Unmarshal(scanner.Bytes(), &raw); err != nil {
			continue
		}

		ts, _ := raw["timestamp"].(string)
		t, _ := time.Parse(time.RFC3339Nano, ts)
		if !t.IsZero() {
			if sessionStart.IsZero() || t.Before(sessionStart) {
				sessionStart = t
			}
			if t.After(sessionEnd) {
				sessionEnd = t
			}
		}

		lineType, _ := raw["type"].(string)
		payload, _ := raw["payload"].(map[string]interface{})

		switch lineType {
		case "session_meta":
			if payload != nil {
				if id, ok := payload["id"].(string); ok {
					sessionID = id
				}
			}
		case "turn_context":
			if payload != nil {
				if m, ok := payload["model"].(string); ok && m != "" {
					model = m
				}
			}
		case "event_msg":
			if payload == nil {
				continue
			}
			payloadType, _ := payload["type"].(string)
			if payloadType == "token_count" {
				infoRaw, _ := payload["info"].(map[string]interface{})
				if infoRaw == nil {
					continue
				}
				totalUsage, _ := infoRaw["total_token_usage"].(map[string]interface{})
				if totalUsage != nil {
					if v, ok := totalUsage["input_tokens"].(float64); ok {
						inputTokens = int64(v)
					}
					if v, ok := totalUsage["output_tokens"].(float64); ok {
						outputTokens = int64(v)
					}
				}
			}
		}
	}

	if model == "" && inputTokens == 0 {
		s.mu.Lock()
		s.seen[key] = true
		s.mu.Unlock()
		return nil
	}

	// Get account info from auth.json
	email, accountID := getActiveAccountInfo()

	duration := int64(0)
	if !sessionStart.IsZero() && !sessionEnd.IsZero() {
		duration = sessionEnd.Sub(sessionStart).Milliseconds()
	}

	logEntry := &models.CodexLog{
		ID:           uuid.New().String(),
		AccountID:    accountID,
		AccountEmail: email,
		RequestPath:  key,
		Model:        model,
		Platform:     localPlatform(),
		InputTokens:  inputTokens,
		OutputTokens: outputTokens,
		Duration:     duration,
		StatusCode:   200,
		CreatedAt:    sessionStart,
	}

	if sessionID != "" {
		logEntry.ID = sessionID
	}

	if s.codexDB != nil {
		s.codexDB.SaveLog(logEntry)
	}

	s.mu.Lock()
	s.seen[key] = true
	s.mu.Unlock()

	log.Printf("[session-scanner] imported session: model=%s tokens=%d+%d dur=%dms (%s)",
		model, inputTokens, outputTokens, duration, filepath.Base(path))
	return nil
}

func localPlatform() string {
	switch runtime.GOOS {
	case "darwin":
		return "macOS"
	case "linux":
		return "Linux"
	case "windows":
		return "Windows"
	default:
		return runtime.GOOS
	}
}

func getActiveAccountInfo() (email, accountID string) {
	home, _ := os.UserHomeDir()
	authFile := filepath.Join(home, ".codex", "auth.json")
	data, err := os.ReadFile(authFile)
	if err != nil {
		return "unknown", ""
	}
	var auth struct {
		Tokens struct {
			Email     string `json:"email"`
			AccountID string `json:"account_id"`
		} `json:"tokens"`
	}
	if err := json.Unmarshal(data, &auth); err != nil {
		return "unknown", ""
	}
	email = auth.Tokens.Email
	if email == "" {
		email = "codex-cli"
	}
	accountID = auth.Tokens.AccountID
	return
}
