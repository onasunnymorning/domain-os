package activities

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	_ "github.com/lib/pq"
	"github.com/onasunnymorning/domain-os/internal/application/rdevalidate"
	"github.com/onasunnymorning/domain-os/internal/application/rdevalidate/rdetest"
	postgres "github.com/onasunnymorning/domain-os/internal/infrastructure/db/postgres"
	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
	"github.com/stretchr/testify/require"
	pgdriver "gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// testDB connects to the throwaway test Postgres the way the postgres package
// tests do (TEST_DB_HOST/TEST_DB_PORT, CI defaults). It fails rather than
// skips: the side-effect proof must run wherever `make test` runs.
func testDB(t *testing.T) *gorm.DB {
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
		// Fresh container: the postgres package tests create the database on
		// first use, but under a parallel `go test ./...` this package may
		// connect first. Create it the same way and retry.
		admin, aerr := sql.Open("postgres", fmt.Sprintf("host=%s port=%s user=postgres password=unittest sslmode=require", host, port))
		require.NoError(t, aerr)
		_, _ = admin.Exec("CREATE DATABASE dos_unittests")
		_ = admin.Close()
		db, err = gorm.Open(pgdriver.Open(dsn))
	}
	require.NoError(t, err, "test Postgres unavailable; run via `make test` (port 5433)")
	require.NoError(t, postgres.AutoMigrate(db))
	return db
}

// registryCounts snapshots every table an escrow *import* would write.
func registryCounts(t *testing.T, db *gorm.DB) map[string]int64 {
	t.Helper()
	out := map[string]int64{}
	for _, table := range []string{"domains", "contacts", "hosts", "registrars", "nndns", "accreditations", "domain_events"} {
		var n int64
		require.NoError(t, db.Table(table).Count(&n).Error)
		out[table] = n
	}
	return out
}

// TestEscrowValidation_NoRegistryWriteSideEffect is the acceptance-criteria
// proof that validation never imports: a full PASS run against real
// repositories leaves every registry table exactly as it was, and only the
// three escrow tables change.
func TestEscrowValidation_NoRegistryWriteSideEffect(t *testing.T) {
	db := testDB(t)
	// The test database is shared with other packages running in parallel, so
	// the before/after counts must come from one snapshot: REPEATABLE READ
	// isolates this transaction from their concurrent inserts while still
	// seeing its own writes.
	tx := db.Begin(&sql.TxOptions{Isolation: sql.LevelRepeatableRead})
	defer tx.Rollback()

	ryid := "evesidefx"
	ro, err := entities.NewRegistryOperator(ryid, "EVE Side-Effect Operator", "ops@example.com")
	require.NoError(t, err)
	_, err = postgres.NewGORMRegistryOperatorRepository(tx).Create(context.Background(), ro)
	require.NoError(t, err)
	tld, err := entities.NewTLD("evesidefx", ryid)
	require.NoError(t, err)
	tldRepo := postgres.NewGormTLDRepo(tx)
	require.NoError(t, tldRepo.Create(context.Background(), tld))

	before := registryCounts(t, tx)

	f := newEVFixture(t, tldRepo, postgres.NewEscrowDepositRepository(tx), postgres.NewEscrowValidationRunRepository(tx), postgres.NewEscrowTrustedKeyRepository(tx))
	f.scope = entities.OperatorID(ryid)
	f.tld = "evesidefx"
	pair := rdetest.BuildPair(t, rdetest.DepositOpts{TLD: "evesidefx", Domains: 25, Contacts: 10, Hosts: 5, Registrars: 2, NNDNs: 3}, f.service, f.registry)
	rydeKey, sigKey := f.upload(t, "example_2026-09-08_full_S1_R0", pair)

	b, err := f.bind(t, rydeKey, sigKey)
	require.NoError(t, err)
	res := f.validate(t, b)
	require.Equal(t, rdevalidate.OutcomePass, res.Outcome, "findings: %v", res.Findings)
	emitted, err := f.emit(t, b, res)
	require.NoError(t, err)
	require.NoError(t, f.finalize(t, b, res, emitted, ""))

	after := registryCounts(t, tx)
	require.Equal(t, before, after, "a validation run must not touch registry data")

	var runs int64
	require.NoError(t, tx.Table("escrow_validation_runs").Where("deposit_id = ?", b.DepositID).Count(&runs).Error)
	require.Equal(t, int64(1), runs)
	var outcome string
	require.NoError(t, tx.Table("escrow_validation_runs").Select("outcome").Where("id = ?", b.ValidationRunID).Scan(&outcome).Error)
	require.Equal(t, string(entities.EscrowValidationPass), outcome)

	// Nothing was staged on disk either: no sqlite database appeared.
	entries, _ := os.ReadDir(os.TempDir())
	for _, e := range entries {
		info, err := e.Info()
		if err == nil && info.ModTime().After(time.Now().Add(-time.Minute)) {
			require.NotContains(t, e.Name(), ".db", "no staging database may be created by validation")
		}
	}
}
