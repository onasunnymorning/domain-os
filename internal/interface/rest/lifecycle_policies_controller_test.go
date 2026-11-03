package rest

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onasunnymorning/domain-os/internal/appcontext"
	"github.com/onasunnymorning/domain-os/internal/application/services"
	"github.com/onasunnymorning/domain-os/internal/infrastructure/memstore"
	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
)

func lifecyclePoliciesRouter(t *testing.T, scopes ...string) (*gin.Engine, *memstore.LifecyclePolicies) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	repo := memstore.NewLifecyclePolicies()
	r := gin.New()
	grant := func(c *gin.Context) {
		ctx := appcontext.WithAuthScopes(c.Request.Context(), scopes)
		c.Request = c.Request.WithContext(appcontext.WithUserID(ctx, "ops@example.com"))
		c.Next()
	}
	NewLifecyclePoliciesController(r, services.NewLifecyclePolicyService(repo), grant)
	return r, repo
}

func TestLifecyclePoliciesRequirePermission(t *testing.T) {
	// Neither the EPP policy nor the platform registry permission reaches a
	// change to the lifecycle policies; reading them needs no permission,
	// since whoever edits a phase picks from them.
	r, _ := lifecyclePoliciesRouter(t, ScopeEPPPolicyAdmin, ScopeRegistryPlatformAdmin)
	for _, m := range []string{http.MethodPut, http.MethodDelete} {
		w := do(r, m, "/lifecycle-policies/gtld-standard", "{}")
		assert.Equal(t, http.StatusForbidden, w.Code, m)
		assert.Contains(t, w.Body.String(), ScopeRegistryPolicyAdmin)
	}
	for _, p := range []string{"/lifecycle-policies", "/lifecycle-policies/gtld-standard"} {
		w := do(r, http.MethodGet, p, "")
		assert.Equal(t, http.StatusOK, w.Code, p)
	}
}

func TestLifecyclePoliciesAPI(t *testing.T) {
	r, repo := lifecyclePoliciesRouter(t, ScopeRegistryPolicyAdmin)

	w := do(r, http.MethodPut, "/lifecycle-policies/short", `{"description":"demo","settings":{"redemptionGP":3,"transferLockPeriod":0}}`)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	// A flat struct: the entity's own UnmarshalJSON would drop "resolved".
	var p struct {
		Name      string                     `json:"name"`
		UpdatedBy string                     `json:"updatedBy"`
		Resolved  entities.ResolvedLifecycle `json:"resolved"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &p))
	assert.Equal(t, "short", p.Name)
	assert.Equal(t, 3, p.Resolved.RedemptionGP)
	assert.Equal(t, 0, p.Resolved.TransferLockPeriod, "0 is zero days")
	assert.Equal(t, entities.PendingDeleteGP, p.Resolved.PendingDeleteGP, "null is the default")
	assert.Contains(t, w.Body.String(), `"pendingdeleteGP":null`)
	assert.Equal(t, "ops@example.com", p.UpdatedBy)

	w = do(r, http.MethodGet, "/lifecycle-policies", "")
	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"name":"gtld-standard"`)
	assert.Contains(t, w.Body.String(), `"name":"short"`)

	w = do(r, http.MethodPut, "/lifecycle-policies/short", `{"settings":{"redemptionGP":400}}`)
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "redemptionGP must be 0 to 365")
	w = do(r, http.MethodPut, "/lifecycle-policies/Short", `{"settings":{}}`)
	assert.Equal(t, http.StatusBadRequest, w.Code)

	// Conditional saves: 412 either way.
	req := httptest.NewRequest(http.MethodPut, "/lifecycle-policies/short", strings.NewReader(`{"settings":{}}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("If-None-Match", "*")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusPreconditionFailed, w.Code)
	req = httptest.NewRequest(http.MethodPut, "/lifecycle-policies/short", strings.NewReader(`{"settings":{}}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("If-Match", `"2020-01-01T00:00:00Z"`)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusPreconditionFailed, w.Code)

	w = do(r, http.MethodDelete, "/lifecycle-policies/gtld-standard", "")
	assert.Equal(t, http.StatusConflict, w.Code)
	repo.Uses["short"] = []entities.LifecyclePolicyUse{{TLD: "example", Phase: "ga"}}
	w = do(r, http.MethodGet, "/lifecycle-policies/short", "")
	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"usedBy":[{"tld":"example","phase":"ga"}]`)
	w = do(r, http.MethodDelete, "/lifecycle-policies/short", "")
	assert.Equal(t, http.StatusConflict, w.Code)
	repo.Uses["short"] = nil
	w = do(r, http.MethodDelete, "/lifecycle-policies/short", "")
	assert.Equal(t, http.StatusNoContent, w.Code)
	w = do(r, http.MethodGet, "/lifecycle-policies/short", "")
	assert.Equal(t, http.StatusNotFound, w.Code)
}
