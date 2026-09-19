package services_test

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strings"
	"testing"

	_ "github.com/lib/pq"
	"github.com/onasunnymorning/domain-os/internal/application/services"
	postgres "github.com/onasunnymorning/domain-os/internal/infrastructure/db/postgres"
	"github.com/stretchr/testify/require"
	pgdriver "gorm.io/driver/postgres"
	"gorm.io/gorm"
	_ "modernc.org/sqlite"
)

// domainsImportDB connects to the throwaway test Postgres the way the other
// DB-backed tests do (TEST_DB_HOST/TEST_DB_PORT), points the importer's own
// DB_* environment at it, and drops the foreign keys on domains so a row can be
// seeded without building a registry, registrar and contact around it. The
// behaviour under test is the upsert, not referential integrity.
func domainsImportDB(t *testing.T) *gorm.DB {
	t.Helper()
	host, port := os.Getenv("TEST_DB_HOST"), os.Getenv("TEST_DB_PORT")
	if host == "" {
		host = "127.0.0.1"
	}
	if port == "" {
		port = "5432"
	}
	dsn := fmt.Sprintf("postgres://postgres:unittest@%s:%s/dos_unittests?sslmode=require", host, port)
	db, err := gorm.Open(pgdriver.Open(dsn))
	if err != nil && strings.Contains(err.Error(), "3D000") {
		admin, aerr := sql.Open("postgres", fmt.Sprintf("host=%s port=%s user=postgres password=unittest sslmode=require", host, port))
		require.NoError(t, aerr)
		_, _ = admin.Exec("CREATE DATABASE dos_unittests")
		_ = admin.Close()
		db, err = gorm.Open(pgdriver.Open(dsn))
	}
	require.NoError(t, err, "test Postgres unavailable; run via `make test` (port 5433)")
	require.NoError(t, postgres.AutoMigrate(db))

	var fks []string
	require.NoError(t, db.Raw(`SELECT conname FROM pg_constraint WHERE conrelid = 'domains'::regclass AND contype = 'f'`).Scan(&fks).Error)
	for _, fk := range fks {
		require.NoError(t, db.Exec(fmt.Sprintf(`ALTER TABLE domains DROP CONSTRAINT IF EXISTS %q`, fk)).Error)
	}

	t.Setenv("DATABASE_URL", "")
	t.Setenv("DB_HOST", host)
	t.Setenv("DB_PORT", port)
	t.Setenv("DB_USER", "postgres")
	t.Setenv("DB_PASS", "unittest")
	t.Setenv("DB_NAME", "dos_unittests")
	t.Setenv("DB_SSLMODE", "require")
	return db
}

func stagedDomains(t *testing.T, rows ...[3]string) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", t.TempDir()+"/staged.db")
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	_, err = db.Exec(`
		CREATE TABLE domains (name TEXT PRIMARY KEY, registrant TEXT, clid TEXT, crrr TEXT, crdate TEXT, exdate TEXT, uprr TEXT, uname TEXT, originalname TEXT);
		CREATE TABLE domain_statuses (domain_name TEXT, status TEXT);`)
	require.NoError(t, err)
	for _, r := range rows { // name, registrant, clid
		_, err = db.Exec(`INSERT INTO domains (name, registrant, clid, uname) VALUES (?, ?, ?, ?)`, r[0], r[1], r[2], r[0])
		require.NoError(t, err)
	}
	return db
}

// A deposit is imported as one TLD. Domains are upserted by name, so a name that
// already exists under a different TLD must be left alone and the import must
// say so, rather than re-assigning the domain (as happened when a `.paco`
// derivative was imported as `gza`).
func TestDirectDBImporter_ImportDomains_DoesNotReassignAnotherTLDsDomain(t *testing.T) {
	db := domainsImportDB(t)

	const name = "domainsguard-keep.paco"
	t.Cleanup(func() { db.Exec(`DELETE FROM domains WHERE name = ?`, name) })
	require.NoError(t, db.Exec(`DELETE FROM domains WHERE name = ?`, name).Error)
	require.NoError(t, db.Exec(`
		INSERT INTO domains (ro_id, name, tld_name, cl_id, registrant_id, auth_info, expiry_date, created_at, updated_at)
		VALUES (7001, ?, 'paco', 'original-registrar', 'original-registrant', 'x', now(), now(), now())`, name).Error)

	importer, err := services.NewDirectDBImporter()
	require.NoError(t, err)

	staged := stagedDomains(t, [3]string{name, "other-registrant", "other-registrar"})
	_, _, _, err = importer.ImportDomains(context.Background(), staged, "gza", nil, "", func(string) {})

	require.Error(t, err, "a name that exists under another TLD must fail the import")
	require.Contains(t, err.Error(), "different TLD")

	var got struct {
		TLDName, ClID, RegistrantID string
	}
	require.NoError(t, db.Raw(`SELECT tld_name, cl_id, registrant_id FROM domains WHERE name = ?`, name).Scan(&got).Error)
	require.Equal(t, "paco", got.TLDName, "the domain must stay in its own TLD")
	require.Equal(t, "original-registrar", got.ClID, "the other TLD's domain must not be rewritten")
	require.Equal(t, "original-registrant", got.RegistrantID)
}

// Re-importing a deposit into the TLD it belongs to is the supported update
// path and must keep working.
func TestDirectDBImporter_ImportDomains_ReimportSameTLDUpdates(t *testing.T) {
	db := domainsImportDB(t)

	const existing, added = "domainsguard-existing.paco", "domainsguard-added.paco"
	cleanup := func() { db.Exec(`DELETE FROM domains WHERE name IN (?, ?)`, existing, added) }
	cleanup()
	t.Cleanup(cleanup)
	require.NoError(t, db.Exec(`
		INSERT INTO domains (ro_id, name, tld_name, cl_id, registrant_id, auth_info, expiry_date, created_at, updated_at)
		VALUES (7002, ?, 'paco', 'old-registrar', 'old-registrant', 'x', now(), now(), now())`, existing).Error)

	importer, err := services.NewDirectDBImporter()
	require.NoError(t, err)

	staged := stagedDomains(t, [3]string{existing, "new-registrant", "new-registrar"}, [3]string{added, "new-registrant", "new-registrar"})
	total, _, _, err := importer.ImportDomains(context.Background(), staged, "paco", nil, "", func(string) {})
	require.NoError(t, err)
	require.Equal(t, int64(2), total)

	var rows []struct {
		Name, TLDName, ClID string
	}
	require.NoError(t, db.Raw(`SELECT name, tld_name, cl_id FROM domains WHERE name IN (?, ?) ORDER BY name`, existing, added).Scan(&rows).Error)
	require.Len(t, rows, 2)
	for _, r := range rows {
		require.Equal(t, "paco", r.TLDName, r.Name)
		require.Equal(t, "new-registrar", r.ClID, "%s should carry the deposit's registrar", r.Name)
	}
}
