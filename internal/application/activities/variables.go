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
	BASEURL = fmt.Sprintf("http://%s:%s", os.Getenv("API_HOST"), os.Getenv("API_PORT"))

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

// GetBearerToken returns a valid bearer token for API requests
func GetBearerToken() string {
	token, err := tokenManager.GetAccessToken()
	if err != nil {
		log.Printf("⚠️ Failed to get access token: %v", err)
		// Fallback to empty string or existing ADMIN_TOKEN if crucial
		return fmt.Sprintf("Bearer %s", os.Getenv("ADMIN_TOKEN"))
	}
	return fmt.Sprintf("Bearer %s", token)
}
