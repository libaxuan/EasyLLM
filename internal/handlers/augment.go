package handlers

import (
	"easyllm/internal/models"
	"easyllm/internal/platforms/augment"
	"easyllm/internal/storage"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type AugmentHandler struct {
	storage    *storage.AugmentStorage
	oauthState *models.AugmentOAuthState
	mu         sync.Mutex
}

func NewAugmentHandler(s *storage.AugmentStorage) *AugmentHandler {
	return &AugmentHandler{storage: s}
}

// RegisterRoutes registers all Augment routes
func (h *AugmentHandler) RegisterRoutes(rg *gin.RouterGroup) {
	g := rg.Group("/augment")
	g.GET("/tokens", h.ListTokens)
	g.POST("/tokens", h.AddToken)
	g.PUT("/tokens/:id", h.UpdateToken)
	g.DELETE("/tokens/:id", h.DeleteToken)
	g.DELETE("/tokens", h.DeleteTokens)
	g.POST("/tokens/:id/check", h.CheckTokenStatus)
	g.POST("/tokens/check-all", h.CheckAllTokensStatus)
	g.POST("/tokens/:id/credit", h.GetCreditInfo)
	g.POST("/tokens/:id/refresh-session", h.RefreshSession)
	g.POST("/tokens/batch-refresh-sessions", h.BatchRefreshSessions)
	g.POST("/oauth/start", h.StartOAuth)
	g.POST("/oauth/complete", h.CompleteOAuth)
	g.POST("/import/session", h.ImportSession)
	g.POST("/import/sessions", h.ImportSessions)
	g.GET("/export", h.ExportJSON)
	g.POST("/import/json", h.ImportJSON)
	g.POST("/sync", h.SyncTokens)
}

// ListTokens returns all tokens
func (h *AugmentHandler) ListTokens(c *gin.Context) {
	tokens, err := h.storage.LoadTokens()
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.APIError{Error: err.Error(), Code: "STORAGE_ERROR"})
		return
	}
	c.JSON(http.StatusOK, tokens)
}

// AddToken adds a new token manually
func (h *AugmentHandler) AddToken(c *gin.Context) {
	var token models.AugmentToken
	if err := c.ShouldBindJSON(&token); err != nil {
		c.JSON(http.StatusBadRequest, models.APIError{Error: err.Error(), Code: "INVALID_REQUEST"})
		return
	}

	if token.ID == "" {
		token.ID = uuid.New().String()
	}
	now := time.Now()
	token.CreatedAt = now
	token.UpdatedAt = now

	if err := h.storage.SaveToken(&token); err != nil {
		c.JSON(http.StatusInternalServerError, models.APIError{Error: err.Error(), Code: "STORAGE_ERROR"})
		return
	}
	c.JSON(http.StatusOK, token)
}

// UpdateToken updates an existing token
func (h *AugmentHandler) UpdateToken(c *gin.Context) {
	id := c.Param("id")

	existing, err := h.storage.GetToken(id)
	if err != nil {
		c.JSON(http.StatusNotFound, models.APIError{Error: "Token not found", Code: "NOT_FOUND"})
		return
	}

	var updates models.AugmentToken
	if err := c.ShouldBindJSON(&updates); err != nil {
		c.JSON(http.StatusBadRequest, models.APIError{Error: err.Error(), Code: "INVALID_REQUEST"})
		return
	}

	// Preserve ID and created_at
	updates.ID = existing.ID
	updates.CreatedAt = existing.CreatedAt
	updates.UpdatedAt = time.Now()

	if err := h.storage.SaveToken(&updates); err != nil {
		c.JSON(http.StatusInternalServerError, models.APIError{Error: err.Error(), Code: "STORAGE_ERROR"})
		return
	}
	c.JSON(http.StatusOK, updates)
}

// DeleteToken removes a token
func (h *AugmentHandler) DeleteToken(c *gin.Context) {
	id := c.Param("id")
	if err := h.storage.DeleteToken(id); err != nil {
		c.JSON(http.StatusInternalServerError, models.APIError{Error: err.Error(), Code: "STORAGE_ERROR"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

// DeleteTokens removes multiple tokens
func (h *AugmentHandler) DeleteTokens(c *gin.Context) {
	var req struct {
		IDs []string `json:"ids"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.APIError{Error: err.Error(), Code: "INVALID_REQUEST"})
		return
	}

	if err := h.storage.DeleteTokens(req.IDs); err != nil {
		c.JSON(http.StatusInternalServerError, models.APIError{Error: err.Error(), Code: "STORAGE_ERROR"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "deleted": len(req.IDs)})
}

// CheckTokenStatus checks the ban/active status of a token
func (h *AugmentHandler) CheckTokenStatus(c *gin.Context) {
	id := c.Param("id")
	token, err := h.storage.GetToken(id)
	if err != nil {
		c.JSON(http.StatusNotFound, models.APIError{Error: "Token not found", Code: "NOT_FOUND"})
		return
	}

	status, err := augment.CheckBanStatus(token.AccessToken, token.TenantURL)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.APIError{Error: err.Error(), Code: "CHECK_ERROR"})
		return
	}

	// Update status in storage
	banStatusJSON := map[string]interface{}{"status": status.Status}
	token.BanStatus = banStatusJSON
	h.storage.SaveToken(token)

	c.JSON(http.StatusOK, status)
}

// CheckAllTokensStatus checks status of all tokens
func (h *AugmentHandler) CheckAllTokensStatus(c *gin.Context) {
	tokens, err := h.storage.LoadTokens()
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.APIError{Error: err.Error(), Code: "STORAGE_ERROR"})
		return
	}

	type tokenInput struct{ ID, Token, TenantURL string }
	inputs := make([]tokenInput, len(tokens))
	for i, t := range tokens {
		inputs[i] = tokenInput{ID: t.ID, Token: t.AccessToken, TenantURL: t.TenantURL}
	}

	// Check concurrently
	semaphore := make(chan struct{}, 5)
	results := make([]models.TokenStatusResult, len(tokens))
	var wg sync.WaitGroup

	for i, t := range inputs {
		wg.Add(1)
		go func(idx int, id, tok, tenantURL string) {
			defer wg.Done()
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			status, err := augment.CheckBanStatus(tok, tenantURL)
			if err != nil {
				msg := err.Error()
				results[idx] = models.TokenStatusResult{
					TokenID: id,
					Status:  models.AccountStatus{Status: "ERROR", ErrorMessage: &msg},
				}
			} else {
				results[idx] = models.TokenStatusResult{TokenID: id, Status: *status}
				// Update storage
				for j := range tokens {
					if tokens[j].ID == id {
						tokens[j].BanStatus = map[string]interface{}{"status": status.Status}
						h.storage.SaveToken(&tokens[j])
						break
					}
				}
			}
		}(i, t.ID, t.Token, t.TenantURL)
	}

	wg.Wait()
	c.JSON(http.StatusOK, results)
}

// GetCreditInfo fetches credit/usage info for a token
func (h *AugmentHandler) GetCreditInfo(c *gin.Context) {
	id := c.Param("id")
	token, err := h.storage.GetToken(id)
	if err != nil {
		c.JSON(http.StatusNotFound, models.APIError{Error: "Token not found", Code: "NOT_FOUND"})
		return
	}

	info, err := augment.GetCreditInfo(token.AccessToken, token.TenantURL)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.APIError{Error: err.Error(), Code: "CREDIT_ERROR"})
		return
	}
	c.JSON(http.StatusOK, info)
}

// RefreshSession refreshes the auth session for a token
func (h *AugmentHandler) RefreshSession(c *gin.Context) {
	id := c.Param("id")
	token, err := h.storage.GetToken(id)
	if err != nil {
		c.JSON(http.StatusNotFound, models.APIError{Error: "Token not found", Code: "NOT_FOUND"})
		return
	}

	if token.AuthSession == nil || *token.AuthSession == "" {
		c.JSON(http.StatusBadRequest, models.APIError{Error: "No auth session available", Code: "NO_SESSION"})
		return
	}

	newSession, err := augment.RefreshAuthSession(*token.AuthSession)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.APIError{Error: err.Error(), Code: "REFRESH_ERROR"})
		return
	}

	token.AuthSession = &newSession
	now := time.Now()
	token.SessionUpdatedAt = &now
	h.storage.SaveToken(token)

	c.JSON(http.StatusOK, gin.H{"success": true, "session_updated": true})
}

// BatchRefreshSessions refreshes sessions for multiple tokens
func (h *AugmentHandler) BatchRefreshSessions(c *gin.Context) {
	var req struct {
		IDs []string `json:"ids"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.APIError{Error: err.Error(), Code: "INVALID_REQUEST"})
		return
	}

	type refreshResult struct {
		ID      string `json:"id"`
		Success bool   `json:"success"`
		Error   string `json:"error,omitempty"`
	}

	results := make([]refreshResult, 0, len(req.IDs))
	semaphore := make(chan struct{}, 5)
	var mu sync.Mutex
	var wg sync.WaitGroup

	for _, id := range req.IDs {
		wg.Add(1)
		go func(tokenID string) {
			defer wg.Done()
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			token, err := h.storage.GetToken(tokenID)
			if err != nil || token.AuthSession == nil {
				mu.Lock()
				results = append(results, refreshResult{ID: tokenID, Success: false, Error: "Token not found or no session"})
				mu.Unlock()
				return
			}

			newSession, err := augment.RefreshAuthSession(*token.AuthSession)
			if err != nil {
				mu.Lock()
				results = append(results, refreshResult{ID: tokenID, Success: false, Error: err.Error()})
				mu.Unlock()
				return
			}

			token.AuthSession = &newSession
			now := time.Now()
			token.SessionUpdatedAt = &now
			h.storage.SaveToken(token)

			mu.Lock()
			results = append(results, refreshResult{ID: tokenID, Success: true})
			mu.Unlock()
		}(id)
	}

	wg.Wait()
	c.JSON(http.StatusOK, gin.H{"results": results})
}

// StartOAuth initiates OAuth flow
func (h *AugmentHandler) StartOAuth(c *gin.Context) {
	state := augment.CreateOAuthState()

	h.mu.Lock()
	h.oauthState = state
	h.mu.Unlock()

	authURL, err := augment.GenerateAuthorizeURL(state)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.APIError{Error: err.Error(), Code: "OAUTH_ERROR"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"auth_url":       authURL,
		"state":          state.State,
		"code_challenge": state.CodeChallenge,
	})
}

// CompleteOAuth completes the OAuth flow with the code
func (h *AugmentHandler) CompleteOAuth(c *gin.Context) {
	var req struct {
		Code string `json:"code"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.APIError{Error: err.Error(), Code: "INVALID_REQUEST"})
		return
	}

	h.mu.Lock()
	state := h.oauthState
	h.mu.Unlock()

	if state == nil {
		c.JSON(http.StatusBadRequest, models.APIError{Error: "No active OAuth flow", Code: "NO_OAUTH_STATE"})
		return
	}

	tokenResp, err := augment.CompleteOAuthFlow(state, req.Code)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.APIError{Error: err.Error(), Code: "OAUTH_ERROR"})
		return
	}

	// Save token
	token := &models.AugmentToken{
		ID:          uuid.New().String(),
		TenantURL:   tokenResp.TenantURL,
		AccessToken: tokenResp.AccessToken,
		EmailNote:   tokenResp.Email,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	if err := h.storage.SaveToken(token); err != nil {
		c.JSON(http.StatusInternalServerError, models.APIError{Error: err.Error(), Code: "STORAGE_ERROR"})
		return
	}

	h.mu.Lock()
	h.oauthState = nil
	h.mu.Unlock()

	c.JSON(http.StatusOK, token)
}

// ImportSession imports a token from an auth session
func (h *AugmentHandler) ImportSession(c *gin.Context) {
	var req models.ImportSessionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.APIError{Error: err.Error(), Code: "INVALID_REQUEST"})
		return
	}

	if req.Session == "" || len(req.Session) < 10 {
		c.JSON(http.StatusBadRequest, models.APIError{Error: "Invalid session", Code: "INVALID_SESSION"})
		return
	}

	// Check duplicate email
	tokenResp, err := augment.ExtractTokenFromSession(req.Session)
	if err != nil {
		c.JSON(http.StatusUnprocessableEntity, models.APIError{Error: err.Error(), Code: "IMPORT_ERROR"})
		return
	}

	if tokenResp.Email != nil && h.storage.EmailExists(*tokenResp.Email) {
		c.JSON(http.StatusConflict, models.APIError{
			Error: "Token with this email already exists",
			Code:  "DUPLICATE_EMAIL",
		})
		return
	}

	now := time.Now()
	token := &models.AugmentToken{
		ID:               uuid.New().String(),
		TenantURL:        tokenResp.TenantURL,
		AccessToken:      tokenResp.AccessToken,
		EmailNote:        tokenResp.Email,
		AuthSession:      &req.Session,
		CreatedAt:        now,
		UpdatedAt:        now,
		SessionUpdatedAt: &now,
		SkipCheck:        boolPtr(false),
	}

	// Set default ban status
	token.BanStatus = map[string]interface{}{"status": "ACTIVE"}

	if err := h.storage.SaveToken(token); err != nil {
		c.JSON(http.StatusInternalServerError, models.APIError{Error: err.Error(), Code: "STORAGE_ERROR"})
		return
	}

	sessionPreview := augment.MaskSession(req.Session)
	result := models.ImportResult{
		Success:        true,
		TokenData:      token,
		SessionPreview: &sessionPreview,
	}
	c.JSON(http.StatusOK, result)
}

// ImportSessions imports multiple tokens from auth sessions
func (h *AugmentHandler) ImportSessions(c *gin.Context) {
	var req models.BatchImportRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.APIError{Error: err.Error(), Code: "INVALID_REQUEST"})
		return
	}

	if len(req.Sessions) == 0 {
		c.JSON(http.StatusBadRequest, models.APIError{Error: "Sessions array is empty", Code: "EMPTY_ARRAY"})
		return
	}

	if len(req.Sessions) > 100 {
		c.JSON(http.StatusBadRequest, models.APIError{Error: "Too many sessions (max 100)", Code: "TOO_MANY"})
		return
	}

	semaphore := make(chan struct{}, 5)
	results := make([]models.ImportResult, len(req.Sessions))
	var wg sync.WaitGroup

	for i, session := range req.Sessions {
		wg.Add(1)
		go func(idx int, sess string) {
			defer wg.Done()
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			preview := augment.MaskSession(sess)

			if len(sess) < 10 {
				msg := "Session too short"
				results[idx] = models.ImportResult{Success: false, Error: &msg, SessionPreview: &preview}
				return
			}

			tokenResp, err := augment.ExtractTokenFromSession(sess)
			if err != nil {
				msg := err.Error()
				results[idx] = models.ImportResult{Success: false, Error: &msg, SessionPreview: &preview}
				return
			}

			if tokenResp.Email != nil && h.storage.EmailExists(*tokenResp.Email) {
				msg := "Duplicate email"
				results[idx] = models.ImportResult{Success: false, Error: &msg, SessionPreview: &preview}
				return
			}

			now := time.Now()
			token := &models.AugmentToken{
				ID:               uuid.New().String(),
				TenantURL:        tokenResp.TenantURL,
				AccessToken:      tokenResp.AccessToken,
				EmailNote:        tokenResp.Email,
				AuthSession:      &sess,
				CreatedAt:        now,
				UpdatedAt:        now,
				SessionUpdatedAt: &now,
				SkipCheck:        boolPtr(false),
				BanStatus:        map[string]interface{}{"status": "ACTIVE"},
			}

			if err := h.storage.SaveToken(token); err != nil {
				msg := err.Error()
				results[idx] = models.ImportResult{Success: false, Error: &msg, SessionPreview: &preview}
				return
			}

			results[idx] = models.ImportResult{Success: true, TokenData: token, SessionPreview: &preview}
		}(i, session)
	}

	wg.Wait()

	successful := 0
	for _, r := range results {
		if r.Success {
			successful++
		}
	}

	c.JSON(http.StatusOK, models.BatchImportResult{
		Total:      len(req.Sessions),
		Successful: successful,
		Failed:     len(req.Sessions) - successful,
		Results:    results,
	})
}

// ExportJSON exports all tokens as JSON
func (h *AugmentHandler) ExportJSON(c *gin.Context) {
	tokens, err := h.storage.LoadTokens()
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.APIError{Error: err.Error(), Code: "STORAGE_ERROR"})
		return
	}

	c.Header("Content-Disposition", "attachment; filename=tokens.json")
	c.JSON(http.StatusOK, tokens)
}

// ImportJSON imports tokens from JSON body
func (h *AugmentHandler) ImportJSON(c *gin.Context) {
	var tokens []models.AugmentToken
	if err := c.ShouldBindJSON(&tokens); err != nil {
		c.JSON(http.StatusBadRequest, models.APIError{Error: err.Error(), Code: "INVALID_REQUEST"})
		return
	}

	imported := 0
	for i := range tokens {
		if tokens[i].ID == "" {
			tokens[i].ID = uuid.New().String()
		}
		if tokens[i].CreatedAt.IsZero() {
			tokens[i].CreatedAt = time.Now()
		}
		tokens[i].UpdatedAt = time.Now()
		if err := h.storage.SaveToken(&tokens[i]); err == nil {
			imported++
		}
	}

	c.JSON(http.StatusOK, gin.H{"imported": imported, "total": len(tokens)})
}

// SyncTokens performs incremental sync
func (h *AugmentHandler) SyncTokens(c *gin.Context) {
	var req models.ClientSyncRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.APIError{Error: err.Error(), Code: "INVALID_REQUEST"})
		return
	}

	resp, err := h.storage.SyncTokens(&req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.APIError{Error: err.Error(), Code: "SYNC_ERROR"})
		return
	}

	c.JSON(http.StatusOK, resp)
}

func boolPtr(b bool) *bool { return &b }
