package rest

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/auth0/go-jwt-middleware/v2/validator"
	"github.com/stretchr/testify/assert"
)

func TestGrantedPermissions_ReadsOnlyTheRBACClaim(t *testing.T) {
	requested := &validator.ValidatedClaims{CustomClaims: &CustomClaims{Scope: "openid " + ScopeEscrowPlatformKeysAdmin}}
	assert.Empty(t, grantedPermissions(requested), "a scope the client asked for grants nothing")

	assigned := &validator.ValidatedClaims{CustomClaims: &CustomClaims{Permissions: []string{ScopeEscrowKeysAdmin}}}
	assert.Equal(t, []string{ScopeEscrowKeysAdmin}, grantedPermissions(assigned))

	assert.Empty(t, grantedPermissions(&validator.ValidatedClaims{}), "no custom claims, no permissions")
}

// TestAuth0Disabled_StaticTokenHoldsEveryPermission covers local development:
// with Auth0 off the static admin token reaches every other endpoint, so it
// must also be able to administer escrow keys (ADR-0009).
func TestAuth0Disabled_StaticTokenHoldsEveryPermission(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(Auth0Middleware("", "", "dev-token", false), ContextPropagationMiddleware())
	r.GET("/keys", func(c *gin.Context) {
		if !requireAuthScope(c, ScopeEscrowPlatformKeysAdmin) {
			return
		}
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/keys", nil)
	req.Header.Set("Authorization", "Bearer dev-token")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	bad := httptest.NewRequest(http.MethodGet, "/keys", nil)
	bad.Header.Set("Authorization", "Bearer wrong")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, bad)
	assert.Equal(t, http.StatusUnauthorized, w.Code, "a wrong token is still refused")
}
