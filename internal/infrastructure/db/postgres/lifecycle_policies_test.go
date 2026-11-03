package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
)

func intp(v int) *int { return &v }

// TestLifecyclePolicyRepository runs the repository against the migrated
// test database, inside a transaction that is rolled back.
func TestLifecyclePolicyRepository(t *testing.T) {
	tx := getTestDB().Begin()
	defer tx.Rollback()
	ctx := context.Background()
	repo := NewLifecyclePolicyRepository(tx)
	noEvents := func(*entities.LifecyclePolicy) ([]entities.DomainEvent, error) { return nil, nil }

	std, err := repo.GetPolicy(ctx, entities.LifecyclePolicyStandard)
	require.NoError(t, err, "seeded by the migration")
	assert.Equal(t, entities.DefaultLifecycle(), std.Resolved())

	p, err := entities.NewLifecyclePolicy("lc-repo-short", "short", entities.LifecycleSettings{RedemptionGP: intp(3), TransferLockPeriod: intp(0)})
	require.NoError(t, err)
	p.UpdatedAt = time.Now().UTC().Truncate(time.Microsecond)
	var before *entities.LifecyclePolicy
	require.NoError(t, repo.SavePolicy(ctx, p, func(b *entities.LifecyclePolicy) ([]entities.DomainEvent, error) {
		before = b
		return []entities.DomainEvent{entities.NewDomainEvent("test", "t", "s", "m", nil)}, nil
	}))
	assert.Nil(t, before, "new")
	got, err := repo.GetPolicy(ctx, "lc-repo-short")
	require.NoError(t, err)
	assert.Equal(t, p.Settings, got.Settings, "nil and 0 both round-trip")
	assert.Equal(t, 3, got.RedemptionDays())
	assert.Equal(t, 0, got.TransferLockDays())
	assert.Equal(t, entities.PendingDeleteGP, got.PendingDeleteDays())

	p.Settings.RedemptionGP = nil
	require.NoError(t, repo.SavePolicy(ctx, p, func(b *entities.LifecyclePolicy) ([]entities.DomainEvent, error) { before = b; return nil, nil }))
	assert.NotNil(t, before, "replaced")
	got, err = repo.GetPolicy(ctx, "lc-repo-short")
	require.NoError(t, err)
	assert.Nil(t, got.Settings.RedemptionGP, "a setting set back to the default is written as NULL")

	ps, err := repo.ListPolicies(ctx)
	require.NoError(t, err)
	names := make([]string, len(ps))
	for i, p := range ps {
		names[i] = p.Name
	}
	assert.Contains(t, names, "lc-repo-short")
	assert.Contains(t, names, entities.LifecyclePolicyStandard)

	// A phase that names the policy keeps it from being deleted.
	ro, err := entities.NewRegistryOperator("lcrepoRy", "lcrepoRy", "lcrepo@example.com")
	require.NoError(t, err)
	_, err = NewGORMRegistryOperatorRepository(tx).Create(ctx, ro)
	require.NoError(t, err)
	tld, err := entities.NewTLD("lcrepo", "lcrepoRy")
	require.NoError(t, err)
	require.NoError(t, NewGormTLDRepo(tx).Create(ctx, tld))
	phase, err := entities.NewPhase("ga-1", "GA", time.Now().UTC())
	require.NoError(t, err)
	phase.TLDName = "lcrepo"
	phase.LifecyclePolicyName = "lc-repo-short"
	stored, err := NewGormPhaseRepository(tx).CreatePhase(ctx, phase)
	require.NoError(t, err)
	require.NotNil(t, stored.LifecyclePolicy, "the policy is loaded with the phase")
	assert.Equal(t, 0, stored.Lifecycle().TransferLockDays())
	uses, err := repo.PolicyUses(ctx, "lc-repo-short")
	require.NoError(t, err)
	assert.Equal(t, []entities.LifecyclePolicyUse{{TLD: "lcrepo", Phase: "ga-1"}}, uses)
	all, err := repo.ListPolicyUses(ctx)
	require.NoError(t, err)
	assert.Equal(t, uses, all["lc-repo-short"], "the same, for every policy in one read")
	err = repo.DeletePolicy(ctx, "lc-repo-short", func(*entities.LifecyclePolicy) []entities.DomainEvent { return nil })
	assert.ErrorIs(t, err, entities.ErrLifecyclePolicyInUse)
	assert.ErrorContains(t, err, "lcrepo/ga-1")

	loaded, err := NewGormTLDRepo(tx).GetByName(ctx, "lcrepo", true)
	require.NoError(t, err)
	require.Len(t, loaded.Phases, 1)
	assert.Equal(t, 0, loaded.Phases[0].Lifecycle().TransferLockDays(), "loaded with the TLD's phases too")
	assert.Equal(t, "lc-repo-short", loaded.Phases[0].LifecyclePolicyName)

	_, err = repo.GetPolicy(ctx, "nope")
	assert.ErrorIs(t, err, entities.ErrLifecyclePolicyNotFound)
	err = repo.DeletePolicy(ctx, "nope", func(*entities.LifecyclePolicy) []entities.DomainEvent { return nil })
	assert.ErrorIs(t, err, entities.ErrLifecyclePolicyNotFound)

	// Unnamed, it goes.
	phase2, err := entities.NewPhase("launch", "Launch", time.Now().UTC())
	require.NoError(t, err)
	phase2.TLDName = "lcrepo"
	_, err = NewGormPhaseRepository(tx).CreatePhase(ctx, phase2)
	require.NoError(t, err)
	require.NoError(t, NewGormPhaseRepository(tx).DeletePhaseByTLDAndName(ctx, entities.PlatformRegistryScope(entities.NewPlatformScope()), "lcrepo", "ga-1"))
	require.NoError(t, repo.DeletePolicy(ctx, "lc-repo-short", func(*entities.LifecyclePolicy) []entities.DomainEvent { return nil }))
	_ = noEvents
}

// TestMigrateLifecyclePolicies runs the migration on a phases table as the
// previous release left it, with the inline lifecycle columns, in a schema
// of its own inside a transaction that is rolled back. It checks the three
// outcomes ADR 0023 decision 6 asks for: phases at the standard values
// name gtld-standard, each other distinct value set becomes one policy
// named after its first phase, no phase changes behaviour, and the inline
// columns are gone. A rerun changes nothing.
func TestMigrateLifecyclePolicies(t *testing.T) {
	tx := getTestDB().Begin()
	defer tx.Rollback()
	for _, stmt := range []string{
		"CREATE SCHEMA lifecycle_migration_test",
		"SET LOCAL search_path TO lifecycle_migration_test",
		// The old shape, plus the column AutoMigrate adds before the migration.
		`CREATE TABLE phases (id bigserial PRIMARY KEY, name text, tld_name text, type text, starts timestamptz,
			registration_gp bigint, renewal_gp bigint, auto_renewal_gp bigint, transfer_gp bigint, pending_transfer_period bigint,
			redemption_gp bigint, pending_delete_gp bigint, pending_restore_period bigint, restore_report_required boolean,
			transfer_lock_period bigint, max_horizon bigint, allow_auto_renew boolean,
			lifecycle_policy_name varchar(32) NOT NULL DEFAULT 'gtld-standard')`,
	} {
		require.NoError(t, tx.Exec(stmt).Error, stmt)
	}
	require.NoError(t, prepareLifecyclePolicies(tx))
	require.NoError(t, tx.Exec(`INSERT INTO phases (name, tld_name, type, starts, registration_gp, renewal_gp, auto_renewal_gp, transfer_gp, pending_transfer_period,
		redemption_gp, pending_delete_gp, pending_restore_period, restore_report_required, transfer_lock_period, max_horizon, allow_auto_renew) VALUES
		('ga-2024', 'alpha', 'GA', '2024-01-01', 5, 5, 45, 5, 5, 30, 5, 7, true, 60, 10, true),
		('ga-2026', 'alpha', 'GA', '2026-01-01', 5, 5, 45, 0, 0, 30, 5, 0, NULL, NULL, 10, NULL),
		('GA-Main', 'beta', 'GA', '2025-01-01', 5, 5, 45, 5, 5, 3, 1, 7, false, 0, 10, false),
		('sunrise', 'beta', 'Launch', '2024-06-01', 5, 5, 45, 5, 5, 3, 1, 7, false, 0, 10, false),
		('ga', 'a-very-long-tld-name-indeed-xyz', 'GA', '2025-01-01', 1, 5, 45, 5, 5, 30, 5, 7, true, 60, 10, true),
		('ga-ote', 'ote', 'GA', '2025-01-01', 0, 5, 45, 5, 5, 30, 5, 7, true, 60, 10, true)`).Error)

	require.NoError(t, migrateLifecyclePolicies(tx))

	var assigned []struct{ TLDName, Name, LifecyclePolicyName string }
	require.NoError(t, tx.Raw("SELECT tld_name, name, lifecycle_policy_name FROM phases ORDER BY id").Scan(&assigned).Error)
	require.Len(t, assigned, 6)
	assert.Equal(t, entities.LifecyclePolicyStandard, assigned[0].LifecyclePolicyName, "explicit standard values")
	assert.Equal(t, entities.LifecyclePolicyStandard, assigned[1].LifecyclePolicyName, "zeros and NULLs in the fields the old code read through helpers meant the default")
	assert.Equal(t, "migrated-beta-ga-main", assigned[2].LifecyclePolicyName, "a distinct set, named after its first phase, lower-cased")
	assert.Equal(t, "migrated-beta-ga-main", assigned[3].LifecyclePolicyName, "shared by the phase with the same values")
	assert.Equal(t, "migrated-a-very-long-tld-name-in", assigned[4].LifecyclePolicyName, "cut to a policy name's 32 characters")
	assert.Equal(t, "migrated-ote-ga-ote", assigned[5].LifecyclePolicyName, "a zero the old code read verbatim is kept as zero days")

	repo := NewLifecyclePolicyRepository(tx)
	beta, err := repo.GetPolicy(context.Background(), "migrated-beta-ga-main")
	require.NoError(t, err)
	assert.Equal(t, entities.ResolvedLifecycle{
		RegistrationGP: 5, RenewalGP: 5, AutoRenewalGP: 45, TransferGP: 5, TransferLockPeriod: 0, PendingTransferPeriod: 5,
		RedemptionGP: 3, PendingDeleteGP: 1, PendingRestorePeriod: 7, RestoreReportRequired: false, AllowAutoRenew: false, MaxHorizon: 10,
	}, beta.Resolved(), "no phase changes behaviour")
	assert.Equal(t, "migrate", beta.UpdatedBy)
	ote, err := repo.GetPolicy(context.Background(), "migrated-ote-ga-ote")
	require.NoError(t, err)
	assert.Equal(t, 0, ote.RegistrationGraceDays(), "a zero the old code read verbatim is kept as zero days")
	long, err := repo.GetPolicy(context.Background(), "migrated-a-very-long-tld-name-in")
	require.NoError(t, err)
	assert.Equal(t, 1, long.RegistrationGraceDays())

	for _, col := range inlineLifecycleColumns {
		assert.False(t, tx.Migrator().HasColumn("phases", col), "%s is dropped", col)
	}
	require.NoError(t, migrateLifecyclePolicies(tx), "a rerun finds nothing to do")
	ps, err := repo.ListPolicies(context.Background())
	require.NoError(t, err)
	assert.Len(t, ps, 4)
}

// TestMigrateLifecyclePolicies_OlderSchema runs the migration on a phases
// table from before the restore and transfer settings existed: their
// columns are absent, not NULL, and read as the defaults the old code gave
// them.
func TestMigrateLifecyclePolicies_OlderSchema(t *testing.T) {
	tx := getTestDB().Begin()
	defer tx.Rollback()
	for _, stmt := range []string{
		"CREATE SCHEMA lifecycle_migration_old_test",
		"SET LOCAL search_path TO lifecycle_migration_old_test",
		`CREATE TABLE phases (id bigserial PRIMARY KEY, name text, tld_name text, type text, starts timestamptz,
			registration_gp bigint, renewal_gp bigint, auto_renewal_gp bigint, redemption_gp bigint, pending_delete_gp bigint,
			transfer_lock_period bigint, max_horizon bigint, allow_auto_renew boolean,
			lifecycle_policy_name varchar(32) NOT NULL DEFAULT 'gtld-standard')`,
	} {
		require.NoError(t, tx.Exec(stmt).Error, stmt)
	}
	require.NoError(t, prepareLifecyclePolicies(tx))
	require.NoError(t, tx.Exec(`INSERT INTO phases (name, tld_name, type, starts, registration_gp, renewal_gp, auto_renewal_gp, redemption_gp, pending_delete_gp, transfer_lock_period, max_horizon, allow_auto_renew) VALUES
		('ga', 'old', 'GA', '2024-01-01', 5, 5, 45, 30, 5, NULL, 10, NULL),
		('ga', 'older', 'GA', '2024-01-01', 5, 5, 45, 10, 5, 0, 10, false)`).Error)
	require.NoError(t, migrateLifecyclePolicies(tx))
	var assigned []struct{ TLDName, LifecyclePolicyName string }
	require.NoError(t, tx.Raw("SELECT tld_name, lifecycle_policy_name FROM phases ORDER BY id").Scan(&assigned).Error)
	assert.Equal(t, entities.LifecyclePolicyStandard, assigned[0].LifecyclePolicyName, "the settings it never had are the defaults")
	assert.Equal(t, "migrated-older-ga", assigned[1].LifecyclePolicyName)
	p, err := NewLifecyclePolicyRepository(tx).GetPolicy(context.Background(), "migrated-older-ga")
	require.NoError(t, err)
	assert.Equal(t, 10, p.RedemptionDays())
	assert.Equal(t, 0, p.TransferLockDays())
	assert.Equal(t, entities.PendingRestorePeriod, p.PendingRestoreDays(), "a column the schema never had: the default")
	assert.True(t, p.RequiresRestoreReport())
	assert.False(t, p.AutoRenewAllowed())
	for _, col := range inlineLifecycleColumns {
		assert.False(t, tx.Migrator().HasColumn("phases", col), "%s is gone", col)
	}
	require.NoError(t, migrateLifecyclePolicies(tx), "a rerun finds nothing to do")
}

// TestMigrateLifecyclePolicies_RefusesWhatNoPolicyMay: a stored value the
// new bounds refuse (a negative period, a zero horizon) fails the migration
// naming the phase; nothing is assigned and the columns stay.
func TestMigrateLifecyclePolicies_RefusesWhatNoPolicyMay(t *testing.T) {
	tx := getTestDB().Begin()
	defer tx.Rollback()
	for _, stmt := range []string{
		"CREATE SCHEMA lifecycle_migration_bad_test",
		"SET LOCAL search_path TO lifecycle_migration_bad_test",
		`CREATE TABLE phases (id bigserial PRIMARY KEY, name text, tld_name text, type text, starts timestamptz,
			registration_gp bigint, renewal_gp bigint, auto_renewal_gp bigint, transfer_gp bigint, pending_transfer_period bigint,
			redemption_gp bigint, pending_delete_gp bigint, pending_restore_period bigint, restore_report_required boolean,
			transfer_lock_period bigint, max_horizon bigint, allow_auto_renew boolean,
			lifecycle_policy_name varchar(32) NOT NULL DEFAULT 'gtld-standard')`,
	} {
		require.NoError(t, tx.Exec(stmt).Error, stmt)
	}
	require.NoError(t, prepareLifecyclePolicies(tx))
	require.NoError(t, tx.Exec(`INSERT INTO phases (name, tld_name, type, starts, registration_gp, renewal_gp, auto_renewal_gp, transfer_gp, pending_transfer_period,
		redemption_gp, pending_delete_gp, pending_restore_period, restore_report_required, transfer_lock_period, max_horizon, allow_auto_renew) VALUES
		('ga', 'bad', 'GA', '2025-01-01', 5, 5, 45, 5, 5, -1, 5, 7, true, 60, 0, true)`).Error)
	err := migrateLifecyclePolicies(tx)
	require.Error(t, err)
	assert.ErrorContains(t, err, "phase ga of bad")
	assert.ErrorContains(t, err, "redemptionGP must be 0 to 365")
	assert.ErrorContains(t, err, "maxHorizon must be 1 to 10")
	assert.True(t, tx.Migrator().HasColumn("phases", "registration_gp"), "the columns stay")
	ps, err := NewLifecyclePolicyRepository(tx).ListPolicies(context.Background())
	require.NoError(t, err)
	assert.Len(t, ps, 1, "nothing was created")
}

func TestMigratedPolicyName(t *testing.T) {
	taken := map[string]bool{}
	assert.Equal(t, "migrated-example-ga-2026", migratedPolicyName("example", "GA-2026", taken))
	assert.Equal(t, "migrated-example-ga-2026-2", migratedPolicyName("example", "ga_2026", taken), "a name taken is numbered")
	assert.Equal(t, "migrated-xn--p1ai-ga", migratedPolicyName("xn--p1ai", "GA", taken))
	long := migratedPolicyName("a-very-long-tld-name-indeed-xyz", "ga", taken)
	assert.Equal(t, "migrated-a-very-long-tld-name-in", long)
	assert.Len(t, long, 32)
	again := migratedPolicyName("a-very-long-tld-name-indeed-xyz", "ga", taken)
	assert.Equal(t, "migrated-a-very-long-tld-name-2", again)
	assert.LessOrEqual(t, len(again), 32)
}
