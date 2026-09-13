package rest

import (
	"testing"

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
