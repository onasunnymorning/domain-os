package auth

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sync"
	"time"
)

// TokenManager handles fetching and caching Auth0 tokens
type TokenManager struct {
	domain       string
	clientID     string
	clientSecret string
	audience     string

	// Cache fields
	cachedToken string
	expiresAt   time.Time
	mutex       sync.Mutex // Ensures thread safety for concurrent workers
}

// NewTokenManager initializes the manager with config
func NewTokenManager(domain, id, secret, audience string) *TokenManager {
	return &TokenManager{
		domain:       domain,
		clientID:     id,
		clientSecret: secret,
		audience:     audience,
	}
}

// GetAccessToken returns a valid token (cached or fresh)
func (tm *TokenManager) GetAccessToken() (string, error) {
	// 1. MOCK BYPASS: Check if we are in local dev mode or Auth0 is disabled
	if os.Getenv("AUTH0_ENABLED") != "true" {
		return os.Getenv("ADMIN_TOKEN"), nil
	}

	tm.mutex.Lock()
	defer tm.mutex.Unlock()

	// 2. Check Cache: If token exists and expires in > 60 seconds, use it
	if tm.cachedToken != "" && time.Now().Add(60*time.Second).Before(tm.expiresAt) {
		return tm.cachedToken, nil
	}

	// 3. Cache Miss: Fetch new token
	// fmt.Println("🔄 Token expired or missing. Fetching fresh system token...")
	token, expiresIn, err := tm.fetchFromAuth0()
	if err != nil {
		return "", err
	}

	// 4. Update Cache
	tm.cachedToken = token
	tm.expiresAt = time.Now().Add(time.Duration(expiresIn) * time.Second)

	return tm.cachedToken, nil
}

// Internal method to perform the HTTP request
func (tm *TokenManager) fetchFromAuth0() (string, int, error) {
	url := fmt.Sprintf("https://%s/oauth/token", tm.domain)
	payload := map[string]string{
		"client_id":     tm.clientID,
		"client_secret": tm.clientSecret,
		"audience":      tm.audience,
		"grant_type":    "client_credentials",
	}

	jsonPayload, _ := json.Marshal(payload)
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonPayload))
	if err != nil {
		return "", 0, err
	}
	req.Header.Add("content-type", "application/json")

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", 0, err
	}
	defer res.Body.Close()

	if res.StatusCode != 200 {
		bodyBytes, _ := io.ReadAll(res.Body)
		return "", 0, fmt.Errorf("failed to retrieve token from Auth0: %s - %s", res.Status, string(bodyBytes))
	}

	// Parse response
	var result struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"` // Seconds
	}

	if err := json.NewDecoder(res.Body).Decode(&result); err != nil {
		return "", 0, err
	}

	return result.AccessToken, result.ExpiresIn, nil
}
