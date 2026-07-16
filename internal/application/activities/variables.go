package activities

import (
	"fmt"
	"log"
	"os"

	"github.com/onasunnymorning/domain-os/internal/infrastructure/auth"
)

var (
	BASEURL      string
	tokenManager *auth.TokenManager
)

func init() {
	BASEURL = os.Getenv("API_URL")
	if BASEURL == "" {
		host := os.Getenv("API_HOST")
		if host == "" {
			host = "localhost"
		}
		port := os.Getenv("API_PORT")
		if port == "" {
			port = "8080"
		}
		BASEURL = fmt.Sprintf("http://%s:%s", host, port)
	}

	// Initialize Token Manager for system authentication
	// We prioritize separate worker credentials if available, otherwise fall back to generic ones (or fail in strict mode)
	clientID := os.Getenv("AUTH0_WORKER_CLIENT_ID")
	if clientID == "" {
		clientID = os.Getenv("AUTH0_CLIENT_ID")
	}
	clientSecret := os.Getenv("AUTH0_WORKER_CLIENT_SECRET")
	if clientSecret == "" {
		clientSecret = os.Getenv("AUTH0_CLIENT_SECRET")
	}

	tokenManager = auth.NewTokenManager(
		os.Getenv("AUTH0_DOMAIN"),
		clientID,
		clientSecret,
		os.Getenv("AUTH0_AUDIENCE"),
	)
}

// GetBearerToken returns a valid bearer token for API requests.
// A token-acquisition failure is returned to the caller rather than silently
// swallowed: the old fallback sent an empty bearer, which the API rejected
// with "Authorization header missing or malformed" — hiding the real cause
// (a failed Auth0 client_credentials grant). ADMIN_TOKEN remains a fallback,
// but only when it is actually set.
func GetBearerToken() (string, error) {
	token, err := tokenManager.GetAccessToken()
	if err != nil {
		if fallback := os.Getenv("ADMIN_TOKEN"); fallback != "" {
			log.Printf("⚠️ Failed to get Auth0 access token, falling back to ADMIN_TOKEN: %v", err)
			return fmt.Sprintf("Bearer %s", fallback), nil
		}
		return "", fmt.Errorf("failed to acquire Auth0 access token (and no ADMIN_TOKEN fallback is set): %w", err)
	}
	return fmt.Sprintf("Bearer %s", token), nil
}
