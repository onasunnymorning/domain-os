package services

import (
	"context"
	"errors"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onasunnymorning/domain-os/internal/application/commands"
	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
	"github.com/onasunnymorning/domain-os/pkg/domain/repositories"
)

// rdapAccessRepo answers Snapshot with what it holds, and GetPolicy from
// policies until gets calls have been made, then not found: a policy
// deleted while a request is handled.
type rdapAccessRepo struct {
	repositories.RDAPAccessPolicyRepository
	policies []*entities.RDAPAccessPolicy
	networks []entities.RDAPAccessNetwork
	gets     int
	calls    int
	err      error // returned by Snapshot when set
}

func (f *rdapAccessRepo) Snapshot(context.Context) ([]*entities.RDAPAccessPolicy, []entities.RDAPAccessNetwork, error) {
	if f.err != nil {
		return nil, nil, f.err
	}
	return f.policies, f.networks, nil
}

func (f *rdapAccessRepo) GetPolicy(_ context.Context, name string) (*entities.RDAPAccessPolicy, error) {
	f.calls++
	if f.calls <= f.gets {
		for _, p := range f.policies {
			if p.Name == name {
				return p, nil
			}
		}
	}
	return nil, entities.ErrRDAPAccessPolicyNotFound
}

func TestRDAPAccessSetRefusesAnUnresolvedNetwork(t *testing.T) {
	repo := &rdapAccessRepo{networks: []entities.RDAPAccessNetwork{{Network: netip.MustParsePrefix("192.0.2.0/24"), Policy: "created-later"}}}
	_, err := NewRDAPAccessPolicyService(repo).AccessSet(context.Background())
	assert.ErrorIs(t, err, entities.ErrRDAPAccessPolicyNotFound)
	assert.ErrorContains(t, err, "192.0.2.0/24=created-later")
}

func TestRDAPAccessAssignToAPolicyDeletedMeanwhile(t *testing.T) {
	partner := &entities.RDAPAccessPolicy{Name: "partner", Limits: entities.AnonymousRDAPAccessLimits()}
	repo := &rdapAccessRepo{policies: []*entities.RDAPAccessPolicy{partner}, gets: 1}
	_, err := NewRDAPAccessPolicyService(repo).AssignNetwork(context.Background(),
		commands.AssignRDAPAccessNetworkCommand{Network: "192.0.2.0/24", Policy: "partner"})
	assert.ErrorIs(t, err, entities.ErrRDAPAccessPolicyNotFound, "not found, not a panic")
}

func TestRDAPServiceRefreshesEachPolicyKindOnItsOwn(t *testing.T) {
	probe := netip.MustParseAddr("192.0.2.7")
	access := &rdapAccessRepo{networks: []entities.RDAPAccessNetwork{{Network: netip.MustParsePrefix("192.0.2.0/24"), Policy: entities.RDAPAccessMonitoring}}}
	strict := entities.BuiltInRDAPPublicationPolicies()[0]
	strict.Name = "strict"
	publication := &rdapSnapshots{policies: []*entities.RDAPPublicationPolicy{strict}, assignments: map[entities.DomainName]string{"example": "strict"}}
	svc := NewRDAPService(RDAPServiceDeps{
		Domains:     rdapDomains{d: &entities.Domain{Name: "a.example", TLDName: "example", ClID: "rar"}},
		Registrars:  rdapRegistrars{},
		TLDs:        rdapTLDs{t: rdapTLD(entities.TLDTypeGTLD, entities.NewContactDataPolicy())},
		Contacts:    rdapContacts{read: new([]string)},
		Publication: NewRDAPPublicationPolicyService(publication, nil),
		Access:      NewRDAPAccessPolicyService(access),
	})
	publicationName := func() string {
		t.Helper()
		rec, err := svc.Domain(context.Background(), "a.example")
		require.NoError(t, err)
		return rec.Publication.Name
	}

	assert.Equal(t, entities.RDAPAccessAnonymous, svc.AccessPolicy(probe).Name, "before any load")
	require.NoError(t, svc.RefreshPolicies(context.Background()))
	assert.Equal(t, entities.RDAPAccessMonitoring, svc.AccessPolicy(probe).Name, "the loaded network")
	assert.Equal(t, "strict", publicationName())

	t.Run("a failed access load keeps the access set, and still replaces publication", func(t *testing.T) {
		access.err = errors.New("database unavailable")
		publication.assignments = map[entities.DomainName]string{} // unassigned: back to the built-in
		assert.Error(t, svc.RefreshPolicies(context.Background()))
		assert.Equal(t, entities.RDAPAccessMonitoring, svc.AccessPolicy(probe).Name)
		assert.Equal(t, entities.RDAPPolicyGTLDRDP2024, publicationName())
	})

	t.Run("a failed publication load keeps publication, and still replaces access", func(t *testing.T) {
		access.err = nil
		access.networks = nil
		publication.assignments = map[entities.DomainName]string{"example": "strict"}
		require.NoError(t, svc.RefreshPolicies(context.Background()))
		publication.err = errors.New("database unavailable")
		access.networks = []entities.RDAPAccessNetwork{{Network: netip.MustParsePrefix("192.0.2.0/24"), Policy: entities.RDAPAccessMonitoring}}
		assert.Error(t, svc.RefreshPolicies(context.Background()))
		assert.Equal(t, "strict", publicationName())
		assert.Equal(t, entities.RDAPAccessMonitoring, svc.AccessPolicy(probe).Name)
	})

	t.Run("an access snapshot with an unresolved network keeps the access set", func(t *testing.T) {
		publication.err = nil
		access.networks = []entities.RDAPAccessNetwork{{Network: netip.MustParsePrefix("192.0.2.0/24"), Policy: "created-later"}}
		assert.ErrorIs(t, svc.RefreshPolicies(context.Background()), entities.ErrRDAPAccessPolicyNotFound)
		assert.Equal(t, entities.RDAPAccessMonitoring, svc.AccessPolicy(probe).Name)
	})
}
