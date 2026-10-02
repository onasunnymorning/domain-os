package postgres

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Schema migrations are owned by exactly one code path - Migrate - and every
// long-running service checks, at startup, that the schema it is about to use
// is at least as new as its own code (CheckSchemaVersion / EnforceSchemaGuard).
// See docs/adr/0010-database-migrations.md.
//
// The shape is "one migrator, guarded consumers":
//
//   - `ryAdminAPI migrate` (and AUTO_MIGRATE=true for local development) runs
//     Migrate: GORM AutoMigrate under a Postgres advisory lock, then records the
//     build version in schema_version.
//   - admin-api, unified-worker and mcp-server refuse to start when
//     schema_version is missing or older than their build version.
//
// AutoMigrate is expand-only (it adds tables, columns and indexes; it never
// drops or renames), so a schema that is NEWER than the running code is fine -
// that is a code rollback, and it is allowed. Only an OLDER schema is refused.

// SchemaVersionRecord is the single row recording which build last migrated
// the schema.
type SchemaVersionRecord struct {
	ID         int    `gorm:"primaryKey;autoIncrement:false"`
	Version    string `gorm:"not null"`
	GitSHA     string
	MigratedAt time.Time `gorm:"not null"`
}

// TableName pins the table name; GORM would otherwise pluralise it.
func (SchemaVersionRecord) TableName() string { return "schema_version" }

// schemaVersionRowID is the primary key of the one row schema_version holds.
const schemaVersionRowID = 1

// migrationLockKey is the Postgres advisory-lock key every migrator takes.
// Any fixed int64 works as long as nothing else in the database uses it; this
// one is the ASCII bytes of "dosmigr" so it is recognisable in pg_locks.
const migrationLockKey int64 = 0x646f736d696772

// ErrMigrationLockTimeout is returned when another migrator held the lock for
// the whole wait.
var ErrMigrationLockTimeout = errors.New("another migration holds the schema lock")

// MigrateOptions configures Migrate.
type MigrateOptions struct {
	// Version and GitSHA are the build being migrated to (buildinfo values).
	Version string
	GitSHA  string
	// LockTimeout bounds how long to wait for another migrator to finish.
	LockTimeout time.Duration
	// LockPoll is how often to retry the lock while waiting. Defaults to 2s.
	LockPoll time.Duration
	// Logf receives progress lines. Defaults to a no-op.
	Logf func(format string, args ...any)
}

// Migrate brings the schema up to date for this build. It is safe to run
// concurrently (an advisory lock serialises migrators) and idempotent (a second
// run applies nothing and leaves the recorded version alone unless this build
// is newer).
//
// The recorded version only ever moves forward: running an OLDER build's
// migrate - e.g. after a code rollback - applies its additive changes, which
// are a subset of what is already there, and keeps the newer version, so the
// newer services it would otherwise lock out keep starting.
//
// Non-release builds (Version "dev" or not MAJOR.MINOR.PATCH) migrate but do
// not record a version: a local build must not stamp a shared database with a
// version no release corresponds to.
func Migrate(ctx context.Context, db *gorm.DB, opts MigrateOptions) error {
	logf := opts.Logf
	if logf == nil {
		logf = func(string, ...any) {}
	}
	poll := opts.LockPoll
	if poll <= 0 {
		poll = 2 * time.Second
	}

	sqlDB, err := db.DB()
	if err != nil {
		return fmt.Errorf("get sql.DB: %w", err)
	}

	// Advisory locks belong to a session, so hold one dedicated connection for
	// the duration. AutoMigrate itself may use other pool connections; the lock
	// only has to exclude other migrators, which all try to take it first.
	conn, err := sqlDB.Conn(ctx)
	if err != nil {
		return fmt.Errorf("open lock connection: %w", err)
	}
	defer conn.Close()

	logf("waiting for schema migration lock (key %d, timeout %s)", migrationLockKey, opts.LockTimeout)
	deadline := time.Now().Add(opts.LockTimeout)
	for {
		var got bool
		if err := conn.QueryRowContext(ctx, "SELECT pg_try_advisory_lock($1)", migrationLockKey).Scan(&got); err != nil {
			return fmt.Errorf("try advisory lock: %w", err)
		}
		if got {
			break
		}
		if time.Now().After(deadline) {
			return ErrMigrationLockTimeout
		}
		logf("schema migration lock is held by another migrator; retrying in %s", poll)
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(poll):
		}
	}
	defer func() {
		// Best effort: the lock is also released when the connection closes.
		_, _ = conn.ExecContext(context.Background(), "SELECT pg_advisory_unlock($1)", migrationLockKey)
	}()
	logf("schema migration lock acquired")

	started := time.Now()
	if err := AutoMigrate(db); err != nil {
		return fmt.Errorf("auto-migrate: %w", err)
	}
	if err := db.AutoMigrate(&SchemaVersionRecord{}); err != nil {
		return fmt.Errorf("auto-migrate schema_version: %w", err)
	}
	logf("schema changes applied in %s", time.Since(started).Round(time.Millisecond))

	if !isReleaseVersion(opts.Version) {
		logf("build version %q is not a release; schema_version left unchanged", opts.Version)
		return nil
	}

	current, err := readSchemaVersion(db)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return fmt.Errorf("read schema_version: %w", err)
	}
	if err == nil && isReleaseVersion(current.Version) && compareVersions(current.Version, opts.Version) > 0 {
		logf("schema_version stays at %s (newer than this build, %s)", current.Version, opts.Version)
		return nil
	}

	rec := SchemaVersionRecord{
		ID:         schemaVersionRowID,
		Version:    opts.Version,
		GitSHA:     opts.GitSHA,
		MigratedAt: time.Now().UTC(),
	}
	if err := db.Clauses(clause.OnConflict{UpdateAll: true}).Create(&rec).Error; err != nil {
		return fmt.Errorf("record schema_version: %w", err)
	}
	logf("schema_version recorded as %s (%s)", rec.Version, rec.GitSHA)
	return nil
}

// ErrSchemaNotMigrated means the database has never been migrated by Migrate.
var ErrSchemaNotMigrated = errors.New("schema_version table is missing")

// ErrSchemaTooOld means the schema predates this build.
var ErrSchemaTooOld = errors.New("schema is older than this build")

// CheckSchemaVersion reports whether the schema is new enough for a build of
// buildVersion. It returns nil for non-release builds (local development is not
// gated), nil when the recorded version is the same or newer, and a wrapped
// ErrSchemaNotMigrated / ErrSchemaTooOld otherwise.
func CheckSchemaVersion(db *gorm.DB, buildVersion string) error {
	if !isReleaseVersion(buildVersion) {
		return nil
	}
	if !db.Migrator().HasTable(&SchemaVersionRecord{}) {
		return fmt.Errorf("%w: this build (%s) needs a migrated schema - run the migrate step (`ryAdminAPI migrate`)", ErrSchemaNotMigrated, buildVersion)
	}
	rec, err := readSchemaVersion(db)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return fmt.Errorf("%w: schema_version has no row; this build (%s) needs a migrated schema - run the migrate step (`ryAdminAPI migrate`)", ErrSchemaNotMigrated, buildVersion)
	}
	if err != nil {
		return fmt.Errorf("read schema_version: %w", err)
	}
	if !isReleaseVersion(rec.Version) {
		// Stamped by something other than a release build; nothing to compare.
		return nil
	}
	if compareVersions(rec.Version, buildVersion) < 0 {
		return fmt.Errorf("%w: schema is at %s but this build needs >= %s - run the migrate step (`ryAdminAPI migrate`) before rolling services", ErrSchemaTooOld, rec.Version, buildVersion)
	}
	return nil
}

// SchemaGuardMode is the SCHEMA_GUARD setting.
type SchemaGuardMode string

// Schema guard modes. Enforce is the default; Off exists as an emergency escape
// hatch and should not be set in a deployed environment.
const (
	SchemaGuardEnforce SchemaGuardMode = "enforce"
	SchemaGuardWarn    SchemaGuardMode = "warn"
	SchemaGuardOff     SchemaGuardMode = "off"
)

// SchemaGuardModeFromEnv reads SCHEMA_GUARD, defaulting to enforce. Unknown
// values are treated as enforce: a typo must not silently disable the check.
func SchemaGuardModeFromEnv() SchemaGuardMode {
	switch SchemaGuardMode(strings.ToLower(strings.TrimSpace(os.Getenv("SCHEMA_GUARD")))) {
	case SchemaGuardWarn:
		return SchemaGuardWarn
	case SchemaGuardOff:
		return SchemaGuardOff
	default:
		return SchemaGuardEnforce
	}
}

// EnforceSchemaGuard runs CheckSchemaVersion according to mode. With enforce it
// returns the check's error (callers exit); with warn it logs and returns nil;
// with off it skips the check. logf receives one line in every mode, so the
// startup log always says what happened.
func EnforceSchemaGuard(db *gorm.DB, buildVersion string, mode SchemaGuardMode, logf func(format string, args ...any)) error {
	if logf == nil {
		logf = func(string, ...any) {}
	}
	if mode == SchemaGuardOff {
		logf("schema guard: off (SCHEMA_GUARD=off) - not checking schema_version")
		return nil
	}
	if !isReleaseVersion(buildVersion) {
		logf("schema guard: skipped for non-release build %q", buildVersion)
		return nil
	}
	err := CheckSchemaVersion(db, buildVersion)
	switch {
	case err == nil:
		logf("schema guard: schema is compatible with build %s", buildVersion)
		return nil
	case mode == SchemaGuardWarn:
		logf("schema guard: WARNING (SCHEMA_GUARD=warn, continuing): %v", err)
		return nil
	default:
		logf("schema guard: refusing to start: %v", err)
		return err
	}
}

func readSchemaVersion(db *gorm.DB) (SchemaVersionRecord, error) {
	var rec SchemaVersionRecord
	err := db.Where("id = ?", schemaVersionRowID).Take(&rec).Error
	return rec, err
}

// isReleaseVersion reports whether v looks like MAJOR.MINOR.PATCH, optionally
// with a leading "v" and a pre-release or build suffix.
func isReleaseVersion(v string) bool {
	_, ok := parseVersion(v)
	return ok
}

// compareVersions returns -1, 0 or 1 comparing MAJOR.MINOR.PATCH only.
// Pre-release suffixes are ignored: a release's schema is its MAJOR.MINOR.PATCH.
// Both arguments must be release versions.
func compareVersions(a, b string) int {
	pa, _ := parseVersion(a)
	pb, _ := parseVersion(b)
	for i := 0; i < 3; i++ {
		switch {
		case pa[i] < pb[i]:
			return -1
		case pa[i] > pb[i]:
			return 1
		}
	}
	return 0
}

func parseVersion(v string) ([3]int, bool) {
	var out [3]int
	v = strings.TrimPrefix(strings.TrimSpace(v), "v")
	if i := strings.IndexAny(v, "-+"); i >= 0 {
		v = v[:i]
	}
	parts := strings.Split(v, ".")
	if len(parts) != 3 {
		return out, false
	}
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 {
			return out, false
		}
		out[i] = n
	}
	return out, true
}
