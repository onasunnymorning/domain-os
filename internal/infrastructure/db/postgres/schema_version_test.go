package postgres

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// These tests share the package test database (TestMain). They own the
// schema_version table and reset it themselves; nothing else in the package
// touches it, but they must not run in parallel with each other.

func resetSchemaVersion(t *testing.T, db *gorm.DB) {
	t.Helper()
	require.NoError(t, db.Migrator().DropTable(&SchemaVersionRecord{}))
}

func setSchemaVersion(t *testing.T, db *gorm.DB, version string) {
	t.Helper()
	require.NoError(t, db.AutoMigrate(&SchemaVersionRecord{}))
	require.NoError(t, db.Save(&SchemaVersionRecord{ID: schemaVersionRowID, Version: version, MigratedAt: time.Now().UTC()}).Error)
}

func TestParseAndCompareVersions(t *testing.T) {
	cases := []struct {
		v       string
		release bool
	}{
		{"0.9.0", true},
		{"v1.2.3", true},
		{"0.10.0-rc.1", true},
		{"1.2.3+build.7", true},
		{"dev", false},
		{"", false},
		{"1.2", false},
		{"1.2.x", false},
		{"unknown", false},
	}
	for _, c := range cases {
		require.Equal(t, c.release, isReleaseVersion(c.v), c.v)
	}

	require.Equal(t, -1, compareVersions("0.9.0", "0.10.0"), "numeric, not lexical")
	require.Equal(t, 1, compareVersions("1.0.0", "0.99.99"))
	require.Equal(t, 0, compareVersions("v0.10.0", "0.10.0-rc.2"), "prefix and pre-release ignored")
}

func TestCheckSchemaVersion(t *testing.T) {
	db := getTestDB()
	t.Cleanup(func() { resetSchemaVersion(t, db) })

	resetSchemaVersion(t, db)
	err := CheckSchemaVersion(db, "0.10.0")
	require.ErrorIs(t, err, ErrSchemaNotMigrated, "no table")

	require.NoError(t, db.AutoMigrate(&SchemaVersionRecord{}))
	err = CheckSchemaVersion(db, "0.10.0")
	require.ErrorIs(t, err, ErrSchemaNotMigrated, "table without a row")

	setSchemaVersion(t, db, "0.9.0")
	err = CheckSchemaVersion(db, "0.10.0")
	require.ErrorIs(t, err, ErrSchemaTooOld)
	require.Contains(t, err.Error(), "schema is at 0.9.0 but this build needs >= 0.10.0")

	require.NoError(t, CheckSchemaVersion(db, "0.9.0"), "equal")
	require.NoError(t, CheckSchemaVersion(db, "0.8.4"), "newer schema = code rollback, allowed")
	require.NoError(t, CheckSchemaVersion(db, "dev"), "non-release builds are not gated")
}

func TestEnforceSchemaGuardModes(t *testing.T) {
	db := getTestDB()
	t.Cleanup(func() { resetSchemaVersion(t, db) })
	setSchemaVersion(t, db, "0.9.0")

	var lines []string
	logf := func(format string, args ...any) { lines = append(lines, format) }

	require.ErrorIs(t, EnforceSchemaGuard(db, "0.10.0", SchemaGuardEnforce, logf), ErrSchemaTooOld)
	require.NoError(t, EnforceSchemaGuard(db, "0.10.0", SchemaGuardWarn, logf))
	require.NoError(t, EnforceSchemaGuard(db, "0.10.0", SchemaGuardOff, logf))
	require.Len(t, lines, 3, "every mode logs exactly one line")

	t.Setenv("SCHEMA_GUARD", "")
	require.Equal(t, SchemaGuardEnforce, SchemaGuardModeFromEnv())
	t.Setenv("SCHEMA_GUARD", "WARN")
	require.Equal(t, SchemaGuardWarn, SchemaGuardModeFromEnv())
	t.Setenv("SCHEMA_GUARD", "of") // typo must not disable the guard
	require.Equal(t, SchemaGuardEnforce, SchemaGuardModeFromEnv())
}

func TestMigrateRecordsVersionAndOnlyMovesForward(t *testing.T) {
	db := getTestDB()
	t.Cleanup(func() { resetSchemaVersion(t, db) })
	resetSchemaVersion(t, db)
	ctx := context.Background()

	require.NoError(t, Migrate(ctx, db, MigrateOptions{Version: "0.10.0", GitSHA: "aaa", LockTimeout: time.Minute}))
	rec, err := readSchemaVersion(db)
	require.NoError(t, err)
	require.Equal(t, "0.10.0", rec.Version)
	require.NoError(t, CheckSchemaVersion(db, "0.10.0"))

	// An older build migrating (a rollback) must not lower the recorded version.
	require.NoError(t, Migrate(ctx, db, MigrateOptions{Version: "0.9.0", GitSHA: "bbb", LockTimeout: time.Minute}))
	rec, err = readSchemaVersion(db)
	require.NoError(t, err)
	require.Equal(t, "0.10.0", rec.Version)
	require.Equal(t, "aaa", rec.GitSHA)

	// A dev build migrates but does not stamp a version.
	require.NoError(t, Migrate(ctx, db, MigrateOptions{Version: "dev", LockTimeout: time.Minute}))
	rec, err = readSchemaVersion(db)
	require.NoError(t, err)
	require.Equal(t, "0.10.0", rec.Version)

	// A newer build moves it forward.
	require.NoError(t, Migrate(ctx, db, MigrateOptions{Version: "0.11.0", GitSHA: "ccc", LockTimeout: time.Minute}))
	rec, err = readSchemaVersion(db)
	require.NoError(t, err)
	require.Equal(t, "0.11.0", rec.Version)
}

func TestMigrateWaitsForTheLock(t *testing.T) {
	db := getTestDB()
	t.Cleanup(func() { resetSchemaVersion(t, db) })
	ctx := context.Background()

	// Hold the lock from another session, as a concurrent migrator would.
	sqlDB, err := db.DB()
	require.NoError(t, err)
	holder, err := sqlDB.Conn(ctx)
	require.NoError(t, err)
	defer holder.Close()
	var got bool
	require.NoError(t, holder.QueryRowContext(ctx, "SELECT pg_try_advisory_lock($1)", migrationLockKey).Scan(&got))
	require.True(t, got)

	err = Migrate(ctx, db, MigrateOptions{Version: "0.10.0", LockTimeout: 300 * time.Millisecond, LockPoll: 50 * time.Millisecond})
	require.True(t, errors.Is(err, ErrMigrationLockTimeout), "got %v", err)

	_, err = holder.ExecContext(ctx, "SELECT pg_advisory_unlock($1)", migrationLockKey)
	require.NoError(t, err)
	require.NoError(t, Migrate(ctx, db, MigrateOptions{Version: "0.10.0", LockTimeout: time.Minute}))
}

func TestConcurrentMigrationsSerialise(t *testing.T) {
	db := getTestDB()
	t.Cleanup(func() { resetSchemaVersion(t, db) })
	resetSchemaVersion(t, db)

	var wg sync.WaitGroup
	errs := make([]error, 3)
	for i := range errs {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			errs[i] = Migrate(context.Background(), db, MigrateOptions{
				Version: "0.10.0", LockTimeout: 2 * time.Minute, LockPoll: 100 * time.Millisecond,
			})
		}(i)
	}
	wg.Wait()
	for i, err := range errs {
		require.NoError(t, err, "migrator %d", i)
	}
	require.NoError(t, CheckSchemaVersion(db, "0.10.0"))
}
