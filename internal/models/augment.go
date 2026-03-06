package models

import (
	"time"
)

// AugmentToken represents an Augment access token and its metadata
type AugmentToken struct {
	ID               string      `json:"id" gorm:"primaryKey"`
	TenantURL        string      `json:"tenant_url"`
	AccessToken      string      `json:"access_token"`
	CreatedAt        time.Time   `json:"created_at"`
	UpdatedAt        time.Time   `json:"updated_at"`
	PortalURL        *string     `json:"portal_url,omitempty"`
	EmailNote        *string     `json:"email_note,omitempty"`
	TagName          *string     `json:"tag_name,omitempty"`
	TagColor         *string     `json:"tag_color,omitempty"`
	BanStatus        interface{} `json:"ban_status,omitempty" gorm:"type:json"`
	PortalInfo       interface{} `json:"portal_info,omitempty" gorm:"type:json"`
	AuthSession      *string     `json:"auth_session,omitempty"`
	Suspensions      interface{} `json:"suspensions,omitempty" gorm:"type:json"`
	BalanceColorMode *string     `json:"balance_color_mode,omitempty"`
	SkipCheck        *bool       `json:"skip_check,omitempty"`
	SessionUpdatedAt *time.Time  `json:"session_updated_at,omitempty"`
	Version          int64       `json:"version" gorm:"default:0"`
}

// AugmentOAuthState holds the state for OAuth flow
type AugmentOAuthState struct {
	CodeVerifier  string `json:"code_verifier"`
	CodeChallenge string `json:"code_challenge"`
	State         string `json:"state"`
	CreationTime  int64  `json:"creation_time"`
}

// AugmentTokenResponse is the response from Augment OAuth
type AugmentTokenResponse struct {
	AccessToken string  `json:"access_token"`
	TenantURL   string  `json:"tenant_url"`
	Email       *string `json:"email,omitempty"`
}

// ParsedCode holds the parsed OAuth code data
type ParsedCode struct {
	Code      string `json:"code"`
	State     string `json:"state"`
	TenantURL string `json:"tenant_url"`
}

// AccountStatus represents the ban/suspension status of an account
type AccountStatus struct {
	Status       string  `json:"status"`
	ErrorMessage *string `json:"error_message,omitempty"`
}

// ModelsResponse is the response from Augment get-models API
type ModelsResponse struct {
	User UserInfo `json:"user"`
}

type UserInfo struct {
	Email string `json:"email"`
}

// CreditInfo holds credit usage information
type CreditInfo struct {
	Used      float64 `json:"used"`
	Total     float64 `json:"total"`
	Remaining float64 `json:"remaining"`
}

// TokenStatusResult contains token validation results
type TokenStatusResult struct {
	TokenID string        `json:"token_id"`
	Status  AccountStatus `json:"status"`
}

// BatchImportRequest for importing multiple sessions
type BatchImportRequest struct {
	Sessions         []string `json:"sessions"`
	DetailedResponse bool     `json:"detailed_response"`
}

// ImportSessionRequest for importing a single session
type ImportSessionRequest struct {
	Session          string `json:"session"`
	DetailedResponse bool   `json:"detailed_response"`
}

// ImportResult is the result of a single session import
type ImportResult struct {
	Success        bool          `json:"success"`
	TokenData      *AugmentToken `json:"token_data,omitempty"`
	Error          *string       `json:"error,omitempty"`
	SessionPreview *string       `json:"session_preview,omitempty"`
}

// BatchImportResult is the result of batch session import
type BatchImportResult struct {
	Total      int            `json:"total"`
	Successful int            `json:"successful"`
	Failed     int            `json:"failed"`
	Results    []ImportResult `json:"results"`
}

// ClientSyncRequest for incremental sync
type ClientSyncRequest struct {
	LastVersion int64                `json:"last_version"`
	Upserts     []ClientTokenChange  `json:"upserts"`
	Deletions   []ClientDelete       `json:"deletions"`
}

type ClientTokenChange struct {
	Token AugmentToken `json:"token"`
}

type ClientDelete struct {
	ID string `json:"id"`
}

// ServerSyncResponse for sync response
type ServerSyncResponse struct {
	Upserts     []AugmentToken `json:"upserts"`
	Deletions   []string       `json:"deletions"`
	NewVersion  int64          `json:"new_version"`
}

// TeamInfo represents team/workspace information
type TeamInfo struct {
	TeamID   string       `json:"team_id"`
	TeamName string       `json:"team_name"`
	Members  []TeamMember `json:"members"`
}

type TeamMember struct {
	UserID string `json:"user_id"`
	Email  string `json:"email"`
	Role   string `json:"role"`
}
