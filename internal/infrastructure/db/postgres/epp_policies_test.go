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
	rars := NewGormRegistrarRepository(tx)
	ctx := context.Background()
	for i, clid := range []string{"rar-pol-a", "rar-pol-b", "rar-pol-c"} {
		rar, err := entities.NewRegistrar(clid, clid+"-name", clid+"@example.com", 8710+i, getValidRegistrarPostalInfoArr())
		require.NoError(t, err)
		_, err = rars.Create(ctx, rar)
		require.NoError(t, err)
	}
	// noEvents records nothing; events records one event and what the
	// repository said was there before.
	noEvents := func() func(*entities.EPPPolicy) []entities.DomainEvent {
		return func(*entities.EPPPolicy) []entities.DomainEvent { return nil }
	}
	var sawPolicy *entities.EPPPolicy
	policyEvent := func(typ string) func(*entities.EPPPolicy) []entities.DomainEvent {
		return func(before *entities.EPPPolicy) []entities.DomainEvent {
			sawPolicy = before
			return []entities.DomainEvent{entities.NewDomainEvent("test", typ, "subject", "msg", map[string]string{"k": "v"})}
		}
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
		p, err = repo.GetPolicy(ctx, entities.EPPPolicyProbation)
		require.NoError(t, err)
		assert.Equal(t, entities.ProbationEPPPolicyLimits(), p.Limits)
	})

	gold := &entities.EPPPolicy{Name: "gold-test", Description: "test", UpdatedAt: time.Now().UTC().Truncate(time.Microsecond), UpdatedBy: "ops",
		Limits: func() entities.EPPPolicyLimits {
			l := entities.DefaultEPPPolicyLimits()
			l.SessionsPerRegistrar, l.SessionsPerLogin, l.SessionsPerFamily, l.CheckNames = 20, 5, 60, 25
			l.Rates.Info, l.FamilyRates.Poll, l.CreatesPerDay = entities.EPPRate{PerSecond: 7, Burst: 21}, entities.EPPRate{PerSecond: 2, Burst: 3}, 500
			return l
		}()}
	t.Run("saved and replaced, each event told what was there", func(t *testing.T) {
		before := countEvents(entities.EventEPPPolicySaved)
		require.NoError(t, repo.SavePolicy(ctx, gold, policyEvent(entities.EventEPPPolicySaved)))
		assert.Nil(t, sawPolicy, "nothing before a create")
		got, err := repo.GetPolicy(ctx, "gold-test")
		require.NoError(t, err)
		assert.Equal(t, gold.Limits, got.Limits)
		assert.True(t, gold.UpdatedAt.Equal(got.UpdatedAt))
		assert.Equal(t, before+1, countEvents(entities.EventEPPPolicySaved))

		replaced := *gold
		replaced.Limits.CheckNames = 30
		require.NoError(t, repo.SavePolicy(ctx, &replaced, policyEvent(entities.EventEPPPolicySaved)))
		require.NotNil(t, sawPolicy)
		assert.Equal(t, 25, sawPolicy.Limits.CheckNames, "the version replaced, read in the change's transaction")
	})
	t.Run("assigned, upserted, then not deletable", func(t *testing.T) {
		var prev []string
		assign := func(clid, policy string) error {
			return repo.AssignPolicy(ctx, entities.ClIDType(clid), policy, func(before string) []entities.DomainEvent {
				prev = append(prev, before)
				return nil
			})
		}
		require.NoError(t, assign("rar-pol-a", entities.EPPPolicyProbation))
		require.NoError(t, assign("rar-pol-a", "gold-test"), "a second assignment replaces the first")
		assert.Equal(t, []string{"", entities.EPPPolicyProbation}, prev)
		as, err := repo.ListAssignments(ctx)
		require.NoError(t, err)
		assert.Equal(t, "gold-test", as["rar-pol-a"])
		assert.ErrorIs(t, repo.DeletePolicy(ctx, "gold-test", noEvents()), entities.ErrEPPPolicyInUse)
		require.NoError(t, assign("rar-pol-a", ""))
		as, err = repo.ListAssignments(ctx)
		require.NoError(t, err)
		assert.NotContains(t, as, entities.ClIDType("rar-pol-a"))
		assert.ErrorIs(t, assign("rar-pol-a", "missing-test"), entities.ErrEPPPolicyNotFound)

		sp := tx.SavePoint("unknown")
		require.NoError(t, sp.Error)
		assert.ErrorIs(t, assign("rar-nobody", "gold-test"), entities.ErrRegistrarNotFound, "the foreign key refuses a registrar that does not exist")
		require.NoError(t, tx.RollbackTo("unknown").Error)
	})
	t.Run("families: members replaced, one family per registrar", func(t *testing.T) {
		var saw *entities.RegistrarFamily
		save := func(f *entities.RegistrarFamily) error {
			return repo.SaveFamily(ctx, f, func(before *entities.RegistrarFamily) []entities.DomainEvent { saw = before; return nil })
		}
		f := &entities.RegistrarFamily{Name: "acme-test", Policy: "gold-test", Members: []entities.ClIDType{"rar-pol-a", "rar-pol-b"}, UpdatedAt: time.Now().UTC()}
		require.NoError(t, save(f))
		assert.Nil(t, saw)
		got, err := repo.GetFamily(ctx, "acme-test")
		require.NoError(t, err)
		assert.Equal(t, []entities.ClIDType{"rar-pol-a", "rar-pol-b"}, got.Members)
		assert.Equal(t, "gold-test", got.Policy)

		f.Members = []entities.ClIDType{"rar-pol-b"}
		require.NoError(t, save(f))
		require.NotNil(t, saw)
		assert.Equal(t, []entities.ClIDType{"rar-pol-a", "rar-pol-b"}, saw.Members, "the family replaced")
		got, err = repo.GetFamily(ctx, "acme-test")
		require.NoError(t, err)
		assert.Equal(t, []entities.ClIDType{"rar-pol-b"}, got.Members)

		err = save(&entities.RegistrarFamily{Name: "other-test", Members: []entities.ClIDType{"rar-pol-b"}, UpdatedAt: time.Now()})
		assert.ErrorIs(t, err, entities.ErrRegistrarInFamily)
		assert.Contains(t, err.Error(), "acme-test")
		sp := tx.SavePoint("member")
		require.NoError(t, sp.Error)
		assert.ErrorIs(t, save(&entities.RegistrarFamily{Name: "other-test", Members: []entities.ClIDType{"rar-nobody"}, UpdatedAt: time.Now()}),
			entities.ErrRegistrarNotFound)
		require.NoError(t, tx.RollbackTo("member").Error)

		assert.ErrorIs(t, repo.DeletePolicy(ctx, "gold-test", noEvents()), entities.ErrEPPPolicyInUse, "the family's policy")
		require.NoError(t, repo.DeleteFamily(ctx, "acme-test", func(before *entities.RegistrarFamily) []entities.DomainEvent { saw = before; return nil }))
		assert.Equal(t, "acme-test", saw.Name)
		_, err = repo.GetFamily(ctx, "acme-test")
		assert.ErrorIs(t, err, entities.ErrRegistrarFamilyNotFound)
		require.NoError(t, save(&entities.RegistrarFamily{Name: "other-test", Members: []entities.ClIDType{"rar-pol-b"}, UpdatedAt: time.Now()}),
			"its members are free again")
	})
	t.Run("a registrar deleted takes its assignment and membership with it", func(t *testing.T) {
		require.NoError(t, repo.AssignPolicy(ctx, "rar-pol-c", "gold-test", func(string) []entities.DomainEvent { return nil }))
		require.NoError(t, repo.SaveFamily(ctx, &entities.RegistrarFamily{Name: "solo-test", Members: []entities.ClIDType{"rar-pol-c"}, UpdatedAt: time.Now()},
			func(*entities.RegistrarFamily) []entities.DomainEvent { return nil }))
		require.NoError(t, rars.Delete(ctx, "rar-pol-c"))
		as, err := repo.ListAssignments(ctx)
		require.NoError(t, err)
		assert.NotContains(t, as, entities.ClIDType("rar-pol-c"))
		f, err := repo.GetFamily(ctx, "solo-test")
		require.NoError(t, err)
		assert.Empty(t, f.Members)
	})
	t.Run("deleted, with what was there", func(t *testing.T) {
		require.NoError(t, repo.DeletePolicy(ctx, "gold-test", policyEvent(entities.EventEPPPolicyDeleted)))
		require.NotNil(t, sawPolicy)
		assert.Equal(t, 30, sawPolicy.Limits.CheckNames)
		_, err := repo.GetPolicy(ctx, "gold-test")
		assert.ErrorIs(t, err, entities.ErrEPPPolicyNotFound)
		assert.ErrorIs(t, repo.DeletePolicy(ctx, "gold-test", noEvents()), entities.ErrEPPPolicyNotFound)
	})
	t.Run("policies stored before command rates are given their starting values", func(t *testing.T) {
		zero := map[string]any{"creates_per_day": 0}
		for _, prefix := range []string{"rate_", "family_rate_"} {
			for _, class := range []string{"check", "info", "transform", "poll"} {
				zero[prefix+class+"_per_second"], zero[prefix+class+"_burst"] = 0, 0
			}
		}
		old := *gold
		old.Name = "old-test"
		old.Limits.CheckNames, old.Limits.Rates.Check.Burst, old.Limits.FamilyRates.Check.Burst = 1500, 1500, 3000
		require.NoError(t, repo.SavePolicy(ctx, &old, noEvents()))
		require.NoError(t, tx.Model(&EPPPolicyRecord{}).Where("name IN ?", []string{entities.EPPPolicyProbation, "old-test"}).Updates(zero).Error)
		require.NoError(t, seedEPPPolicies(tx))
		p, err := repo.GetPolicy(ctx, entities.EPPPolicyProbation)
		require.NoError(t, err)
		assert.Equal(t, entities.ProbationEPPPolicyLimits(), p.Limits, "a built-in policy its own")
		g, err := repo.GetPolicy(ctx, "old-test")
		require.NoError(t, err)
		want := entities.DefaultEPPPolicyLimits()
		want.Rates.Check.Burst = 1500 // raised to hold its 1500-name check; the family's 3000 already does
		assert.Equal(t, want.Rates, g.Limits.Rates, "any other the default's")
		assert.Equal(t, want.FamilyRates, g.Limits.FamilyRates)
		assert.Zero(t, g.Limits.CreatesPerDay)
		assert.Equal(t, 1500, g.Limits.CheckNames, "the rest is kept")
		assert.NoError(t, g.Limits.Validate(), "a valid policy")
	})
}
