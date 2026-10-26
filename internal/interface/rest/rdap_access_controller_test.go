package rest

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/netip"
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

type fakeRDAPAccess struct {
	mu       sync.Mutex
	policies map[string]*entities.RDAPAccessPolicy
	networks map[netip.Prefix]string
	events   []entities.DomainEvent
}

func (f *fakeRDAPAccess) ListPolicies(context.Context) ([]*entities.RDAPAccessPolicy, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []*entities.RDAPAccessPolicy
	for _, p := range f.policies {
		out = append(out, p)
	}
	return out, nil
}

func (f *fakeRDAPAccess) GetPolicy(_ context.Context, name string) (*entities.RDAPAccessPolicy, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if p, ok := f.policies[name]; ok {
		return p, nil
	}
	return nil, fmt.Errorf("%w: %s", entities.ErrRDAPAccessPolicyNotFound, name)
}

func (f *fakeRDAPAccess) SavePolicy(_ context.Context, p *entities.RDAPAccessPolicy, events func(*entities.RDAPAccessPolicy) []entities.DomainEvent) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.events = append(f.events, events(f.policies[p.Name])...)
	f.policies[p.Name] = p
	return nil
}

func (f *fakeRDAPAccess) DeletePolicy(_ context.Context, name string, events func(*entities.RDAPAccessPolicy) []entities.DomainEvent) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, p := range f.networks {
		if p == name {
			return entities.ErrRDAPAccessPolicyInUse
		}
	}
	before, ok := f.policies[name]
	if !ok {
		return entities.ErrRDAPAccessPolicyNotFound
	}
	delete(f.policies, name)
	f.events = append(f.events, events(before)...)
	return nil
}

func (f *fakeRDAPAccess) ListNetworks(context.Context) ([]entities.RDAPAccessNetwork, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []entities.RDAPAccessNetwork
	for n, p := range f.networks {
		out = append(out, entities.RDAPAccessNetwork{Network: n, Policy: p})
	}
	return out, nil
}

func (f *fakeRDAPAccess) AssignNetwork(_ context.Context, n netip.Prefix, policy string, events func(string) []entities.DomainEvent) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.policies[policy]; policy != "" && !ok {
		return entities.ErrRDAPAccessPolicyNotFound
	}
	prev := f.networks[n]
	if policy == "" {
		delete(f.networks, n)
	} else {
		f.networks[n] = policy
	}
	f.events = append(f.events, events(prev)...)
	return nil
}

var _ repositories.RDAPAccessPolicyRepository = (*fakeRDAPAccess)(nil)

func rdapAccessRouter(t *testing.T, scopes ...string) (*gin.Engine, *fakeRDAPAccess) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	repo := &fakeRDAPAccess{policies: map[string]*entities.RDAPAccessPolicy{}, networks: map[netip.Prefix]string{}}
	r := gin.New()
	grant := func(c *gin.Context) {
		ctx := appcontext.WithAuthScopes(c.Request.Context(), scopes)
		c.Request = c.Request.WithContext(appcontext.WithUserID(ctx, "ops@example.com"))
		c.Next()
	}
	NewRDAPAccessController(r, services.NewRDAPAccessPolicyService(repo), grant)
	return r, repo
}

func TestRDAPAccessAPI(t *testing.T) {
	forbidden, _ := rdapAccessRouter(t, ScopeEPPPolicyAdmin)
	assert.Equal(t, http.StatusForbidden, do(forbidden, http.MethodGet, "/rdap/access-policies", "").Code)

	r, repo := rdapAccessRouter(t, ScopeRDAPPolicyAdmin)

	t.Run("the built-ins are listed", func(t *testing.T) {
		w := do(r, http.MethodGet, "/rdap/access-policies", "")
		require.Equal(t, http.StatusOK, w.Code)
		var ps []entities.RDAPAccessPolicy
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &ps))
		require.Len(t, ps, 2)
		assert.Equal(t, entities.AnonymousRDAPAccessLimits(), ps[0].Limits)
	})

	t.Run("an invalid policy is 400", func(t *testing.T) {
		w := do(r, http.MethodPut, "/rdap/access-policies/bad", `{"limits":{"perAddress":{"per_second":0,"burst":0},"notFoundCost":1}}`)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("save, assign a network, refuse deleting it while assigned", func(t *testing.T) {
		w := do(r, http.MethodPut, "/rdap/access-policies/partner", `{"limits":{"perAddress":{"per_second":10,"burst":40},"notFoundCost":1}}`)
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		w = do(r, http.MethodPut, "/rdap/access-networks", `{"network":"192.0.2.9/24","policy":"partner"}`)
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		assert.JSONEq(t, `{"network":"192.0.2.0/24","policy":"partner"}`, w.Body.String(), "stored masked")
		assert.Equal(t, http.StatusConflict, do(r, http.MethodDelete, "/rdap/access-policies/partner", "").Code)
		assert.Equal(t, http.StatusBadRequest, do(r, http.MethodPut, "/rdap/access-networks", `{"network":"nope","policy":"partner"}`).Code)
	})

	t.Run("assigning a built-in stores it first", func(t *testing.T) {
		w := do(r, http.MethodPut, "/rdap/access-networks", `{"network":"198.51.100.7","policy":"monitoring"}`)
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		_, stored := repo.policies[entities.RDAPAccessMonitoring]
		assert.True(t, stored)
	})

	t.Run("built-ins cannot be deleted", func(t *testing.T) {
		assert.Equal(t, http.StatusConflict, do(r, http.MethodDelete, "/rdap/access-policies/anonymous", "").Code)
	})

	t.Run("every change has its event", func(t *testing.T) {
		types := map[string]int{}
		for _, e := range repo.events {
			types[e.Type]++
			assert.Equal(t, "ops@example.com", e.Actor)
		}
		assert.Positive(t, types[entities.EventRDAPAccessPolicySaved])
		assert.Positive(t, types[entities.EventRDAPAccessNetworkAssigned])
	})
}
