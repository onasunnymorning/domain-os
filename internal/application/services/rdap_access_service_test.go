package services

import (
	"context"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"

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
}

func (f *rdapAccessRepo) Snapshot(context.Context) ([]*entities.RDAPAccessPolicy, []entities.RDAPAccessNetwork, error) {
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
