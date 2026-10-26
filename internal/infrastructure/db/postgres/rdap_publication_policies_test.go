package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
)

// TestRDAPPublicationPolicyRepository runs the repository against the
// migrated test database, inside a transaction that is rolled back.
func TestRDAPPublicationPolicyRepository(t *testing.T) {
	tx := getTestDB().Begin()
	defer tx.Rollback()
	repo := NewRDAPPublicationPolicyRepository(tx)
	ctx := context.Background()

	ro, err := entities.NewRegistryOperator("ry-rdap", "ry-rdap", "ry@rdap.example")
	require.NoError(t, err)
	_, err = NewGORMRegistryOperatorRepository(tx).Create(ctx, ro)
	require.NoError(t, err)
	tld, err := entities.NewTLD("rdappoltest", "ry-rdap")
	require.NoError(t, err)
	require.NoError(t, NewGormTLDRepo(tx).Create(ctx, tld))

	var saw *entities.RDAPPublicationPolicy
	event := func(typ string) func(*entities.RDAPPublicationPolicy) []entities.DomainEvent {
		return func(before *entities.RDAPPublicationPolicy) []entities.DomainEvent {
			saw = before
			return []entities.DomainEvent{entities.NewDomainEvent("test", typ, "subject", "msg", map[string]string{"k": "v"})}
		}
	}
	countEvents := func(typ string) int64 {
		var n int64
		require.NoError(t, tx.Table("domain_events").Where("type = ?", typ).Count(&n).Error)
		return n
	}

	t.Run("the migration seeded the built-in policies", func(t *testing.T) {
		require.NoError(t, seedRDAPPublicationPolicies(tx), "seeding again keeps what is there")
		for _, b := range entities.BuiltInRDAPPublicationPolicies() {
			p, err := repo.GetPolicy(ctx, b.Name)
			require.NoError(t, err)
			assert.Equal(t, b.Roles, p.Roles)
		}
	})

	custom := entities.BuiltInRDAPPublicationPolicies()[0]
	custom.Name, custom.ContactFormURI, custom.UpdatedBy = "custom-test", "https://registry.example/contact", "ops"
	custom.UpdatedAt = time.Now().UTC().Truncate(time.Microsecond)

	t.Run("save, read back and record the event", func(t *testing.T) {
		require.NoError(t, repo.SavePolicy(ctx, custom, event("test.saved")))
		assert.Nil(t, saw, "nothing was there before")
		got, err := repo.GetPolicy(ctx, "custom-test")
		require.NoError(t, err)
		assert.Equal(t, custom, got)
		assert.EqualValues(t, 1, countEvents("test.saved"))
	})

	t.Run("assign, and a policy in use cannot be deleted", func(t *testing.T) {
		var prev string
		require.NoError(t, repo.AssignPolicy(ctx, "rdappoltest", "custom-test", func(before string) []entities.DomainEvent { prev = before; return nil }))
		assert.Empty(t, prev)
		a, err := repo.ListAssignments(ctx)
		require.NoError(t, err)
		assert.Equal(t, "custom-test", a["rdappoltest"])
		assert.ErrorIs(t, repo.DeletePolicy(ctx, "custom-test", event("test.deleted")), entities.ErrRDAPPublicationPolicyInUse)
	})

	t.Run("assignment refuses an unknown TLD or policy", func(t *testing.T) {
		assert.ErrorIs(t, repo.AssignPolicy(ctx, "rdappoltest", "no-such", func(string) []entities.DomainEvent { return nil }), entities.ErrRDAPPublicationPolicyNotFound)
		assert.ErrorIs(t, repo.AssignPolicy(ctx, "no-such-tld", "custom-test", func(string) []entities.DomainEvent { return nil }), entities.ErrTLDNotFound)
	})

	t.Run("unassign, then delete", func(t *testing.T) {
		require.NoError(t, repo.AssignPolicy(ctx, "rdappoltest", "", func(string) []entities.DomainEvent { return nil }))
		require.NoError(t, repo.DeletePolicy(ctx, "custom-test", event("test.deleted")))
		require.NotNil(t, saw)
		assert.Equal(t, "custom-test", saw.Name)
		_, err := repo.GetPolicy(ctx, "custom-test")
		assert.ErrorIs(t, err, entities.ErrRDAPPublicationPolicyNotFound)
	})
}
