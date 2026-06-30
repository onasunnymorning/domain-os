package postgres

import (
	"database/sql"
	"fmt"
	"log"
	"strings"

	_ "github.com/lib/pq"     // Standard postgres driver (in case we need to create the database)
	"gorm.io/driver/postgres" // Gorm postgres driver
	"gorm.io/gorm"
)

func AutoMigrate(db *gorm.DB) error {
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
		&PremiumList{},
		&PremiumLabel{},
		&FX{},
		&TLDDNSRecord{},
		&DomainEventRecord{},
	)
	if err != nil {
		return err
	}

	// Manual indexes for tables where GORM tags can't be used (M2M join tables, composites).
	// Using IF NOT EXISTS for idempotency.
	manualIndexes := []string{
		// domain_hosts: reverse lookup by host_ro_id — PK is (domain_ro_id, host_ro_id) so host-only lookups are full table scans
		"CREATE INDEX IF NOT EXISTS idx_domain_hosts_host_ro_id ON domain_hosts (host_ro_id)",
		// phases: composite for activeGAPhaseFilter EXISTS subquery that runs on every lifecycle query
		"CREATE INDEX IF NOT EXISTS idx_phases_tld_type_starts ON phases (tld_name, type, starts)",
		// accreditations: tld_name is second in composite PK, can't be used for tld_name-only GROUP BY
		"CREATE INDEX IF NOT EXISTS idx_accreditations_tld_name ON accreditations (tld_name)",
		// domains: composite index to support expiring/purgeable domain queries filtered by cl_id and tld_name
		"CREATE INDEX IF NOT EXISTS idx_domains_clid_tld_expiry ON domains (cl_id, tld_name, expiry_date)",
		"CREATE INDEX IF NOT EXISTS idx_domains_clid_tld_purge ON domains (cl_id, tld_name, purge_date)",
		// domain_events: global index to support global list of recent events ordered by occurred_at DESC
		"CREATE INDEX IF NOT EXISTS idx_domain_events_occurred_at ON domain_events (occurred_at DESC)",
		// domain_events: type index for type-filtered event search queries
		"CREATE INDEX IF NOT EXISTS idx_domain_events_type ON domain_events (type)",
		// domain_events: partial actor index for actor-filtered event search queries
		"CREATE INDEX IF NOT EXISTS idx_domain_events_actor ON domain_events (actor) WHERE actor != ''",
		// domain_events: partial roid index for roid-filtered event search queries
		"CREATE INDEX IF NOT EXISTS idx_domain_events_roid ON domain_events (ro_id) WHERE ro_id != ''",
	}
	for _, idx := range manualIndexes {
		if err := db.Exec(idx).Error; err != nil {
			log.Printf("Warning: failed to create index: %s — %v", idx, err)
		}
	}

	// Drop legacy domain_events indexes that have no matching queries in the codebase.
	// These were replaced by a composite (subject, occurred_at DESC) index via GORM tags.
	// GORM AutoMigrate only adds indexes, never removes them, so we clean up manually.
	legacyIndexes := []string{
		"DROP INDEX IF EXISTS idx_domain_events_source",
		"DROP INDEX IF EXISTS idx_domain_events_trace_id",
		"DROP INDEX IF EXISTS idx_domain_events_correlation_id",
		"DROP INDEX IF EXISTS idx_domain_events_subject",
	}
	for _, idx := range legacyIndexes {
		if err := db.Exec(idx).Error; err != nil {
			log.Printf("Warning: failed to drop legacy index: %s — %v", idx, err)
		}
	}

	return nil
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

type Config struct {
	User        string
	Pass        string
	Host        string
	Port        string
	DBName      string
	SSLmode     string
	AutoMigrate bool
}

func NewConnection(cfg Config) (*gorm.DB, error) {
	dsn := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=%s", cfg.User, cfg.Pass, cfg.Host, cfg.Port, cfg.DBName, cfg.SSLmode)
	gormDB, err := gorm.Open(postgres.Open(dsn))
	if err != nil {
		errMsg := err.Error()
		if strings.Contains(errMsg, fmt.Sprintf("database \"%s\" does not exist", cfg.DBName)) {
			log.Printf("Database '%s' does not exist. Attempting to create it...", cfg.DBName)
			if err := CreateDB(cfg.User, cfg.Pass, cfg.Host, cfg.DBName, cfg.Port); err != nil {
				log.Println(err)
				return nil, fmt.Errorf("failed to create database: %w", err)
			}
			// Retry the connection after creating the database
			gormDB, err = gorm.Open(postgres.Open(dsn))
			if err != nil {
				// If there is still an issue establishing the connection, return the error
				return nil, fmt.Errorf("failed to connect to database: %w, after creating it", err)
			}
		} else {
			return nil, fmt.Errorf("failed to connect to database: %w", err)
		}
	}

	if cfg.AutoMigrate {
		log.Println("Auto migrating database")
		if err = AutoMigrate(gormDB); err != nil {
			return gormDB, fmt.Errorf("failed to migrate database: %w", err)
		}
	} else {
		log.Println("Skipping auto migration")
	}

	return gormDB, nil
}

// NewConnectionFromURL opens a GORM connection using a single database URL
// (e.g. postgres://user:pass@host:5432/dbname?sslmode=require).
// This is useful for managed database providers like Neon that provide a
// single connection string.
func NewConnectionFromURL(databaseURL string, autoMigrate bool) (*gorm.DB, error) {
	gormDB, err := gorm.Open(postgres.Open(databaseURL))
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	if autoMigrate {
		log.Println("Auto migrating database")
		if err = AutoMigrate(gormDB); err != nil {
			return gormDB, fmt.Errorf("failed to migrate database: %w", err)
		}
	} else {
		log.Println("Skipping auto migration")
	}

	return gormDB, nil
}
