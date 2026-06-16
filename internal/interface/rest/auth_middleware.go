package rest

import (
	"context"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/auth0/go-jwt-middleware/v2/jwks"
	"github.com/auth0/go-jwt-middleware/v2/validator"
	"github.com/gin-gonic/gin"
)

// CustomClaims contains custom data we want from the Auth0 token.
type CustomClaims struct {
	Scope string `json:"scope"`
}

// Validate does nothing for this example, but is required by the validator interface.
func (c *CustomClaims) Validate(ctx context.Context) error {
	return nil
}

// Auth0Middleware validates the Auth0 access token or falls back to a legacy token.
func Auth0Middleware(domain, audience, legacyToken string, auth0Enabled bool) gin.HandlerFunc {
	// If Auth0 is disabled, fall back immediately to simple token auth
	if !auth0Enabled {
		return func(c *gin.Context) {
			authHeader := c.GetHeader("Authorization")
			if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Authorization header missing or malformed"})
				return
			}
			token := strings.TrimPrefix(authHeader, "Bearer ")
			if token != legacyToken {
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Invalid token"})
				return
			}
			c.Set("userid", "legacy-admin")
			c.Next()
		}
	}

	issuerURL, err := url.Parse("https://" + domain + "/")
	if err != nil {
		log.Fatalf("Failed to parse the issuer url: %v", err)
	}

	provider := jwks.NewCachingProvider(issuerURL, 5*time.Minute)

	jwtValidator, err := validator.New(
		provider.KeyFunc,
		validator.RS256,
		issuerURL.String(),
		[]string{audience},
		validator.WithCustomClaims(
			func() validator.CustomClaims {
				return &CustomClaims{}
			},
		),
		validator.WithAllowedClockSkew(time.Minute),
	)
	if err != nil {
		log.Fatalf("Failed to set up the jwt validator: %v", err)
	}

	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Authorization header missing or malformed"})
			return
		}

		tokenString := strings.TrimPrefix(authHeader, "Bearer ")

		// Validate as Auth0 JWT
		claims, err := jwtValidator.ValidateToken(c.Request.Context(), tokenString)
		if err != nil {
			log.Printf("[Auth0Middleware] Token validation failed: %v", err)
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error":   "Invalid token",
				"details": err.Error(),
			})
			return
		}

		// Set the user ID in the context (sub claim in Auth0)
		if validatedClaims, ok := claims.(*validator.ValidatedClaims); ok {
			c.Set("userid", validatedClaims.RegisteredClaims.Subject)
		}

		c.Next()
	}
}
