package services

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/netip"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/go-pg/pg/v10"
	"github.com/onasunnymorning/domain-os/internal/application/commands"
	dbModels "github.com/onasunnymorning/domain-os/internal/infrastructure/db/postgres"
	"github.com/onasunnymorning/domain-os/internal/infrastructure/snowflakeidgenerator"
	"github.com/onasunnymorning/domain-os/internal/infrastructure/storage"
	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
)

// RunReport collects details about the import execution
type RunReport struct {
	Timestamp        time.Time      `json:"timestamp"`
	SkippedHosts     []SkippedItem  `json:"skipped_hosts"`
	ModifiedDomains  []ModifiedItem `json:"modified_domains"`
	SkippedDSRecords []SkippedItem  `json:"skipped_ds_records"`
}

type SkippedItem struct {
	Name   string `json:"name"`
	Reason string `json:"reason"`
}

type ModifiedItem struct {
	Name          string `json:"name"`
	Field         string `json:"field"`
	OriginalValue string `json:"original_value"`
	Info          string `json:"info"`
}

// Merge appends other's items to r. The importers collect a batch's items
// apart and merge them once the batch commits, so a batch that rolls back and
// is retried is not reported twice.
func (r *RunReport) Merge(other RunReport) {
	r.SkippedHosts = append(r.SkippedHosts, other.SkippedHosts...)
	r.ModifiedDomains = append(r.ModifiedDomains, other.ModifiedDomains...)
	r.SkippedDSRecords = append(r.SkippedDSRecords, other.SkippedDSRecords...)
}

// DirectDBImporter handles high-speed imports directly into Postgres
type DirectDBImporter struct {
	PG    *pg.DB
	S3    *storage.S3Client
	IDGen *snowflakeidgenerator.IDGenerator
	// Repository is the repository ID the importer mints ROIDs with: every
	// imported object gets a new ROID of this registry's (ADR 0017).
	Repository entities.RepositoryID
	Report     *RunReport

	batchSize int // rows per committed batch in the cursor-paged phases; 0 is defaultImportBatchSize
}

const defaultImportBatchSize = 5000

func (s *DirectDBImporter) rowsPerBatch() int {
	if s.batchSize > 0 {
		return s.batchSize
	}
	return defaultImportBatchSize
}

// DefaultImportConns is the connections an importer opens when its caller
// sets no budget: bulk upserts need a few, never many.
const DefaultImportConns = 5

// ImportPool is the importer's own connection pool (its driver is go-pg,
// apart from the process's GORM pool). A process opens one and shares it
// across every importer it builds, so concurrent imports draw on one bounded
// pool rather than one each.
type ImportPool struct{ db *pg.DB }

// Close closes the pool.
func (p *ImportPool) Close() error { return p.db.Close() }

// NewDirectDBImporter connects an importer that mints ROIDs with repository,
// the process's configured repository ID (config.LoadRepositoryID), to the
// database the environment names: DATABASE_URL (Neon/Render), else the
// DB_* variables (the production credential path on AWS ECS, also used
// locally), on a pool of its own that the caller closes (importer.PG). The
// CLI import path uses it; the worker opens one ImportPool and builds its
// importers on it with NewDirectDBImporterWith.
func NewDirectDBImporter(repository entities.RepositoryID) (*DirectDBImporter, error) {
	pgURL := os.Getenv("DATABASE_URL")
	if pgURL == "" {
		pgURL = fallbackPGURL()
	}
	pool, err := OpenImportPool(pgURL, DefaultImportConns)
	if err != nil {
		return nil, err
	}
	return NewDirectDBImporterWith(repository, pool)
}

// NewDirectDBImporterWith builds an importer on pool, which it does not own:
// the caller closes it, after every importer built on it is done.
func NewDirectDBImporterWith(repository entities.RepositoryID, pool *ImportPool) (*DirectDBImporter, error) {
	if err := repository.Validate(); err != nil {
		return nil, fmt.Errorf("direct importer: %w", err)
	}
	if pool == nil || pool.db == nil {
		return nil, errors.New("direct importer: no import pool")
	}
	return newDirectDBImporter(repository, pool.db)
}

// OpenImportPool connects to the database at pgURL with a pool of at most
// conns connections (DefaultImportConns when conns is not positive).
func OpenImportPool(pgURL string, conns int) (*ImportPool, error) {
	if conns <= 0 {
		conns = DefaultImportConns
	}

	opt, err := pg.ParseURL(sanitizePGURLForGoPG(pgURL))
	if err != nil {
		return nil, fmt.Errorf("failed to parse PG URL: %w", err)
	}

	// Neon requires SNI to route connections to the correct endpoint.
	// go-pg doesn't set TLSConfig.ServerName automatically (unlike pgx),
	// so we must set it explicitly from the parsed host address.
	if opt.TLSConfig != nil && opt.TLSConfig.ServerName == "" {
		// opt.Addr is "host:port" — extract just the host for SNI
		host := opt.Addr
		if idx := strings.LastIndex(host, ":"); idx > 0 {
			host = host[:idx]
		}
		opt.TLSConfig.ServerName = host
	}

	// Pool & resilience tuning for bulk import workloads.
	// An import only needs a few connections; timeouts prevent indefinite stalls.
	opt.PoolSize = conns
	opt.MinIdleConns = 1
	opt.DialTimeout = 10 * time.Second
	opt.ReadTimeout = 90 * time.Second // Bulk upserts with 5K+ rows can take time
	opt.WriteTimeout = 90 * time.Second
	opt.PoolTimeout = 30 * time.Second
	opt.MaxRetries = 3               // Retry transient connection failures
	opt.RetryStatementTimeout = true // Retry on statement_timeout errors too

	return &ImportPool{db: pg.Connect(opt)}, nil
}

// newDirectDBImporter assembles an importer on db with its S3 client, ID
// generator and an empty run report.
func newDirectDBImporter(repository entities.RepositoryID, db *pg.DB) (*DirectDBImporter, error) {
	// S3 Client is optional for some use-cases (like local CLI import)
	s3c, err := storage.NewS3ClientFromEnv()
	if err != nil {
		// Non-fatal: ImportToDirectDB does not use S3. Methods that do will
		// fail on the nil client, so surface the reason here rather than
		// letting it show up later as an unexplained nil dereference.
		log.Printf("Warning: failed to init S3 client, S3-backed methods will not work: %v", err)
	}

	idGen, err := snowflakeidgenerator.NewIDGenerator()
	if err != nil {
		return nil, fmt.Errorf("failed to init id generator: %w", err)
	}

	return &DirectDBImporter{
		PG:         db,
		S3:         s3c,
		IDGen:      idGen,
		Repository: repository,
		Report: &RunReport{
			Timestamp:        time.Now(),
			SkippedHosts:     []SkippedItem{},
			ModifiedDomains:  []ModifiedItem{},
			SkippedDSRecords: []SkippedItem{},
		},
	}, nil
}

// SaveReport writes the run report to the specified path
func (s *DirectDBImporter) SaveReport(path string) error {
	data, err := json.MarshalIndent(s.Report, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}

func getEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func parseJiscTime(s string) (time.Time, error) {
	if s == "" {
		return time.Time{}, fmt.Errorf("empty string")
	}
	// Try RFC3339 first
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t, nil
	}
	// Try SQL format seen in JISC export (YYYY-MM-DD HH:MM:SS)
	if t, err := time.Parse("2006-01-02 15:04:05", s); err == nil {
		// Assume UTC if generic
		return t.UTC(), nil
	}
	return time.Time{}, fmt.Errorf("unknown format")
}

// --- Import Contacts ---

// ImportContacts bulk-upserts contacts from a staged SQLite DB into Postgres.
// Returns (total, inserted, updated, skipped, error).
// skipped counts contacts that were present in the staged DB but excluded due to
// unmapped CLIDs or RoID generation failures — these will NOT be in Postgres, so
// any domain referencing them as registrant/admin/tech/billing will fail the FK gate.
//
// It starts after from (the zero cursor for a fresh run) with from's counts,
// and reports a cursor to heartbeat after every committed batch.
func (s *DirectDBImporter) ImportContacts(ctx context.Context, sqliteDB *sql.DB, clidMap map[string]string, from ImportCursor, heartbeat func(ImportCursor)) (int64, int64, int64, int64, error) {
	from, restarted := ResumeCursor(PhaseContacts, from)
	if restarted {
		log.Printf("IngestContacts: cannot resume exactly from a legacy cursor; starting over")
	}
	batchSize := s.rowsPerBatch()
	lastKey := from.LastKey
	total := from.Processed
	skipped := from.Skipped
	var inserted int64
	var updated int64

	// Count total
	var totalRows int64
	if err := sqliteDB.QueryRow("SELECT COUNT(*) FROM contacts").Scan(&totalRows); err != nil {
		log.Printf("IngestContacts: Failed to count total rows: %v", err)
	}

	for {
		rows, err := sqliteDB.Query(`SELECT id, roid, voice, fax, email, clid, crrr, crdate, uprr, "update" FROM contacts WHERE id > ? ORDER BY id LIMIT ?`, lastKey, batchSize)
		if err != nil {
			log.Printf("IngestContacts: Query failed: %v", err)
			return total, inserted, updated, skipped, err
		}

		var batch []*dbModels.Contact
		currentBatchMaxKey := ""

		rowCount := 0
		for rows.Next() {
			rowCount++
			var id, roid, voice, fax, email, clid, crrr, crdate, uprr, upDate sql.NullString
			if err := rows.Scan(&id, &roid, &voice, &fax, &email, &clid, &crrr, &crdate, &uprr, &upDate); err != nil {
				rows.Close()
				log.Printf("IngestContacts: Scan failed: %v", err)
				return total, inserted, updated, skipped, err
			}
			currentBatchMaxKey = id.String

			// Strict Mapping Logic
			mapClid := func(r string) (string, bool) {
				tr := strings.TrimSpace(r)
				if tr == "" {
					return tr, true
				}
				if clidMap == nil {
					return tr, true
				}
				if v, ok := clidMap[tr]; ok && v != "" {
					return v, true
				}
				return "", false
			}

			mappedClID, ok := mapClid(clid.String)
			if !ok {
				// Contact excluded: unmapped CLID — will NOT appear in Postgres.
				// Any domain referencing this contact as registrant/admin/tech/billing
				// will trigger a FK violation at IngestDomains time.
				skipped++
				continue
			}

			// A deposit carries no authInfo, so mint one per contact. The upsert below never sets
			// auth_info on conflict, so re-running a batch cannot replace one already stored.
			authInfo, aerr := entities.GenerateAuthInfo()
			if aerr != nil {
				rows.Close()
				return total, inserted, updated, skipped, fmt.Errorf("generate authInfo for contact import: %w", aerr)
			}

			c := &entities.Contact{
				ID:        entities.ClIDType(id.String),
				RoID:      entities.RoidType(roid.String),
				Email:     email.String,
				ClID:      entities.ClIDType(mappedClID),
				AuthInfo:  authInfo,
				CreatedAt: time.Now().UTC(),
				UpdatedAt: time.Now().UTC(),
			}

			// Parse dates
			if crdate.Valid {
				if t, err := parseJiscTime(crdate.String); err == nil {
					c.CreatedAt = t
				}
			}
			if upDate.Valid {
				if t, err := parseJiscTime(upDate.String); err == nil {
					c.UpdatedAt = t
				}
			}
			// Map refs
			if crrr.Valid {
				if m, ok := mapClid(crrr.String); ok {
					c.CrRr = entities.ClIDType(m)
				}
			}
			if uprr.Valid {
				if m, ok := mapClid(uprr.String); ok {
					c.UpRr = entities.ClIDType(m)
				}
			}
			if voice.Valid {
				c.Voice = entities.E164Type(voice.String)
			}
			if fax.Valid {
				c.Fax = entities.E164Type(fax.String)
			}

			// Generate valid RoID for persistence
			newID := s.IDGen.GenerateID()
			if r, err := entities.NewRoidType(newID, entities.RoidTypeContact, s.Repository); err == nil {
				c.RoID = r
			} else {
				// Should not happen, but safeguard — still counts as a skip
				skipped++
				continue
			}

			batch = append(batch, dbModels.ToDBContact(c))
		}
		rows.Close()

		if len(batch) == 0 {
			if currentBatchMaxKey == "" {
				break
			}
		}

		if len(batch) > 0 {
			batchLen := int64(len(batch))

			// Upsert inside a transaction — batches WAL writes.
			var res pg.Result
			txErr := s.PG.RunInTransaction(ctx, func(tx *pg.Tx) error {
				var err error
				res, err = tx.Model(&batch).
					ExcludeColumn("doms_where_registrant", "doms_where_admin", "doms_where_tech", "doms_where_billing").
					OnConflict("(id) DO UPDATE").
					Set("voice = EXCLUDED.voice, fax = EXCLUDED.fax, email = EXCLUDED.email, cl_id = EXCLUDED.cl_id, cr_rr = EXCLUDED.cr_rr, up_rr = EXCLUDED.up_rr, updated_at = EXCLUDED.updated_at").
					Insert()
				return err
			})
			if txErr != nil {
				return total, inserted, updated, skipped, fmt.Errorf("bulk upsert contacts failed: %w", txErr)
			}
			affected := int64(res.RowsAffected())
			if affected < batchLen {
				inserted += affected
			} else {
				inserted += batchLen
			}
			total += batchLen
		}

		if (total)%10000 == 0 {
			log.Printf("IngestContacts: Processed %d / %d records (Inserted: %d, Updated: %d, Skipped: %d)", total, totalRows, inserted, updated, skipped)
		}

		lastKey = currentBatchMaxKey
		if lastKey == "" && len(batch) > 0 {
			lastKey = batch[len(batch)-1].ID
		}

		heartbeat(ImportCursor{Version: ImportCursorVersion, LastKey: lastKey, Processed: total, Total: totalRows, Skipped: skipped})
	}
	log.Printf("IngestContacts: Finished. Total: %d, Inserted: %d, Updated: %d, Skipped: %d", total, inserted, updated, skipped)
	return total, inserted, updated, skipped, nil
}

// --- Import Hosts ---

// ImportHosts bulk-upserts hosts from a staged SQLite DB into Postgres, paging
// on (name, clid). Returns (total, inserted, updated, error). It starts after
// from (the zero cursor for a fresh run) with from's counts, and reports a
// cursor to heartbeat after every committed batch.
func (s *DirectDBImporter) ImportHosts(ctx context.Context, sqliteDB *sql.DB, clidMap map[string]string, from ImportCursor, heartbeat func(ImportCursor)) (int64, int64, int64, error) {
	from, restarted := ResumeCursor(PhaseHosts, from)
	if restarted {
		log.Printf("IngestHosts: cannot resume exactly from a legacy cursor; starting over")
	}
	batchSize := s.rowsPerBatch()
	lastKey := from.LastKey
	lastClID := from.LastClID // Secondary cursor for stable pagination across same-name hosts
	total := from.Processed
	var inserted int64
	var updated int64

	// Count total
	var totalRows int64
	if err := sqliteDB.QueryRow("SELECT COUNT(*) FROM hosts").Scan(&totalRows); err != nil {
		log.Printf("IngestHosts: Failed to count total rows: %v", err)
	}

	for {
		rows, err := sqliteDB.Query(`SELECT name, clid, crrr, uprr FROM hosts WHERE (name, clid) > (?, ?) ORDER BY name, clid LIMIT ?`, lastKey, lastClID, batchSize)
		if err != nil {
			log.Printf("IngestHosts: Query failed: %v", err)
			return total, inserted, updated, err
		}

		var entitiesBatch []*entities.Host
		var batchReport RunReport

		type rawHost struct {
			Name, ClID, CrRr, UpRr string
		}
		var rawHosts []rawHost

		for rows.Next() {
			var name, clid, crrr, uprr sql.NullString
			if err := rows.Scan(&name, &clid, &crrr, &uprr); err != nil {
				rows.Close()
				log.Printf("IngestHosts: Scan failed: %v", err)
				return total, inserted, updated, err
			}
			rawHosts = append(rawHosts, rawHost{name.String, clid.String, crrr.String, uprr.String})
		}
		rows.Close()

		if len(rawHosts) == 0 {
			break
		}

		first := rawHosts[0].Name
		last := rawHosts[len(rawHosts)-1].Name

		addrMap := make(map[string][]string)
		if aRows, err := sqliteDB.Query(`SELECT host_name, ip_address FROM host_addresses WHERE host_name >= ? AND host_name <= ?`, first, last); err == nil {
			for aRows.Next() {
				var hn, ip sql.NullString
				if err := aRows.Scan(&hn, &ip); err == nil && hn.Valid {
					addrMap[hn.String] = append(addrMap[hn.String], ip.String)
				}
			}
			aRows.Close()
		}

		statusMap := make(map[string]entities.HostStatus)
		if sRows, err := sqliteDB.Query(`SELECT host_name, status FROM host_statuses WHERE host_name >= ? AND host_name <= ?`, first, last); err == nil {
			for sRows.Next() {
				var hn, st sql.NullString
				if err := sRows.Scan(&hn, &st); err == nil && hn.Valid {
					hs := statusMap[hn.String]
					val := strings.ToLower(st.String)
					if val == "ok" {
						hs.OK = true
					}
					if val == "linked" {
						hs.Linked = true
					}
					statusMap[hn.String] = hs
				}
			}
			sRows.Close()
		}

		for _, r := range rawHosts {
			mapClid := func(v string) (string, bool) {
				tv := strings.TrimSpace(v)
				if tv == "" {
					return tv, true
				}
				if clidMap == nil {
					return tv, true
				}
				if mv, ok := clidMap[tv]; ok {
					return mv, true
				}
				return "", false
			}

			mClID, ok := mapClid(r.ClID)
			if !ok {
				// Record skipped (unmapped CLID or invalid data)
				continue
			}

			dn, err := entities.NewDomainName(r.Name)
			if err != nil || dn == nil {
				msg := fmt.Sprintf("Invalid name (ClID: %s)", r.ClID)
				log.Printf("SKIP HOST: %s - %s", r.Name, msg)
				batchReport.SkippedHosts = append(batchReport.SkippedHosts, SkippedItem{Name: r.Name, Reason: msg})
				// Record skipped (unmapped CLID or invalid data)
				continue
			}
			newID := s.IDGen.GenerateID()
			roid, err := entities.NewRoidType(newID, entities.RoidTypeHost, s.Repository)
			if err != nil {
				msg := fmt.Sprintf("Failed to generate RoID: %v", err)
				log.Printf("SKIP HOST: %s - %s", r.Name, msg)
				batchReport.SkippedHosts = append(batchReport.SkippedHosts, SkippedItem{Name: r.Name, Reason: msg})
				// Record skipped (unmapped CLID or invalid data)
				continue
			}

			h := &entities.Host{
				Name:      *dn,
				ClID:      entities.ClIDType(mClID),
				CreatedAt: time.Now().UTC(),
				UpdatedAt: time.Now().UTC(),
				RoID:      roid,
			}
			if m, ok := mapClid(r.CrRr); ok {
				h.CrRr = entities.ClIDType(m)
			}
			if m, ok := mapClid(r.UpRr); ok {
				h.UpRr = entities.ClIDType(m)
			}

			addrs := addrMap[r.Name]
			for _, ip := range addrs {
				if parsed, err := netip.ParseAddr(ip); err == nil {
					h.Addresses = append(h.Addresses, parsed)
				}
			}
			h.Status = statusMap[r.Name]

			entitiesBatch = append(entitiesBatch, h)
		}

		if len(entitiesBatch) > 0 {
			var dbHosts []*dbModels.Host
			var dbAddrs []*dbModels.HostAddress

			// Deduplicate by (name, cl_id) within the batch to prevent
			// PG error 21000: "ON CONFLICT DO UPDATE cannot affect row a second time".
			// Last occurrence wins (keeps the most recent data for a given key).
			seen := make(map[string]int) // key: "name|cl_id" → index
			for _, ent := range entitiesBatch {
				dbH := dbModels.ToDBHost(ent)
				key := dbH.Name + "|" + dbH.ClID
				if idx, exists := seen[key]; exists {
					// Replace previous entry
					dbHosts[idx] = dbH
				} else {
					seen[key] = len(dbHosts)
					dbHosts = append(dbHosts, dbH)
				}
				for _, a := range dbH.Addresses {
					val := a
					dbAddrs = append(dbAddrs, &val)
				}
			}
			batchLen := int64(len(dbHosts))

			// Upsert inside a transaction — batches WAL writes.
			txErr := s.PG.RunInTransaction(ctx, func(tx *pg.Tx) error {
				if _, err := tx.Model(&dbHosts).ExcludeColumn("addresses").
					OnConflict("(name, cl_id) DO UPDATE").
					Set("cr_rr = EXCLUDED.cr_rr, up_rr = EXCLUDED.up_rr, updated_at = EXCLUDED.updated_at, ok = EXCLUDED.ok, linked = EXCLUDED.linked, pending_create = EXCLUDED.pending_create, pending_delete = EXCLUDED.pending_delete, pending_update = EXCLUDED.pending_update, pending_transfer = EXCLUDED.pending_transfer, client_delete_prohibited = EXCLUDED.client_delete_prohibited, client_update_prohibited = EXCLUDED.client_update_prohibited, server_delete_prohibited = EXCLUDED.server_delete_prohibited, server_update_prohibited = EXCLUDED.server_update_prohibited").
					Insert(); err != nil {
					return fmt.Errorf("bulk upsert hosts: %w", err)
				}
				if len(dbAddrs) > 0 {
					if _, err := tx.Model(&dbAddrs).OnConflict("DO NOTHING").Insert(); err != nil {
						return fmt.Errorf("bulk insert host addresses: %w", err)
					}
				}
				return nil
			})
			if txErr != nil {
				return total, inserted, updated, txErr
			}
			inserted += batchLen
			total += batchLen
		}
		s.Report.Merge(batchReport)

		if (total)%10000 == 0 {
			log.Printf("IngestHosts: Processed %d / %d records (Inserted: %d, Updated: %d)", total, totalRows, inserted, updated)
		}

		lastKey = rawHosts[len(rawHosts)-1].Name
		lastClID = rawHosts[len(rawHosts)-1].ClID

		heartbeat(ImportCursor{Version: ImportCursorVersion, LastKey: lastKey, LastClID: lastClID, Processed: total, Total: totalRows})
	}
	log.Printf("IngestHosts: Finished. Total: %d, Inserted: %d, Updated: %d", total, inserted, updated)
	return total, inserted, updated, nil
}

// --- Import Domains ---

// explainNameUnderTLDViolation turns the database's check-constraint error into
// the same sentence the upsert guard gives.
//
// The guard above only sees names that already exist: a deposit whose names
// belong to another TLD entirely is caught one layer down, by
// ck_domains_name_under_tld, which rejects the proposed row before ON CONFLICT
// is reached. Both mean the same thing to the operator reading the workflow
// failure, so they should read the same. See issue #415.
func explainNameUnderTLDViolation(err error, tld string) error {
	if err == nil {
		return nil
	}
	var pgErr pg.Error
	if errors.As(err, &pgErr) && pgErr.Field('C') == "23514" &&
		strings.Contains(err.Error(), "ck_domains_name_under_tld") {
		return fmt.Errorf("the deposit's names do not belong to this TLD: a name was filed under %q that is not directly beneath it (%w)", tld, err)
	}
	return err
}

// ImportDomains bulk-upserts domains (and, when staged, their DS records) from
// a staged SQLite DB into Postgres as tld. Returns (total, inserted, updated,
// error). It starts after from (the zero cursor for a fresh run) with from's
// counts, and reports a cursor to heartbeat after every committed batch.
func (s *DirectDBImporter) ImportDomains(ctx context.Context, sqliteDB *sql.DB, tld string, clidMap map[string]string, from ImportCursor, heartbeat func(ImportCursor)) (int64, int64, int64, error) {
	from, restarted := ResumeCursor(PhaseDomains, from)
	if restarted {
		log.Printf("IngestDomains: cannot resume exactly from a legacy cursor; starting over")
	}
	batchSize := s.rowsPerBatch()
	lastKey := from.LastKey
	total := from.Processed
	var inserted int64
	var updated int64

	// Count total
	var totalRows int64
	if err := sqliteDB.QueryRow("SELECT COUNT(*) FROM domains").Scan(&totalRows); err != nil {
		log.Printf("IngestDomains: Failed to count total rows: %v", err)
	}

	dsStaged, err := stagedTableExists(sqliteDB, stagedDSRecordsTable)
	if err != nil {
		return total, inserted, updated, fmt.Errorf("IngestDomains: check for staged DS records: %w", err)
	}
	if !dsStaged {
		log.Printf("IngestDomains: the staging database holds no DNSSEC data; existing DS records are left as they are")
	}
	var dsWritten int64

	rgpStaged, err := stagedTableExists(sqliteDB, "domain_rgp_statuses")
	if err != nil {
		return total, inserted, updated, fmt.Errorf("IngestDomains: check for staged RGP statuses: %w", err)
	}
	policy := s.endOfLifePolicy(ctx, tld)

	for {
		rows, err := sqliteDB.Query(`SELECT name, registrant, clid, crrr, crdate, exdate, uprr, uname, originalname FROM domains WHERE name > ? ORDER BY name LIMIT ?`, lastKey, batchSize)
		if err != nil {
			log.Printf("IngestDomains: Query failed: %v", err)
			return total, inserted, updated, err
		}

		var rawDomains []struct {
			Name, Reg, ClID, CrRr, CrDate, ExDate, UpRr, UName, Org string
		}
		currentBatchMaxKey := ""

		for rows.Next() {
			var r struct {
				Name, Reg, ClID, CrRr, CrDate, ExDate, UpRr, UName, Org sql.NullString
			}
			if err := rows.Scan(&r.Name, &r.Reg, &r.ClID, &r.CrRr, &r.CrDate, &r.ExDate, &r.UpRr, &r.UName, &r.Org); err != nil {
				rows.Close()
				log.Printf("IngestDomains: Scan failed: %v", err)
				return total, inserted, updated, err
			}
			rawDomains = append(rawDomains, struct{ Name, Reg, ClID, CrRr, CrDate, ExDate, UpRr, UName, Org string }{
				r.Name.String, r.Reg.String, r.ClID.String, r.CrRr.String, r.CrDate.String, r.ExDate.String, r.UpRr.String, r.UName.String, r.Org.String,
			})
			currentBatchMaxKey = r.Name.String
		}
		rows.Close()

		if len(rawDomains) == 0 {
			break
		}

		first := rawDomains[0].Name
		last := rawDomains[len(rawDomains)-1].Name

		statusMap := make(map[string]entities.DomainStatus)
		if sRows, err := sqliteDB.Query(`SELECT domain_name, status FROM domain_statuses WHERE domain_name >= ? AND domain_name <= ?`, first, last); err == nil {
			for sRows.Next() {
				var dn, st sql.NullString
				if err := sRows.Scan(&dn, &st); err == nil && dn.Valid && st.Valid {
					ds := statusMap[dn.String]

					// Normalize status string fully to handle case, snake_case, or kebab-case
					normalizedStatus := strings.ToLower(st.String)
					normalizedStatus = strings.ReplaceAll(normalizedStatus, "_", "")
					normalizedStatus = strings.ReplaceAll(normalizedStatus, "-", "")

					switch normalizedStatus {
					case "ok":
						ds.OK = true
					case "inactive":
						ds.Inactive = true
					case "clienttransferprohibited":
						ds.ClientTransferProhibited = true
					case "clientupdateprohibited":
						ds.ClientUpdateProhibited = true
					case "clientdeleteprohibited":
						ds.ClientDeleteProhibited = true
					case "clientrenewprohibited":
						ds.ClientRenewProhibited = true
					case "clienthold":
						ds.ClientHold = true
					case "servertransferprohibited":
						ds.ServerTransferProhibited = true
					case "serverupdateprohibited":
						ds.ServerUpdateProhibited = true
					case "serverdeleteprohibited":
						ds.ServerDeleteProhibited = true
					case "serverrenewprohibited":
						ds.ServerRenewProhibited = true
					case "serverhold":
						ds.ServerHold = true
					case "pendingcreate":
						ds.PendingCreate = true
					case "pendingrenew":
						ds.PendingRenew = true
					case "pendingtransfer":
						ds.PendingTransfer = true
					case "pendingupdate":
						ds.PendingUpdate = true
					case "pendingrestore":
						// An RGP status, not an EPP one, in some deposits:
						// entities.Domain.ApplyDepositEndOfLife reads it.
						ds.PendingRestore = true
					case "pendingdelete":
						ds.PendingDelete = true
					}
					statusMap[dn.String] = ds
				}
			}
			sRows.Close()
		}

		// The RGP statuses: a deleted domain's end of life
		// (entities.Domain.ApplyDepositEndOfLife).
		rgpMap := map[string][]string{}
		if rgpStaged {
			rRows, err := sqliteDB.Query(`SELECT domain_name, rgp_status FROM domain_rgp_statuses WHERE domain_name >= ? AND domain_name <= ?`, first, last)
			if err != nil {
				return total, inserted, updated, fmt.Errorf("IngestDomains: read RGP statuses: %w", err)
			}
			for rRows.Next() {
				var dn, st sql.NullString
				if err := rRows.Scan(&dn, &st); err == nil && dn.Valid && st.Valid {
					rgpMap[dn.String] = append(rgpMap[dn.String], st.String)
				}
			}
			rRows.Close()
		}
		importedAt := time.Now().UTC()

		var batchReport RunReport
		var dsMap map[string][]entities.DSRecord
		if dsStaged {
			if dsMap, batchReport.SkippedDSRecords, err = readStagedDSRecords(sqliteDB, first, last); err != nil {
				return total, inserted, updated, err
			}
		}

		var entitiesBatch []*entities.Domain

		for _, r := range rawDomains {
			mapClid := func(v string) (string, bool) {
				tv := strings.TrimSpace(v)
				if tv == "" {
					return tv, true
				}
				if clidMap == nil {
					return tv, true
				}
				if mv, ok := clidMap[tv]; ok {
					return mv, true
				}
				return "", false
			}
			mClID, ok := mapClid(r.ClID)
			if !ok {
				// Record skipped (unmapped CLID or invalid data)
				continue
			}

			dn, _ := entities.NewDomainName(r.Name)
			tldn, _ := entities.NewDomainName(tld)

			newID := s.IDGen.GenerateID()
			roid, err := entities.NewRoidType(newID, entities.RoidTypeDomain, s.Repository)
			if err != nil {
				// Record skipped (unmapped CLID or invalid data)
				continue
			}

			// A deposit carries no authInfo, and under RFC 9154 a domain has no transfer code until its
			// registrar sets one: the domain is imported without one. The upsert below never sets auth_info
			// on conflict, so re-running a batch cannot clear one a registrar has set since.
			d := &entities.Domain{
				Name:         *dn,
				TLDName:      *tldn,
				ClID:         entities.ClIDType(mClID),
				RegistrantID: entities.ClIDType(r.Reg),
				UName:        entities.DomainName(r.UName),
				RoID:         roid,
				UpdatedAt:    time.Now().UTC(),
			}

			if st, ok := statusMap[r.Name]; ok {
				d.Status = st
			}
			d.ApplyDepositEndOfLife(rgpMap[r.Name], &policy, importedAt)
			d.DSRecords = dsMap[r.Name]

			if t, err := parseJiscTime(r.CrDate); err == nil {
				d.CreatedAt = t
			} else {
				d.CreatedAt = time.Now().UTC()
			}
			if t, err := parseJiscTime(r.ExDate); err == nil && !t.IsZero() {
				d.ExpiryDate = t
			} else {
				// Fallback to avoid not-null constraint failure
				fallback := time.Now().AddDate(1, 0, 0).UTC()
				msg := fmt.Sprintf("Setting fallback: %s. Parse Err: %v", fallback, err)
				log.Printf("MODIFY DOMAIN: '%s' missing/invalid ExpiryDate ('%s'). %s", d.Name, r.ExDate, msg)
				batchReport.ModifiedDomains = append(batchReport.ModifiedDomains, ModifiedItem{
					Name:          d.Name.String(),
					Field:         "ExpiryDate",
					OriginalValue: r.ExDate,
					Info:          msg,
				})
				d.ExpiryDate = fallback
			}
			if m, ok := mapClid(r.CrRr); ok {
				d.CrRr = entities.ClIDType(m)
			}
			if m, ok := mapClid(r.UpRr); ok {
				d.UpRr = entities.ClIDType(m)
			}

			entitiesBatch = append(entitiesBatch, d)
		}

		if len(entitiesBatch) > 0 {
			var dbBatch []*dbModels.Domain
			batchLen := int64(len(entitiesBatch))
			for _, ent := range entitiesBatch {
				dbBatch = append(dbBatch, dbModels.ToDBDomain(ent))
			}

			// Upsert inside a transaction — PG batches WAL writes and defers
			// constraint checks, improving throughput for large batches.
			var res pg.Result
			txErr := s.PG.RunInTransaction(ctx, func(tx *pg.Tx) error {
				var err error
				res, err = tx.Model(&dbBatch).
					ExcludeColumn("hosts", "tld", "ds_records").
					OnConflict("(name) DO UPDATE").
					Set("cl_id = EXCLUDED.cl_id, cr_rr = EXCLUDED.cr_rr, up_rr = EXCLUDED.up_rr, registrant_id = EXCLUDED.registrant_id, expiry_date = EXCLUDED.expiry_date, u_name = EXCLUDED.u_name, original_name = EXCLUDED.original_name, drop_catch = EXCLUDED.drop_catch, renewed_years = EXCLUDED.renewed_years, updated_at = EXCLUDED.updated_at, ok = EXCLUDED.ok, inactive = EXCLUDED.inactive, client_transfer_prohibited = EXCLUDED.client_transfer_prohibited, client_update_prohibited = EXCLUDED.client_update_prohibited, client_delete_prohibited = EXCLUDED.client_delete_prohibited, client_renew_prohibited = EXCLUDED.client_renew_prohibited, client_hold = EXCLUDED.client_hold, server_transfer_prohibited = EXCLUDED.server_transfer_prohibited, server_update_prohibited = EXCLUDED.server_update_prohibited, server_delete_prohibited = EXCLUDED.server_delete_prohibited, server_renew_prohibited = EXCLUDED.server_renew_prohibited, server_hold = EXCLUDED.server_hold, pending_create = EXCLUDED.pending_create, pending_renew = EXCLUDED.pending_renew, pending_transfer = EXCLUDED.pending_transfer, pending_update = EXCLUDED.pending_update, pending_restore = EXCLUDED.pending_restore, pending_delete = EXCLUDED.pending_delete, redemption_period_end = EXCLUDED.redemption_period_end, purge_date = EXCLUDED.purge_date, pending_restore_end = EXCLUDED.pending_restore_end").
					// A name that already exists under another TLD is not ours to
					// update: without this guard the upsert would rewrite that
					// domain's registrar, registrant and statuses from this deposit.
					Where("?TableAlias.tld_name = EXCLUDED.tld_name").
					Insert()
				if err != nil {
					return err
				}
				// Rows the guard filtered out are not counted, so a shortfall means
				// some domains belong to another TLD. Fail the batch rather than
				// import a deposit that silently skipped part of itself.
				if affected := int64(res.RowsAffected()); affected < batchLen {
					return fmt.Errorf("%d of %d domains already exist under a different TLD than %q and were not imported; the deposit's names do not belong to this TLD", batchLen-affected, batchLen, tld)
				}
				if !dsStaged {
					return nil
				}
				n, err := replaceDSRecords(tx, entitiesBatch)
				dsWritten += n
				return err
			})
			if txErr != nil {
				return total, inserted, updated, fmt.Errorf("bulk upsert domains failed: %w", explainNameUnderTLDViolation(txErr, tld))
			}
			// For ON CONFLICT DO UPDATE, RowsAffected = all rows (inserts + updates).
			// We can't distinguish without a pre-count, so we report total processed
			// and estimate: on first import everything is inserted, on re-import
			// everything is updated.
			inserted += batchLen
			total += batchLen
		}
		s.Report.Merge(batchReport)

		if (total)%10000 == 0 {
			log.Printf("IngestDomains: Processed %d / %d records (Inserted: %d, Updated: %d)", total, totalRows, inserted, updated)
		}

		// The cursor is the staged name, which is what the query compares. The
		// entity's name is normalised (lower-cased, dots trimmed) and can sort
		// on either side of it, skipping or repeating rows.
		lastKey = currentBatchMaxKey

		heartbeat(ImportCursor{Version: ImportCursorVersion, LastKey: lastKey, Processed: total, Total: totalRows})
	}
	log.Printf("IngestDomains: Finished. Total: %d, Inserted: %d, Updated: %d, DS records written: %d", total, inserted, updated, dsWritten)
	return total, inserted, updated, nil
}

// stagedTableExists reports whether the staging database has the named table.
// endOfLifePolicy is the policy a deleted domain's imported end of life
// follows (entities.Domain.ApplyDepositEndOfLife): the redemption, pending
// delete and pending restore periods of tld's current GA phase, or the
// defaults where the phase cannot be read or sets none.
func (s *DirectDBImporter) endOfLifePolicy(ctx context.Context, tld string) entities.PhasePolicy {
	policy := entities.NewPhasePolicy()
	if s.PG == nil {
		return policy
	}
	var row struct {
		RedemptionGP         int `pg:"redemption_gp"`
		PendingDeleteGP      int `pg:"pending_delete_gp"`
		PendingRestorePeriod int `pg:"pending_restore_period"`
	}
	_, err := s.PG.QueryOneContext(ctx, &row, `SELECT COALESCE(redemption_gp, 0) AS redemption_gp, COALESCE(pending_delete_gp, 0) AS pending_delete_gp,
		COALESCE(pending_restore_period, 0) AS pending_restore_period FROM phases
		WHERE tld_name = ? AND type = 'GA' AND starts <= now() AND (ends IS NULL OR ends > now()) ORDER BY starts DESC LIMIT 1`, tld)
	if err != nil {
		log.Printf("IngestDomains: no current GA phase policy for %s (%v); deleted domains get the default periods", tld, err)
		return policy
	}
	policy.UpdatePolicy(&entities.PhasePolicy{RedemptionGP: row.RedemptionGP, PendingDeleteGP: row.PendingDeleteGP, PendingRestorePeriod: row.PendingRestorePeriod})
	return policy
}

func stagedTableExists(sqliteDB *sql.DB, table string) (bool, error) {
	var n int
	if err := sqliteDB.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = ?`, table).Scan(&n); err != nil {
		return false, err
	}
	return n > 0, nil
}

// readStagedDSRecords returns the staged DS records of the domains named first
// to last, in deposit order. Every algorithm is kept: which ones a registrar
// may submit is policy for EPP, not for a deposit. A record that is not well
// formed, or repeats one already read for its domain, is skipped and returned
// for the report rather than failing the batch, as RDEDomain.ToEntity does on
// the other path.
func readStagedDSRecords(sqliteDB *sql.DB, first, last string) (map[string][]entities.DSRecord, []SkippedItem, error) {
	rows, err := sqliteDB.Query(`SELECT domain_name, key_tag, algorithm, digest_type, digest FROM `+stagedDSRecordsTable+` WHERE domain_name >= ? AND domain_name <= ? ORDER BY domain_name, id`, first, last)
	if err != nil {
		return nil, nil, fmt.Errorf("IngestDomains: read staged DS records: %w", err)
	}
	defer rows.Close()

	out := make(map[string][]entities.DSRecord)
	seen := make(map[string]map[entities.DSRecord]bool)
	var skipped []SkippedItem
	for rows.Next() {
		var name, keyTag, alg, digestType, digest sql.NullString
		if err := rows.Scan(&name, &keyTag, &alg, &digestType, &digest); err != nil {
			return nil, nil, fmt.Errorf("IngestDomains: scan staged DS record: %w", err)
		}
		rec, err := parseStagedDSRecord(keyTag.String, alg.String, digestType.String, digest.String)
		if err == nil && seen[name.String][rec] {
			err = fmt.Errorf("%w: %s", entities.ErrDuplicateDSRecord, rec)
		}
		if err != nil {
			msg := fmt.Sprintf("DS record %q %q %q %q: %v", keyTag.String, alg.String, digestType.String, digest.String, err)
			log.Printf("SKIP DS RECORD: %s - %s", name.String, msg)
			skipped = append(skipped, SkippedItem{Name: name.String, Reason: msg})
			continue
		}
		if seen[name.String] == nil {
			seen[name.String] = make(map[entities.DSRecord]bool)
		}
		seen[name.String][rec] = true
		out[name.String] = append(out[name.String], rec)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, fmt.Errorf("IngestDomains: read staged DS records: %w", err)
	}
	return out, skipped, nil
}

// parseStagedDSRecord parses one staged row's fields and validates the record.
func parseStagedDSRecord(keyTag, alg, digestType, digest string) (entities.DSRecord, error) {
	kt, err := strconv.ParseUint(strings.TrimSpace(keyTag), 10, 16)
	if err != nil {
		return entities.DSRecord{}, fmt.Errorf("%w: key tag: %w", entities.ErrInvalidDSRecord, err)
	}
	a, err := strconv.ParseUint(strings.TrimSpace(alg), 10, 8)
	if err != nil {
		return entities.DSRecord{}, fmt.Errorf("%w: algorithm: %w", entities.ErrInvalidDSRecord, err)
	}
	dt, err := strconv.ParseUint(strings.TrimSpace(digestType), 10, 8)
	if err != nil {
		return entities.DSRecord{}, fmt.Errorf("%w: digest type: %w", entities.ErrInvalidDSRecord, err)
	}
	return entities.NewDSRecord(uint16(kt), uint8(a), uint8(dt), digest)
}

// replaceDSRecords makes each domain's DS records in the database the ones the
// deposit holds for it: a re-import replaces them, and clears them from a domain
// the deposit now has unsigned. Domains are matched by name, as LinkDomainHosts
// does, because an upsert that hit an existing row kept that row's ro_id rather
// than the one generated for the batch. It runs in the batch's transaction, so
// a resumed import never sees a domain without its DS records.
func replaceDSRecords(tx *pg.Tx, batch []*entities.Domain) (int64, error) {
	names := make([]string, 0, len(batch))
	var valueStrings []string
	var args []interface{}
	for _, d := range batch {
		names = append(names, d.Name.String())
		for _, ds := range d.DSRecords {
			valueStrings = append(valueStrings, "(?, ?, ?, ?, ?, ?)")
			args = append(args, len(valueStrings), d.Name.String(), int(ds.KeyTag), int(ds.Algorithm), int(ds.DigestType), ds.Digest)
		}
	}

	if _, err := tx.Exec(`DELETE FROM domain_ds_records WHERE domain_ro_id IN (SELECT ro_id FROM domains WHERE name IN (?))`, pg.In(names)); err != nil {
		return 0, fmt.Errorf("clear DS records: %w", err)
	}
	if len(valueStrings) == 0 {
		return 0, nil
	}

	// Ordered so ids follow deposit order, which is the order the repository
	// reads a domain's records back in.
	q := fmt.Sprintf(`
		INSERT INTO domain_ds_records (domain_ro_id, key_tag, algorithm, digest_type, digest)
		SELECT d.ro_id, v.key_tag, v.algorithm, v.digest_type, v.digest
		FROM (VALUES %s) AS v(ord, dn, key_tag, algorithm, digest_type, digest)
		JOIN domains d ON d.name = v.dn
		ORDER BY v.ord
	`, strings.Join(valueStrings, ","))
	res, err := tx.Exec(q, args...)
	if err != nil {
		return 0, fmt.Errorf("insert DS records: %w", err)
	}
	return int64(res.RowsAffected()), nil
}

// --- Link Domain Hosts ---

func (s *DirectDBImporter) LinkDomainHosts(ctx context.Context, sqliteDB *sql.DB, lastKey string, heartbeat func(processed string)) (int64, int64, error) {
	const batchSize = 10000
	var total int64
	var inserted int64
	var lastDomain, lastNS string

	if lastKey != "" {
		parts := strings.SplitN(lastKey, "|", 2)
		if len(parts) == 2 {
			lastDomain, lastNS = parts[0], parts[1]
		}
	}

	// The dedup table below is a TEMP table, which lives in one session: every
	// statement that touches it, from its creation through each batch to the
	// drop, must run on the same connection. Hold one from the pool for the
	// whole operation; the pool is shared, and another linking activity on
	// another connection cannot see or drop this session's table.
	conn := s.PG.Conn()
	defer conn.Close()

	// Pre-materialize a host dedup lookup table (once, before the loop).
	// When multiple hosts share the same name (from cross-TLD imports with
	// different cl_id), we pick one deterministically. This avoids a costly
	// DISTINCT ON + ORDER BY sort in every batch INSERT.
	if _, err := conn.ExecContext(ctx, `
		CREATE TEMP TABLE IF NOT EXISTS _host_dedup AS
		SELECT DISTINCT ON (name) name, ro_id FROM hosts ORDER BY name, ro_id
	`); err != nil {
		return 0, 0, fmt.Errorf("LinkDomainHosts: failed to build host dedup table: %w", err)
	}
	if _, err := conn.ExecContext(ctx, `CREATE INDEX IF NOT EXISTS _idx_host_dedup_name ON _host_dedup(name)`); err != nil {
		log.Printf("LinkDomainHosts: index on _host_dedup failed (non-fatal): %v", err)
	}
	defer func() { _, _ = conn.Exec("DROP TABLE IF EXISTS _host_dedup") }() // best-effort cleanup of a scratch object, on the session that owns it

	for {
		rows, err := sqliteDB.Query(`SELECT domain_name, nameserver FROM domain_nameservers WHERE (domain_name > ?) OR (domain_name = ? AND nameserver > ?) ORDER BY domain_name, nameserver LIMIT ?`, lastDomain, lastDomain, lastNS, batchSize)
		if err != nil {
			return total, inserted, err
		}

		type link struct {
			DomainName string
			HostName   string
		}
		var links []link

		for rows.Next() {
			var d, n sql.NullString
			if err := rows.Scan(&d, &n); err != nil {
				rows.Close()
				return total, inserted, err
			}
			if d.Valid && n.Valid {
				links = append(links, link{d.String, n.String})
				lastDomain = d.String
				lastNS = n.String
			}
		}
		rows.Close()

		if len(links) == 0 {
			break
		}

		var valueStrings []string
		var args []interface{}
		for _, l := range links {
			valueStrings = append(valueStrings, "(?, ?)")
			args = append(args, l.DomainName, l.HostName)
		}

		// Simple JOIN against pre-deduped host lookup — no DISTINCT ON or
		// ORDER BY needed per batch. Transaction batches WAL writes.
		q := fmt.Sprintf(`
			INSERT INTO domain_hosts (domain_ro_id, host_ro_id)
			SELECT d.ro_id, h.ro_id
			FROM (VALUES %s) AS v(dn, hn)
			JOIN domains d ON d.name = v.dn
			JOIN _host_dedup h ON h.name = v.hn
			ON CONFLICT DO NOTHING
		`, strings.Join(valueStrings, ","))

		var batchInserted int64
		if err := conn.RunInTransaction(ctx, func(tx *pg.Tx) error {
			res, err := tx.Exec(q, args...)
			if err == nil {
				batchInserted = int64(res.RowsAffected())
			}
			return err
		}); err != nil {
			return total, inserted, fmt.Errorf("bulk link failed: %w", err)
		}

		total += int64(len(links))
		inserted += batchInserted
		heartbeat(fmt.Sprintf("%s|%s", lastDomain, lastNS))
	}

	return total, inserted, nil
}

// --- Import NNDNs ---

// newImportedNNDN maps one escrow NNDN row to its database model. The deposit
// carries no reason, so every imported NNDN is tagged as coming from the escrow
// import; that is what lets an operator tell them apart from names blocked by hand.
func newImportedNNDN(aName, uName, tld, nameState string, createdAt time.Time) *dbModels.NNDN {
	return &dbModels.NNDN{
		Name:      strings.ToLower(aName),
		UName:     uName,
		TLDName:   strings.ToLower(tld),
		NameState: nameState,
		Reason:    commands.NNDNReasonEscrowImport,
		CreatedAt: createdAt,
		UpdatedAt: time.Now().UTC(),
	}
}

// ImportNNDNs bulk-upserts NNDNs from a staged SQLite DB into Postgres as tld.
// Returns (total, inserted, updated, error). It starts after from (the zero
// cursor for a fresh run) with from's counts, and reports a cursor to
// heartbeat after every committed batch.
func (s *DirectDBImporter) ImportNNDNs(ctx context.Context, sqliteDB *sql.DB, tld string, from ImportCursor, heartbeat func(ImportCursor)) (int64, int64, int64, error) {
	from, restarted := ResumeCursor(PhaseNNDNs, from)
	if restarted {
		log.Printf("IngestNNDNs: cannot resume exactly from a legacy cursor; starting over")
	}
	batchSize := s.rowsPerBatch()
	lastKey := from.LastKey
	total := from.Processed
	var inserted int64
	var updated int64

	// Count total
	var totalRows int64
	if err := sqliteDB.QueryRow("SELECT COUNT(*) FROM nndns").Scan(&totalRows); err != nil {
		log.Printf("IngestNNDNs: Failed to count total rows: %v", err)
	}

	for {
		rows, err := sqliteDB.Query(`SELECT aname, uname, idntableid, originalname, namestate, crdate FROM nndns WHERE aname > ? ORDER BY aname LIMIT ?`, lastKey, batchSize)
		if err != nil {
			log.Printf("IngestNNDNs: Query failed: %v", err)
			return total, inserted, updated, err
		}

		var rawNNDNs []struct {
			AName, UName, IDNTableID, OriginalName, NameState, CrDate string
		}
		currentBatchMaxKey := ""

		for rows.Next() {
			var r struct {
				AName, UName, IDNTableID, OriginalName, NameState, CrDate sql.NullString
			}
			if err := rows.Scan(&r.AName, &r.UName, &r.IDNTableID, &r.OriginalName, &r.NameState, &r.CrDate); err != nil {
				rows.Close()
				log.Printf("IngestNNDNs: Scan failed: %v", err)
				return total, inserted, updated, err
			}
			rawNNDNs = append(rawNNDNs, struct{ AName, UName, IDNTableID, OriginalName, NameState, CrDate string }{
				r.AName.String, r.UName.String, r.IDNTableID.String, r.OriginalName.String, r.NameState.String, r.CrDate.String,
			})
			currentBatchMaxKey = r.AName.String
		}
		rows.Close()

		if len(rawNNDNs) == 0 {
			break
		}

		var entitiesBatch []*dbModels.NNDN

		for _, r := range rawNNDNs {
			createdAt := time.Now().UTC()
			if t, err := parseJiscTime(r.CrDate); err == nil && !t.IsZero() {
				createdAt = t
			}

			entitiesBatch = append(entitiesBatch, newImportedNNDN(r.AName, r.UName, tld, r.NameState, createdAt))
		}

		if len(entitiesBatch) > 0 {
			batchLen := int64(len(entitiesBatch))

			// Upsert inside a transaction — batches WAL writes.
			txErr := s.PG.RunInTransaction(ctx, func(tx *pg.Tx) error {
				_, err := tx.Model(&entitiesBatch).
					ExcludeColumn("tld").
					OnConflict("(name) DO UPDATE").
					// Only fill a reason that is missing (NULL or ''): one an operator set by hand is kept.
					Set("name_state = EXCLUDED.name_state, u_name = EXCLUDED.u_name, updated_at = EXCLUDED.updated_at, reason = COALESCE(NULLIF(nndn.reason, ''), EXCLUDED.reason)").
					Insert()
				return err
			})
			if txErr != nil {
				return total, inserted, updated, fmt.Errorf("bulk upsert NNDNs failed: %w", txErr)
			}
			inserted += batchLen
			total += batchLen
		}

		if (total)%10000 == 0 {
			log.Printf("IngestNNDNs: Processed %d / %d records (Inserted: %d, Updated: %d)", total, totalRows, inserted, updated)
		}

		lastKey = currentBatchMaxKey

		heartbeat(ImportCursor{Version: ImportCursorVersion, LastKey: lastKey, Processed: total, Total: totalRows})
	}
	log.Printf("IngestNNDNs: Finished. Total: %d, Inserted: %d, Updated: %d", total, inserted, updated)
	return total, inserted, updated, nil
}

// --- Accredit Registrars ---

// AccreditRegistrars accredits all unique registrars mapped in the staging DB for the given TLD
func (s *DirectDBImporter) AccreditRegistrars(ctx context.Context, sqliteDB *sql.DB, tld string, heartbeat func(processed string)) (int64, error) {
	var total int64

	// We MUST query registrar_mapping to get the actual Postgres registrar_cl_id.
	// Querying the registrars table directly might return the escrow ID, violating the FK constraint.
	rows, err := sqliteDB.Query(`SELECT DISTINCT registrar_clid FROM registrar_mapping WHERE registrar_clid IS NOT NULL AND registrar_clid != ''`)
	if err != nil {
		log.Printf("AccreditRegistrars: Query failed: %v", err)
		return 0, nil // Graceful exit to not block ingestion if registrar_mapping table is missing
	}
	defer rows.Close()

	var clIDs []string
	for rows.Next() {
		var clid sql.NullString
		if err := rows.Scan(&clid); err != nil {
			log.Printf("AccreditRegistrars: Scan failed: %v", err)
			return total, err
		}
		if clid.Valid && strings.TrimSpace(clid.String) != "" {
			clIDs = append(clIDs, strings.TrimSpace(clid.String))
		}
	}

	if len(clIDs) == 0 {
		log.Printf("AccreditRegistrars: Finished. No registrars found to accredit.")
		return 0, nil
	}

	const batchSize = 1000
	for i := 0; i < len(clIDs); i += batchSize {
		end := i + batchSize
		if end > len(clIDs) {
			end = len(clIDs)
		}

		batch := clIDs[i:end]
		var valueStrings []string
		var args []interface{}

		for _, clid := range batch {
			valueStrings = append(valueStrings, "(?, ?)")
			args = append(args, strings.ToLower(tld), clid)
		}

		q := fmt.Sprintf(`
			INSERT INTO accreditations (tld_name, registrar_cl_id) 
			VALUES %s 
			ON CONFLICT DO NOTHING
		`, strings.Join(valueStrings, ","))

		res, err := s.PG.Exec(q, args...)
		if err != nil {
			return total, fmt.Errorf("bulk insert accreditations failed: %w", err)
		}

		total += int64(res.RowsAffected())
		heartbeat(fmt.Sprintf("%d/%d", i+len(batch), len(clIDs)))
	}

	log.Printf("AccreditRegistrars: Finished. Total new accreditations: %d", total)
	return total, nil
}

// fallbackPGURL assembles a postgres URL from the individual DB_* env vars.
// It uses net/url so credentials containing URL-reserved characters
// (:, @, /, ?, #, …) — which RDS- and Secrets Manager-generated passwords
// routinely include — are percent-escaped rather than corrupting the URL,
// mirroring buildDSN in internal/infrastructure/db/postgres/connection.go.
func fallbackPGURL() string {
	u := url.URL{
		Scheme:   "postgres",
		User:     url.UserPassword(getEnv("DB_USER", "postgres"), getEnv("DB_PASS", "postgres")),
		Host:     fmt.Sprintf("%s:%s", getEnv("DB_HOST", "localhost"), getEnv("DB_PORT", "5432")),
		Path:     "/" + getEnv("DB_NAME", "domain_os"),
		RawQuery: "sslmode=" + url.QueryEscape(getEnv("DB_SSLMODE", "disable")),
	}
	return u.String()
}

// sanitizePGURLForGoPG strips query parameters that go-pg/pg doesn't support.
// go-pg only accepts: sslmode, application_name, connect_timeout.
// Neon DATABASE_URLs typically include channel_binding=require which causes
// pg.ParseURL to fail. This function preserves only the supported params.
func sanitizePGURLForGoPG(rawURL string) string {
	supported := map[string]bool{
		"sslmode":          true,
		"application_name": true,
		"connect_timeout":  true,
	}

	u, err := url.Parse(rawURL)
	if err != nil {
		return rawURL // return as-is, let pg.ParseURL report the error
	}

	q := u.Query()
	cleaned := url.Values{}
	for k, v := range q {
		if supported[k] {
			cleaned[k] = v
		}
	}
	u.RawQuery = cleaned.Encode()
	return u.String()
}
