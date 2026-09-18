package rest

import (
	"net/http"

	"github.com/auth0/go-jwt-middleware/v2/validator"
	"github.com/gin-gonic/gin"
	"github.com/onasunnymorning/domain-os/internal/appcontext"
)

// authScopesKey is the gin key the authentication middleware stores granted
// OAuth scopes under before ContextPropagationMiddleware moves them onto the
// request context (appcontext.WithAuthScopes, INV-15).
const authScopesKey = "auth_scopes"

// Permissions checked by the admin API. They are the first per-permission
// checks in this API (ADR-0009); ADR-0002 will generalise them. With Auth0
// enabled a principal holds them through its Auth0 roles; with Auth0 disabled
// the static admin token holds both, because it already reaches everything.
const (
	// ScopeEscrowKeysAdmin allows changing an operator's escrow parties, keys
	// and arrangements, for the operator in the request scope.
	ScopeEscrowKeysAdmin = "escrow:keys:admin"
	// ScopeEscrowPlatformKeysAdmin allows changing the platform's escrow
	// parties, keys and default arrangement, and reading the platform view.
	ScopeEscrowPlatformKeysAdmin = "escrow:platform-keys:admin" //nolint:gosec // G101: an OAuth scope name, not a credential
)

// grantedPermissions returns the permissions an Auth0 access token grants,
// from its RBAC "permissions" claim only. The "scope" claim is deliberately
// ignored: when RBAC is off, Auth0 puts any scope a client requests into it, so
// a user could grant themselves a permission by asking for it. Without RBAC the
// permissions claim is absent and every check fails closed.
func grantedPermissions(claims *validator.ValidatedClaims) []string {
	custom, ok := claims.CustomClaims.(*CustomClaims)
	if !ok || custom == nil {
		return nil
	}
	return append([]string(nil), custom.Permissions...)
}

// hasAuthScope reports whether the request's principal was granted scope.
func hasAuthScope(c *gin.Context, scope string) bool {
	return appcontext.HasAuthScope(c.Request.Context(), scope)
}

// requireAuthScope aborts with 403 unless the principal holds scope.
func requireAuthScope(c *gin.Context, scope string) bool {
	if hasAuthScope(c, scope) {
		return true
	}
	c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "this action requires the " + scope + " permission"})
	return false
}
