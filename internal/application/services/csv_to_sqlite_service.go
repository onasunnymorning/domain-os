package services

import (
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
	_ "modernc.org/sqlite" // Pure Go SQLite driver
)

// CSVToSQLiteService converts escrow CSV files to SQLite database
type CSVToSQLiteService struct {
	baseFilename string
	db           *sql.DB
}

// NewCSVToSQLiteService creates a new CSV to SQLite conversion service
func NewCSVToSQLiteService(baseFilename string) *CSVToSQLiteService {
	return &CSVToSQLiteService{
		baseFilename: baseFilename,
	}
}

// Variables rather than constants so tests can shrink them; not modified at
// runtime.
var (
	// heartbeatInterval is how often an import phase reports liveness. It must
	// stay comfortably below the HeartbeatTimeout configured for the
	// BuildStagingDatabase activity, since Temporal fails the activity when no
	// heartbeat arrives within that window.
	heartbeatInterval = 30 * time.Second

	// commitBatchSize bounds how many rows accumulate before a commit. Loading a
	// multi-million-row escrow inside one transaction grows the journal without
	// limit and makes the whole build one all-or-nothing unit of work.
	commitBatchSize = 50000
)

const (
	// maxRowWarnings caps per-phase warning output so a systematically malformed
	// file cannot emit millions of log lines.
	maxRowWarnings = 20

	// sqliteCacheKiB sets the per-connection page cache. Negative means KiB
	// rather than pages, so this is ~128 MiB.
	sqliteCacheKiB = -131072
)

// heartbeater throttles progress reports so import loops can call it on every
// row without flooding the Temporal SDK.
type heartbeater struct {
	fn   HeartbeatFunc
	last time.Time
}

// newHeartbeater accepts a nil fn, which makes every call a no-op. The CLI
// converts CSVs without a Temporal activity context and passes nil.
func newHeartbeater(fn HeartbeatFunc) *heartbeater {
	return &heartbeater{fn: fn, last: time.Now()}
}

// beat reports progress if the throttle interval has elapsed since the last one.
func (h *heartbeater) beat(phase string, count int) {
	if h == nil || h.fn == nil || time.Since(h.last) < heartbeatInterval {
		return
	}
	h.last = time.Now()
	h.fn(phase, count)
}

// during runs fn while emitting heartbeats from a background goroutine. Use it
// for work that blocks inside the driver and cannot report progress from the
// inside, such as index builds and aggregate UPDATEs.
func (h *heartbeater) during(phase string, fn func() error) error {
	if h == nil || h.fn == nil {
		return fn()
	}

	interval := heartbeatInterval
	if interval <= 0 {
		interval = time.Millisecond
	}

	done := make(chan struct{})
	stopped := make(chan struct{})
	go func() {
		defer close(stopped)
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				h.fn(phase)
			}
		}
	}()

	err := fn()
	// Wait for the ticker goroutine to exit so no heartbeat can land after this
	// returns; callers must be able to assume fn's phase is over.
	close(done)
	<-stopped
	return err
}

// csvImportSpec describes one CSV file loaded into one table. Every phase runs
// through importCSV so batching, heartbeating and malformed-row handling are
// identical everywhere: a phase that reported no progress is what previously
// stalled BuildStagingDatabase until Temporal timed it out.
type csvImportSpec struct {
	suffix    string // appended to baseFilename, e.g. "-contacts.csv"
	label     string // used in log lines and heartbeat details
	insertSQL string
	minFields int
	bind      func(record []string) []any
}

// escrowImports lists the CSV phases in load order. Contacts come first so the
// downstream domain rows reference contacts that are already present.
func escrowImports() []csvImportSpec {
	return []csvImportSpec{
		{
			suffix:    "-contacts.csv",
			label:     "contacts",
			insertSQL: `INSERT INTO contacts (id, roid, voice, fax, email, clid, crrr, crdate, uprr, "update") VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			minFields: 10,
			bind: func(r []string) []any {
				return []any{r[0], r[1], r[2], r[3], r[4], r[5], r[6], r[7], r[8], r[9]}
			},
		},
		{
			suffix:    "-contactPostalInfo.csv",
			label:     "contact postal info records",
			insertSQL: `INSERT INTO contact_postal_info (contact_id, type, name, org, street1, street2, street3, city, state_province, postal_code, country_code) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			minFields: 11,
			bind: func(r []string) []any {
				return []any{r[0], r[1], r[2], r[3], r[4], r[5], r[6], r[7], r[8], r[9], r[10]}
			},
		},
		{
			suffix:    "-contactStatuses.csv",
			label:     "contact statuses",
			insertSQL: `INSERT INTO contact_statuses (contact_id, status) VALUES (?, ?)`,
			minFields: 2,
			bind: func(r []string) []any {
				return []any{r[0], r[1]}
			},
		},
		{
			suffix:    "-domains.csv",
			label:     "domains",
			insertSQL: `INSERT INTO domains (name, roid, uname, idntableid, originalname, registrant, clid, crrr, crdate, exdate, uprr, "update") VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			minFields: 12,
			bind: func(r []string) []any {
				return []any{strings.ToLower(r[0]), r[1], r[2], r[3], r[4], r[5], r[6], r[7], r[8], r[9], r[10], r[11]}
			},
		},
		{
			suffix:    "-domainNameservers.csv",
			label:     "domain-nameserver relationships",
			insertSQL: `INSERT INTO domain_nameservers (domain_name, nameserver) VALUES (?, ?)`,
			minFields: 2,
			bind: func(r []string) []any {
				return []any{strings.ToLower(r[0]), strings.ToLower(r[1])}
			},
		},
		{
			suffix:    "-hosts.csv",
			label:     "hosts",
			insertSQL: `INSERT INTO hosts (name, roid, clid, crrr, crdate, uprr, "update") VALUES (?, ?, ?, ?, ?, ?, ?)`,
			minFields: 7,
			bind: func(r []string) []any {
				return []any{strings.ToLower(r[0]), r[1], r[2], r[3], r[4], r[5], r[6]}
			},
		},
		{
			suffix:    "-hostAddresses.csv",
			label:     "host addresses",
			insertSQL: `INSERT INTO host_addresses (host_name, ip_address, ip_version) VALUES (?, ?, ?)`,
			minFields: 3,
			bind: func(r []string) []any {
				return []any{strings.ToLower(r[0]), r[1], r[2]}
			},
		},
		{
			suffix:    "-domainStatuses.csv",
			label:     "domain statuses",
			insertSQL: `INSERT INTO domain_statuses (domain_name, status) VALUES (?, ?)`,
			minFields: 2,
			bind: func(r []string) []any {
				return []any{strings.ToLower(r[0]), r[1]}
			},
		},
		{
			suffix:    "-domainRgpStatus.csv",
			label:     "domain RGP statuses",
			insertSQL: `INSERT INTO domain_rgp_statuses (domain_name, rgp_status) VALUES (?, ?)`,
			minFields: 2,
			bind: func(r []string) []any {
				return []any{strings.ToLower(r[0]), r[1]}
			},
		},
		{
			suffix:    "-hostStatuses.csv",
			label:     "host statuses",
			insertSQL: `INSERT INTO host_statuses (host_name, status) VALUES (?, ?)`,
			minFields: 2,
			bind: func(r []string) []any {
				return []any{strings.ToLower(r[0]), r[1]}
			},
		},
		{
			suffix:    "-registrars.csv",
			label:     "registrars",
			insertSQL: `INSERT INTO registrars (id, name, gurid, status, voice, fax, email, url, crdate, "update") VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			minFields: 10,
			bind: func(r []string) []any {
				gurid := 0
				if r[2] != "" {
					if val, err := strconv.Atoi(r[2]); err == nil {
						gurid = val
					}
				}
				return []any{r[0], r[1], gurid, r[3], r[4], r[5], r[6], r[7], r[8], r[9]}
			},
		},
		{
			suffix:    "-registrarPostalInfo.csv",
			label:     "registrar postal info records",
			insertSQL: `INSERT INTO registrar_postal_info (registrar_id, type, street1, street2, street3, city, state_province, postal_code, country_code) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			minFields: 9,
			bind: func(r []string) []any {
				return []any{r[0], r[1], r[2], r[3], r[4], r[5], r[6], r[7], r[8]}
			},
		},
		{
			suffix:    "-nndns.csv",
			label:     "NNDNs",
			insertSQL: `INSERT INTO nndns (aname, uname, idntableid, originalname, namestate, crdate) VALUES (?, ?, ?, ?, ?, ?)`,
			minFields: 6,
			bind: func(r []string) []any {
				return []any{r[0], r[1], r[2], r[3], r[4], r[5]}
			},
		},
	}
}

// sqliteDSN builds a DSN carrying bulk-load pragmas. The staging database is a
// disposable artifact, rebuilt from the escrow CSVs on every attempt and thrown
// away when the activity fails, so durability is worth trading for throughput.
// Under the driver defaults the build is dominated by journal writes and fsyncs,
// which cost far more on container storage than on a local SSD.
func sqliteDSN(dbPath string) string {
	q := url.Values{}
	for _, pragma := range []string{
		"journal_mode(off)",
		"synchronous(off)",
		"temp_store(memory)",
		fmt.Sprintf("cache_size(%d)", sqliteCacheKiB),
	} {
		q.Add("_pragma", pragma)
	}
	return "file:" + dbPath + "?" + q.Encode()
}

// ConvertToSQLite creates an SQLite database from the CSV files. heartbeat may
// be nil when running outside a Temporal activity, as the CLI does.
func (svc *CSVToSQLiteService) ConvertToSQLite(dbPath string, heartbeat HeartbeatFunc) error {
	log.Printf("Converting CSV files to SQLite database: %s", dbPath)

	// Open/create SQLite database
	var err error
	svc.db, err = sql.Open("sqlite", sqliteDSN(dbPath))
	if err != nil {
		return fmt.Errorf("failed to open SQLite database: %w", err)
	}
	defer svc.db.Close()

	// The pragmas above are per-connection, so hold the pool to a single
	// connection. It also keeps the import from contending with itself.
	svc.db.SetMaxOpenConns(1)

	hb := newHeartbeater(heartbeat)

	// Create database schema
	if err := svc.createSchema(); err != nil {
		return fmt.Errorf("failed to create schema: %w", err)
	}

	for _, spec := range escrowImports() {
		if err := svc.importCSV(spec, hb); err != nil {
			return fmt.Errorf("failed to import %s: %w", spec.label, err)
		}
	}

	// Registrar mapping is JSON rather than CSV, and small enough to load in a
	// single transaction. It references no other table, so its position here is
	// not load-bearing.
	if err := svc.withTx(svc.importRegistrarMapping); err != nil {
		return fmt.Errorf("failed to import registrar mapping: %w", err)
	}

	// Create indexes for better query performance
	if err := svc.createIndexes(hb); err != nil {
		return err
	}

	// Enrich registrar object counts from imported data.
	// The registrars CSV doesn't include domain/host/contact counts, so we
	// compute them from the actual data. ResolveRegistrars now uses a LEFT JOIN
	// query instead, but this keeps the DB self-consistent for other consumers.
	if err := svc.enrichRegistrarCounts(hb); err != nil {
		return err
	}

	log.Printf("✅ Successfully created SQLite database with escrow data")
	return nil
}

// withTx runs fn inside a transaction, committing on success.
func (svc *CSVToSQLiteService) withTx(fn func(*sql.Tx) error) error {
	tx, err := svc.db.Begin()
	if err != nil {
		return err
	}
	if err := fn(tx); err != nil {
		tx.Rollback()
		return err
	}
	return tx.Commit()
}

// importCSV streams one CSV file into its table, committing in batches and
// heartbeating on a timer.
func (svc *CSVToSQLiteService) importCSV(spec csvImportSpec, hb *heartbeater) error {
	csvFile := svc.baseFilename + spec.suffix
	if !svc.fileExists(csvFile) {
		log.Printf("⚠️  Skipping %s: %s not found", spec.label, csvFile)
		return nil
	}

	file, err := os.Open(csvFile)
	if err != nil {
		return err
	}
	defer file.Close()

	reader := csv.NewReader(file)
	// Rows are checked against spec.minFields below. Without this the reader
	// aborts the entire file on the first row whose field count differs from
	// the header's, which those per-row checks clearly did not intend.
	reader.FieldsPerRecord = -1

	// Skip header
	if _, err := reader.Read(); err != nil {
		if err == io.EOF {
			return nil
		}
		return err
	}

	batch, err := newInsertBatch(svc.db, spec.insertSQL)
	if err != nil {
		return err
	}
	defer batch.close()

	phase := "importing " + spec.label
	var inserted, skipped, failed int

	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		// Heartbeat from the read loop rather than off successful inserts, so a
		// run of skipped or failing rows still registers as liveness.
		hb.beat(phase, inserted)

		if len(record) < spec.minFields {
			skipped++
			continue
		}

		if err := batch.exec(spec.bind(record)); err != nil {
			failed++
			if failed <= maxRowWarnings {
				log.Printf("Warning: failed to insert %s row (key=%q): %v", spec.label, record[0], err)
			}
			if failed == maxRowWarnings {
				log.Printf("Warning: further %s insert errors suppressed", spec.label)
			}
			continue
		}
		inserted++

		if err := batch.maybeFlush(); err != nil {
			return fmt.Errorf("commit batch after %d rows: %w", inserted, err)
		}
	}

	if err := batch.commit(); err != nil {
		return err
	}

	log.Printf("✅ Imported %d %s", inserted, spec.label)
	if skipped > 0 || failed > 0 {
		log.Printf("⚠️  %s: skipped %d malformed row(s), %d insert error(s)", spec.label, skipped, failed)
	}
	return nil
}

// insertBatch owns the transaction and prepared statement for one import phase,
// committing every commitBatchSize rows so the journal stays bounded.
type insertBatch struct {
	db      *sql.DB
	query   string
	tx      *sql.Tx
	stmt    *sql.Stmt
	pending int
}

func newInsertBatch(db *sql.DB, query string) (*insertBatch, error) {
	b := &insertBatch{db: db, query: query}
	if err := b.begin(); err != nil {
		return nil, err
	}
	return b, nil
}

func (b *insertBatch) begin() error {
	tx, err := b.db.Begin()
	if err != nil {
		return err
	}
	stmt, err := tx.Prepare(b.query)
	if err != nil {
		tx.Rollback()
		return err
	}
	b.tx, b.stmt, b.pending = tx, stmt, 0
	return nil
}

// exec reports only the row-level insert error; callers treat it as a skippable
// bad row, so batch lifecycle errors must not surface here.
func (b *insertBatch) exec(args []any) error {
	if _, err := b.stmt.Exec(args...); err != nil {
		return err
	}
	b.pending++
	return nil
}

// maybeFlush commits and reopens once the batch is full.
func (b *insertBatch) maybeFlush() error {
	if b.pending < commitBatchSize {
		return nil
	}
	if err := b.commit(); err != nil {
		return err
	}
	return b.begin()
}

// commit finalises the open transaction, if any.
func (b *insertBatch) commit() error {
	if b.tx == nil {
		return nil
	}
	tx, stmt := b.tx, b.stmt
	b.tx, b.stmt, b.pending = nil, nil, 0
	stmt.Close()
	return tx.Commit()
}

// close rolls back anything still open; a no-op after a successful commit.
func (b *insertBatch) close() {
	if b.tx == nil {
		return
	}
	tx, stmt := b.tx, b.stmt
	b.tx, b.stmt, b.pending = nil, nil, 0
	stmt.Close()
	tx.Rollback()
}

// createSchema creates the database tables
func (svc *CSVToSQLiteService) createSchema() error {
	schema := `
	-- Core domain data
	CREATE TABLE IF NOT EXISTS domains (
		name TEXT PRIMARY KEY,
		roid TEXT,
		uname TEXT,
		idntableid TEXT,
		originalname TEXT,
		registrant TEXT,
		clid TEXT,
		crrr TEXT,
		crdate TEXT,
		exdate TEXT,
		uprr TEXT,
		"update" TEXT
	);

	-- Domain nameservers (many-to-many relationship)
	CREATE TABLE IF NOT EXISTS domain_nameservers (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		domain_name TEXT NOT NULL,
		nameserver TEXT NOT NULL,
		FOREIGN KEY (domain_name) REFERENCES domains(name)
	);

	-- Domain statuses
	CREATE TABLE IF NOT EXISTS domain_statuses (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		domain_name TEXT NOT NULL,
		status TEXT NOT NULL,
		FOREIGN KEY (domain_name) REFERENCES domains(name)
	);

	-- Domain RGP statuses
	CREATE TABLE IF NOT EXISTS domain_rgp_statuses (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		domain_name TEXT NOT NULL,
		rgp_status TEXT NOT NULL,
		FOREIGN KEY (domain_name) REFERENCES domains(name)
	);

	-- Hosts
	CREATE TABLE IF NOT EXISTS hosts (
		name TEXT PRIMARY KEY,
		roid TEXT,
		clid TEXT,
		crrr TEXT,
		crdate TEXT,
		uprr TEXT,
		"update" TEXT
	);

	-- Host IP addresses
	CREATE TABLE IF NOT EXISTS host_addresses (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		host_name TEXT NOT NULL,
		ip_address TEXT NOT NULL,
		ip_version TEXT,
		FOREIGN KEY (host_name) REFERENCES hosts(name)
	);

	-- Host statuses
	CREATE TABLE IF NOT EXISTS host_statuses (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		host_name TEXT NOT NULL,
		status TEXT NOT NULL,
		FOREIGN KEY (host_name) REFERENCES hosts(name)
	);

	-- Registrar data
	CREATE TABLE IF NOT EXISTS registrars (
		id TEXT PRIMARY KEY,
		name TEXT,
		gurid INTEGER,
		status TEXT,
		voice TEXT,
		fax TEXT,
		email TEXT,
		url TEXT,
		crdate TEXT,
		"update" TEXT,
		domain_count INTEGER DEFAULT 0,
		host_count INTEGER DEFAULT 0,
		contact_count INTEGER DEFAULT 0
	);

	-- Registrar postal information
	CREATE TABLE IF NOT EXISTS registrar_postal_info (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		registrar_id TEXT NOT NULL,
		type TEXT NOT NULL,
		street1 TEXT,
		street2 TEXT,
		street3 TEXT,
		city TEXT,
		state_province TEXT,
		postal_code TEXT,
		country_code TEXT,
		FOREIGN KEY (registrar_id) REFERENCES registrars(id)
	);

	-- Registrar mapping from escrow registrar IDs to mapped registrar clids in our system
	CREATE TABLE IF NOT EXISTS registrar_mapping (
		escrow_id TEXT PRIMARY KEY,
		name TEXT,
		gurid INTEGER,
		registrar_clid TEXT
	);

	-- Contacts (linked contacts only are exported by the analyzer)
	CREATE TABLE IF NOT EXISTS contacts (
		id TEXT PRIMARY KEY,
		roid TEXT,
		voice TEXT,
		fax TEXT,
		email TEXT,
		clid TEXT,
		crrr TEXT,
		crdate TEXT,
		uprr TEXT,
		"update" TEXT
	);

	-- Contact postal info (optional INT/LOC rows)
	CREATE TABLE IF NOT EXISTS contact_postal_info (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		contact_id TEXT NOT NULL,
		type TEXT NOT NULL,
		name TEXT,
		org TEXT,
		street1 TEXT,
		street2 TEXT,
		street3 TEXT,
		city TEXT,
		state_province TEXT,
		postal_code TEXT,
		country_code TEXT,
		FOREIGN KEY (contact_id) REFERENCES contacts(id)
	);

	-- Contact statuses
	CREATE TABLE IF NOT EXISTS contact_statuses (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		contact_id TEXT NOT NULL,
		status TEXT NOT NULL,
		FOREIGN KEY (contact_id) REFERENCES contacts(id)
	);

	-- NNDNs
	CREATE TABLE IF NOT EXISTS nndns (
		aname TEXT PRIMARY KEY,
		uname TEXT,
		idntableid TEXT,
		originalname TEXT,
		namestate TEXT,
		crdate TEXT
	);
	`

	_, err := svc.db.Exec(schema)
	return err
}

// createIndexes creates indexes for better query performance.
// Each index is built inside a background heartbeat: on a multi-million-row
// escrow a single CREATE INDEX can outlast the activity's heartbeat timeout on
// its own, and it reports nothing while it runs.
func (svc *CSVToSQLiteService) createIndexes(hb *heartbeater) error {
	indexes := []string{
		"CREATE INDEX IF NOT EXISTS idx_domain_nameservers_domain ON domain_nameservers(domain_name)",
		"CREATE INDEX IF NOT EXISTS idx_domain_nameservers_ns ON domain_nameservers(nameserver)",
		"CREATE INDEX IF NOT EXISTS idx_domain_statuses_domain ON domain_statuses(domain_name)",
		"CREATE INDEX IF NOT EXISTS idx_domain_rgp_statuses_domain ON domain_rgp_statuses(domain_name)",
		"CREATE INDEX IF NOT EXISTS idx_host_addresses_host ON host_addresses(host_name)",
		"CREATE INDEX IF NOT EXISTS idx_host_addresses_ip ON host_addresses(ip_address)",
		"CREATE INDEX IF NOT EXISTS idx_host_statuses_host ON host_statuses(host_name)",
		"CREATE INDEX IF NOT EXISTS idx_domains_registrant ON domains(registrant)",
		"CREATE INDEX IF NOT EXISTS idx_domains_clid ON domains(clid)",
		"CREATE INDEX IF NOT EXISTS idx_registrar_postal_info_registrar ON registrar_postal_info(registrar_id)",
		"CREATE INDEX IF NOT EXISTS idx_registrars_name ON registrars(name)",
		"CREATE INDEX IF NOT EXISTS idx_registrars_gurid ON registrars(gurid)",
		"CREATE INDEX IF NOT EXISTS idx_registrar_mapping_clid ON registrar_mapping(registrar_clid)",
		// Hosts
		"CREATE INDEX IF NOT EXISTS idx_hosts_clid ON hosts(clid)",
		// Contacts
		"CREATE INDEX IF NOT EXISTS idx_contacts_clid ON contacts(clid)",
		"CREATE INDEX IF NOT EXISTS idx_contact_statuses_contact ON contact_statuses(contact_id)",
		"CREATE INDEX IF NOT EXISTS idx_contact_postal_contact ON contact_postal_info(contact_id)",
	}

	for i, indexSQL := range indexes {
		err := hb.during(fmt.Sprintf("creating index %d/%d", i+1, len(indexes)), func() error {
			_, err := svc.db.Exec(indexSQL)
			return err
		})
		if err != nil {
			return fmt.Errorf("failed to create index: %w", err)
		}
	}

	log.Printf("✅ Created database indexes for fast lookups")
	return nil
}

// enrichRegistrarCounts populates domain_count, host_count, and contact_count
// on the registrars table by counting actual references in the data tables.
// This ensures the counts reflect reality rather than relying on external
// analysis files or stale metadata.
func (svc *CSVToSQLiteService) enrichRegistrarCounts(hb *heartbeater) error {
	// One correlated aggregate per registrar over the full data tables; like the
	// index builds, it blocks without reporting progress.
	err := hb.during("enriching registrar counts", func() error {
		_, err := svc.db.Exec(`
			UPDATE registrars SET
				domain_count = COALESCE((SELECT COUNT(*) FROM domains WHERE clid = registrars.id), 0),
				host_count = COALESCE((SELECT COUNT(*) FROM hosts WHERE clid = registrars.id), 0),
				contact_count = COALESCE((SELECT COUNT(*) FROM contacts WHERE clid = registrars.id), 0)
		`)
		return err
	})
	if err != nil {
		return fmt.Errorf("enrichRegistrarCounts: %w", err)
	}
	log.Printf("✅ Enriched registrar counts from imported data")
	return nil
}

func (svc *CSVToSQLiteService) fileExists(filename string) bool {
	_, err := os.Stat(filename)
	return err == nil
}

// importRegistrarMapping imports registrar mapping from JSON if present
// Expected filename: baseFilename + "-registrarMapping.json" (or "-registrar-map.json" as a fallback)
func (svc *CSVToSQLiteService) importRegistrarMapping(tx *sql.Tx) error {
	// Determine candidate file names
	primary := svc.baseFilename + "-registrarMapping.json"
	fallback := svc.baseFilename + "-registrar-map.json"
	analysis := svc.baseFilename + "-analysis.json"

	var filePath string
	if svc.fileExists(primary) {
		filePath = primary
	} else if svc.fileExists(fallback) {
		filePath = fallback
	} else if svc.fileExists(analysis) {
		// Fallback: extract registrarMapping from the analysis JSON envelope
		f, err := os.Open(analysis)
		if err != nil {
			return err
		}
		defer f.Close()

		// Check if file is empty (0 bytes = no analysis data)
		fi, err := f.Stat()
		if err != nil {
			return err
		}
		if fi.Size() == 0 {
			return nil // empty analysis file, nothing to import
		}

		// Minimal envelope to avoid depending on full struct
		var envelope struct {
			RegistrarMapping entities.RegistrarMapping `json:"registrarMapping"`
		}
		dec := json.NewDecoder(f)
		if err := dec.Decode(&envelope); err != nil {
			if err == io.EOF {
				return nil // no data to parse
			}
			return fmt.Errorf("failed to parse analysis json for registrar mapping: %w", err)
		}

		if len(envelope.RegistrarMapping) == 0 {
			// Nothing to import
			return nil
		}

		// Prepare insert
		stmt, err := tx.Prepare(`INSERT OR REPLACE INTO registrar_mapping (escrow_id, name, gurid, registrar_clid) VALUES (?, ?, ?, ?)`)
		if err != nil {
			return err
		}
		defer stmt.Close()

		inserted := 0
		for escrowID, info := range envelope.RegistrarMapping {
			gurid := int(info.GurID)
			mapped := string(info.RegistrarClID)
			if _, err = stmt.Exec(escrowID, info.Name, gurid, mapped); err != nil {
				log.Printf("Warning: failed to insert registrar mapping %s -> %s: %v", escrowID, mapped, err)
				continue
			}
			inserted++
		}

		log.Printf("✅ Imported %d registrar mapping rows (from analysis.json)", inserted)
		return nil
	} else {
		// Not present; it's optional
		return nil
	}

	// Open and decode JSON
	f, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer f.Close()

	dec := json.NewDecoder(f)
	var mapping entities.RegistrarMapping
	if err := dec.Decode(&mapping); err != nil {
		return fmt.Errorf("failed to parse registrar mapping json (%s): %w", filepath.Base(filePath), err)
	}

	// Prepare insert
	stmt, err := tx.Prepare(`INSERT OR REPLACE INTO registrar_mapping (escrow_id, name, gurid, registrar_clid) VALUES (?, ?, ?, ?)`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	inserted := 0
	for escrowID, info := range mapping {
		gurid := int(info.GurID)
		mapped := string(info.RegistrarClID)
		if _, err = stmt.Exec(escrowID, info.Name, gurid, mapped); err != nil {
			log.Printf("Warning: failed to insert registrar mapping %s -> %s: %v", escrowID, mapped, err)
			continue
		}
		inserted++
	}

	log.Printf("✅ Imported %d registrar mapping rows", inserted)
	return nil
}

// Query helper methods for easy lookups

type DomainInfo struct {
	Name        string
	Registrant  string
	ClID        string
	CreatedDate string
	ExpiryDate  string
	Nameservers []string
	Statuses    []string
	RgpStatuses []string
}

type HostInfo struct {
	Name        string
	ClID        string
	CreatedDate string
	IPAddresses []string
	Statuses    []string
}

// GetDomainInfo retrieves complete domain information including nameservers and statuses
func (svc *CSVToSQLiteService) GetDomainInfo(domainName string) (*DomainInfo, error) {
	if svc.db == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	// Get basic domain info
	var info DomainInfo
	err := svc.db.QueryRow(`
		SELECT name, registrant, clid, crdate, exdate
		FROM domains WHERE name = ?
	`, domainName).Scan(&info.Name, &info.Registrant, &info.ClID, &info.CreatedDate, &info.ExpiryDate)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // Domain not found
		}
		return nil, err
	}

	// Get nameservers
	rows, err := svc.db.Query("SELECT nameserver FROM domain_nameservers WHERE domain_name = ?", domainName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var ns string
		if err := rows.Scan(&ns); err != nil {
			continue
		}
		info.Nameservers = append(info.Nameservers, ns)
	}

	// Get statuses
	rows, err = svc.db.Query("SELECT status FROM domain_statuses WHERE domain_name = ?", domainName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var status string
		if err := rows.Scan(&status); err != nil {
			continue
		}
		info.Statuses = append(info.Statuses, status)
	}

	// Get RGP statuses
	rows, err = svc.db.Query("SELECT rgp_status FROM domain_rgp_statuses WHERE domain_name = ?", domainName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var rgpStatus string
		if err := rows.Scan(&rgpStatus); err != nil {
			continue
		}
		info.RgpStatuses = append(info.RgpStatuses, rgpStatus)
	}

	return &info, nil
}

// GetHostInfo retrieves complete host information including IP addresses and statuses
func (svc *CSVToSQLiteService) GetHostInfo(hostName string) (*HostInfo, error) {
	if svc.db == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	// Get basic host info
	var info HostInfo
	err := svc.db.QueryRow(`
		SELECT name, clid, crdate
		FROM hosts WHERE name = ?
	`, hostName).Scan(&info.Name, &info.ClID, &info.CreatedDate)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // Host not found
		}
		return nil, err
	}

	// Get IP addresses
	rows, err := svc.db.Query("SELECT ip_address FROM host_addresses WHERE host_name = ?", hostName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var ip string
		if err := rows.Scan(&ip); err != nil {
			continue
		}
		info.IPAddresses = append(info.IPAddresses, ip)
	}

	// Get statuses
	rows, err = svc.db.Query("SELECT status FROM host_statuses WHERE host_name = ?", hostName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var status string
		if err := rows.Scan(&status); err != nil {
			continue
		}
		info.Statuses = append(info.Statuses, status)
	}

	return &info, nil
}

// FindDomainsByNameserver finds all domains using a specific nameserver
func (svc *CSVToSQLiteService) FindDomainsByNameserver(nameserver string) ([]string, error) {
	if svc.db == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	rows, err := svc.db.Query("SELECT domain_name FROM domain_nameservers WHERE nameserver = ?", nameserver)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var domains []string
	for rows.Next() {
		var domain string
		if err := rows.Scan(&domain); err != nil {
			continue
		}
		domains = append(domains, domain)
	}

	return domains, nil
}

// RegistrarInfo holds basic registrar information
type RegistrarInfo struct {
	ID         string
	Name       string
	GurID      int
	Status     string
	Voice      string
	Fax        string
	Email      string
	URL        string
	CrDate     string
	Update     string
	PostalInfo []RegistrarPostalInfo
}

// RegistrarPostalInfo holds registrar postal address information
type RegistrarPostalInfo struct {
	Type          string
	Street1       string
	Street2       string
	Street3       string
	City          string
	StateProvince string
	PostalCode    string
	CountryCode   string
}

// GetRegistrarInfo retrieves complete registrar information including postal info
func (svc *CSVToSQLiteService) GetRegistrarInfo(registrarID string) (*RegistrarInfo, error) {
	if svc.db == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	// Get basic registrar info
	var info RegistrarInfo
	err := svc.db.QueryRow(`
		SELECT id, name, gurid, status, voice, fax, email, url, crdate, "update"
		FROM registrars WHERE id = ?
	`, registrarID).Scan(&info.ID, &info.Name, &info.GurID, &info.Status, &info.Voice, &info.Fax, &info.Email, &info.URL, &info.CrDate, &info.Update)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // Registrar not found
		}
		return nil, err
	}

	// Get postal info
	rows, err := svc.db.Query("SELECT type, street1, street2, street3, city, state_province, postal_code, country_code FROM registrar_postal_info WHERE registrar_id = ?", registrarID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var postal RegistrarPostalInfo
		if err := rows.Scan(&postal.Type, &postal.Street1, &postal.Street2, &postal.Street3, &postal.City, &postal.StateProvince, &postal.PostalCode, &postal.CountryCode); err != nil {
			continue
		}
		info.PostalInfo = append(info.PostalInfo, postal)
	}

	return &info, nil
}

// FindRegistrarsByName finds registrars matching a name pattern
func (svc *CSVToSQLiteService) FindRegistrarsByName(namePattern string) ([]RegistrarInfo, error) {
	if svc.db == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	rows, err := svc.db.Query("SELECT id, name, gurid, status, email FROM registrars WHERE name LIKE ?", "%"+namePattern+"%")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var registrars []RegistrarInfo
	for rows.Next() {
		var reg RegistrarInfo
		if err := rows.Scan(&reg.ID, &reg.Name, &reg.GurID, &reg.Status, &reg.Email); err != nil {
			continue
		}
		registrars = append(registrars, reg)
	}

	return registrars, nil
}

// OpenDatabase opens an existing SQLite database for querying
func OpenEscrowDatabase(dbPath string) (*CSVToSQLiteService, error) {
	svc := &CSVToSQLiteService{}
	var err error
	svc.db, err = sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}
	return svc, nil
}

// GetDatabaseStats returns basic statistics about the database
func (svc *CSVToSQLiteService) GetDatabaseStats() (map[string]int, error) {
	if svc.db == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	stats := make(map[string]int)
	tables := []string{"domains", "hosts", "domain_nameservers", "host_addresses", "domain_statuses", "domain_rgp_statuses", "host_statuses", "registrars", "registrar_postal_info"}

	for _, table := range tables {
		var count int
		err := svc.db.QueryRow(fmt.Sprintf("SELECT COUNT(*) FROM %s", table)).Scan(&count)
		if err != nil {
			return nil, err
		}
		stats[table] = count
	}

	return stats, nil
}

// Close closes the database connection
func (svc *CSVToSQLiteService) Close() error {
	if svc.db != nil {
		return svc.db.Close()
	}
	return nil
}
