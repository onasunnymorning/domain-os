package postgres

import (
	"context"
	"net/netip"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
)

// TestRDAPAccessPolicyRepository runs the repository against the migrated
// test database, inside a transaction that is rolled back.
func TestRDAPAccessPolicyRepository(t *testing.T) {
	tx := getTestDB().Begin()
	defer tx.Rollback()
	repo := NewRDAPAccessPolicyRepository(tx)
	ctx := context.Background()
	none := func(string) []entities.DomainEvent { return nil }

	t.Run("the migration seeded the built-in policies", func(t *testing.T) {
		require.NoError(t, seedRDAPAccessPolicies(tx))
		p, err := repo.GetPolicy(ctx, entities.RDAPAccessAnonymous)
		require.NoError(t, err)
		assert.Equal(t, entities.AnonymousRDAPAccessLimits(), p.Limits)
	})

	partner := &entities.RDAPAccessPolicy{Name: "partner-test", UpdatedAt: time.Now().UTC().Truncate(time.Microsecond), UpdatedBy: "ops",
		Limits: entities.RDAPAccessLimits{PerAddress: entities.EPPRate{PerSecond: 10, Burst: 40}, NotFoundCost: 2}}

	t.Run("save and read back, with no aggregate limit", func(t *testing.T) {
		var before *entities.RDAPAccessPolicy
		require.NoError(t, repo.SavePolicy(ctx, partner, func(b *entities.RDAPAccessPolicy) []entities.DomainEvent { before = b; return nil }))
		assert.Nil(t, before)
		got, err := repo.GetPolicy(ctx, "partner-test")
		require.NoError(t, err)
		assert.Equal(t, partner, got)
	})

	network := netip.MustParsePrefix("192.0.2.0/24")
	t.Run("assign a network; the policy cannot be deleted", func(t *testing.T) {
		require.NoError(t, repo.AssignNetwork(ctx, network, "partner-test", none))
		nets, err := repo.ListNetworks(ctx)
		require.NoError(t, err)
		assert.Contains(t, nets, entities.RDAPAccessNetwork{Network: network, Policy: "partner-test"})
		policies, snapNets, err := repo.Snapshot(ctx)
		require.NoError(t, err)
		assert.Contains(t, snapNets, entities.RDAPAccessNetwork{Network: network, Policy: "partner-test"})
		var names []string
		for _, p := range policies {
			names = append(names, p.Name)
		}
		assert.Contains(t, names, "partner-test", "the snapshot holds every assigned network's policy")
		assert.ErrorIs(t, repo.DeletePolicy(ctx, "partner-test", func(*entities.RDAPAccessPolicy) []entities.DomainEvent { return nil }), entities.ErrRDAPAccessPolicyInUse)
		assert.ErrorIs(t, repo.AssignNetwork(ctx, network, "no-such", none), entities.ErrRDAPAccessPolicyNotFound)
	})

	t.Run("unassign, then delete", func(t *testing.T) {
		var prev string
		require.NoError(t, repo.AssignNetwork(ctx, network, "", func(p string) []entities.DomainEvent { prev = p; return nil }))
		assert.Equal(t, "partner-test", prev)
		require.NoError(t, repo.DeletePolicy(ctx, "partner-test", func(*entities.RDAPAccessPolicy) []entities.DomainEvent { return nil }))
	})
}
