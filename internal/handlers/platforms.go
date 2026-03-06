package handlers

import (
	"easyllm/internal/models"
	"easyllm/internal/storage"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// CursorHandler manages Cursor accounts
type CursorHandler struct{ storage *storage.CursorStorage }

func NewCursorHandler(s *storage.CursorStorage) *CursorHandler { return &CursorHandler{storage: s} }

func (h *CursorHandler) RegisterRoutes(rg *gin.RouterGroup) {
	g := rg.Group("/cursor")
	g.GET("/accounts", h.List)
	g.POST("/accounts", h.Add)
	g.PUT("/accounts/:id", h.Update)
	g.DELETE("/accounts/:id", h.Delete)
	g.DELETE("/accounts", h.DeleteMany)
	g.POST("/accounts/:id/activate", h.Activate)
	g.POST("/import", h.Import)
}

func (h *CursorHandler) List(c *gin.Context) {
	list, err := h.storage.List()
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.APIError{Error: err.Error(), Code: "STORAGE_ERROR"})
		return
	}
	c.JSON(http.StatusOK, list)
}

func (h *CursorHandler) Add(c *gin.Context) {
	var a models.CursorAccount
	if err := c.ShouldBindJSON(&a); err != nil {
		c.JSON(http.StatusBadRequest, models.APIError{Error: err.Error(), Code: "INVALID_REQUEST"})
		return
	}
	if a.ID == "" {
		a.ID = uuid.New().String()
	}
	a.CreatedAt = time.Now()
	a.UpdatedAt = time.Now()
	if err := h.storage.Save(&a); err != nil {
		c.JSON(http.StatusInternalServerError, models.APIError{Error: err.Error(), Code: "STORAGE_ERROR"})
		return
	}
	c.JSON(http.StatusOK, a)
}

func (h *CursorHandler) Update(c *gin.Context) {
	var a models.CursorAccount
	if err := c.ShouldBindJSON(&a); err != nil {
		c.JSON(http.StatusBadRequest, models.APIError{Error: err.Error(), Code: "INVALID_REQUEST"})
		return
	}
	a.ID = c.Param("id")
	if err := h.storage.Save(&a); err != nil {
		c.JSON(http.StatusInternalServerError, models.APIError{Error: err.Error(), Code: "STORAGE_ERROR"})
		return
	}
	c.JSON(http.StatusOK, a)
}

func (h *CursorHandler) Delete(c *gin.Context) {
	if err := h.storage.Delete(c.Param("id")); err != nil {
		c.JSON(http.StatusInternalServerError, models.APIError{Error: err.Error(), Code: "STORAGE_ERROR"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (h *CursorHandler) DeleteMany(c *gin.Context) {
	var req struct {
		IDs []string `json:"ids"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.APIError{Error: err.Error(), Code: "INVALID_REQUEST"})
		return
	}
	if err := h.storage.DeleteMany(req.IDs); err != nil {
		c.JSON(http.StatusInternalServerError, models.APIError{Error: err.Error(), Code: "STORAGE_ERROR"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (h *CursorHandler) Activate(c *gin.Context) {
	if err := h.storage.SetActive(c.Param("id")); err != nil {
		c.JSON(http.StatusInternalServerError, models.APIError{Error: err.Error(), Code: "STORAGE_ERROR"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (h *CursorHandler) Import(c *gin.Context) {
	var accounts []models.CursorAccount
	if err := c.ShouldBindJSON(&accounts); err != nil {
		c.JSON(http.StatusBadRequest, models.APIError{Error: err.Error(), Code: "INVALID_REQUEST"})
		return
	}
	imported := 0
	for i := range accounts {
		if accounts[i].ID == "" {
			accounts[i].ID = uuid.New().String()
		}
		accounts[i].CreatedAt = time.Now()
		accounts[i].UpdatedAt = time.Now()
		if err := h.storage.Save(&accounts[i]); err == nil {
			imported++
		}
	}
	c.JSON(http.StatusOK, gin.H{"imported": imported})
}

// WindsurfHandler manages Windsurf accounts
type WindsurfHandler struct{ storage *storage.WindsurfStorage }

func NewWindsurfHandler(s *storage.WindsurfStorage) *WindsurfHandler {
	return &WindsurfHandler{storage: s}
}

func (h *WindsurfHandler) RegisterRoutes(rg *gin.RouterGroup) {
	g := rg.Group("/windsurf")
	g.GET("/accounts", h.List)
	g.POST("/accounts", h.Add)
	g.PUT("/accounts/:id", h.Update)
	g.DELETE("/accounts/:id", h.Delete)
	g.DELETE("/accounts", h.DeleteMany)
	g.POST("/accounts/:id/activate", h.Activate)
}

func (h *WindsurfHandler) List(c *gin.Context) {
	list, err := h.storage.List()
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.APIError{Error: err.Error(), Code: "STORAGE_ERROR"})
		return
	}
	c.JSON(http.StatusOK, list)
}

func (h *WindsurfHandler) Add(c *gin.Context) {
	var a models.WindsurfAccount
	if err := c.ShouldBindJSON(&a); err != nil {
		c.JSON(http.StatusBadRequest, models.APIError{Error: err.Error(), Code: "INVALID_REQUEST"})
		return
	}
	if a.ID == "" {
		a.ID = uuid.New().String()
	}
	a.CreatedAt = time.Now()
	if err := h.storage.Save(&a); err != nil {
		c.JSON(http.StatusInternalServerError, models.APIError{Error: err.Error(), Code: "STORAGE_ERROR"})
		return
	}
	c.JSON(http.StatusOK, a)
}

func (h *WindsurfHandler) Update(c *gin.Context) {
	var a models.WindsurfAccount
	if err := c.ShouldBindJSON(&a); err != nil {
		c.JSON(http.StatusBadRequest, models.APIError{Error: err.Error(), Code: "INVALID_REQUEST"})
		return
	}
	a.ID = c.Param("id")
	if err := h.storage.Save(&a); err != nil {
		c.JSON(http.StatusInternalServerError, models.APIError{Error: err.Error(), Code: "STORAGE_ERROR"})
		return
	}
	c.JSON(http.StatusOK, a)
}

func (h *WindsurfHandler) Delete(c *gin.Context) {
	if err := h.storage.Delete(c.Param("id")); err != nil {
		c.JSON(http.StatusInternalServerError, models.APIError{Error: err.Error(), Code: "STORAGE_ERROR"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (h *WindsurfHandler) DeleteMany(c *gin.Context) {
	var req struct {
		IDs []string `json:"ids"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.APIError{Error: err.Error(), Code: "INVALID_REQUEST"})
		return
	}
	if err := h.storage.DeleteMany(req.IDs); err != nil {
		c.JSON(http.StatusInternalServerError, models.APIError{Error: err.Error(), Code: "STORAGE_ERROR"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (h *WindsurfHandler) Activate(c *gin.Context) {
	if err := h.storage.SetActive(c.Param("id")); err != nil {
		c.JSON(http.StatusInternalServerError, models.APIError{Error: err.Error(), Code: "STORAGE_ERROR"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

// AntigravityHandler manages Antigravity accounts
type AntigravityHandler struct{ storage *storage.AntigravityStorage }

func NewAntigravityHandler(s *storage.AntigravityStorage) *AntigravityHandler {
	return &AntigravityHandler{storage: s}
}

func (h *AntigravityHandler) RegisterRoutes(rg *gin.RouterGroup) {
	g := rg.Group("/antigravity")
	g.GET("/accounts", h.List)
	g.POST("/accounts", h.Add)
	g.PUT("/accounts/:id", h.Update)
	g.DELETE("/accounts/:id", h.Delete)
	g.DELETE("/accounts", h.DeleteMany)
	g.POST("/accounts/:id/activate", h.Activate)
}

func (h *AntigravityHandler) List(c *gin.Context) {
	list, err := h.storage.List()
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.APIError{Error: err.Error(), Code: "STORAGE_ERROR"})
		return
	}
	c.JSON(http.StatusOK, list)
}

func (h *AntigravityHandler) Add(c *gin.Context) {
	var a models.AntigravityAccount
	if err := c.ShouldBindJSON(&a); err != nil {
		c.JSON(http.StatusBadRequest, models.APIError{Error: err.Error(), Code: "INVALID_REQUEST"})
		return
	}
	if a.ID == "" {
		a.ID = uuid.New().String()
	}
	a.CreatedAt = time.Now()
	if err := h.storage.Save(&a); err != nil {
		c.JSON(http.StatusInternalServerError, models.APIError{Error: err.Error(), Code: "STORAGE_ERROR"})
		return
	}
	c.JSON(http.StatusOK, a)
}

func (h *AntigravityHandler) Update(c *gin.Context) {
	var a models.AntigravityAccount
	if err := c.ShouldBindJSON(&a); err != nil {
		c.JSON(http.StatusBadRequest, models.APIError{Error: err.Error(), Code: "INVALID_REQUEST"})
		return
	}
	a.ID = c.Param("id")
	if err := h.storage.Save(&a); err != nil {
		c.JSON(http.StatusInternalServerError, models.APIError{Error: err.Error(), Code: "STORAGE_ERROR"})
		return
	}
	c.JSON(http.StatusOK, a)
}

func (h *AntigravityHandler) Delete(c *gin.Context) {
	if err := h.storage.Delete(c.Param("id")); err != nil {
		c.JSON(http.StatusInternalServerError, models.APIError{Error: err.Error(), Code: "STORAGE_ERROR"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (h *AntigravityHandler) DeleteMany(c *gin.Context) {
	var req struct {
		IDs []string `json:"ids"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.APIError{Error: err.Error(), Code: "INVALID_REQUEST"})
		return
	}
	if err := h.storage.DeleteMany(req.IDs); err != nil {
		c.JSON(http.StatusInternalServerError, models.APIError{Error: err.Error(), Code: "STORAGE_ERROR"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (h *AntigravityHandler) Activate(c *gin.Context) {
	if err := h.storage.SetActive(c.Param("id")); err != nil {
		c.JSON(http.StatusInternalServerError, models.APIError{Error: err.Error(), Code: "STORAGE_ERROR"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

// ClaudeHandler manages Claude accounts
type ClaudeHandler struct{ storage *storage.ClaudeStorage }

func NewClaudeHandler(s *storage.ClaudeStorage) *ClaudeHandler { return &ClaudeHandler{storage: s} }

func (h *ClaudeHandler) RegisterRoutes(rg *gin.RouterGroup) {
	g := rg.Group("/claude")
	g.GET("/accounts", h.List)
	g.POST("/accounts", h.Add)
	g.PUT("/accounts/:id", h.Update)
	g.DELETE("/accounts/:id", h.Delete)
	g.DELETE("/accounts", h.DeleteMany)
}

func (h *ClaudeHandler) List(c *gin.Context) {
	list, err := h.storage.List()
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.APIError{Error: err.Error(), Code: "STORAGE_ERROR"})
		return
	}
	c.JSON(http.StatusOK, list)
}

func (h *ClaudeHandler) Add(c *gin.Context) {
	var a models.ClaudeAccount
	if err := c.ShouldBindJSON(&a); err != nil {
		c.JSON(http.StatusBadRequest, models.APIError{Error: err.Error(), Code: "INVALID_REQUEST"})
		return
	}
	if a.ID == "" {
		a.ID = uuid.New().String()
	}
	a.CreatedAt = time.Now()
	if err := h.storage.Save(&a); err != nil {
		c.JSON(http.StatusInternalServerError, models.APIError{Error: err.Error(), Code: "STORAGE_ERROR"})
		return
	}
	c.JSON(http.StatusOK, a)
}

func (h *ClaudeHandler) Update(c *gin.Context) {
	var a models.ClaudeAccount
	if err := c.ShouldBindJSON(&a); err != nil {
		c.JSON(http.StatusBadRequest, models.APIError{Error: err.Error(), Code: "INVALID_REQUEST"})
		return
	}
	a.ID = c.Param("id")
	if err := h.storage.Save(&a); err != nil {
		c.JSON(http.StatusInternalServerError, models.APIError{Error: err.Error(), Code: "STORAGE_ERROR"})
		return
	}
	c.JSON(http.StatusOK, a)
}

func (h *ClaudeHandler) Delete(c *gin.Context) {
	if err := h.storage.Delete(c.Param("id")); err != nil {
		c.JSON(http.StatusInternalServerError, models.APIError{Error: err.Error(), Code: "STORAGE_ERROR"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (h *ClaudeHandler) DeleteMany(c *gin.Context) {
	var req struct {
		IDs []string `json:"ids"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.APIError{Error: err.Error(), Code: "INVALID_REQUEST"})
		return
	}
	if err := h.storage.DeleteMany(req.IDs); err != nil {
		c.JSON(http.StatusInternalServerError, models.APIError{Error: err.Error(), Code: "STORAGE_ERROR"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}
