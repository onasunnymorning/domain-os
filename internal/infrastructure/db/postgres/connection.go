package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net/url"
	"strings"
	"time"

	"github.com/onasunnymorning/domain-os/internal/buildinfo"

	_ "github.com/lib/pq"     // Standard postgres driver (in case we need to create the database)
	"gorm.io/driver/postgres" // Gorm postgres driver
	"gorm.io/gorm"
)

func AutoMigrate(db *gorm.DB) error {
	if err := migrateContactDisclose(db); err != nil {
		return err
	}
	err := db.AutoMigrate(
		&IANARegistrar{},
		&Spec5Label{},
		&RegistryOperator{},
		&TLD{},
		&Phase{},
		&Price{},
		&Fee{},
		&NNDN{},
		&Registrar{},
		&Contact{},
		&Host{},
		&HostAddress{},
		&Domain{},
		&DomainDSRecord{},
		&PremiumList{},
		&PremiumLabel{},
		&FX{},
		&TLDDNSRecord{},
		&DomainEventRecord{},
		&DomainTombstoneRecord{},
		&ZoneSlavingRecord{},
		&SerialCheckRunRecord{},
		&SerialObservationRecord{},
		&EscrowDepositRecord{},
		&EscrowValidationRunRecord{},
		&EscrowSanitizationRunRecord{},
		&EscrowPartyRecord{},
		&EscrowKeyVersionRecord{},
		&EscrowArrangementRecord{},
		&EscrowKeyAuditEventRecord{},
		&EPPLoginRecord{},
		&EPPPasswordRecord{},
		&EPPCertificatePinRecord{},
		&EPPAllowedNetworkRecord{},
		&EPPTransactionRecord{},
		&DomainTransferRecord{},
		&PollMessageRecord{},
		&EPPPolicyRecord{},
		&EPPPolicyAssignmentRecord{},
		&RegistrarFamilyRecord{},
		&RegistrarFamilyMemberRecord{},
		&EPPCreateOutcomeRecord{},
		&RDAPPublicationPolicyRecord{},
		&RDAPPublicationAssignmentRecord{},
		&RDAPAccessPolicyRecord{},
		&RDAPAccessNetworkRecord{},
		&InstallationSetting{},
	)
	if err != nil {
		return err
	}

	if err := fillContactDisclose(db); err != nil {
		return err
	}
	if err := migrateDomainAuthInfo(db); err != nil {
		return err
	}
	if err := migrateLegacyPendingRestores(db); err != nil {
		return err
	}
	if err := seedEPPPolicies(db); err != nil {
		return err
	}
	if err := ensureEPPPolicyForeignKeys(db); err != nil {
		return err
	}
	if err := seedRDAPPublicationPolicies(db); err != nil {
		return err
	}
	if err := seedRDAPAccessPolicies(db); err != nil {
		return err
	}
	if err := ensureRDAPPublicationForeignKeys(db); err != nil {
		return err
	}

	// Manual indexes for tables where GORM tags can't be used (M2M join tables, composites).
	// Using IF NOT EXISTS for idempotency.
	manualIndexes := []string{
		// domain_hosts: reverse lookup by host_ro_id — PK is (domain_ro_id, host_ro_id) so host-only lookups are full table scans
		"CREATE INDEX IF NOT EXISTS idx_domain_hosts_host_ro_id ON domain_hosts (host_ro_id)",
		// hosts: the hosts under a domain (hostUnderDomainSQL). A name's suffix
		// is a prefix of its reverse, and text_pattern_ops lets LIKE 'x%' use
		// the index whatever the database's collation.
		"CREATE INDEX IF NOT EXISTS idx_hosts_name_reverse ON hosts (reverse(name) text_pattern_ops)",
		// phases: composite for activeGAPhaseFilter EXISTS subquery that runs on every lifecycle query
		"CREATE INDEX IF NOT EXISTS idx_phases_tld_type_starts ON phases (tld_name, type, starts)",
		// accreditations: tld_name is second in composite PK, can't be used for tld_name-only GROUP BY
		"CREATE INDEX IF NOT EXISTS idx_accreditations_tld_name ON accreditations (tld_name)",
		// domain_events: global index to support global list of recent events ordered by occurred_at DESC
		"CREATE INDEX IF NOT EXISTS idx_domain_events_occurred_at ON domain_events (occurred_at DESC)",
		// domain_events: type index for type-filtered event search queries
		"CREATE INDEX IF NOT EXISTS idx_domain_events_type ON domain_events (type)",
		// domain_events: partial actor index for actor-filtered event search queries
		"CREATE INDEX IF NOT EXISTS idx_domain_events_actor ON domain_events (actor) WHERE actor != ''",
		// domain_events: partial roid index for roid-filtered event search queries
		"CREATE INDEX IF NOT EXISTS idx_domain_events_roid ON domain_events (ro_id) WHERE ro_id != ''",
		// domain_events: partial index on id for purged events with roid to support tombstone backfill pagination
		"CREATE INDEX IF NOT EXISTS idx_domain_events_purged_id ON domain_events (id) WHERE type = 'domain.purged' AND ro_id != ''",
		// domain_events: partial index on trace_id for trace-filtered event search queries
		"CREATE INDEX IF NOT EXISTS idx_domain_events_trace_id ON domain_events (trace_id) WHERE trace_id != ''",
		// domain_events: partial index on correlation_id for correlation-filtered event search queries
		"CREATE INDEX IF NOT EXISTS idx_domain_events_correlation_id ON domain_events (correlation_id) WHERE correlation_id != ''",
		// escrow_deposits: one record per exact artifact set per tenant/TLD/profile — a replay binds to it, a concurrent duplicate bind fails (issue #412, generalised for #415)
		"CREATE UNIQUE INDEX IF NOT EXISTS uq_escrow_deposits_tenant_tld_profile_digests ON escrow_deposits (tenant_id, tld, profile, artifact_sha256, signature_sha256)",
		// escrow_validation_runs: runs for one deposit, newest first (ListByDeposit)
		"CREATE INDEX IF NOT EXISTS idx_escrow_validation_runs_deposit_started ON escrow_validation_runs (deposit_id, started_at DESC)",
		// escrow_sanitization_runs: one derivative per source run per policy version — a
		// different policy version is a separate, separately traceable derivative and
		// never an overwrite (issue #415)
		"CREATE UNIQUE INDEX IF NOT EXISTS uq_escrow_sanitization_source_policy ON escrow_sanitization_runs (tenant_id, source_validation_run_id, policy_version)",
		// escrow_sanitization_runs: tenant listing, newest first
		"CREATE INDEX IF NOT EXISTS idx_escrow_sanitization_runs_tenant_tld_started ON escrow_sanitization_runs (tenant_id, tld, started_at DESC)",
	}
	manualIndexes = append(manualIndexes, escrowKeyManualIndexes...)
	for _, idx := range manualIndexes {
		if err := db.Exec(idx).Error; err != nil {
			log.Printf("Warning: failed to create index: %s — %v", idx, err)
		}
	}

	// Partial indexes that replace full ones dropped below. Unlike the loop
	// above, a failure here is fatal, and it runs before the drops: a sweep
	// must never lose its index because the replacement failed to build.
	for _, idx := range replacementIndexes {
		if err := db.Exec(idx).Error; err != nil {
			return fmt.Errorf("failed to create replacement index: %s: %w", idx, err)
		}
	}

	// Drop legacy domain_events indexes that have no matching queries in the codebase.
	// These were replaced by a composite (subject, occurred_at DESC) index via GORM tags.
	// GORM AutoMigrate only adds indexes, never removes them, so we clean up manually.
	legacyIndexes := []string{
		"DROP INDEX IF EXISTS idx_domain_events_source",
		"DROP INDEX IF EXISTS idx_domain_events_subject",
		// domains carried single-column indexes on cl_id and tld_name beside the
		// composites above, which serve the same lookups as a left prefix; every
		// domain write maintained both. Their struct tags are gone, and GORM never
		// drops an index, so they go here.
		"DROP INDEX IF EXISTS idx_domains_cl_id",
		"DROP INDEX IF EXISTS idx_domains_tld_name",
		// Replaced by replacementIndexes, which exist by now: the full ones indexed
		// every domain's purge date and restore end, almost all of them zero, and
		// every event's published flag, almost all of them true.
		"DROP INDEX IF EXISTS idx_domains_purge_date",
		"DROP INDEX IF EXISTS idx_domains_pending_restore_end",
		"DROP INDEX IF EXISTS idx_domain_events_published",
		// escrow_deposits carried ryde_*/sig_* columns until #415 generalised the
		// record to Artifact*/Signature* so it can also hold an unsigned .xml or
		// .xml.gz deposit. AutoMigrate adds the new columns but never drops the
		// old ones, and the old unique index goes with them.
		"DROP INDEX IF EXISTS uq_escrow_deposits_tenant_tld_digests",
		"ALTER TABLE escrow_deposits DROP COLUMN IF EXISTS ryde_object_key",
		"ALTER TABLE escrow_deposits DROP COLUMN IF EXISTS ryde_sha256",
		"ALTER TABLE escrow_deposits DROP COLUMN IF EXISTS ryde_bytes",
		"ALTER TABLE escrow_deposits DROP COLUMN IF EXISTS sig_object_key",
		"ALTER TABLE escrow_deposits DROP COLUMN IF EXISTS sig_sha256",
		"ALTER TABLE escrow_deposits DROP COLUMN IF EXISTS sig_bytes",
		// escrow_trusted_keys (#412) was replaced by the key registry (#429,
		// ADR-0009) before it held production data, so it is dropped, not migrated.
		"DROP TABLE IF EXISTS escrow_trusted_keys",
	}
	for _, idx := range legacyIndexes {
		if err := db.Exec(idx).Error; err != nil {
			log.Printf("Warning: failed to run legacy cleanup statement: %s — %v", idx, err)
		}
	}

	// A domain or NNDN name must belong to the TLD it is filed under — see
	// constraints.go for why the database is where that is enforced.
	//
	// Unlike the index loops above, a failure here is fatal: a constraint that
	// was never created looks exactly like one that is holding. The statements
	// are PL/pgSQL, so they only run on Postgres — several unit suites migrate
	// an in-memory SQLite database through this same function.
	if db.Dialector.Name() == "postgres" {
		for _, stmt := range nameUnderTLDConstraints() {
			if err := db.Exec(stmt).Error; err != nil {
				return fmt.Errorf("failed to add name/tld_name constraint: %w", err)
			}
		}
	}

	return nil
}

// replacementIndexes are the indexes that replaced others on the two hottest
// write tables: the composites that cover the dropped single-column ones as
// a left prefix, and the partial ones that replaced full ones. AutoMigrate
// creates them before it drops what they replace, and fails if it cannot.
var replacementIndexes = []string{
	// domains: composite indexes for the expiring/purgeable domain queries filtered by
	// cl_id and tld_name. They also serve every lookup by cl_id alone or by cl_id and
	// tld_name, as a left prefix, which is why the single-column cl_id index could go.
	"CREATE INDEX IF NOT EXISTS idx_domains_clid_tld_expiry ON domains (cl_id, tld_name, expiry_date)",
	"CREATE INDEX IF NOT EXISTS idx_domains_clid_tld_purge ON domains (cl_id, tld_name, purge_date)",
	// domains: the same queries filtered by tld_name only (global/CLI workflows). The
	// first also serves every lookup by tld_name alone (the counts, the zone builds),
	// which is why the single-column tld_name index could go.
	"CREATE INDEX IF NOT EXISTS idx_domains_tld_expiry ON domains (tld_name, expiry_date)",
	"CREATE INDEX IF NOT EXISTS idx_domains_tld_purge ON domains (tld_name, purge_date)",
	// domains: the purge sweep with neither cl_id nor tld (purgeableFilter). Only a
	// pendingDelete domain has a purge date that matters, so the index holds only
	// those rows; the query says "pending_delete = true" in the same words so the
	// planner can match it.
	"CREATE INDEX IF NOT EXISTS idx_domains_pending_delete_purge ON domains (purge_date) WHERE pending_delete = true",
	// domains: the lapsed-restore sweep (ListLapsedRestores). Same shape: a restore
	// is pending on a handful of domains at a time, so the index is tiny.
	"CREATE INDEX IF NOT EXISTS idx_domains_pending_restore_pending_end ON domains (pending_restore_end, ro_id) WHERE pending_restore = true",
	// domain_events: the outbox relay reads only unpublished events, oldest first, and
	// almost every row is published. A full index on the boolean cost a write per
	// event for nothing; this one holds only the backlog.
	"CREATE INDEX IF NOT EXISTS idx_domain_events_unpublished ON domain_events (occurred_at) WHERE published = false",
}

func CreateDB(dbUser, dbPass, dbHost, dbName, dbPort string) error {
	// Connect to the server
	dsn := fmt.Sprintf("host=%s port=%s user=%s password=%s sslmode=require", dbHost, dbPort, dbUser, dbPass)
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return err
	}
	defer db.Close()

	// Create the database
	createDatabaseCommand := fmt.Sprintf("CREATE DATABASE %s", dbName)
	_, err = db.Exec(createDatabaseCommand)
	if err != nil {
		return err
	}

	return nil
}

// Config locates the database and bounds the pool. It has the same fields,
// in the same order, as internal/config.Database, which reads them from
// the environment at the composition root (INV-17): a cmd converts with
// postgres.Config(db). Keep the two in step.
type Config struct {
	// URL is the whole connection string, when known; it takes precedence
	// over the parts below.
	URL         string
	User        string
	Pass        string
	Host        string
	Port        string
	DBName      string
	SSLmode     string
	AutoMigrate bool
	// MaxOpenConns bounds the pool: DefaultMaxOpenConns when zero, never
	// below MinMaxOpenConns.
	MaxOpenConns int
}

// buildDSN assembles a postgres connection URL from cfg. It uses net/url so
// credentials containing URL-reserved characters (:, @, /, ?, #, space, …) —
// which RDS- and Secrets Manager-generated passwords routinely include — are
// percent-escaped rather than corrupting the URL. Interpolating cfg.Pass raw
// makes the parser read the password bytes as the host/port and fail with
// "invalid port ... after host", panicking every deployment on startup.
func buildDSN(cfg Config) string {
	u := url.URL{
		Scheme:   "postgres",
		User:     url.UserPassword(cfg.User, cfg.Pass),
		Host:     fmt.Sprintf("%s:%s", cfg.Host, cfg.Port),
		Path:     "/" + cfg.DBName,
		RawQuery: "sslmode=" + url.QueryEscape(cfg.SSLmode),
	}
	return u.String()
}

// Pool bounds, applied to every connection this package opens. database/sql
// defaults to an unbounded pool that keeps two idle connections: a burst
// opens as many connections as it has goroutines, each with a TLS
// handshake, and drops them again a moment later. On a managed Postgres the
// connection slots are the scarce, priced resource, and one process's burst
// starves the others. Config.MaxOpenConns bounds the pool; the rest follows
// from it.
const (
	// DefaultMaxOpenConns is the bound when Config.MaxOpenConns is zero.
	DefaultMaxOpenConns = 20
	// MinMaxOpenConns is the smallest pool that works: Migrate holds one
	// connection for its advisory lock while AutoMigrate runs on another,
	// so a pool of one hangs every migration for good.
	MinMaxOpenConns = 2
	connMaxLifetime = 30 * time.Minute
	connMaxIdleTime = 5 * time.Minute
)

// poolSize is the bound to apply for Config.MaxOpenConns n: the default for
// zero, n otherwise, and an error below MinMaxOpenConns.
func poolSize(n int) (int, error) {
	switch {
	case n == 0:
		return DefaultMaxOpenConns, nil
	case n < MinMaxOpenConns:
		return 0, fmt.Errorf("a pool of %d connections is below %d, the least a migration needs (one holds its lock while the other runs it)", n, MinMaxOpenConns)
	}
	return n, nil
}

// configurePool bounds db's connection pool to maxOpen.
func configurePool(db *gorm.DB, maxOpen int) error {
	sqlDB, err := db.DB()
	if err != nil {
		return fmt.Errorf("get sql.DB: %w", err)
	}
	n, err := poolSize(maxOpen)
	if err != nil {
		return err
	}
	sqlDB.SetMaxOpenConns(n)
	sqlDB.SetMaxIdleConns(max(2, n/4))
	sqlDB.SetConnMaxLifetime(connMaxLifetime)
	sqlDB.SetConnMaxIdleTime(connMaxIdleTime)
	return nil
}

// open connects to dsn with a pool bounded to maxOpen.
func open(dsn string, maxOpen int) (*gorm.DB, error) {
	db, err := gorm.Open(postgres.Open(dsn))
	if err != nil {
		return nil, err
	}
	if err := configurePool(db, maxOpen); err != nil {
		return nil, err
	}
	return db, nil
}

// dsn is cfg's connection string: URL as given when it is set, otherwise
// one built from the parts.
func dsn(cfg Config) string {
	if cfg.URL != "" {
		return cfg.URL
	}
	return buildDSN(cfg)
}

// OpenExisting connects to an existing database and changes nothing: no
// database is created and nothing is migrated. A read-only process (the
// RDAP server, ADR 0020) uses it, so a mistyped DB_NAME fails at start
// instead of creating an empty database.
func OpenExisting(cfg Config) (*gorm.DB, error) {
	db, err := open(dsn(cfg), cfg.MaxOpenConns)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}
	return db, nil
}

// NewConnection connects to cfg's database, creating it when it does not
// exist (when cfg names it by its parts: a URL is connected to as given)
// and migrating it when cfg.AutoMigrate is set.
func NewConnection(cfg Config) (*gorm.DB, error) {
	dsn := dsn(cfg)
	gormDB, err := open(dsn, cfg.MaxOpenConns)
	if err != nil {
		errMsg := err.Error()
		if cfg.URL == "" && strings.Contains(errMsg, fmt.Sprintf("database %q does not exist", cfg.DBName)) {
			log.Printf("Database '%s' does not exist. Attempting to create it...", cfg.DBName)
			if err := CreateDB(cfg.User, cfg.Pass, cfg.Host, cfg.DBName, cfg.Port); err != nil {
				// Two processes starting against an empty cluster (e.g. two
				// `migrate` runs) both see the database missing; the loser's
				// CREATE DATABASE fails on the catalog's unique index. The
				// database exists either way, so carry on and connect.
				if !isDuplicateDatabaseError(err) {
					log.Println(err)
					return nil, fmt.Errorf("failed to create database: %w", err)
				}
				log.Printf("Database '%s' was created concurrently by another process", cfg.DBName)
			}
			// Retry the connection after creating the database
			gormDB, err = open(dsn, cfg.MaxOpenConns)
			if err != nil {
				// If there is still an issue establishing the connection, return the error
				return nil, fmt.Errorf("failed to connect to database: %w, after creating it", err)
			}
		} else {
			return nil, fmt.Errorf("failed to connect to database: %w", err)
		}
	}

	if cfg.AutoMigrate {
		if err := autoMigrateOnConnect(gormDB); err != nil {
			return gormDB, err
		}
	} else {
		log.Println("Skipping auto migration")
	}

	return gormDB, nil
}

// autoMigrateOnConnect is the AUTO_MIGRATE=true path (local development,
// docker-compose, the registrar importer). It goes through the same Migrate as
// `ryAdminAPI migrate` - advisory lock, AutoMigrate, schema_version - so a
// locally migrated database passes the schema guard like a deployed one, and
// two processes starting together serialise instead of deadlocking.
func autoMigrateOnConnect(gormDB *gorm.DB) error {
	log.Println("Auto migrating database")
	err := Migrate(context.Background(), gormDB, MigrateOptions{
		Version:     buildinfo.Version,
		GitSHA:      buildinfo.GitSHA,
		LockTimeout: 10 * time.Minute,
		Logf:        log.Printf,
	})
	if err != nil {
		return fmt.Errorf("failed to migrate database: %w", err)
	}
	return nil
}

// isDuplicateDatabaseError reports whether err is Postgres refusing a CREATE
// DATABASE because the database already exists - either the clean
// duplicate_database error (42P04) or, when two creates race, the unique
// violation on pg_database's name index.
func isDuplicateDatabaseError(err error) bool {
	msg := err.Error()
	return strings.Contains(msg, "already exists") || strings.Contains(msg, "pg_database_datname_index")
}
