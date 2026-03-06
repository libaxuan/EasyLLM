package openai

import (
	"bytes"
	"easyllm/config"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const codexResponsesURL = "https://chatgpt.com/backend-api/codex/responses"
const defaultInstructions = "You are Codex, based on GPT-5. You are running as a coding agent in the Codex CLI on a user's computer."

// QuotaInfo holds percentage-based quota data extracted from x-codex-* headers.
type QuotaInfo struct {
	// Legacy fields (kept for backward compat; populated from percentage data)
	Total     int64  // synthetic: 100
	Remaining int64  // synthetic: 100 - 7d used percent
	Used      int64  // synthetic: 7d used percent
	ResetAt   string // from 7d reset seconds, formatted

	// New percentage-based fields (from x-codex-* headers)
	Codex5hUsedPercent     *float64 // 5h window used %
	Codex5hResetSeconds    *int64   // 5h reset countdown (seconds)
	Codex5hWindowMinutes   *int64   // 5h window duration (minutes)
	Codex7dUsedPercent     *float64 // 7d window used %
	Codex7dResetSeconds    *int64   // 7d reset countdown (seconds)
	Codex7dWindowMinutes   *int64   // 7d window duration (minutes)

	IsForbidden bool // 402/403 response
}

// FetchQuota calls the ChatGPT Codex Responses API (streaming) and reads
// the x-codex-primary-*/x-codex-secondary-* response headers to get
// percentage-based quota information. We only need the response headers,
// so the body is closed immediately (no need to read the full stream).
func FetchQuota(accessToken, chatgptAccountID string) (*QuotaInfo, error) {
	cfg := config.Get()
	transport := &http.Transport{}
	if cfg.Proxy.Enabled && cfg.Proxy.Host != "" {
		proxyURLStr := fmt.Sprintf("http://%s:%d", cfg.Proxy.Host, cfg.Proxy.Port)
		if cfg.Proxy.Username != "" {
			proxyURLStr = fmt.Sprintf("http://%s:%s@%s:%d",
				url.QueryEscape(cfg.Proxy.Username),
				url.QueryEscape(cfg.Proxy.Password),
				cfg.Proxy.Host, cfg.Proxy.Port)
		}
		if u, err := url.Parse(proxyURLStr); err == nil {
			transport.Proxy = http.ProxyURL(u)
		}
	}
	client := &http.Client{
		Transport: transport,
		Timeout:   15 * time.Second,
	}

	body, _ := json.Marshal(map[string]interface{}{
		"model": "gpt-5.1-codex",
		"input": []map[string]interface{}{
			{
				"role": "user",
				"content": []map[string]string{
					{"type": "input_text", "text": "hi"},
				},
			},
		},
		"instructions": defaultInstructions,
		"store":        false,
		"stream":       true,
	})

	req, err := http.NewRequest("POST", codexResponsesURL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Host", "chatgpt.com")
	if chatgptAccountID != "" {
		req.Header.Set("chatgpt-account-id", chatgptAccountID)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("quota request failed: %w", err)
	}
	// Close body immediately — quota headers are in the response headers,
	// no need to consume the streaming body (which would take 10s+).
	resp.Body.Close()

	if resp.StatusCode == 401 {
		return nil, fmt.Errorf("HTTP 401: Token expired or invalid")
	}
	if resp.StatusCode == 402 || resp.StatusCode == 403 {
		return &QuotaInfo{IsForbidden: true}, nil
	}

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		info := ParseCodexHeaders(resp.Header)
		return info, nil
	}

	return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
}

// ParseCodexHeaders extracts quota info from the x-codex-* response headers,
// mapping primary/secondary windows to 5h/7d based on window-minutes.
func ParseCodexHeaders(h http.Header) *QuotaInfo {
	primaryUsedPct := parseFloatHeader(h, "x-codex-primary-used-percent")
	primaryResetSec := parseInt64Header(h, "x-codex-primary-reset-after-seconds")
	primaryWindowMin := parseInt64Header(h, "x-codex-primary-window-minutes")

	secondaryUsedPct := parseFloatHeader(h, "x-codex-secondary-used-percent")
	secondaryResetSec := parseInt64Header(h, "x-codex-secondary-reset-after-seconds")
	secondaryWindowMin := parseInt64Header(h, "x-codex-secondary-window-minutes")

	if primaryUsedPct == nil && secondaryUsedPct == nil &&
		primaryWindowMin == nil && secondaryWindowMin == nil {
		// Also try legacy x-ratelimit-* headers as fallback
		return parseLegacyHeaders(h)
	}

	info := &QuotaInfo{}

	hasPrimary := primaryWindowMin != nil
	hasSecondary := secondaryWindowMin != nil

	if hasPrimary && hasSecondary {
		if *primaryWindowMin <= *secondaryWindowMin {
			// primary = 5h, secondary = 7d
			info.Codex5hUsedPercent = primaryUsedPct
			info.Codex5hResetSeconds = primaryResetSec
			info.Codex5hWindowMinutes = primaryWindowMin
			info.Codex7dUsedPercent = secondaryUsedPct
			info.Codex7dResetSeconds = secondaryResetSec
			info.Codex7dWindowMinutes = secondaryWindowMin
		} else {
			// primary = 7d, secondary = 5h
			info.Codex5hUsedPercent = secondaryUsedPct
			info.Codex5hResetSeconds = secondaryResetSec
			info.Codex5hWindowMinutes = secondaryWindowMin
			info.Codex7dUsedPercent = primaryUsedPct
			info.Codex7dResetSeconds = primaryResetSec
			info.Codex7dWindowMinutes = primaryWindowMin
		}
	} else if hasPrimary {
		if *primaryWindowMin <= 360 {
			info.Codex5hUsedPercent = primaryUsedPct
			info.Codex5hResetSeconds = primaryResetSec
			info.Codex5hWindowMinutes = primaryWindowMin
		} else {
			info.Codex7dUsedPercent = primaryUsedPct
			info.Codex7dResetSeconds = primaryResetSec
			info.Codex7dWindowMinutes = primaryWindowMin
		}
	} else if hasSecondary {
		if *secondaryWindowMin <= 360 {
			info.Codex5hUsedPercent = secondaryUsedPct
			info.Codex5hResetSeconds = secondaryResetSec
			info.Codex5hWindowMinutes = secondaryWindowMin
		} else {
			info.Codex7dUsedPercent = secondaryUsedPct
			info.Codex7dResetSeconds = secondaryResetSec
			info.Codex7dWindowMinutes = secondaryWindowMin
		}
	} else {
		// No window-minutes but have used-percent
		if primaryUsedPct != nil {
			info.Codex7dUsedPercent = primaryUsedPct
			info.Codex7dResetSeconds = primaryResetSec
		}
		if secondaryUsedPct != nil {
			info.Codex5hUsedPercent = secondaryUsedPct
			info.Codex5hResetSeconds = secondaryResetSec
		}
	}

	// Populate legacy fields from 7d data for backward compat
	if info.Codex7dUsedPercent != nil {
		info.Total = 100
		info.Used = int64(math.Round(*info.Codex7dUsedPercent))
		info.Remaining = 100 - info.Used
		if info.Codex7dResetSeconds != nil {
			info.ResetAt = formatResetSeconds(*info.Codex7dResetSeconds)
		}
	}

	return info
}

// parseLegacyHeaders tries the old x-ratelimit-* headers as fallback.
func parseLegacyHeaders(h http.Header) *QuotaInfo {
	info := &QuotaInfo{}

	if v := findHeader(h, "x-ratelimit-limit-requests"); v != "" {
		info.Total, _ = strconv.ParseInt(v, 10, 64)
	}
	if v := findHeader(h, "x-ratelimit-remaining-requests"); v != "" {
		info.Remaining, _ = strconv.ParseInt(v, 10, 64)
	}
	if v := findHeader(h, "x-ratelimit-reset-requests"); v != "" {
		info.ResetAt = v
	}

	if info.Total > 0 {
		info.Used = info.Total - info.Remaining
		pct := float64(info.Used) / float64(info.Total) * 100
		info.Codex7dUsedPercent = &pct
	}

	if info.Total == 0 && info.Codex7dUsedPercent == nil {
		return nil
	}
	return info
}

func formatResetSeconds(seconds int64) string {
	days := seconds / 86400
	hours := (seconds % 86400) / 3600
	minutes := (seconds % 3600) / 60
	var parts []string
	if days > 0 {
		parts = append(parts, fmt.Sprintf("%dd", days))
	}
	if hours > 0 {
		parts = append(parts, fmt.Sprintf("%dh", hours))
	}
	if minutes > 0 || len(parts) == 0 {
		parts = append(parts, fmt.Sprintf("%dm", minutes))
	}
	return strings.Join(parts, "")
}

func parseFloatHeader(h http.Header, key string) *float64 {
	v := findHeader(h, key)
	if v == "" {
		return nil
	}
	f, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return nil
	}
	return &f
}

func parseInt64Header(h http.Header, key string) *int64 {
	v := findHeader(h, key)
	if v == "" {
		return nil
	}
	i, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		return nil
	}
	return &i
}

func findHeader(h http.Header, key string) string {
	if v := h.Get(key); v != "" {
		return v
	}
	lower := strings.ToLower(key)
	for k, vals := range h {
		if strings.ToLower(k) == lower && len(vals) > 0 {
			return vals[0]
		}
	}
	return ""
}
