package rest

import (
	"context"
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
	"github.com/onasunnymorning/domain-os/pkg/domain/repositories"
)

type policyRegistrars struct {
	repositories.RegistrarRepository
}

func (policyRegistrars) GetByClID(_ context.Context, clid string, _ bool) (*entities.Registrar, error) {
	if !strings.HasPrefix(clid, "rar-") {
		return nil, entities.ErrRegistrarNotFound
	}
	return &entities.Registrar{ClID: entities.ClIDType(clid)}, nil
}

func eppPoliciesRouter(t *testing.T, scopes ...string) (*gin.Engine, *memstore.EPPPolicies) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	repo := memstore.NewEPPPolicies()
	r := gin.New()
	grant := func(c *gin.Context) {
		ctx := appcontext.WithAuthScopes(c.Request.Context(), scopes)
		c.Request = c.Request.WithContext(appcontext.WithUserID(ctx, "ops@example.com"))
		c.Next()
	}
	NewEPPPoliciesController(r, services.NewEPPPolicyService(repo, policyRegistrars{}), grant)
	return r, repo
}

func do(r *gin.Engine, method, path, body string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	return w
}

func TestEPPPoliciesRequirePermission(t *testing.T) {
	r, _ := eppPoliciesRouter(t, ScopeEPPSessionsAdmin)
	for _, p := range []string{"/epp/policies", "/epp/families", "/registrars/rar-a/epp/policy"} {
		w := do(r, http.MethodGet, p, "")
		assert.Equal(t, http.StatusForbidden, w.Code, p)
		assert.Contains(t, w.Body.String(), ScopeEPPPolicyAdmin)
	}
}

func TestEPPPoliciesAPI(t *testing.T) {
	r, repo := eppPoliciesRouter(t, ScopeEPPPolicyAdmin)
	limits := `{"limits":{"sessions_per_registrar":20,"sessions_per_login":5,"sessions_per_family":60,"check_names":25}}`

	w := do(r, http.MethodPut, "/epp/policies/gold", limits)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var p entities.EPPPolicy
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &p))
	assert.Equal(t, 25, p.Limits.CheckNames)
	assert.Equal(t, "ops@example.com", p.UpdatedBy)

	w = do(r, http.MethodGet, "/epp/policies", "")
	require.Equal(t, http.StatusOK, w.Code)
	var ps []entities.EPPPolicy
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &ps))
	assert.Len(t, ps, 3, "default, gold, probation")

	assert.Equal(t, http.StatusBadRequest, do(r, http.MethodPut, "/epp/policies/silver", `{"limits":{"check_names":0}}`).Code)
	assert.Equal(t, http.StatusBadRequest, do(r, http.MethodPut, "/epp/policies/silver", `not json`).Code)

	w = do(r, http.MethodPut, "/registrars/rar-a/epp/policy", `{"policy":"gold"}`)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Equal(t, http.StatusNotFound, do(r, http.MethodPut, "/registrars/nobody/epp/policy", `{"policy":"gold"}`).Code)
	assert.Equal(t, http.StatusNotFound, do(r, http.MethodPut, "/registrars/rar-a/epp/policy", `{"policy":"platinum"}`).Code)

	w = do(r, http.MethodPut, "/epp/families/acme", `{"policy":"gold","members":["rar-a","rar-b"]}`)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Equal(t, http.StatusConflict, do(r, http.MethodPut, "/epp/families/other", `{"members":["rar-b"]}`).Code)

	w = do(r, http.MethodGet, "/registrars/rar-b/epp/policy", "")
	require.Equal(t, http.StatusOK, w.Code)
	var rp entities.EPPRegistrarPolicy
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &rp))
	assert.Equal(t, "default", rp.Policy.Name)
	assert.Equal(t, "acme", rp.Family.Name)
	assert.Equal(t, "gold", rp.FamilyPolicy.Name)

	assert.Equal(t, http.StatusConflict, do(r, http.MethodDelete, "/epp/policies/default", "").Code)
	assert.Equal(t, http.StatusConflict, do(r, http.MethodDelete, "/epp/policies/gold", "").Code, "in use")
	assert.Equal(t, http.StatusNoContent, do(r, http.MethodDelete, "/epp/families/acme", "").Code)
	assert.Equal(t, http.StatusNotFound, do(r, http.MethodGet, "/epp/families/acme", "").Code)
	assert.NotEmpty(t, repo.Events)
}
