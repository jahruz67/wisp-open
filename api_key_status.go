package main

import (
	"net/http"
	"strings"
	"time"
)

// APIKeyStatus describes whether a provider's saved credential is usable.
type APIKeyStatus struct {
	Valid bool   `json:"valid"`
	State string `json:"state"`
}

type cachedAPIKeyStatus struct {
	APIKeyStatus
	key     string
	checked time.Time
}

// GetAPIKeyStatus checks saved credentials without exposing them to the UI.
func (a *App) GetAPIKeyStatus() map[string]APIKeyStatus {
	a.apiKeyStatusMu.Lock()
	defer a.apiKeyStatusMu.Unlock()
	if a.apiKeyStatuses == nil {
		a.apiKeyStatuses = make(map[string]cachedAPIKeyStatus)
	}

	providers := []struct{ name, key, endpoint string }{
		{"groq", a.config.GroqAPIKey, "https://api.groq.com/openai/v1/models"},
		{"mistral", a.config.MistralAPIKey, "https://api.mistral.ai/v1/models"},
	}
	result := make(map[string]APIKeyStatus)
	client := &http.Client{
		Timeout: 8 * time.Second,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	for _, provider := range providers {
		key := strings.TrimSpace(provider.key)
		previous := a.apiKeyStatuses[provider.name]
		if previous.key == key && time.Since(previous.checked) < 5*time.Minute {
			result[provider.name] = previous.APIKeyStatus
			continue
		}
		status := cachedAPIKeyStatus{APIKeyStatus: APIKeyStatus{State: "missing"}, key: key, checked: time.Now()}
		if key != "" {
			status.State = "unavailable"
			// A temporary outage must not discard a previously verified key.
			status.Valid = previous.key == key && previous.Valid
			req, err := http.NewRequest(http.MethodGet, provider.endpoint, nil)
			if err == nil {
				req.Header.Set("Authorization", "Bearer "+key)
				resp, requestErr := client.Do(req)
				if requestErr == nil {
					resp.Body.Close()
					switch resp.StatusCode {
					case http.StatusOK:
						status.Valid, status.State = true, "valid"
					case http.StatusUnauthorized, http.StatusForbidden:
						status.Valid, status.State = false, "invalid"
					}
				}
			}
		}
		// Retry transient failures on the next refresh, with a short backoff.
		if status.State == "unavailable" {
			status.checked = time.Now().Add(-5*time.Minute + 15*time.Second)
		}
		a.apiKeyStatuses[provider.name] = status
		result[provider.name] = status.APIKeyStatus
	}
	return result
}
