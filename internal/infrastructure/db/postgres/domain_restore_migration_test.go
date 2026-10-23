package postgres

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
)

// TestMigrateLegacyPendingRestores runs the migration on restores as the
// previous release left them: pendingRestore, pendingDelete cleared, no
// deadline (the column NULL, as AutoMigrate adds it). It works in a schema
// of its own, inside a transaction that is rolled back.
func TestMigrateLegacyPendingRestores(t *testing.T) {
	tx := getTestDB().Begin()
	defer tx.Rollback()
	redemption := time.Now().UTC().AddDate(0, 0, 20).Truncate(time.Second)
	for _, stmt := range []string{
		"CREATE SCHEMA restore_migration_test",
		"SET LOCAL search_path TO restore_migration_test",
		`CREATE TABLE domains (ro_id bigint PRIMARY KEY, ok boolean, pending_delete boolean, pending_restore boolean,
			redemption_period_end timestamptz, pending_restore_end timestamptz)`,
	} {
		require.NoError(t, tx.Exec(stmt).Error, stmt)
	}
	pending := time.Now().UTC().AddDate(0, 0, 2).Truncate(time.Second)
	require.NoError(t, tx.Exec(`INSERT INTO domains VALUES
		(1, false, false, true, ?, NULL),
		(2, false, true, true, ?, '0001-01-01 00:00:00+00'),
		(3, false, true, true, ?, ?),
		(4, true, false, false, NULL, NULL)`, redemption, redemption, redemption, pending).Error)

	type row struct {
		RoID                                   int64
		OK, PendingDelete, PendingRestore      bool
		RedemptionPeriodEnd, PendingRestoreEnd *time.Time
	}
	read := func() map[int64]row {
		var rows []row
		require.NoError(t, tx.Raw("SELECT * FROM domains ORDER BY ro_id").Scan(&rows).Error)
		out := map[int64]row{}
		for _, r := range rows {
			out[r.RoID] = r
		}
		return out
	}

	require.NoError(t, migrateLegacyPendingRestores(tx))
	got := read()
	for _, id := range []int64{1, 2} {
		r := got[id]
		assert.True(t, r.PendingDelete, "domain %d is pendingDelete, as RFC 3915 has it", id)
		assert.True(t, r.PendingRestore)
		require.NotNil(t, r.PendingRestoreEnd)
		assert.WithinDuration(t, time.Now().AddDate(0, 0, entities.PendingRestorePeriod), *r.PendingRestoreEnd, time.Minute, "its report is due")
		assert.True(t, redemption.Equal(*r.RedemptionPeriodEnd), "its redemption period is kept")
	}
	assert.True(t, pending.Equal(*got[3].PendingRestoreEnd), "a restore with a deadline is left alone")
	assert.False(t, got[4].PendingDelete, "a live domain is left alone")
	assert.True(t, got[4].OK)

	require.NoError(t, migrateLegacyPendingRestores(tx))
	assert.Equal(t, got, read(), "a rerun changes nothing")
}
