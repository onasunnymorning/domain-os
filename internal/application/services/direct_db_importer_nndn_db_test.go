package services_test

import (
	"context"
	"database/sql"
	"testing"

	_ "github.com/lib/pq"
	"github.com/onasunnymorning/domain-os/internal/application/services"
	postgres "github.com/onasunnymorning/domain-os/internal/infrastructure/db/postgres"
	"github.com/stretchr/testify/require"
	_ "modernc.org/sqlite"
)

// TestDirectDBImporter_ImportNNDNs_ReasonBackfill runs the importer against a
// real Postgres to prove the upsert's reason handling: new names are tagged,
// names that already exist without a reason are backfilled, and a reason an
// operator set by hand survives a re-import. It runs in a database of its own
// (authInfoTestDB); `make test` provides the Postgres on TEST_DB_PORT (5433).
func TestDirectDBImporter_ImportNNDNs_ReasonBackfill(t *testing.T) {
	gdb := authInfoTestDB(t)

	importer, err := services.NewDirectDBImporter()
	require.NoError(t, err)
	defer importer.PG.Close()

	const tld = "nndnreason"
	const ry = "nndnreasonry"
	require.NoError(t, gdb.Create(&postgres.RegistryOperator{RyID: ry, Name: "NNDN Reason Registry", Email: "test@test.com"}).Error)
	require.NoError(t, gdb.Create(&postgres.TLD{Name: tld, Type: "generic", RyID: ry}).Error)
	t.Cleanup(func() {
		gdb.Exec("DELETE FROM nndns WHERE tld_name = ?", tld)
		gdb.Exec("DELETE FROM tlds WHERE name = ?", tld)
		gdb.Exec("DELETE FROM registry_operators WHERE ry_id = ?", ry)
	})

	// Rows that exist before the import. NULL is what the importer itself wrote
	// before it tagged reasons; '' is what the GORM/REST path writes.
	require.NoError(t, gdb.Exec(`INSERT INTO nndns (name, tld_name, name_state, reason, created_at, updated_at) VALUES
		('null-reason.nndnreason',  'nndnreason', 'blocked', NULL,           now(), now()),
		('empty-reason.nndnreason', 'nndnreason', 'blocked', '',             now(), now()),
		('manual.nndnreason',       'nndnreason', 'blocked', 'trademark',    now(), now())`).Error)

	sqliteDB, err := sql.Open("sqlite", t.TempDir()+"/staged.db")
	require.NoError(t, err)
	defer sqliteDB.Close()
	_, err = sqliteDB.Exec(`CREATE TABLE nndns (aname TEXT PRIMARY KEY, uname TEXT, idntableid TEXT, originalname TEXT, namestate TEXT, crdate TEXT);
		INSERT INTO nndns (aname, uname, namestate, crdate) VALUES
		('null-reason.nndnreason',  '', 'blocked', '2024-01-19 15:04:05'),
		('empty-reason.nndnreason', '', 'blocked', '2024-01-19 15:04:05'),
		('manual.nndnreason',       '', 'blocked', '2024-01-19 15:04:05'),
		('brand-new.nndnreason',    '', 'blocked', '2024-01-19 15:04:05');`)
	require.NoError(t, err)

	total, _, _, err := importer.ImportNNDNs(context.Background(), sqliteDB, tld, "", func(string) {})
	require.NoError(t, err)
	require.Equal(t, int64(4), total)

	var rows []struct {
		Name   string
		Reason string
	}
	require.NoError(t, gdb.Raw(`SELECT name, COALESCE(reason, '<NULL>') AS reason FROM nndns WHERE tld_name = ?`, tld).Scan(&rows).Error)
	got := map[string]string{}
	for _, r := range rows {
		got[r.Name] = r.Reason
	}
	require.Equal(t, map[string]string{
		"null-reason.nndnreason":  "RDE-import", // backfilled
		"empty-reason.nndnreason": "RDE-import", // backfilled
		"manual.nndnreason":       "trademark",  // operator's reason preserved
		"brand-new.nndnreason":    "RDE-import",
	}, got)
}
