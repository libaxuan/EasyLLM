package augment

import (
	"crypto/rand"
	"crypto/sha256"
	"easyllm/config"
	"easyllm/internal/models"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	clientID        = "v"
	authBaseURL     = "https://auth.augmentcode.com"
	authContinuePath = "/auth/continue"
	apiRedirectURI  = "vscode://augment.vscode-augment/auth/result"
)

// CreateOAuthState generates a new OAuth state with PKCE
func CreateOAuthState() *models.AugmentOAuthState {
	codeVerifierBytes := generateRandomBytes(32)
	codeVerifier := base64URLEncode(codeVerifierBytes)

	codeChallenge := base64URLEncode(sha256Hash([]byte(codeVerifier)))

	stateBytes := generateRandomBytes(8)
	state := base64URLEncode(stateBytes)

	return &models.AugmentOAuthState{
		CodeVerifier:  codeVerifier,
		CodeChallenge: codeChallenge,
		State:         state,
		CreationTime:  time.Now().UnixMilli(),
	}
}

// GenerateAuthorizeURL builds the OAuth authorization URL
func GenerateAuthorizeURL(oauthState *models.AugmentOAuthState) (string, error) {
	u, err := url.Parse(authBaseURL + "/authorize")
	if err != nil {
		return "", err
	}

	q := u.Query()
	q.Set("response_type", "code")
	q.Set("code_challenge", oauthState.CodeChallenge)
	q.Set("client_id", clientID)
	q.Set("state", oauthState.State)
	q.Set("prompt", "login")
	u.RawQuery = q.Encode()

	return u.String(), nil
}

// CompleteOAuthFlow exchanges the auth code for an access token
func CompleteOAuthFlow(oauthState *models.AugmentOAuthState, codeInput string) (*models.AugmentTokenResponse, error) {
	var parsedCode models.ParsedCode
	if err := json.Unmarshal([]byte(codeInput), &parsedCode); err != nil {
		return nil, fmt.Errorf("failed to parse code: %w", err)
	}

	token, err := getAccessToken(parsedCode.TenantURL, oauthState.CodeVerifier, parsedCode.Code)
	if err != nil {
		return nil, err
	}

	// Get user email
	var email *string
	modelsResp, err := GetModels(token, parsedCode.TenantURL)
	if err == nil {
		e := modelsResp.User.Email
		email = &e
	}

	return &models.AugmentTokenResponse{
		AccessToken: token,
		TenantURL:   parsedCode.TenantURL,
		Email:       email,
	}, nil
}

// ExtractTokenFromSession extracts access token using session cookie
func ExtractTokenFromSession(session string) (*models.AugmentTokenResponse, error) {
	codeVerifierBytes := generateRandomBytes(32)
	codeVerifier := base64URLEncode(codeVerifierBytes)
	codeChallenge := base64URLEncode(sha256Hash([]byte(codeVerifier)))

	stateBytes := generateRandomBytes(42)
	state := base64URLEncode(stateBytes)

	html, _, err := getAuthContinueWithCookie(session, codeChallenge, state)
	if err != nil {
		return nil, err
	}

	code, _, tenantURL, email, err := parseAuthDataFromHTML(html)
	if err != nil {
		return nil, fmt.Errorf("SESSION_ERROR_OR_ACCOUNT_BANNED")
	}

	tokenURL := tenantURL + "token"
	token, err := exchangeCodeForToken(tokenURL, codeVerifier, code, apiRedirectURI)
	if err != nil {
		return nil, err
	}

	return &models.AugmentTokenResponse{
		AccessToken: token,
		TenantURL:   tenantURL,
		Email:       email,
	}, nil
}

// RefreshAuthSession refreshes an existing auth session
func RefreshAuthSession(existingSession string) (string, error) {
	codeVerifierBytes := generateRandomBytes(32)
	codeVerifier := base64URLEncode(codeVerifierBytes)
	codeChallenge := base64URLEncode(sha256Hash([]byte(codeVerifier)))
	stateBytes := generateRandomBytes(42)
	state := base64URLEncode(stateBytes)

	html, newSession, err := getAuthContinueWithCookie(existingSession, codeChallenge, state)
	if err != nil {
		return "", err
	}

	if _, _, _, _, err := parseAuthDataFromHTML(html); err != nil {
		return "", fmt.Errorf("SESSION_ERROR_OR_ACCOUNT_BANNED")
	}

	if newSession != "" {
		return newSession, nil
	}
	return existingSession, nil
}

func getAuthContinueWithCookie(session, codeChallenge, state string) (string, string, error) {
	u, err := url.Parse(authBaseURL + authContinuePath)
	if err != nil {
		return "", "", err
	}

	q := u.Query()
	q.Set("response_type", "code")
	q.Set("code_challenge", codeChallenge)
	q.Set("client_id", clientID)
	q.Set("state", state)
	q.Set("prompt", "login")
	q.Set("redirect_uri", apiRedirectURI)
	u.RawQuery = q.Encode()

	client := createHTTPClient()
	req, err := http.NewRequest("GET", u.String(), nil)
	if err != nil {
		return "", "", err
	}

	req.Header.Set("Cookie", "session="+session)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")

	resp, err := client.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("failed to fetch auth continue: %w", err)
	}
	defer resp.Body.Close()

	// Extract new session from cookies
	newSession := ""
	for _, cookie := range resp.Cookies() {
		if cookie.Name == "session" {
			newSession = cookie.Value
			break
		}
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", "", err
	}

	return string(body), newSession, nil
}

func parseAuthDataFromHTML(html string) (code, state, tenantURL string, email *string, err error) {
	marker := "window.__INITIAL_STATE__"
	idx := strings.Index(html, marker)
	if idx == -1 {
		return "", "", "", nil, fmt.Errorf("missing initial state")
	}

	// Find the JSON object
	start := strings.Index(html[idx:], "{")
	if start == -1 {
		return "", "", "", nil, fmt.Errorf("missing initial state object")
	}
	start += idx

	depth := 0
	end := start
	for i, ch := range html[start:] {
		if ch == '{' {
			depth++
		} else if ch == '}' {
			depth--
			if depth == 0 {
				end = start + i + 1
				break
			}
		}
	}

	var initialState map[string]interface{}
	if err := json.Unmarshal([]byte(html[start:end]), &initialState); err != nil {
		return "", "", "", nil, fmt.Errorf("invalid initial state JSON: %w", err)
	}

	clientCode, ok := initialState["client_code"].(map[string]interface{})
	if !ok {
		return "", "", "", nil, fmt.Errorf("missing client_code")
	}

	code, _ = clientCode["code"].(string)
	state, _ = clientCode["state"].(string)
	tenantURL, _ = clientCode["tenant_url"].(string)

	if code == "" || state == "" || tenantURL == "" {
		return "", "", "", nil, fmt.Errorf("missing required OAuth fields")
	}

	if emailStr, ok := initialState["email"].(string); ok {
		email = &emailStr
	}

	return code, state, tenantURL, email, nil
}

func getAccessToken(tenantURL, codeVerifier, code string) (string, error) {
	tokenURL := tenantURL
	if !strings.HasSuffix(tokenURL, "/") {
		tokenURL += "/"
	}
	tokenURL += "token"

	return exchangeCodeForToken(tokenURL, codeVerifier, code, "")
}

func exchangeCodeForToken(tokenURL, codeVerifier, code, redirectURI string) (string, error) {
	payload := map[string]string{
		"grant_type":    "authorization_code",
		"client_id":     clientID,
		"code_verifier": codeVerifier,
		"redirect_uri":  redirectURI,
		"code":          code,
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	client := createHTTPClient()
	req, err := http.NewRequest("POST", tokenURL, strings.NewReader(string(data)))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to exchange token: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var tokenResp struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return "", fmt.Errorf("failed to parse token response: %w", err)
	}

	if tokenResp.AccessToken == "" {
		return "", fmt.Errorf("empty access token in response")
	}

	return tokenResp.AccessToken, nil
}

// createHTTPClient returns an HTTP client, optionally with proxy
func createHTTPClient() *http.Client {
	cfg := config.Get()
	transport := &http.Transport{}

	if cfg.Proxy.Enabled && cfg.Proxy.Host != "" {
		proxyURL := fmt.Sprintf("http://%s:%d", cfg.Proxy.Host, cfg.Proxy.Port)
		if cfg.Proxy.Username != "" {
			proxyURL = fmt.Sprintf("http://%s:%s@%s:%d",
				cfg.Proxy.Username, cfg.Proxy.Password,
				cfg.Proxy.Host, cfg.Proxy.Port)
		}
		if u, err := url.Parse(proxyURL); err == nil {
			transport.Proxy = http.ProxyURL(u)
		}
	}

	return &http.Client{
		Transport: transport,
		Timeout:   30 * time.Second,
	}
}

// Helper functions
func generateRandomBytes(n int) []byte {
	b := make([]byte, n)
	rand.Read(b)
	return b
}

func base64URLEncode(data []byte) string {
	return base64.RawURLEncoding.EncodeToString(data)
}

func sha256Hash(data []byte) []byte {
	h := sha256.Sum256(data)
	return h[:]
}

// MaskSession masks a session string for logging
func MaskSession(session string) string {
	if len(session) <= 5 {
		return "***"
	}
	return session[:4] + "***" + session[len(session)-1:]
}
