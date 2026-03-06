package augment

import (
	"easyllm/internal/models"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// CheckBanStatus checks if an account is banned/suspended
func CheckBanStatus(token, tenantURL string) (*models.AccountStatus, error) {
	client := createHTTPClient()

	baseURL := tenantURL
	if !strings.HasSuffix(baseURL, "/") {
		baseURL += "/"
	}

	apiURL := baseURL + "find-missing"

	req, err := http.NewRequest("POST", apiURL, strings.NewReader("{}"))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case 200:
		return &models.AccountStatus{Status: "ACTIVE"}, nil
	case 401:
		return &models.AccountStatus{Status: "INVALID_TOKEN"}, nil
	case 403:
		return &models.AccountStatus{Status: "SUSPENDED"}, nil
	default:
		body, _ := io.ReadAll(resp.Body)
		msg := fmt.Sprintf("HTTP %d: %s", resp.StatusCode, string(body))
		return &models.AccountStatus{Status: "ERROR", ErrorMessage: &msg}, nil
	}
}

// GetModels fetches user model info from Augment API
func GetModels(token, tenantURL string) (*models.ModelsResponse, error) {
	client := createHTTPClient()

	baseURL := tenantURL
	if !strings.HasSuffix(baseURL, "/") {
		baseURL += "/"
	}
	apiURL := baseURL + "get-models"

	req, err := http.NewRequest("POST", apiURL, strings.NewReader("{}"))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("API returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var modelsResp models.ModelsResponse
	if err := json.Unmarshal(body, &modelsResp); err != nil {
		return nil, fmt.Errorf("failed to parse models response: %w", err)
	}

	return &modelsResp, nil
}

// GetCreditInfo fetches credit/quota information
func GetCreditInfo(token, tenantURL string) (map[string]interface{}, error) {
	client := createHTTPClient()

	baseURL := tenantURL
	if !strings.HasSuffix(baseURL, "/") {
		baseURL += "/"
	}
	apiURL := baseURL + "get-subscription-v2"

	req, err := http.NewRequest("POST", apiURL, strings.NewReader("{}"))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}

	return result, nil
}

// GetPortalInfo fetches portal/subscription information
func GetPortalInfo(token, tenantURL string) (map[string]interface{}, error) {
	client := createHTTPClient()

	baseURL := tenantURL
	if !strings.HasSuffix(baseURL, "/") {
		baseURL += "/"
	}
	apiURL := baseURL + "get-portal-link"

	payload := `{"return_url": ""}`
	req, err := http.NewRequest("POST", apiURL, strings.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}

	return result, nil
}

// BatchCheckStatus checks the status of multiple tokens concurrently
func BatchCheckStatus(tokens []struct{ ID, Token, TenantURL string }, maxConcurrent int) []models.TokenStatusResult {
	if maxConcurrent <= 0 {
		maxConcurrent = 5
	}

	semaphore := make(chan struct{}, maxConcurrent)
	results := make([]models.TokenStatusResult, len(tokens))
	done := make(chan struct{})

	for i, t := range tokens {
		go func(idx int, id, tok, tenantURL string) {
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			status, err := CheckBanStatus(tok, tenantURL)
			if err != nil {
				msg := err.Error()
				results[idx] = models.TokenStatusResult{
					TokenID: id,
					Status:  models.AccountStatus{Status: "ERROR", ErrorMessage: &msg},
				}
			} else {
				results[idx] = models.TokenStatusResult{
					TokenID: id,
					Status:  *status,
				}
			}
			done <- struct{}{}
		}(i, t.ID, t.Token, t.TenantURL)
	}

	for range tokens {
		<-done
	}

	return results
}
