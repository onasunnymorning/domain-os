package rest

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onasunnymorning/domain-os/internal/appcontext"
	"github.com/onasunnymorning/domain-os/internal/application/services"
	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
	"github.com/onasunnymorning/domain-os/pkg/domain/repositories"
)

// fakeRDAPPublication is an in-memory RDAPPublicationPolicyRepository that
// keeps the events each change wrote.
type fakeRDAPPublication struct {
	mu          sync.Mutex
	policies    map[string]*entities.RDAPPublicationPolicy
	assignments map[entities.DomainName]string
	events      []entities.DomainEvent
}

func (f *fakeRDAPPublication) ListPolicies(context.Context) ([]*entities.RDAPPublicationPolicy, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []*entities.RDAPPublicationPolicy
	for _, p := range f.policies {
		out = append(out, p)
	}
	return out, nil
}

func (f *fakeRDAPPublication) GetPolicy(_ context.Context, name string) (*entities.RDAPPublicationPolicy, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if p, ok := f.policies[name]; ok {
		return p, nil
	}
	return nil, fmt.Errorf("%w: %s", entities.ErrRDAPPublicationPolicyNotFound, name)
}

func (f *fakeRDAPPublication) SavePolicy(_ context.Context, p *entities.RDAPPublicationPolicy, events func(*entities.RDAPPublicationPolicy) []entities.DomainEvent) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.events = append(f.events, events(f.policies[p.Name])...)
	f.policies[p.Name] = p
	return nil
}

func (f *fakeRDAPPublication) DeletePolicy(_ context.Context, name string, events func(*entities.RDAPPublicationPolicy) []entities.DomainEvent) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	before, ok := f.policies[name]
	if !ok {
		return entities.ErrRDAPPublicationPolicyNotFound
	}
	for _, p := range f.assignments {
		if p == name {
			return entities.ErrRDAPPublicationPolicyInUse
		}
	}
	delete(f.policies, name)
	f.events = append(f.events, events(before)...)
	return nil
}

func (f *fakeRDAPPublication) Snapshot(ctx context.Context) ([]*entities.RDAPPublicationPolicy, map[entities.DomainName]string, error) {
	p, _ := f.ListPolicies(ctx)
	a, _ := f.ListAssignments(ctx)
	return p, a, nil
}

func (f *fakeRDAPPublication) ListAssignments(context.Context) (map[entities.DomainName]string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := map[entities.DomainName]string{}
	for k, v := range f.assignments {
		out[k] = v
	}
	return out, nil
}

func (f *fakeRDAPPublication) AssignPolicy(_ context.Context, tld entities.DomainName, policy string, events func(string) []entities.DomainEvent) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.policies[policy]; policy != "" && !ok {
		return entities.ErrRDAPPublicationPolicyNotFound
	}
	prev := f.assignments[tld]
	if policy == "" {
		delete(f.assignments, tld)
	} else {
		f.assignments[tld] = policy
	}
	f.events = append(f.events, events(prev)...)
	return nil
}

var _ repositories.RDAPPublicationPolicyRepository = (*fakeRDAPPublication)(nil)

type publicationTLDs struct{ repositories.TLDRepository }

func (publicationTLDs) GetByName(_ context.Context, name string, _ bool) (*entities.TLD, error) {
	switch name {
	case "gtld":
		return &entities.TLD{Name: "gtld", Type: entities.TLDTypeGTLD}, nil
	case "cc":
		return &entities.TLD{Name: "cc", Type: entities.TLDTypeCCTLD}, nil
	}
	return nil, entities.ErrTLDNotFound
}

func rdapPublicationRouter(t *testing.T, scopes ...string) (*gin.Engine, *fakeRDAPPublication) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	repo := &fakeRDAPPublication{policies: map[string]*entities.RDAPPublicationPolicy{}, assignments: map[entities.DomainName]string{}}
	r := gin.New()
	grant := func(c *gin.Context) {
		ctx := appcontext.WithAuthScopes(c.Request.Context(), scopes)
		c.Request = c.Request.WithContext(appcontext.WithUserID(ctx, "ops@example.com"))
		c.Next()
	}
	NewRDAPPublicationController(r, services.NewRDAPPublicationPolicyService(repo, publicationTLDs{}), grant)
	return r, repo
}

func TestRDAPPublicationRequiresPermission(t *testing.T) {
	r, _ := rdapPublicationRouter(t, ScopeEPPPolicyAdmin)
	assert.Equal(t, http.StatusForbidden, do(r, http.MethodGet, "/rdap/publication-policies", "").Code)
	assert.Equal(t, http.StatusForbidden, do(r, http.MethodGet, "/tlds/gtld/rdap/publication-policy", "").Code)
}

// thickPolicy publishes the registrant's name and withholds everything else.
const thickPolicy = `{"description":"thick","contactFormUri":"https://registry.example/contact","roles":{"registrant":{
	"handle":{"rule":"withhold","method":"removal"},"name":{"rule":"publish"},"org":{"rule":"publish-if-disclosed"},
	"street":{"rule":"withhold","method":"emptyValue"},"city":{"rule":"withhold","method":"emptyValue"},"sp":{"rule":"publish"},
	"pc":{"rule":"withhold","method":"emptyValue"},"cc":{"rule":"publish"},"voice":{"rule":"withhold","method":"emptyValue"},
	"fax":{"rule":"withhold","method":"removal"},"email":{"rule":"withhold","method":"replacementValue"}}}}`

func TestRDAPPublicationAPI(t *testing.T) {
	r, repo := rdapPublicationRouter(t, ScopeRDAPPolicyAdmin)

	t.Run("the built-ins are listed before any is stored", func(t *testing.T) {
		w := do(r, http.MethodGet, "/rdap/publication-policies", "")
		require.Equal(t, http.StatusOK, w.Code)
		var ps []entities.RDAPPublicationPolicy
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &ps))
		require.Len(t, ps, 2)
		assert.Equal(t, entities.RDAPPolicyCCTLDMinimal, ps[0].Name)
	})

	t.Run("a TLD with no assignment has the built-in for its type", func(t *testing.T) {
		w := do(r, http.MethodGet, "/tlds/cc/rdap/publication-policy", "")
		require.Equal(t, http.StatusOK, w.Code)
		assert.JSONEq(t, `{"tld":"cc","policy":"cctld-minimal","assigned":false}`, w.Body.String())
		assert.Equal(t, http.StatusNotFound, do(r, http.MethodGet, "/tlds/nope/rdap/publication-policy", "").Code)
	})

	t.Run("save a policy, with its event", func(t *testing.T) {
		w := do(r, http.MethodPut, "/rdap/publication-policies/thick", thickPolicy)
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		require.Len(t, repo.events, 1)
		assert.Equal(t, entities.EventRDAPPublicationPolicySaved, repo.events[0].Type)
		assert.Equal(t, "ops@example.com", repo.events[0].Actor)
	})

	t.Run("an invalid policy is 400", func(t *testing.T) {
		w := do(r, http.MethodPut, "/rdap/publication-policies/broken", `{"roles":{"registrant":{"name":{"rule":"publish"}}}}`)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("assign, and a policy in use cannot be deleted", func(t *testing.T) {
		w := do(r, http.MethodPut, "/tlds/gtld/rdap/publication-policy", `{"policy":"thick"}`)
		require.Equal(t, http.StatusOK, w.Code)
		assert.JSONEq(t, `{"tld":"gtld","policy":"thick","assigned":true}`, w.Body.String())
		assert.Equal(t, http.StatusConflict, do(r, http.MethodDelete, "/rdap/publication-policies/thick", "").Code)
		assert.Equal(t, http.StatusNotFound, do(r, http.MethodPut, "/tlds/gtld/rdap/publication-policy", `{"policy":"no-such"}`).Code)
	})

	t.Run("assigning a built-in stores it first", func(t *testing.T) {
		w := do(r, http.MethodPut, "/tlds/cc/rdap/publication-policy", `{"policy":"gtld-rdp-2024"}`)
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		_, stored := repo.policies[entities.RDAPPolicyGTLDRDP2024]
		assert.True(t, stored)
	})

	t.Run("built-ins cannot be deleted", func(t *testing.T) {
		assert.Equal(t, http.StatusConflict, do(r, http.MethodDelete, "/rdap/publication-policies/cctld-minimal", "").Code)
	})

	t.Run("an assignment without a policy member is refused, not read as removal", func(t *testing.T) {
		require.Equal(t, http.StatusOK, do(r, http.MethodPut, "/tlds/gtld/rdap/publication-policy", `{"policy":"thick"}`).Code)
		for _, body := range []string{`{}`, `{"polciy":"thick"}`, `{"policy":null}`, `null`} {
			assert.Equal(t, http.StatusBadRequest, do(r, http.MethodPut, "/tlds/gtld/rdap/publication-policy", body).Code, body)
		}
		assert.Equal(t, "thick", repo.assignments["gtld"], "the assignment is untouched")
	})

	t.Run("unassign, then delete", func(t *testing.T) {
		require.Equal(t, http.StatusOK, do(r, http.MethodPut, "/tlds/gtld/rdap/publication-policy", `{"policy":""}`).Code)
		assert.Equal(t, http.StatusNoContent, do(r, http.MethodDelete, "/rdap/publication-policies/thick", "").Code)
	})
}

func TestRDAPPublicationStaticAdminToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &fakeRDAPPublication{policies: map[string]*entities.RDAPPublicationPolicy{}, assignments: map[entities.DomainName]string{}}
	r := gin.New()
	// The admin API's chain (cmd/api/ry-admin): authenticate, then copy the
	// grants into the request's context.
	auth := Auth0Middleware("", "", "dev-token", false)
	chain := func(c *gin.Context) {
		auth(c)
		if !c.IsAborted() {
			ContextPropagationMiddleware()(c)
		}
		c.Next()
	}
	NewRDAPPublicationController(r, services.NewRDAPPublicationPolicyService(repo, publicationTLDs{}), chain)
	req := httptest.NewRequest(http.MethodGet, "/rdap/publication-policies", nil)
	req.Header.Set("Authorization", "Bearer dev-token")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code, "with Auth0 off, the static admin token administers RDAP policies")
}
