package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

// TokenSource obtains a Keystone token (password or application credential) and caches it.
// Same auth_url and credentials as the OpenStack Terraform provider can be used.
type TokenSource struct {
	IdentityEndpoint string
	HTTPClient       *http.Client

	// Password auth (compatible with OpenStack provider fields)
	UsePassword       bool
	UserName          string
	Password          string
	UserDomainName    string
	TenantName        string
	ProjectDomainName string

	// Application credential (alternative to password)
	AppCredID     string
	AppCredSecret string

	mu        sync.Mutex
	token     string
	expiresAt time.Time
}

type keystoneTokenResponse struct {
	Token struct {
		ExpiresAt time.Time `json:"expires_at"`
	} `json:"token"`
}

// Token returns a valid Keystone token, refreshing before expiry.
func (s *TokenSource) Token(ctx context.Context) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.token != "" && time.Now().Add(1*time.Minute).Before(s.expiresAt) {
		return s.token, nil
	}

	if s.HTTPClient == nil {
		s.HTTPClient = http.DefaultClient
	}

	url := strings.TrimSuffix(s.IdentityEndpoint, "/") + "/v3/auth/tokens"

	var payload []byte
	var err error

	if s.UsePassword {
		projectDomain := s.ProjectDomainName
		if projectDomain == "" {
			projectDomain = "Default"
		}
		body := map[string]interface{}{
			"auth": map[string]interface{}{
				"identity": map[string]interface{}{
					"methods": []string{"password"},
					"password": map[string]interface{}{
						"user": map[string]interface{}{
							"name":     s.UserName,
							"password": s.Password,
							"domain": map[string]interface{}{
								"name": s.UserDomainName,
							},
						},
					},
				},
				"scope": map[string]interface{}{
					"project": map[string]interface{}{
						"name": s.TenantName,
						"domain": map[string]interface{}{
							"name": projectDomain,
						},
					},
				},
			},
		}
		payload, err = json.Marshal(body)
	} else {
		body := map[string]interface{}{
			"auth": map[string]interface{}{
				"identity": map[string]interface{}{
					"methods": []string{"application_credential"},
					"application_credential": map[string]interface{}{
						"id":     s.AppCredID,
						"secret": s.AppCredSecret,
					},
				},
			},
		}
		payload, err = json.Marshal(body)
	}
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.HTTPClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		b, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("keystone token request failed: %s: %s", resp.Status, strings.TrimSpace(string(b)))
	}

	subject := resp.Header.Get("X-Subject-Token")
	if subject == "" {
		return "", fmt.Errorf("keystone response missing X-Subject-Token header")
	}

	var decoded keystoneTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&decoded); err != nil {
		return "", fmt.Errorf("decode token response: %w", err)
	}

	s.token = subject
	if decoded.Token.ExpiresAt.IsZero() {
		s.expiresAt = time.Now().Add(1 * time.Hour)
	} else {
		s.expiresAt = decoded.Token.ExpiresAt
	}

	return s.token, nil
}
