package storage

import (
	"easyllm/internal/models"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"gorm.io/gorm"
)

// AugmentStorage handles CRUD for AugmentToken
type AugmentStorage struct {
	db      *gorm.DB
	jsonMu  sync.Mutex
	jsonPath string
}

func NewAugmentStorage(db *gorm.DB, dataDir string) *AugmentStorage {
	return &AugmentStorage{
		db:       db,
		jsonPath: filepath.Join(dataDir, "tokens.json"),
	}
}

// SaveToken upserts a token
func (s *AugmentStorage) SaveToken(token *models.AugmentToken) error {
	if token.ID == "" {
		return fmt.Errorf("token ID cannot be empty")
	}
	token.UpdatedAt = time.Now()
	return s.db.Save(token).Error
}

// LoadTokens returns all tokens
func (s *AugmentStorage) LoadTokens() ([]models.AugmentToken, error) {
	var tokens []models.AugmentToken
	err := s.db.Order("created_at desc").Find(&tokens).Error
	return tokens, err
}

// GetToken returns a single token by ID
func (s *AugmentStorage) GetToken(id string) (*models.AugmentToken, error) {
	var token models.AugmentToken
	if err := s.db.Where("id = ?", id).First(&token).Error; err != nil {
		return nil, err
	}
	return &token, nil
}

// DeleteToken removes a token by ID
func (s *AugmentStorage) DeleteToken(id string) error {
	return s.db.Where("id = ?", id).Delete(&models.AugmentToken{}).Error
}

// DeleteTokens removes multiple tokens
func (s *AugmentStorage) DeleteTokens(ids []string) error {
	return s.db.Where("id IN ?", ids).Delete(&models.AugmentToken{}).Error
}

// UpdateToken updates specific fields of a token
func (s *AugmentStorage) UpdateToken(token *models.AugmentToken) error {
	token.UpdatedAt = time.Now()
	return s.db.Save(token).Error
}

// GetTokenByEmail finds a token by email
func (s *AugmentStorage) GetTokenByEmail(email string) (*models.AugmentToken, error) {
	var token models.AugmentToken
	err := s.db.Where("email_note = ?", email).First(&token).Error
	if err != nil {
		return nil, err
	}
	return &token, nil
}

// EmailExists checks if an email already exists in storage
func (s *AugmentStorage) EmailExists(email string) bool {
	var count int64
	s.db.Model(&models.AugmentToken{}).Where("LOWER(email_note) = LOWER(?)", email).Count(&count)
	return count > 0
}

// ExportToJSON exports all tokens to a JSON file
func (s *AugmentStorage) ExportToJSON(path string) error {
	tokens, err := s.LoadTokens()
	if err != nil {
		return err
	}

	data, err := json.MarshalIndent(tokens, "", "  ")
	if err != nil {
		return err
	}

	s.jsonMu.Lock()
	defer s.jsonMu.Unlock()
	return os.WriteFile(path, data, 0644)
}

// ImportFromJSON imports tokens from a JSON file
func (s *AugmentStorage) ImportFromJSON(path string) (int, error) {
	s.jsonMu.Lock()
	defer s.jsonMu.Unlock()

	data, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}

	var tokens []models.AugmentToken
	if err := json.Unmarshal(data, &tokens); err != nil {
		// Try legacy format
		var legacyTokens []map[string]interface{}
		if err2 := json.Unmarshal(data, &legacyTokens); err2 != nil {
			return 0, fmt.Errorf("failed to parse JSON: %w", err)
		}
		tokens = convertLegacyTokens(legacyTokens)
	}

	imported := 0
	for _, t := range tokens {
		if t.ID == "" {
			continue
		}
		if err := s.db.Save(&t).Error; err != nil {
			continue
		}
		imported++
	}
	return imported, nil
}

// GetMaxVersion returns the max version number
func (s *AugmentStorage) GetMaxVersion() (int64, error) {
	var maxVersion int64
	err := s.db.Model(&models.AugmentToken{}).Select("COALESCE(MAX(version), 0)").Scan(&maxVersion).Error
	return maxVersion, err
}

// SyncTokens performs incremental sync
func (s *AugmentStorage) SyncTokens(req *models.ClientSyncRequest) (*models.ServerSyncResponse, error) {
	maxVersion, err := s.GetMaxVersion()
	if err != nil {
		return nil, err
	}

	// Process upserts from client
	for _, change := range req.Upserts {
		t := change.Token
		s.db.Save(&t)
	}

	// Process deletions from client
	for _, del := range req.Deletions {
		s.db.Where("id = ?", del.ID).Delete(&models.AugmentToken{})
	}

	// Get updates since last version from server
	var serverUpdates []models.AugmentToken
	s.db.Where("version > ?", req.LastVersion).Find(&serverUpdates)

	newVersion := maxVersion + 1

	return &models.ServerSyncResponse{
		Upserts:    serverUpdates,
		Deletions:  []string{},
		NewVersion: newVersion,
	}, nil
}

// SaveTokensJSON saves tokens directly from JSON string (for bulk import)
func (s *AugmentStorage) SaveTokensJSON(jsonData string) error {
	var tokens []models.AugmentToken
	if err := json.Unmarshal([]byte(jsonData), &tokens); err != nil {
		return err
	}

	for i := range tokens {
		if err := s.db.Save(&tokens[i]).Error; err != nil {
			return err
		}
	}
	return nil
}

// EnsureDataDir ensures the data directory for JSON export exists
func (s *AugmentStorage) EnsureDataDir() error {
	dir := filepath.Dir(s.jsonPath)
	return os.MkdirAll(dir, 0755)
}

func convertLegacyTokens(legacyTokens []map[string]interface{}) []models.AugmentToken {
	var tokens []models.AugmentToken
	for _, lt := range legacyTokens {
		t := models.AugmentToken{}
		if v, ok := lt["id"].(string); ok {
			t.ID = v
		}
		if v, ok := lt["tenant_url"].(string); ok {
			t.TenantURL = v
		}
		if v, ok := lt["access_token"].(string); ok {
			t.AccessToken = v
		}
		if v, ok := lt["email_note"].(string); ok {
			t.EmailNote = &v
		}
		if v, ok := lt["auth_session"].(string); ok {
			t.AuthSession = &v
		}
		if t.ID != "" && t.TenantURL != "" && t.AccessToken != "" {
			t.CreatedAt = time.Now()
			t.UpdatedAt = time.Now()
			tokens = append(tokens, t)
		}
	}
	return tokens
}
