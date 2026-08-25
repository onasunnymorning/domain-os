package rest

import (
	"errors"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
)

// TenantIDHeader carries the administrative-plane (operator) tenant scope.
//
// Root-asserted interim, per ADR-0006: today every admin-API caller presents
// the one shared root token, so the header is trusted because the caller is
// already the only tenant there is. It is *not* an authorization boundary. When
// the Auth0 tenant claim lands (ADR-0002), the claim replaces this header and
// OperatorScopeFromRequest is the only function that has to change.
const TenantIDHeader = "X-Tenant-ID"

var (
	// ErrMissingOperatorScope is returned when the request carries no operator scope.
	ErrMissingOperatorScope = errors.New("operator scope is required: set the " + TenantIDHeader + " header")
	// ErrInvalidOperatorScope is returned when the operator scope is present but malformed.
	ErrInvalidOperatorScope = errors.New("invalid operator scope in " + TenantIDHeader + " header")
)

// OperatorScopeFromRequest derives the operator (registry operator) tenant
// scope for an admin-API request.
//
// This is the single derivation point for the administrative plane — ADR-0006
// requires that reads of TenantIDHeader live here and nowhere else, so that
// swapping the source for an authenticated claim is a one-function change
// rather than a sweep over controllers.
//
// The caller is guaranteed to be authenticated: every route that calls this is
// registered behind authMW. What this function adds is *shape* — the scope is
// returned as a typed entities.OperatorID that has passed RyID (ClIDType)
// validation, so a handler cannot pass an arbitrary string down the stack.
func OperatorScopeFromRequest(c *gin.Context) (entities.OperatorID, error) {
	raw := strings.TrimSpace(c.GetHeader(TenantIDHeader))
	if raw == "" {
		return entities.OperatorID(""), ErrMissingOperatorScope
	}

	scope, err := entities.NewOperatorID(raw)
	if err != nil {
		return entities.OperatorID(""), errors.Join(ErrInvalidOperatorScope, err)
	}

	return scope, nil
}
