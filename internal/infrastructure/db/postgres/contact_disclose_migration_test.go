package postgres

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMigrateContactDisclose runs the migration on a contacts table in the
// shape the previous release left it: address disclosure in addr_int and
// addr_loc, and no disclose_ columns. It works in a schema of its own,
// inside a transaction that is rolled back.
func TestMigrateContactDisclose(t *testing.T) {
	tx := getTestDB().Begin()
	defer tx.Rollback()
	for _, stmt := range []string{
		"CREATE SCHEMA disclose_migration_test",
		"SET LOCAL search_path TO disclose_migration_test",
		"CREATE TABLE contacts (id text PRIMARY KEY, addr_int boolean, addr_loc boolean)",
		"INSERT INTO contacts VALUES ('both', true, true), ('loc', false, true), ('none', false, false)",
	} {
		require.NoError(t, tx.Exec(stmt).Error, stmt)
	}

	require.NoError(t, migrateContactDisclose(tx))
	read := func() map[string][2]bool {
		var rows []struct {
			ID                               string
			DiscloseAddrInt, DiscloseAddrLoc bool
		}
		require.NoError(t, tx.Raw("SELECT id, disclose_addr_int, disclose_addr_loc FROM contacts").Scan(&rows).Error)
		out := map[string][2]bool{}
		for _, r := range rows {
			out[r.ID] = [2]bool{r.DiscloseAddrInt, r.DiscloseAddrLoc}
		}
		return out
	}
	assert.Equal(t, map[string][2]bool{"both": {true, true}, "loc": {false, true}, "none": {false, false}}, read(), "the stored flags are copied")

	t.Run("it runs once: a flag changed since is not copied over again", func(t *testing.T) {
		require.NoError(t, tx.Exec("UPDATE contacts SET disclose_addr_int = false WHERE id = 'both'").Error)
		require.NoError(t, migrateContactDisclose(tx))
		assert.Equal(t, [2]bool{false, true}, read()["both"])
	})
	t.Run("a database without the old columns is left alone", func(t *testing.T) {
		require.NoError(t, tx.Exec("ALTER TABLE contacts DROP COLUMN addr_int, DROP COLUMN addr_loc, DROP COLUMN disclose_addr_int, DROP COLUMN disclose_addr_loc").Error)
		require.NoError(t, migrateContactDisclose(tx))
		var n int64
		require.NoError(t, tx.Raw(`SELECT COUNT(*) FROM information_schema.columns
			WHERE table_schema = current_schema() AND table_name = 'contacts' AND column_name LIKE 'disclose_%'`).Scan(&n).Error)
		assert.Zero(t, n)
	})
}
