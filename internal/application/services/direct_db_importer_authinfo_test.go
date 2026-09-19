package services_test

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	_ "github.com/lib/pq"
	"github.com/onasunnymorning/domain-os/internal/application/services"
	postgres "github.com/onasunnymorning/domain-os/internal/infrastructure/db/postgres"
	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
	"github.com/stretchr/testify/require"
	pgdriver "gorm.io/driver/postgres"
	"gorm.io/gorm"
	_ "modernc.org/sqlite"
)

// retiredEscrowAuthInfo is the constant the import used to stamp on every domain and contact.
// It sits in a public repository, so it must never be stored again.
const retiredEscrowAuthInfo = "escr0W1mP*rt"

// The importer batches at 5000 rows; seed past one batch so both batches are exercised.
const directImportRows = 5050

// authInfoTestDB connects to the throwaway test Postgres the way the other database tests do
// (TEST_DB_HOST/TEST_DB_PORT), migrates it, and points the importer at it through the DB_* variables
// NewDirectDBImporter reads. It fails rather than skips, like the activities database tests: the
// proof that no shared authInfo is stored has to run wherever `make test` runs.
func authInfoTestDB(t *testing.T) *gorm.DB {
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
		// Fresh container: create the database the way the postgres package tests do, then retry.
		admin, aerr := sql.Open("postgres", fmt.Sprintf("host=%s port=%s user=postgres password=unittest sslmode=require", host, port))
		require.NoError(t, aerr)
		_, _ = admin.Exec("CREATE DATABASE dos_unittests")
		_ = admin.Close()
		db, err = gorm.Open(pgdriver.Open(dsn))
	}
	require.NoError(t, err, "test Postgres unavailable; run via `make test` (port 5433)")
	require.NoError(t, postgres.AutoMigrate(db))

	t.Setenv("DATABASE_URL", "")
	t.Setenv("DB_HOST", host)
	t.Setenv("DB_PORT", port)
	t.Setenv("DB_USER", "postgres")
	t.Setenv("DB_PASS", "unittest")
	t.Setenv("DB_NAME", "dos_unittests")
	t.Setenv("DB_SSLMODE", "require")
	return db
}

// seedFixtures creates the operator, TLD and registrar an import must reference (contacts and domains
// are foreign-keyed to the registrar), and removes everything this test wrote when it finishes so it
// can run again against a used database. It returns the registrar's ClID.
func seedFixtures(t *testing.T, db *gorm.DB, tld string, gurid int) string {
	t.Helper()
	ryID, clID := tld+"op", "rar"+tld
	cleanup := func() {
		db.Exec("DELETE FROM domains WHERE tld_name = ?", tld)
		db.Exec("DELETE FROM contacts WHERE cl_id = ?", clID)
		db.Exec("DELETE FROM registrars WHERE cl_id = ?", clID)
		db.Exec("DELETE FROM tlds WHERE name = ?", tld)
		db.Exec("DELETE FROM registry_operators WHERE ry_id = ?", ryID)
	}
	cleanup()
	t.Cleanup(cleanup)
	ctx := context.Background()

	ro, err := entities.NewRegistryOperator(ryID, "AuthInfo Import Operator", "ops@example.com")
	require.NoError(t, err)
	_, err = postgres.NewGORMRegistryOperatorRepository(db).Create(ctx, ro)
	require.NoError(t, err)

	tldEnt, err := entities.NewTLD(tld, ryID)
	require.NoError(t, err)
	require.NoError(t, postgres.NewGormTLDRepo(db).Create(ctx, tldEnt))

	var postal [2]*entities.RegistrarPostalInfo
	for i, kind := range []string{"loc", "int"} {
		addr, err := entities.NewAddress("Ollantaytambo", "PE")
		require.NoError(t, err)
		postal[i], err = entities.NewRegistrarPostalInfo(kind, addr)
		require.NoError(t, err)
	}
	rar, err := entities.NewRegistrar(clID, "AuthInfo Import Registrar", "rar@example.com", gurid, postal)
	require.NoError(t, err)
	_, err = postgres.NewGormRegistrarRepository(db).Create(ctx, rar)
	require.NoError(t, err)
	return clID
}

func stagedSQLite(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "staged.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	_, err = db.Exec(`
		CREATE TABLE contacts (id TEXT PRIMARY KEY, roid TEXT, voice TEXT, fax TEXT, email TEXT, clid TEXT, crrr TEXT, crdate TEXT, uprr TEXT, "update" TEXT);
		CREATE TABLE domains (name TEXT PRIMARY KEY, registrant TEXT, clid TEXT, crrr TEXT, crdate TEXT, exdate TEXT, uprr TEXT, uname TEXT, originalname TEXT);
		CREATE TABLE domain_statuses (domain_name TEXT, status TEXT);
	`)
	require.NoError(t, err)
	return db
}

func storedAuthInfos(t *testing.T, db *gorm.DB, query string, args ...any) map[string]string {
	t.Helper()
	type row struct{ Key, AuthInfo string }
	var rows []row
	require.NoError(t, db.Raw(query, args...).Scan(&rows).Error)
	out := make(map[string]string, len(rows))
	for _, r := range rows {
		out[r.Key] = r.AuthInfo
	}
	return out
}

// requireGeneratedAuthInfos asserts every stored authInfo is compliant, is not the retired constant, and
// is not shared with another object.
func requireGeneratedAuthInfos(t *testing.T, want int, stored map[string]string) {
	t.Helper()
	require.Len(t, stored, want)
	seen := make(map[string]struct{}, len(stored))
	for key, a := range stored {
		require.NotEqual(t, retiredEscrowAuthInfo, a, "%s was stored with the retired shared authInfo", key)
		require.NoError(t, entities.AuthInfoType(a).Validate(), "%s was stored with a non-compliant authInfo", key)
		seen[a] = struct{}{}
	}
	require.Len(t, seen, len(stored), "an authInfo was shared between objects")
}

func TestDirectDBImporter_ImportContacts_StoresAGeneratedAuthInfoPerContact(t *testing.T) {
	db := authInfoTestDB(t)
	const tld = "aicontacts"
	clID := seedFixtures(t, db, tld, 91001)
	staged := stagedSQLite(t)

	tx, err := staged.Begin()
	require.NoError(t, err)
	for i := 0; i < directImportRows; i++ {
		_, err := tx.Exec(`INSERT INTO contacts (id, email, clid) VALUES (?, 'someone@example.invalid', ?)`, fmt.Sprintf("%sc%05d", tld, i), clID)
		require.NoError(t, err)
	}
	require.NoError(t, tx.Commit())

	importer, err := services.NewDirectDBImporter()
	require.NoError(t, err)
	t.Cleanup(func() { _ = importer.PG.Close() })
	noop := func(string) {}

	total, _, _, skipped, err := importer.ImportContacts(context.Background(), staged, nil, "", noop)
	require.NoError(t, err)
	require.Equal(t, int64(directImportRows), total)
	require.Zero(t, skipped)

	q := `SELECT id AS key, auth_info FROM contacts WHERE cl_id = ?`
	first := storedAuthInfos(t, db, q, clID)
	requireGeneratedAuthInfos(t, directImportRows, first)

	// Importing the same escrow again is an upsert. It must not mint a new authInfo for a contact that
	// already has one, or a retried batch would silently rotate credentials a registrar may already hold.
	_, _, _, _, err = importer.ImportContacts(context.Background(), staged, nil, "", noop)
	require.NoError(t, err)
	require.Equal(t, first, storedAuthInfos(t, db, q, clID), "re-import replaced stored authInfos")
}

func TestDirectDBImporter_ImportDomains_StoresAGeneratedAuthInfoPerDomain(t *testing.T) {
	db := authInfoTestDB(t)
	const tld = "aidomains"
	clID := seedFixtures(t, db, tld, 91002)
	staged := stagedSQLite(t)

	tx, err := staged.Begin()
	require.NoError(t, err)
	for i := 0; i < directImportRows; i++ {
		_, err := tx.Exec(`INSERT INTO domains (name, clid, exdate) VALUES (?, ?, '2030-01-01T00:00:00Z')`, fmt.Sprintf("d%05d.%s", i, tld), clID)
		require.NoError(t, err)
	}
	require.NoError(t, tx.Commit())

	importer, err := services.NewDirectDBImporter()
	require.NoError(t, err)
	t.Cleanup(func() { _ = importer.PG.Close() })
	noop := func(string) {}

	total, _, _, err := importer.ImportDomains(context.Background(), staged, tld, nil, "", noop)
	require.NoError(t, err)
	require.Equal(t, int64(directImportRows), total)

	q := `SELECT name AS key, auth_info FROM domains WHERE tld_name = ?`
	first := storedAuthInfos(t, db, q, tld)
	requireGeneratedAuthInfos(t, directImportRows, first)

	// As with contacts, a re-import must leave stored authInfos alone.
	_, _, _, err = importer.ImportDomains(context.Background(), staged, tld, nil, "", noop)
	require.NoError(t, err)
	require.Equal(t, first, storedAuthInfos(t, db, q, tld), "re-import replaced stored authInfos")
}
