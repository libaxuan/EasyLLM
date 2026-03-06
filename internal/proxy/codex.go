package proxy

import (
	"bytes"
	"easyllm/internal/models"
	openaiplatform "easyllm/internal/platforms/openai"
	"easyllm/internal/storage"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
)

// poolEntry is a unified proxy pool entry from any source
type poolEntry struct {
	id               string
	email            string
	accessToken      string
	chatgptAccountID string // chatgpt-account-id header value (OAuth accounts)
	source           string // "codex" | "openai"
	requests         *int64
}

// CodexProxy manages the unified proxy pool
type CodexProxy struct {
	mu           sync.RWMutex
	pool         []poolEntry
	tokenIndex   map[string]*poolEntry // token → poolEntry for O(1) lookup
	strategy     string
	currentIndex int64
	codexDB      *storage.CodexStorage
	openaiDB     *storage.OpenAIStorage
	enabled      bool
	httpClient   *http.Client
}

var globalProxy *CodexProxy
var proxyMu sync.Mutex

func GetProxy() *CodexProxy {
	proxyMu.Lock()
	defer proxyMu.Unlock()
	return globalProxy
}

func InitProxy(codexDB *storage.CodexStorage, openaiDB *storage.OpenAIStorage, strategy string) *CodexProxy {
	proxyMu.Lock()
	defer proxyMu.Unlock()
	globalProxy = &CodexProxy{
		codexDB:    codexDB,
		openaiDB:   openaiDB,
		strategy:   strategy,
		enabled:    true,
		tokenIndex: make(map[string]*poolEntry),
		httpClient: &http.Client{
			Timeout: 180 * time.Second,
			Transport: &http.Transport{
				DisableCompression:  true,
				MaxIdleConns:        100,
				MaxIdleConnsPerHost: 20,
				IdleConnTimeout:     90 * time.Second,
			},
		},
	}
	globalProxy.Refresh()
	return globalProxy
}

// Refresh reloads the pool from both CodexAccount table and proxy-enabled OpenAI OAuth accounts.
// Existing in-memory request counters are preserved for entries that remain in the pool.
func (p *CodexProxy) Refresh() {
	// Snapshot existing counters before taking the write lock
	p.mu.RLock()
	oldCounters := make(map[string]*int64, len(p.pool))
	for i := range p.pool {
		oldCounters[p.pool[i].id] = p.pool[i].requests
	}
	p.mu.RUnlock()

	var entries []poolEntry

	// Dedicated Codex pool accounts
	if p.codexDB != nil {
		accounts, err := p.codexDB.LoadEnabledAccounts()
		if err == nil {
			for _, a := range accounts {
				cnt := a.RequestCount
				if old, ok := oldCounters[a.ID]; ok && old != nil {
					cnt = atomic.LoadInt64(old)
				}
				entries = append(entries, poolEntry{
					id:          a.ID,
					email:       a.Email,
					accessToken: a.AccessToken,
					source:      "codex",
					requests:    &cnt,
				})
			}
		}
	}

	// OpenAI OAuth accounts with proxy_enabled = true
	if p.openaiDB != nil {
		accounts, err := p.openaiDB.ListProxyEnabled()
		if err == nil {
			for _, a := range accounts {
				if a.AccessToken == nil || *a.AccessToken == "" {
					continue
				}
				cnt := int64(0)
				if old, ok := oldCounters[a.ID]; ok && old != nil {
					cnt = atomic.LoadInt64(old)
				}
				accountID := ""
				if a.ChatGPTAccountID != nil {
					accountID = *a.ChatGPTAccountID
				}
				entries = append(entries, poolEntry{
					id:               a.ID,
					email:            a.Email,
					accessToken:      *a.AccessToken,
					chatgptAccountID: accountID,
					source:           "openai",
					requests:         &cnt,
				})
			}
		}
	}

	// Build token→entry index for O(1) lookup in matchIncomingToken / IsKnownToken
	idx := make(map[string]*poolEntry, len(entries))
	for i := range entries {
		idx[entries[i].accessToken] = &entries[i]
	}

	p.mu.Lock()
	defer p.mu.Unlock()
	p.pool = entries
	p.tokenIndex = idx
}

func (p *CodexProxy) GetPoolStatus() *models.CodexPoolStatus {
	p.mu.RLock()
	defer p.mu.RUnlock()

	var totalRequests int64
	for _, e := range p.pool {
		if e.requests != nil {
			totalRequests += *e.requests
		}
	}

	accts := make([]models.CodexAccount, len(p.pool))
	for i, e := range p.pool {
		cnt := int64(0)
		if e.requests != nil {
			cnt = *e.requests
		}
		accts[i] = models.CodexAccount{
			ID:           e.id,
			Email:        e.email,
			AccessToken:  e.accessToken,
			Enabled:      true,
			RequestCount: cnt,
		}
	}

	return &models.CodexPoolStatus{
		TotalAccounts:   len(p.pool),
		EnabledAccounts: len(p.pool),
		TotalRequests:   totalRequests,
		Accounts:        accts,
	}
}

// matchIncomingToken checks if the incoming request's Bearer token matches
// any managed account (pool or all OpenAI OAuth accounts). Returns a poolEntry
// for logging purposes, or nil if no match.
func (p *CodexProxy) matchIncomingToken(r *http.Request) *poolEntry {
	auth := r.Header.Get("Authorization")
	if len(auth) <= 7 {
		return nil
	}
	token := auth[7:] // strip "Bearer "

	// O(1) lookup via token index (covers pool entries)
	p.mu.RLock()
	if entry, ok := p.tokenIndex[token]; ok {
		p.mu.RUnlock()
		return entry
	}
	p.mu.RUnlock()

	// Fallback: check all OAuth accounts (the account may not be in the pool)
	if p.openaiDB != nil {
		account, err := p.openaiDB.GetByAccessToken(token)
		if err == nil && account != nil {
			cnt := int64(0)
			accountID := ""
			if account.ChatGPTAccountID != nil {
				accountID = *account.ChatGPTAccountID
			}
			return &poolEntry{
				id:               account.ID,
				email:            account.Email,
				accessToken:      token,
				chatgptAccountID: accountID,
				source:           "openai",
				requests:         &cnt,
			}
		}
	}

	return nil
}

func (p *CodexProxy) pickEntry() *poolEntry {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if len(p.pool) == 0 {
		return nil
	}

	switch p.strategy {
	case "random":
		idx := rand.Intn(len(p.pool))
		return &p.pool[idx]
	case "least_used":
		least := &p.pool[0]
		leastVal := int64(0)
		if least.requests != nil {
			leastVal = atomic.LoadInt64(least.requests)
		}
		for i := 1; i < len(p.pool); i++ {
			if p.pool[i].requests != nil {
				v := atomic.LoadInt64(p.pool[i].requests)
				if v < leastVal {
					least = &p.pool[i]
					leastVal = v
				}
			}
		}
		return least
	default: // round_robin
		idx := int(atomic.AddInt64(&p.currentIndex, 1)-1) % len(p.pool)
		return &p.pool[idx]
	}
}

// ProxyRequest forwards a /v1/* request to the correct upstream.
// For Codex-compatible paths it routes to chatgpt.com/backend-api/codex/*
// and injects the chatgpt-account-id header required by the ChatGPT Codex API.
//
// Passthrough mode: when the incoming request carries an Authorization token
// that matches a known managed account, the proxy forwards the request as-is
// (no pool rotation) but still logs the request. This enables Codex CLI to
// route through the proxy for logging while keeping its own auth.
func (p *CodexProxy) ProxyRequest(w http.ResponseWriter, r *http.Request) {
	// Try passthrough first: match the incoming token to a managed account
	entry := p.matchIncomingToken(r)
	passthrough := entry != nil

	if !passthrough {
		if !p.enabled {
			writeError(w, http.StatusServiceUnavailable, "Proxy is disabled", "service_unavailable")
			return
		}
		entry = p.pickEntry()
		if entry == nil {
			writeError(w, http.StatusServiceUnavailable, "No available accounts in pool", "no_available_account")
			return
		}
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Failed to read request body", "invalid_request")
		return
	}

	if !passthrough {
		// Normalize body for chatgpt.com Codex backend requirements.
		body, _ = normalizeCodexBody(body)
	}

	upstreamURL := buildUpstreamURL(r.URL.Path, r.URL.RawQuery)

	upstreamReq, err := http.NewRequest(r.Method, upstreamURL, bytes.NewReader(body))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to create upstream request", "internal_error")
		return
	}

	// Copy headers; in passthrough mode keep original Authorization
	for key, values := range r.Header {
		lower := strings.ToLower(key)
		if lower == "chatgpt-account-id" {
			continue
		}
		if lower == "authorization" && !passthrough {
			continue
		}
		for _, v := range values {
			upstreamReq.Header.Add(key, v)
		}
	}
	if !passthrough {
		upstreamReq.Header.Set("Authorization", "Bearer "+entry.accessToken)
	}

	// chatgpt-account-id is required by the ChatGPT Codex backend API
	if entry.chatgptAccountID != "" {
		upstreamReq.Header.Set("chatgpt-account-id", entry.chatgptAccountID)
	}

	// Headers required by chatgpt.com/backend-api/codex to identify the client
	// as a legitimate Codex CLI agent and bypass Cloudflare bot detection.
	if upstreamReq.Header.Get("User-Agent") == "" {
		upstreamReq.Header.Set("User-Agent", "codex_cli_rs/0.98.0")
	}
	if upstreamReq.Header.Get("OpenAI-Beta") == "" {
		upstreamReq.Header.Set("OpenAI-Beta", "responses=experimental")
	}
	if upstreamReq.Header.Get("originator") == "" {
		upstreamReq.Header.Set("originator", "codex_cli_rs")
	}

	shouldLog := r.Method == http.MethodPost

	startTime := time.Now()
	resp, err := p.httpClient.Do(upstreamReq)
	if err != nil {
		writeError(w, http.StatusBadGateway, fmt.Sprintf("Upstream request failed: %v", err), "upstream_error")
		return
	}
	defer resp.Body.Close()

	// Capture rate-limit headers and persist to the OpenAI account
	p.saveRateLimits(entry, resp)

	// Persist stats for codex-source accounts
	if entry.source == "codex" && p.codexDB != nil {
		p.codexDB.IncrementRequestCount(entry.id)
	}
	if entry.requests != nil {
		atomic.AddInt64(entry.requests, 1)
	}

	// For /models responses in passthrough mode, disable WebSocket support
	// so Codex CLI falls back to HTTP (which respects chatgpt_base_url).
	isModelsReq := r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/models")
	if isModelsReq && passthrough {
		respBody, readErr := io.ReadAll(resp.Body)
		if readErr == nil {
			respBody = disableWebSocketInModels(respBody)
		}
		copyResponseHeaders(w, resp)
		w.Header().Set("Content-Length", fmt.Sprintf("%d", len(respBody)))
		w.WriteHeader(resp.StatusCode)
		w.Write(respBody) //nolint:errcheck
		return
	}

	copyResponseHeaders(w, resp)
	w.WriteHeader(resp.StatusCode)

	// Stream the response body; capture the last SSE "data:" line for token usage.
	flusher, canFlush := w.(http.Flusher)
	buf := make([]byte, 8192)
	var lastDataLine string
	var streamBuf strings.Builder
	for {
		n, readErr := resp.Body.Read(buf)
		if n > 0 {
			w.Write(buf[:n]) //nolint:errcheck
			if canFlush {
				flusher.Flush()
			}
			if shouldLog {
				streamBuf.Write(buf[:n])
				remaining := streamBuf.String()
				for {
					idx := strings.Index(remaining, "\n")
					if idx < 0 {
						break
					}
					line := strings.TrimSpace(remaining[:idx])
					remaining = remaining[idx+1:]
					if strings.HasPrefix(line, "data: ") {
						lastDataLine = line[6:]
					}
				}
				streamBuf.Reset()
				if remaining != "" {
					streamBuf.WriteString(remaining)
				}
			}
		}
		if readErr != nil {
			break
		}
	}

	duration := time.Since(startTime).Milliseconds()

	if shouldLog {
		p.saveLog(entry, body, r.URL.Path, lastDataLine, resp.StatusCode, duration, r.UserAgent())
	}
}

func copyResponseHeaders(w http.ResponseWriter, resp *http.Response) {
	for key, values := range resp.Header {
		lower := strings.ToLower(key)
		if lower == "transfer-encoding" || lower == "connection" || lower == "keep-alive" || lower == "content-length" {
			continue
		}
		for _, v := range values {
			w.Header().Add(key, v)
		}
	}
}

// disableWebSocketInModels rewrites the models JSON to prevent clients
// from using WebSocket connections, forcing them to use HTTP POST instead.
func disableWebSocketInModels(body []byte) []byte {
	var data map[string]interface{}
	if err := json.Unmarshal(body, &data); err != nil {
		return body
	}
	models, ok := data["models"].([]interface{})
	if !ok {
		return body
	}
	for _, m := range models {
		model, ok := m.(map[string]interface{})
		if !ok {
			continue
		}
		model["prefer_websockets"] = false
		model["supports_websockets"] = false
	}
	out, err := json.Marshal(data)
	if err != nil {
		return body
	}
	return out
}

func (p *CodexProxy) saveRateLimits(entry *poolEntry, resp *http.Response) {
	if entry.source != "openai" || p.openaiDB == nil {
		return
	}
	info := openaiplatform.ParseCodexHeaders(resp.Header)
	if info == nil {
		return
	}
	acc, err := p.openaiDB.Get(entry.id)
	if err != nil || acc == nil {
		return
	}
	acc.QuotaTotal = &info.Total
	used := info.Used
	acc.QuotaUsed = &used
	if info.ResetAt != "" {
		acc.QuotaResetAt = &info.ResetAt
	}
	acc.Quota5hUsedPercent = info.Codex5hUsedPercent
	acc.Quota5hResetSeconds = info.Codex5hResetSeconds
	acc.Quota5hWindowMinutes = info.Codex5hWindowMinutes
	acc.Quota7dUsedPercent = info.Codex7dUsedPercent
	acc.Quota7dResetSeconds = info.Codex7dResetSeconds
	acc.Quota7dWindowMinutes = info.Codex7dWindowMinutes
	now := time.Now()
	acc.QuotaUpdatedAt = &now
	_ = p.openaiDB.Save(acc)
}

func (p *CodexProxy) saveLog(entry *poolEntry, requestBody []byte, requestPath, lastSSEData string, statusCode int, durationMs int64, userAgent string) {
	if p.codexDB == nil {
		return
	}
	var reqData map[string]interface{}
	json.Unmarshal(requestBody, &reqData) //nolint:errcheck
	model := ""
	if m, ok := reqData["model"].(string); ok {
		model = m
	}

	var inputTokens, outputTokens int64
	if lastSSEData != "" {
		inputTokens, outputTokens = extractUsageFromSSE(lastSSEData)
	}

	p.codexDB.SaveLog(&models.CodexLog{
		ID:           uuid.New().String(),
		AccountID:    entry.id,
		AccountEmail: entry.email,
		RequestPath:  requestPath,
		Model:        model,
		Platform:     parsePlatform(userAgent),
		InputTokens:  inputTokens,
		OutputTokens: outputTokens,
		Duration:     durationMs,
		StatusCode:   statusCode,
		CreatedAt:    time.Now(),
	})
}

func parsePlatform(ua string) string {
	ua = strings.ToLower(ua)
	switch {
	case strings.Contains(ua, "iphone") || strings.Contains(ua, "ipad"):
		return "iOS"
	case strings.Contains(ua, "android"):
		return "Android"
	case strings.Contains(ua, "macintosh") || strings.Contains(ua, "mac os") || strings.Contains(ua, "darwin"):
		return "macOS"
	case strings.Contains(ua, "windows"):
		return "Windows"
	case strings.Contains(ua, "linux"):
		return "Linux"
	case strings.Contains(ua, "codex_cli"):
		return "Codex CLI"
	case ua == "":
		return ""
	default:
		return "Other"
	}
}

func extractUsageFromSSE(data string) (inputTokens, outputTokens int64) {
	var obj map[string]interface{}
	if err := json.Unmarshal([]byte(data), &obj); err != nil {
		return 0, 0
	}
	// Look for usage in response.completed events or top-level response object
	resp, _ := obj["response"].(map[string]interface{})
	if resp == nil {
		resp = obj
	}
	usage, _ := resp["usage"].(map[string]interface{})
	if usage == nil {
		return 0, 0
	}
	if v, ok := usage["input_tokens"].(float64); ok {
		inputTokens = int64(v)
	}
	if v, ok := usage["output_tokens"].(float64); ok {
		outputTokens = int64(v)
	}
	return
}

// buildUpstreamURL maps /v1/* to https://chatgpt.com/backend-api/codex/*
// which is the real Codex backend. This matches the upstream target used by
// the original augment-token-mng Tauri app.
// client_version is always appended because the chatgpt.com Codex API requires it.
func buildUpstreamURL(path, query string) string {
	const base = "https://chatgpt.com"
	const clientVersion = "0.98.0"
	mapped := mapCodexPath(path)
	if strings.Contains(query, "client_version=") {
		return base + mapped + "?" + query
	}
	var qs string
	if query != "" {
		qs = query + "&client_version=" + clientVersion
	} else {
		qs = "client_version=" + clientVersion
	}
	return base + mapped + "?" + qs
}

// mapCodexPath converts /v1/<tail> → /backend-api/codex/<tail>
// and passes through any path already starting with /backend-api/codex.
func mapCodexPath(path string) string {
	if path == "/v1" {
		return "/backend-api/codex"
	}
	if tail := strings.TrimPrefix(path, "/v1/"); tail != path {
		return "/backend-api/codex/" + tail
	}
	if strings.HasPrefix(path, "/backend-api/codex") {
		return path
	}
	return path
}

// knownUnsupportedParams are stripped before forwarding to avoid upstream errors.
var knownUnsupportedParams = map[string]bool{
	"max_output_tokens":      true,
	"prompt_cache_retention": true,
	"safety_identifier":      true,
}

// normalizeCodexBody normalises a Responses-API request body for the
// chatgpt.com/backend-api/codex backend which has a few quirks:
//   - input must be a JSON array of message objects, not a plain string
//   - instructions field must be present
//   - stream must be true (the backend only supports streaming)
//
// Returns the normalised body and whether streaming was force-enabled.
func normalizeCodexBody(body []byte) ([]byte, bool) {
	var data map[string]interface{}
	if err := json.Unmarshal(body, &data); err != nil {
		return body, false
	}

	modified := false
	streamForced := false

	// Convert string input to message-list format
	if inputRaw, ok := data["input"]; ok {
		if text, ok := inputRaw.(string); ok {
			data["input"] = []interface{}{
				map[string]interface{}{
					"role": "user",
					"content": []interface{}{
						map[string]interface{}{"type": "input_text", "text": text},
					},
				},
			}
			modified = true
		}
	}

	// Add default instructions if missing
	if _, ok := data["instructions"]; !ok {
		data["instructions"] = "You are a helpful assistant."
		modified = true
	}

	// chatgpt.com Codex backend only supports streaming responses
	if v, ok := data["stream"]; !ok || v == false || v == nil {
		data["stream"] = true
		streamForced = true
		modified = true
	}

	// chatgpt.com Codex backend requires store = false
	if v, ok := data["store"]; !ok || v == true {
		data["store"] = false
		modified = true
	}

	// Strip unsupported params
	for param := range knownUnsupportedParams {
		if _, ok := data[param]; ok {
			delete(data, param)
			modified = true
		}
	}

	if !modified {
		return body, false
	}
	cleaned, err := json.Marshal(data)
	if err != nil {
		return body, false
	}
	return cleaned, streamForced
}

// cleanRequestBody is kept for compatibility; actual normalisation happens in normalizeCodexBody.
func cleanRequestBody(body []byte) []byte {
	cleaned, _ := normalizeCodexBody(body)
	return cleaned
}

func writeError(w http.ResponseWriter, status int, message, code string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"error": map[string]interface{}{
			"message": message,
			"type":    code,
			"code":    fmt.Sprintf("%d", status),
		},
	})
}

// IsKnownToken checks if a Bearer token belongs to any managed account.
func (p *CodexProxy) IsKnownToken(token string) bool {
	if token == "" {
		return false
	}
	p.mu.RLock()
	_, found := p.tokenIndex[token]
	p.mu.RUnlock()
	if found {
		return true
	}

	if p.openaiDB != nil {
		account, err := p.openaiDB.GetByAccessToken(token)
		if err == nil && account != nil {
			return true
		}
	}
	return false
}

func (p *CodexProxy) IsEnabled() bool  { return p.enabled }
func (p *CodexProxy) SetEnabled(v bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.enabled = v
}

func (p *CodexProxy) GetStrategy() string { return p.strategy }
func (p *CodexProxy) SetStrategy(s string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.strategy = s
}

func (p *CodexProxy) PoolSize() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return len(p.pool)
}

func (p *CodexProxy) TotalRequests() int64 {
	p.mu.RLock()
	defer p.mu.RUnlock()
	var total int64
	for _, e := range p.pool {
		if e.requests != nil {
			total += atomic.LoadInt64(e.requests)
		}
	}
	return total
}
