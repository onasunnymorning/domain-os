package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
)

// TestEPPPolicyRepository runs the policy repository against the migrated
// test database, inside a transaction that is rolled back.
func TestEPPPolicyRepository(t *testing.T) {
	tx := getTestDB().Begin()
	defer tx.Rollback()
	repo := NewEPPPolicyRepository(tx)
	ctx := context.Background()
	event := func(typ string) []entities.DomainEvent {
		return []entities.DomainEvent{entities.NewDomainEvent("test", typ, "subject", "msg", map[string]string{"k": "v"})}
	}
	countEvents := func(typ string) int64 {
		var n int64
		require.NoError(t, tx.Table("domain_events").Where("type = ?", typ).Count(&n).Error)
		return n
	}

	t.Run("the migration seeded the built-in policies", func(t *testing.T) {
		require.NoError(t, seedEPPPolicies(tx), "seeding again keeps what is there")
		p, err := repo.GetPolicy(ctx, entities.EPPPolicyDefault)
		require.NoError(t, err)
		assert.Equal(t, entities.DefaultEPPPolicyLimits(), p.Limits)
		_, err = repo.GetPolicy(ctx, entities.EPPPolicyProbation)
		require.NoError(t, err)
	})

	gold := &entities.EPPPolicy{Name: "gold-test", Description: "test", UpdatedAt: time.Now().UTC().Truncate(time.Microsecond), UpdatedBy: "ops",
		Limits: entities.EPPPolicyLimits{SessionsPerRegistrar: 20, SessionsPerLogin: 5, SessionsPerFamily: 60, CheckNames: 25}}
	t.Run("saved and read back, with its event", func(t *testing.T) {
		before := countEvents(entities.EventEPPPolicySaved)
		require.NoError(t, repo.SavePolicy(ctx, gold, event(entities.EventEPPPolicySaved)))
		got, err := repo.GetPolicy(ctx, "gold-test")
		require.NoError(t, err)
		assert.Equal(t, gold.Limits, got.Limits)
		assert.True(t, gold.UpdatedAt.Equal(got.UpdatedAt))
		assert.Equal(t, before+1, countEvents(entities.EventEPPPolicySaved))
		gold.Limits.CheckNames = 30
		require.NoError(t, repo.SavePolicy(ctx, gold, nil), "replaced")
		got, err = repo.GetPolicy(ctx, "gold-test")
		require.NoError(t, err)
		assert.Equal(t, 30, got.Limits.CheckNames)
	})
	t.Run("assigned, then not deletable", func(t *testing.T) {
		require.NoError(t, repo.AssignPolicy(ctx, "rar-pol-a", "gold-test", nil))
		as, err := repo.ListAssignments(ctx)
		require.NoError(t, err)
		assert.Equal(t, "gold-test", as["rar-pol-a"])
		assert.ErrorIs(t, repo.DeletePolicy(ctx, "gold-test", nil), entities.ErrEPPPolicyInUse)
		require.NoError(t, repo.AssignPolicy(ctx, "rar-pol-a", "", nil))
		as, err = repo.ListAssignments(ctx)
		require.NoError(t, err)
		assert.NotContains(t, as, entities.ClIDType("rar-pol-a"))
		assert.ErrorIs(t, repo.AssignPolicy(ctx, "rar-pol-a", "missing-test", nil), entities.ErrEPPPolicyNotFound)
	})
	t.Run("families: members replaced, one family per registrar", func(t *testing.T) {
		f := &entities.RegistrarFamily{Name: "acme-test", Policy: "gold-test", Members: []entities.ClIDType{"rar-pol-a", "rar-pol-b"}, UpdatedAt: time.Now().UTC()}
		require.NoError(t, repo.SaveFamily(ctx, f, event(entities.EventRegistrarFamilySaved)))
		got, err := repo.GetFamily(ctx, "acme-test")
		require.NoError(t, err)
		assert.Equal(t, []entities.ClIDType{"rar-pol-a", "rar-pol-b"}, got.Members)
		assert.Equal(t, "gold-test", got.Policy)

		f.Members = []entities.ClIDType{"rar-pol-b"}
		require.NoError(t, repo.SaveFamily(ctx, f, nil))
		got, err = repo.GetFamily(ctx, "acme-test")
		require.NoError(t, err)
		assert.Equal(t, []entities.ClIDType{"rar-pol-b"}, got.Members)

		sp := tx.SavePoint("other")
		require.NoError(t, sp.Error)
		err = repo.SaveFamily(ctx, &entities.RegistrarFamily{Name: "other-test", Members: []entities.ClIDType{"rar-pol-b"}, UpdatedAt: time.Now()}, nil)
		assert.ErrorIs(t, err, entities.ErrRegistrarInFamily)
		assert.Contains(t, err.Error(), "acme-test")
		require.NoError(t, tx.RollbackTo("other").Error)

		assert.ErrorIs(t, repo.DeletePolicy(ctx, "gold-test", nil), entities.ErrEPPPolicyInUse, "the family's policy")
		require.NoError(t, repo.DeleteFamily(ctx, "acme-test", nil))
		_, err = repo.GetFamily(ctx, "acme-test")
		assert.ErrorIs(t, err, entities.ErrRegistrarFamilyNotFound)
		require.NoError(t, repo.SaveFamily(ctx, &entities.RegistrarFamily{Name: "other-test", Members: []entities.ClIDType{"rar-pol-b"}, UpdatedAt: time.Now()}, nil),
			"its members are free again")
		assert.ErrorIs(t, repo.DeleteFamily(ctx, "acme-test", nil), entities.ErrRegistrarFamilyNotFound)
	})
	t.Run("deleted", func(t *testing.T) {
		require.NoError(t, repo.DeletePolicy(ctx, "gold-test", event(entities.EventEPPPolicyDeleted)))
		_, err := repo.GetPolicy(ctx, "gold-test")
		assert.ErrorIs(t, err, entities.ErrEPPPolicyNotFound)
		assert.ErrorIs(t, repo.DeletePolicy(ctx, "gold-test", nil), entities.ErrEPPPolicyNotFound)
	})
}
