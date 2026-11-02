package rest

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

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
	outcomes := memstore.NewEPPCreateOutcomes()
	_ = outcomes.AddCreateOutcomes(context.Background(), []entities.EPPCreateOutcomes{
		{ClID: "rar-a", Day: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), Succeeded: 3, Failed: 140},
		{ClID: "rar-a", Day: time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC), Failed: 9},
	})
	r := gin.New()
	grant := func(c *gin.Context) {
		ctx := appcontext.WithAuthScopes(c.Request.Context(), scopes)
		c.Request = c.Request.WithContext(appcontext.WithUserID(ctx, "ops@example.com"))
		c.Next()
	}
	NewEPPPoliciesController(r, services.NewEPPPolicyService(repo, policyRegistrars{}, outcomes), grant)
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
	for _, p := range []string{"/epp/policies", "/epp/families", "/registrars/rar-a/epp/policy", "/registrars/rar-a/epp/create-outcomes"} {
		w := do(r, http.MethodGet, p, "")
		assert.Equal(t, http.StatusForbidden, w.Code, p)
		assert.Contains(t, w.Body.String(), ScopeEPPPolicyAdmin)
	}
}

func TestEPPPoliciesAPI(t *testing.T) {
	r, repo := eppPoliciesRouter(t, ScopeEPPPolicyAdmin)
	limits := `{"limits":{"sessions_per_registrar":20,"sessions_per_login":5,"sessions_per_family":60,"check_names":25,` +
		`"rates":{"check":{"per_second":250,"burst":1000},"info":{"per_second":20,"burst":100},"transform":{"per_second":10,"burst":50},"poll":{"per_second":5,"burst":20}},` +
		`"family_rates":{"check":{"per_second":750,"burst":3000},"info":{"per_second":60,"burst":300},"transform":{"per_second":30,"burst":150},"poll":{"per_second":15,"burst":60}},` +
		`"creates_per_day":100,"failed_creates":{"failures":100,"ratio_percent":90},"family_failed_creates":{"failures":300,"ratio_percent":90}}}`

	w := do(r, http.MethodPut, "/epp/policies/gold", limits)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var p entities.EPPPolicy
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &p))
	assert.Equal(t, 25, p.Limits.CheckNames)
	assert.Equal(t, entities.EPPRate{PerSecond: 10, Burst: 50}, p.Limits.Rates.Transform)
	assert.Equal(t, entities.EPPRate{PerSecond: 750, Burst: 3000}, p.Limits.FamilyRates.Check)
	assert.Equal(t, 100, p.Limits.CreatesPerDay)
	assert.Equal(t, "ops@example.com", p.UpdatedBy)

	w = do(r, http.MethodGet, "/epp/policies", "")
	require.Equal(t, http.StatusOK, w.Code)
	var ps []entities.EPPPolicy
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &ps))
	assert.Len(t, ps, 3, "default, gold, probation")

	assert.Equal(t, http.StatusBadRequest, do(r, http.MethodPut, "/epp/policies/silver", `{"limits":{"check_names":0}}`).Code)
	assert.Equal(t, http.StatusBadRequest, do(r, http.MethodPut, "/epp/policies/silver", `not json`).Code)
	w = do(r, http.MethodPut, "/epp/policies/silver", strings.Replace(limits, `"burst":1000`, `"burst":10`, 1))
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "rates.check.burst must be at least check_names (25)")

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

// PUT with If-None-Match: * is create-only: it never replaces what is there.
func TestEPPPoliciesCreateOnly(t *testing.T) {
	r, _ := eppPoliciesRouter(t, ScopeEPPPolicyAdmin)
	limits := `{"description":"first","limits":{"sessions_per_registrar":20,"sessions_per_login":5,"sessions_per_family":60,"check_names":25,` +
		`"rates":{"check":{"per_second":250,"burst":1000},"info":{"per_second":20,"burst":100},"transform":{"per_second":10,"burst":50},"poll":{"per_second":5,"burst":20}},` +
		`"family_rates":{"check":{"per_second":750,"burst":3000},"info":{"per_second":60,"burst":300},"transform":{"per_second":30,"burst":150},"poll":{"per_second":15,"burst":60}},` +
		`"creates_per_day":100,"failed_creates":{"failures":100,"ratio_percent":90},"family_failed_creates":{"failures":300,"ratio_percent":90}}}`
	createOnly := func(method, path, body string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("If-None-Match", "*")
		r.ServeHTTP(w, req)
		return w
	}

	require.Equal(t, http.StatusOK, createOnly(http.MethodPut, "/epp/policies/gold", limits).Code, "a new name is created")
	w := createOnly(http.MethodPut, "/epp/policies/gold", strings.Replace(limits, "first", "second", 1))
	assert.Equal(t, http.StatusPreconditionFailed, w.Code, "an existing name fails the precondition (RFC 9110)")
	assert.Contains(t, w.Body.String(), "already exists")
	w = createOnly(http.MethodPut, "/epp/policies/default", limits)
	assert.Equal(t, http.StatusPreconditionFailed, w.Code, "a built-in is never replaced by a create")
	w = do(r, http.MethodGet, "/epp/policies/gold", "")
	assert.Contains(t, w.Body.String(), `"description":"first"`, "the refused save changed nothing")
	assert.Equal(t, http.StatusOK, do(r, http.MethodPut, "/epp/policies/gold", strings.Replace(limits, "first", "second", 1)).Code, "a plain PUT still replaces")

	require.Equal(t, http.StatusOK, createOnly(http.MethodPut, "/epp/families/acme", `{"members":["rar-a"]}`).Code)
	w = createOnly(http.MethodPut, "/epp/families/acme", `{"members":[]}`)
	assert.Equal(t, http.StatusPreconditionFailed, w.Code, "an existing family keeps its members")
	w = do(r, http.MethodGet, "/epp/families/acme", "")
	assert.Contains(t, w.Body.String(), `"rar-a"`)
}

// PUT with If-Match: "<updated_at>" is refused once the stored version differs.
func TestEPPPoliciesIfMatch(t *testing.T) {
	r, _ := eppPoliciesRouter(t, ScopeEPPPolicyAdmin)
	limits := `{"description":"v1","limits":{"sessions_per_registrar":20,"sessions_per_login":5,"sessions_per_family":60,"check_names":25,` +
		`"rates":{"check":{"per_second":250,"burst":1000},"info":{"per_second":20,"burst":100},"transform":{"per_second":10,"burst":50},"poll":{"per_second":5,"burst":20}},` +
		`"family_rates":{"check":{"per_second":750,"burst":3000},"info":{"per_second":60,"burst":300},"transform":{"per_second":30,"burst":150},"poll":{"per_second":15,"burst":60}},` +
		`"creates_per_day":100,"failed_creates":{"failures":100,"ratio_percent":90},"family_failed_creates":{"failures":300,"ratio_percent":90}}}`
	withMatch := func(path, body, etag string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPut, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("If-Match", etag)
		r.ServeHTTP(w, req)
		return w
	}
	w := do(r, http.MethodPut, "/epp/policies/gold", limits)
	require.Equal(t, http.StatusOK, w.Code)
	var p entities.EPPPolicy
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &p))
	v1 := p.UpdatedAt.Format(time.RFC3339Nano)

	require.Equal(t, http.StatusOK, withMatch("/epp/policies/gold", strings.Replace(limits, "v1", "v2", 1), `"`+v1+`"`).Code, "the version read is the one stored")
	w = withMatch("/epp/policies/gold", strings.Replace(limits, "v1", "v3", 1), v1)
	assert.Equal(t, http.StatusPreconditionFailed, w.Code, "saved since: refused")
	assert.Contains(t, w.Body.String(), "changed since it was read")
	w = do(r, http.MethodGet, "/epp/policies/gold", "")
	assert.Contains(t, w.Body.String(), `"description":"v2"`, "the refused save changed nothing")
	assert.Equal(t, http.StatusBadRequest, withMatch("/epp/policies/gold", limits, "yesterday").Code)

	// A built-in not stored yet reads with a zero updated_at; its first
	// conditional save passes with that, and the next one is refused.
	w = do(r, http.MethodGet, "/epp/policies/probation", "")
	require.Equal(t, http.StatusOK, w.Code)
	var pb entities.EPPPolicy
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &pb))
	require.True(t, pb.UpdatedAt.IsZero(), "unstored built-in")
	zero := pb.UpdatedAt.Format(time.RFC3339Nano)
	require.Equal(t, http.StatusOK, withMatch("/epp/policies/probation", limits, `"`+zero+`"`).Code, "the first edit of an unstored built-in")
	assert.Equal(t, http.StatusPreconditionFailed, withMatch("/epp/policies/probation", limits, `"`+zero+`"`).Code, "a concurrent first edit")
	assert.Equal(t, http.StatusPreconditionFailed, withMatch("/epp/policies/nothere", limits, `"`+zero+`"`).Code, "a custom policy that does not exist")

	require.Equal(t, http.StatusOK, do(r, http.MethodPut, "/epp/families/acme", `{"members":["rar-a"]}`).Code)
	assert.Equal(t, http.StatusPreconditionFailed, withMatch("/epp/families/acme", `{"members":[]}`, `"2000-01-01T00:00:00Z"`).Code)
	w = do(r, http.MethodGet, "/epp/families/acme", "")
	assert.Contains(t, w.Body.String(), `"rar-a"`, "members kept")
}

func TestEPPCreateOutcomesAPI(t *testing.T) {
	r, _ := eppPoliciesRouter(t, ScopeEPPPolicyAdmin)
	w := do(r, http.MethodGet, "/registrars/rar-a/epp/create-outcomes?from=2026-01-01&to=2026-01-31", "")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var got []entities.EPPCreateOutcomes
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	require.Len(t, got, 2)
	assert.Equal(t, int64(140), got[0].Failed)

	w = do(r, http.MethodGet, "/registrars/rar-a/epp/create-outcomes?from=2026-01-02&to=2026-01-02", "")
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	assert.Len(t, got, 1)
	w = do(r, http.MethodGet, "/registrars/rar-a/epp/create-outcomes", "")
	assert.Equal(t, "[]", w.Body.String(), "the last 30 days: none")

	assert.Equal(t, http.StatusBadRequest, do(r, http.MethodGet, "/registrars/rar-a/epp/create-outcomes?from=yesterday", "").Code)
	w = do(r, http.MethodGet, "/registrars/rar-a/epp/create-outcomes?from=2026-10-02&to=2026-10-01", "")
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "invalid range of days")
	assert.Equal(t, http.StatusBadRequest, do(r, http.MethodGet, "/registrars/rar-a/epp/create-outcomes?from=2024-01-01&to=2026-01-01", "").Code)
	assert.Equal(t, http.StatusNotFound, do(r, http.MethodGet, "/registrars/nobody/epp/create-outcomes", "").Code)
}
