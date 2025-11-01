package activities

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/onasunnymorning/domain-os/internal/application/commands"
	"github.com/onasunnymorning/domain-os/internal/application/services"
	"github.com/onasunnymorning/domain-os/internal/domain/entities"
	pg "github.com/onasunnymorning/domain-os/internal/infrastructure/db/postgres"
	"github.com/onasunnymorning/domain-os/internal/infrastructure/snowflakeidgenerator"
	"github.com/onasunnymorning/domain-os/internal/infrastructure/storage"
	"go.temporal.io/sdk/activity"
	_ "modernc.org/sqlite"
)

// ValidateInputArgs represents the basic input validation parameters
type ValidateInputArgs struct {
	TLD       string
	ObjectKey string
}

// ValidateInputResult returns the outcome of basic validation
type ValidateInputResult struct {
	TLD       string
	ObjectKey string
	Exists    bool
}

// EscrowImportActivities groups escrow-related activity methods
type EscrowImportActivities struct{}

// ValidateInput checks that required inputs are present and object exists in S3/MinIO
func (a *EscrowImportActivities) ValidateInput(ctx context.Context, args ValidateInputArgs) (ValidateInputResult, error) {
	tld := strings.TrimSpace(args.TLD)
	if tld == "" {
		return ValidateInputResult{}, fmt.Errorf("tld is required")
	}
	key := strings.TrimSpace(args.ObjectKey)
	if key == "" {
		return ValidateInputResult{}, fmt.Errorf("objectKey is required")
	}

	s3c, err := storage.NewS3ClientFromEnv()
	if err != nil {
		return ValidateInputResult{}, err
	}
	exists, err := s3c.Exists(ctx, key)
	if err != nil {
		return ValidateInputResult{}, err
	}

	return ValidateInputResult{TLD: tld, ObjectKey: key, Exists: exists}, nil
}

// StreamingAnalysisArgs parameters
type StreamingAnalysisArgs struct {
	TLD           string
	ObjectKey     string
	MapRegistrars bool
}

// StreamingAnalysisResult outcome
type StreamingAnalysisResult struct {
	RunPrefix    string            // escrow/<tld>/<yyyyMMdd>/<workflowId>
	BaseFilename string            // derived base name used for CSVs
	ArtifactKeys map[string]string // filename -> s3 key
	Counts       map[string]int64  // optional counts (best-effort)
	// Analysis findings for graceful workflow control
	HasIssues       bool     // true if errors or missing contacts were detected
	AnalysisErrors  []string // from analysis.errors
	MissingContacts []string // from analysis.missingContacts
}

// StreamingAnalysis downloads the XML, runs the streaming analyzer to generate CSVs, and uploads them back to S3
func (a *EscrowImportActivities) StreamingAnalysis(ctx context.Context, args StreamingAnalysisArgs) (StreamingAnalysisResult, error) {
	info := activity.GetInfo(ctx)
	wfID := info.WorkflowExecution.ID
	if wfID == "" {
		wfID = "unknown"
	}

	tld := strings.TrimSpace(args.TLD)
	if tld == "" {
		return StreamingAnalysisResult{}, fmt.Errorf("tld is required")
	}
	key := strings.TrimSpace(args.ObjectKey)
	if key == "" {
		return StreamingAnalysisResult{}, fmt.Errorf("objectKey is required")
	}

	s3c, err := storage.NewS3ClientFromEnv()
	if err != nil {
		return StreamingAnalysisResult{}, err
	}

	// Build run prefix as approved: escrow/<tld>/<yyyyMMdd>/<workflowId>
	day := time.Now().UTC().Format("20060102")
	runPrefix := filepath.ToSlash(filepath.Join("escrow", tld, day, wfID))

	// Download XML object to temp file
	xmlPath, err := s3c.DownloadToFile(ctx, key)
	if err != nil {
		return StreamingAnalysisResult{}, fmt.Errorf("download failed: %w", err)
	}
	// Clean up the local file after processing
	defer os.Remove(xmlPath)

	// Run streaming analysis
	streamingSvc, err := services.NewStreamingXMLEscrowService(xmlPath)
	if err != nil {
		return StreamingAnalysisResult{}, fmt.Errorf("streaming service init failed: %w", err)
	}
	if err := streamingSvc.StreamAnalyze(args.MapRegistrars); err != nil {
		return StreamingAnalysisResult{}, fmt.Errorf("stream analyze failed: %w", err)
	}

	// Determine base filename and expected artifacts; upload those that exist
	base := strings.TrimSuffix(xmlPath, filepath.Ext(xmlPath))

	// Inspect analysis JSON first (if present) so we can fail fast after uploading artifacts
	analysisIssues := struct {
		Errors          []string `json:"errors"`
		MissingContacts []string `json:"missingContacts"`
	}{}
	hasAnalysisIssues := false
	analysisEnvelope := struct {
		Analysis struct {
			Errors          []string `json:"errors"`
			MissingContacts []string `json:"missingContacts"`
		} `json:"analysis"`
	}{}
	analysisPath := base + "-analysis.json"
	if st, statErr := os.Stat(analysisPath); statErr == nil && st.Size() > 0 {
		if data, rerr := os.ReadFile(analysisPath); rerr == nil {
			if jerr := json.Unmarshal(data, &analysisEnvelope); jerr == nil {
				analysisIssues.Errors = analysisEnvelope.Analysis.Errors
				analysisIssues.MissingContacts = analysisEnvelope.Analysis.MissingContacts
				if len(analysisIssues.Errors) > 0 || len(analysisIssues.MissingContacts) > 0 {
					hasAnalysisIssues = true
				}
			}
		}
	}
	candidates := []string{
		base + "-domains.csv",
		base + "-domainStatuses.csv",
		base + "-domainNameservers.csv",
		base + "-DomainDnssec.csv",
		base + "-domainTransfers.csv",
		base + "-domainRgpStatus.csv",
		base + "-contacts.csv",
		base + "-contactStatuses.csv",
		base + "-contactPostalInfo.csv",
		base + "-uniqueDomainContactIDs.csv",
		base + "-hosts.csv",
		base + "-hostAddresses.csv",
		base + "-hostStatuses.csv",
		base + "-registrars.csv",
		base + "-registrarPostalInfo.csv",
		base + "-registrarMapping.csv",
		base + "-nndns.csv",
		// Helpful troubleshooting artifacts
		base + "-analysis.json",
		base + "-registrarMapping.json",
		base + "-registrar-map.json",
	}
	artifacts := map[string]string{}
	counts := map[string]int64{}

	for _, f := range candidates {
		if _, err := os.Stat(f); err == nil {
			// Upload to S3 under runPrefix/<filename>
			filename := filepath.Base(f)
			objKey := runPrefix + "/" + filename
			// Choose content type based on file extension
			ctype := "text/csv"
			switch strings.ToLower(filepath.Ext(filename)) {
			case ".json":
				ctype = "application/json"
			case ".csv":
				ctype = "text/csv"
			default:
				ctype = "application/octet-stream"
			}
			if upErr := s3c.UploadFile(ctx, objKey, f, ctype); upErr != nil {
				return StreamingAnalysisResult{}, fmt.Errorf("upload %s failed: %w", filename, upErr)
			}
			artifacts[filename] = objKey
			// best-effort count lines (minus header)
			if strings.HasSuffix(strings.ToLower(filename), ".csv") {
				if c, cerr := countCSVLines(f); cerr == nil && c > 0 {
					// crude mapping for some expected types
					name := strings.ToLower(filename)
					switch {
					case strings.Contains(name, "domains.csv") && !strings.Contains(name, "status") && !strings.Contains(name, "nameserver"):
						counts["domains"] = int64(c - 1)
					case strings.Contains(name, "contacts.csv") && !strings.Contains(name, "status") && !strings.Contains(name, "postal"):
						counts["contacts"] = int64(c - 1)
					case strings.Contains(name, "hosts.csv") && !strings.Contains(name, "status"):
						counts["hosts"] = int64(c - 1)
					}
				}
			}
			// remove local file to keep disk usage low
			_ = os.Remove(f)
		}
	}

	return StreamingAnalysisResult{
		RunPrefix:       runPrefix,
		BaseFilename:    filepath.Base(base),
		ArtifactKeys:    artifacts,
		Counts:          counts,
		HasIssues:       hasAnalysisIssues,
		AnalysisErrors:  analysisIssues.Errors,
		MissingContacts: analysisIssues.MissingContacts,
	}, nil
}

// countCSVLines returns the number of lines in a CSV file
func countCSVLines(path string) (int, error) {
	// Lightweight line counting; not parsing CSV fully to reduce memory
	f, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer f.Close()
	buf := make([]byte, 32*1024)
	count := 0
	for {
		n, readErr := f.Read(buf)
		count += strings.Count(string(buf[:n]), "\n")
		if readErr != nil {
			if readErr.Error() == "EOF" {
				break
			}
			break
		}
	}
	return count, nil
}

// ConvertToSQLiteArgs parameters
type ConvertToSQLiteArgs struct {
	TLD          string
	RunPrefix    string
	BaseFilename string
	ArtifactKeys map[string]string // filename -> s3 key
}

// ConvertToSQLiteResult outcome
type ConvertToSQLiteResult struct {
	DBKey        string
	RunPrefix    string
	BaseFilename string
	// Best-effort metadata
	RegistrarMappingRowCount int
}

// ConvertToSQLite downloads CSV artifacts, builds an SQLite DB, and uploads it to S3 under the runPrefix
func (a *EscrowImportActivities) ConvertToSQLite(ctx context.Context, args ConvertToSQLiteArgs) (ConvertToSQLiteResult, error) {
	tld := strings.TrimSpace(args.TLD)
	if tld == "" {
		return ConvertToSQLiteResult{}, fmt.Errorf("tld is required")
	}
	runPrefix := strings.TrimSpace(args.RunPrefix)
	if runPrefix == "" {
		return ConvertToSQLiteResult{}, fmt.Errorf("runPrefix is required")
	}
	base := strings.TrimSpace(args.BaseFilename)
	if base == "" {
		return ConvertToSQLiteResult{}, fmt.Errorf("baseFilename is required")
	}

	s3c, err := storage.NewS3ClientFromEnv()
	if err != nil {
		return ConvertToSQLiteResult{}, err
	}

	// Prepare a working directory
	workDir, err := os.MkdirTemp("", "escrow-sqlite-*")
	if err != nil {
		return ConvertToSQLiteResult{}, err
	}
	defer os.RemoveAll(workDir)

	// Download required CSVs if present
	// We'll download all provided artifacts to workDir preserving filenames
	for filename, key := range args.ArtifactKeys {
		dst := filepath.Join(workDir, filename)
		// Download to temp first, then move to expected name
		tmpPath, err := s3c.DownloadToFile(ctx, key)
		if err != nil {
			return ConvertToSQLiteResult{}, fmt.Errorf("download artifact %s failed: %w", filename, err)
		}
		// Rename to the desired destination
		if err := os.Rename(tmpPath, dst); err != nil {
			// If rename across fs fails, try a copy
			in, oerr := os.Open(tmpPath)
			if oerr != nil {
				return ConvertToSQLiteResult{}, oerr
			}
			defer in.Close()
			out, cerr := os.Create(dst)
			if cerr != nil {
				return ConvertToSQLiteResult{}, cerr
			}
			if _, cerr = io.Copy(out, in); cerr != nil {
				out.Close()
				return ConvertToSQLiteResult{}, cerr
			}
			out.Close()
			_ = os.Remove(tmpPath)
		}
	}

	// Build DB using CSVToSQLiteService; basePath must include workDir
	basePath := filepath.Join(workDir, base)
	dbPath := basePath + ".db"
	svc := services.NewCSVToSQLiteService(basePath)
	if err := svc.ConvertToSQLite(dbPath); err != nil {
		return ConvertToSQLiteResult{}, fmt.Errorf("csv to sqlite failed: %w", err)
	}

	// Count registrar mapping rows (optional table)
	mappingRows := 0
	if db, oerr := sql.Open("sqlite", dbPath); oerr == nil {
		defer db.Close()
		var c int64
		if err := db.QueryRow(`SELECT COUNT(1) FROM registrar_mapping`).Scan(&c); err == nil {
			if c > 0 {
				mappingRows = int(c)
			}
		}
	}

	// Upload DB to S3 under runPrefix/base.db
	dbKey := runPrefix + "/" + filepath.Base(dbPath)
	if err := s3c.UploadFile(ctx, dbKey, dbPath, "application/octet-stream"); err != nil {
		return ConvertToSQLiteResult{}, fmt.Errorf("upload db failed: %w", err)
	}

	return ConvertToSQLiteResult{DBKey: dbKey, RunPrefix: runPrefix, BaseFilename: base, RegistrarMappingRowCount: mappingRows}, nil
}

// ImportFromSQLiteArgs parameters
type ImportFromSQLiteArgs struct {
	TLD   string
	DBKey string
}

// ImportFromSQLiteResult outcome
type ImportFromSQLiteResult struct {
	DBKey  string
	Counts map[string]int64
	// Run-time observability (large payloads stored out-of-band)
	EventsKey string           // S3 key where detailed events are stored (JSON)
	Tallies   map[string]int64 // compact counters safe to return
}

// ReportEvent captures warnings/errors/skips during workflow activities
type ReportEvent struct {
	Level     string            `json:"level"`    // info|warn|error
	Activity  string            `json:"activity"` // e.g., ImportFromSQLite.hosts
	Code      string            `json:"code"`     // machine-readable code
	Message   string            `json:"message"`
	Object    string            `json:"object,omitempty"`
	Context   map[string]string `json:"context,omitempty"`
	Timestamp string            `json:"timestamp"`
}

func nowUTC() string { return time.Now().UTC().Format(time.RFC3339) }

// ImportFromSQLite currently inspects the SQLite DB and returns row counts as a sanity check.
// In a later iteration, this will stream-import data into Postgres in chunks.
func (a *EscrowImportActivities) ImportFromSQLite(ctx context.Context, args ImportFromSQLiteArgs) (ImportFromSQLiteResult, error) {
	tld := strings.TrimSpace(args.TLD)
	if tld == "" {
		return ImportFromSQLiteResult{}, fmt.Errorf("tld is required")
	}
	dbKey := strings.TrimSpace(args.DBKey)
	if dbKey == "" {
		return ImportFromSQLiteResult{}, fmt.Errorf("dbKey is required")
	}

	s3c, err := storage.NewS3ClientFromEnv()
	if err != nil {
		return ImportFromSQLiteResult{}, err
	}
	// Download DB locally
	dbPath, err := s3c.DownloadToFile(ctx, dbKey)
	if err != nil {
		return ImportFromSQLiteResult{}, fmt.Errorf("download db failed: %w", err)
	}
	defer os.Remove(dbPath)

	// Open SQLite DB
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return ImportFromSQLiteResult{}, fmt.Errorf("open sqlite failed: %w", err)
	}
	defer db.Close()

	// Load registrar clid mapping from SQLite (if present)
	clidMap := map[string]string{}
	if rows, qerr := db.Query(`SELECT escrow_id, registrar_clid FROM registrar_mapping`); qerr == nil {
		defer rows.Close()
		for rows.Next() {
			var escrowID, mapped sql.NullString
			if err := rows.Scan(&escrowID, &mapped); err == nil {
				if escrowID.Valid {
					if mapped.Valid && strings.TrimSpace(mapped.String) != "" {
						clidMap[escrowID.String] = mapped.String
					}
				}
			}
		}
	}

	// Connect to Postgres using env
	pgCfg := pg.Config{
		User:        os.Getenv("DB_USER"),
		Pass:        os.Getenv("DB_PASS"),
		Host:        os.Getenv("DB_HOST"),
		Port:        os.Getenv("DB_PORT"),
		DBName:      os.Getenv("DB_NAME"),
		SSLmode:     defaultStr(os.Getenv("DB_SSLMODE"), "disable"),
		AutoMigrate: false,
	}
	gdb, err := pg.NewConnection(pgCfg)
	if err != nil {
		return ImportFromSQLiteResult{}, fmt.Errorf("postgres connection failed: %w", err)
	}

	// Build services
	idgen, err := snowflakeidgenerator.NewIDGenerator()
	if err != nil {
		return ImportFromSQLiteResult{}, fmt.Errorf("id generator failed: %w", err)
	}
	roidSvc := services.NewRoidService(idgen)

	// Repositories
	domRepo := pg.NewDomainRepository(gdb)
	hostRepo := pg.NewGormHostRepository(gdb)
	addrRepo := pg.NewGormHostAddressRepository(gdb)
	// Repos required by DomainService constructor but unused in BulkCreate
	nndnRepo := pg.NewGormNNDNRepository(gdb)
	tldRepo := pg.NewGormTLDRepo(gdb)
	phaseRepo := pg.NewGormPhaseRepository(gdb)
	premLabelRepo := pg.NewGORMPremiumLabelRepository(gdb)
	fxRepo := pg.NewFXRepository(gdb)
	rarRepo := pg.NewGormRegistrarRepository(gdb)

	// Services
	hostSvc := services.NewHostService(hostRepo, addrRepo, roidSvc)
	contactRepo := pg.NewContactRepository(gdb)
	contactSvc := services.NewContactService(contactRepo, *roidSvc)
	domSvc := services.NewDomainService(domRepo, hostRepo, *roidSvc, nndnRepo, tldRepo, phaseRepo, premLabelRepo, fxRepo, rarRepo)

	counts := map[string]int64{}
	events := []ReportEvent{}
	tallies := map[string]int64{}

	// 0) Import contacts in chunks (must exist before domains due to FK on registrant)
	if err := a.importContactsChunked(ctx, db, contactSvc, counts, clidMap, &events, tallies); err != nil {
		return ImportFromSQLiteResult{}, fmt.Errorf("contact import failed: %w", err)
	}

	// 1) Import hosts in chunks
	if err := a.importHostsChunked(ctx, db, hostSvc, counts, clidMap, &events, tallies); err != nil {
		return ImportFromSQLiteResult{}, fmt.Errorf("host import failed: %w", err)
	}

	// 2) Import domains in chunks (without host associations yet)
	if err := a.importDomainsChunked(ctx, db, tld, domSvc, counts, clidMap, &events, tallies); err != nil {
		return ImportFromSQLiteResult{}, fmt.Errorf("domain import failed: %w", err)
	}

	// 3) Link domain nameservers → domain_hosts via DomainService to ensure statuses update
	if err := a.linkDomainHosts(ctx, db, domSvc, counts, &events, tallies); err != nil {
		return ImportFromSQLiteResult{}, fmt.Errorf("domain-host linking failed: %w", err)
	}

	// Also compute a few table counts from SQLite for sanity (best-effort)
	for _, t := range []string{"domains", "hosts", "domain_nameservers"} {
		var c int64
		q := "SELECT COUNT(1) FROM " + t
		if err := db.QueryRow(q).Scan(&c); err == nil {
			counts["sqlite_"+t] = c
		}
	}

	// Persist detailed events out-of-band to avoid exceeding payload limits
	// Derive runPrefix from dbKey: runPrefix/base.db
	runPrefix := path.Dir(dbKey)
	if len(events) > 0 {
		// Write events to a temp JSON and upload under runPrefix/import-events.json
		tmp, _ := os.CreateTemp("", "escrow-import-events-*.json")
		if tmp != nil {
			enc := json.NewEncoder(tmp)
			enc.SetIndent("", "  ")
			_ = enc.Encode(map[string]any{
				"tld":       tld,
				"dbKey":     dbKey,
				"createdAt": time.Now().UTC().Format(time.RFC3339),
				"events":    events,
			})
			tmp.Close()
			if s3c2, e2 := storage.NewS3ClientFromEnv(); e2 == nil {
				_ = s3c2.UploadFile(ctx, runPrefix+"/import-events.json", tmp.Name(), "application/json")
			}
			_ = os.Remove(tmp.Name())
		}
	}

	return ImportFromSQLiteResult{DBKey: dbKey, Counts: counts, EventsKey: runPrefix + "/import-events.json", Tallies: tallies}, nil
}

// FinalizeAndQAArgs parameters
type FinalizeAndQAArgs struct {
	TLD            string
	RunPrefix      string
	AnalysisCounts map[string]int64
	SqliteCounts   map[string]int64
	// Optional: include analysis findings for summary
	AnalysisErrors  []string
	MissingContacts []string
}

// FinalizeAndQAResult outcome
type FinalizeAndQAResult struct {
	SummaryKey string
}

// FinalizeAndQA writes a summary JSON to S3 comparing analysis and sqlite counts
func (a *EscrowImportActivities) FinalizeAndQA(ctx context.Context, args FinalizeAndQAArgs) (FinalizeAndQAResult, error) {
	tld := strings.TrimSpace(args.TLD)
	if tld == "" {
		return FinalizeAndQAResult{}, fmt.Errorf("tld is required")
	}
	runPrefix := strings.TrimSpace(args.RunPrefix)
	if runPrefix == "" {
		return FinalizeAndQAResult{}, fmt.Errorf("runPrefix is required")
	}

	// Derive Postgres counts for this TLD (and a couple of global-ish numbers)
	pgCfg := pg.Config{
		User:        os.Getenv("DB_USER"),
		Pass:        os.Getenv("DB_PASS"),
		Host:        os.Getenv("DB_HOST"),
		Port:        os.Getenv("DB_PORT"),
		DBName:      os.Getenv("DB_NAME"),
		SSLmode:     defaultStr(os.Getenv("DB_SSLMODE"), "disable"),
		AutoMigrate: false,
	}
	gdb, err := pg.NewConnection(pgCfg)
	if err != nil {
		return FinalizeAndQAResult{}, fmt.Errorf("postgres connection failed: %w", err)
	}

	type cRow struct{ Cnt int64 }
	var row cRow
	pgCounts := map[string]int64{}

	// Domains for this TLD
	if r := gdb.Raw("SELECT COUNT(1) AS cnt FROM domains WHERE tld_name = ?", tld).Scan(&row); r.Error == nil {
		pgCounts["postgres_domains_for_tld"] = row.Cnt
	}
	// Domain↔Host links for this TLD
	row = cRow{}
	if r := gdb.Raw(`
		SELECT COUNT(1) AS cnt
		FROM domain_hosts dh
		INNER JOIN domains d ON d.ro_id = dh.domain_ro_id
		WHERE d.tld_name = ?
	`, tld).Scan(&row); r.Error == nil {
		pgCounts["postgres_domain_hosts_links_for_tld"] = row.Cnt
	}
	// Distinct hosts linked to domains in this TLD (approximate host count for this import)
	row = cRow{}
	if r := gdb.Raw(`
		SELECT COUNT(DISTINCT dh.host_ro_id) AS cnt
		FROM domain_hosts dh
		INNER JOIN domains d ON d.ro_id = dh.domain_ro_id
		WHERE d.tld_name = ?
	`, tld).Scan(&row); r.Error == nil {
		pgCounts["postgres_distinct_hosts_linked_for_tld"] = row.Cnt
	}

	// Separate out import counters (from args.SqliteCounts we also stored *_imported keys)
	importCounts := map[string]int64{}
	for k, v := range args.SqliteCounts {
		if strings.HasSuffix(k, "_imported") || k == "domain_hosts_linked" {
			importCounts[k] = v
		}
	}

	// Build discrepancy summary focusing on key entities
	aDomains := args.AnalysisCounts["domains"]
	aHosts := args.AnalysisCounts["hosts"]
	sDomains := args.SqliteCounts["sqlite_domains"]
	sHosts := args.SqliteCounts["sqlite_hosts"]
	sLinks := args.SqliteCounts["sqlite_domain_nameservers"]
	iDomains := args.SqliteCounts["domains_imported"]
	iHosts := args.SqliteCounts["hosts_imported"]
	iLinks := args.SqliteCounts["domain_hosts_linked"]
	pDomains := pgCounts["postgres_domains_for_tld"]
	pDistinctHosts := pgCounts["postgres_distinct_hosts_linked_for_tld"]
	pLinks := pgCounts["postgres_domain_hosts_links_for_tld"]

	discrepancies := map[string]any{
		"domains": map[string]any{
			"analysis": aDomains,
			"sqlite":   sDomains,
			"imported": iDomains,
			"postgres": pDomains,
			"deltas": map[string]int64{
				"analysis_vs_sqlite":   aDomains - sDomains,
				"analysis_vs_postgres": aDomains - pDomains,
				"sqlite_vs_imported":   sDomains - iDomains,
				"imported_vs_postgres": iDomains - pDomains,
			},
		},
		"hosts": map[string]any{
			"analysis":                 aHosts,
			"sqlite":                   sHosts,
			"imported":                 iHosts,
			"postgres_distinct_linked": pDistinctHosts,
			"deltas": map[string]int64{
				"analysis_vs_sqlite":            aHosts - sHosts,
				"analysis_vs_postgres_distinct": aHosts - pDistinctHosts,
				"sqlite_vs_imported":            sHosts - iHosts,
				"imported_vs_postgres_distinct": iHosts - pDistinctHosts,
			},
		},
		"domain_host_links": map[string]any{
			"sqlite_domain_nameservers": sLinks,
			"imported_links":            iLinks,
			"postgres_links":            pLinks,
			"deltas": map[string]int64{
				"sqlite_vs_imported":   sLinks - iLinks,
				"imported_vs_postgres": iLinks - pLinks,
			},
		},
	}

	summary := map[string]any{
		"tld":            tld,
		"runPrefix":      runPrefix,
		"analysisCounts": args.AnalysisCounts,
		"sqliteCounts":   args.SqliteCounts,
		"importCounts":   importCounts,
		"postgresCounts": pgCounts,
		"discrepancies":  discrepancies,
		"completedAt":    time.Now().UTC().Format(time.RFC3339),
	}

	// Attach analysis issues if provided
	if len(args.AnalysisErrors) > 0 || len(args.MissingContacts) > 0 {
		summary["analysisFindings"] = map[string]any{
			"errors":          args.AnalysisErrors,
			"missingContacts": args.MissingContacts,
		}
	}

	// Write to temp file
	tmp, err := os.CreateTemp("", "escrow-summary-*.json")
	if err != nil {
		return FinalizeAndQAResult{}, err
	}
	enc := json.NewEncoder(tmp)
	enc.SetIndent("", "  ")
	if err := enc.Encode(summary); err != nil {
		tmp.Close()
		return FinalizeAndQAResult{}, err
	}
	tmp.Close()

	s3c, err := storage.NewS3ClientFromEnv()
	if err != nil {
		return FinalizeAndQAResult{}, err
	}
	// Upload to S3 under runPrefix/summary.json
	key := runPrefix + "/summary.json"
	if err := s3c.UploadFile(ctx, key, tmp.Name(), "application/json"); err != nil {
		return FinalizeAndQAResult{}, err
	}
	// Clean up local temp
	_ = os.Remove(tmp.Name())

	// Also persist a minimal run-report.json if one doesn't already exist
	reportKey := runPrefix + "/run-report.json"
	if exists, _ := s3c.Exists(ctx, reportKey); !exists {
		report := map[string]any{
			"tld":           tld,
			"runPrefix":     runPrefix,
			"completedAt":   time.Now().UTC().Format(time.RFC3339),
			"analysis":      args.AnalysisCounts,
			"sqlite":        args.SqliteCounts,
			"postgres":      pgCounts,
			"discrepancies": discrepancies,
		}
		if len(args.AnalysisErrors) > 0 || len(args.MissingContacts) > 0 {
			report["analysisFindings"] = map[string]any{
				"errors":          args.AnalysisErrors,
				"missingContacts": args.MissingContacts,
			}
		}
		tmp2, err := os.CreateTemp("", "escrow-run-report-*.json")
		if err == nil {
			enc2 := json.NewEncoder(tmp2)
			enc2.SetIndent("", "  ")
			if err := enc2.Encode(report); err == nil {
				tmp2.Close()
				_ = s3c.UploadFile(ctx, reportKey, tmp2.Name(), "application/json")
				_ = os.Remove(tmp2.Name())
			} else {
				tmp2.Close()
				_ = os.Remove(tmp2.Name())
			}
		}
	}

	return FinalizeAndQAResult{SummaryKey: key}, nil
}

// ---- helpers for ImportFromSQLite ----

func defaultStr(s, def string) string {
	if strings.TrimSpace(s) == "" {
		return def
	}
	return s
}

// importHostsChunked reads hosts and addresses from SQLite and bulk-creates them in Postgres.
func (a *EscrowImportActivities) importHostsChunked(ctx context.Context, sqldb *sql.DB, hostSvc *services.HostService, counts map[string]int64, clidMap map[string]string, events *[]ReportEvent, tallies map[string]int64) error {
	const pageSize = 1000
	offset := 0
	created := int64(0)

	for {
		rows, err := sqldb.Query(`
			SELECT name, clid, crrr, uprr FROM hosts ORDER BY name LIMIT ? OFFSET ?
		`, pageSize, offset)
		if err != nil {
			return err
		}

		cmds := make([]*commands.CreateHostCommand, 0, pageSize)
		names := make([]string, 0, pageSize)
		clids := make([]string, 0, pageSize)
		for rows.Next() {
			var name, clid, crrr, uprr sql.NullString
			if err := rows.Scan(&name, &clid, &crrr, &uprr); err != nil {
				rows.Close()
				return err
			}
			// map clids using registrar mapping if available
			mapClid := func(raw string) string {
				r := strings.TrimSpace(raw)
				if r == "" {
					return raw
				}
				if v, ok := clidMap[r]; ok && strings.TrimSpace(v) != "" {
					return v
				}
				return raw
			}
			cmd := &commands.CreateHostCommand{
				Name: name.String,
				ClID: entities.ClIDType(mapClid(clid.String)),
			}
			if crrr.Valid {
				cmd.CrRr = entities.ClIDType(mapClid(crrr.String))
			}
			if uprr.Valid {
				cmd.UpRr = entities.ClIDType(mapClid(uprr.String))
			}
			cmds = append(cmds, cmd)
			names = append(names, name.String)
			clids = append(clids, clid.String)
		}
		rows.Close()

		if len(cmds) == 0 {
			break
		}

		// Attach addresses per host
		for i, hn := range names {
			clid := clids[i]
			// gather addresses
			addrRows, err := sqldb.Query(`SELECT ip_address FROM host_addresses WHERE host_name = ?`, hn)
			if err != nil {
				return err
			}
			for addrRows.Next() {
				var ip string
				if err := addrRows.Scan(&ip); err == nil && ip != "" {
					cmds[i].Addresses = append(cmds[i].Addresses, ip)
				}
			}
			addrRows.Close()
			_ = clid // clid is already on the command

			// gather statuses
			stRows, err := sqldb.Query(`SELECT status FROM host_statuses WHERE host_name = ?`, hn)
			if err == nil {
				hs := entities.HostStatus{}
				for stRows.Next() {
					var st string
					if err := stRows.Scan(&st); err == nil {
						switch strings.ToLower(strings.TrimSpace(st)) {
						case "ok":
							hs.OK = true
						case "linked":
							hs.Linked = true
						case "pendingcreate":
							hs.PendingCreate = true
						case "pendingdelete":
							hs.PendingDelete = true
						case "pendingupdate":
							hs.PendingUpdate = true
						case "pendingtransfer":
							hs.PendingTransfer = true
						case "clientdeleteprohibited":
							hs.ClientDeleteProhibited = true
						case "clientupdateprohibited":
							hs.ClientUpdateProhibited = true
						case "serverdeleteprohibited":
							hs.ServerDeleteProhibited = true
						case "serverupdateprohibited":
							hs.ServerUpdateProhibited = true
						}
					}
				}
				stRows.Close()
				// Ensure OK is set when no prohibitions/pending are present (per RFC and our validation rules)
				if !hs.OK && !hs.PendingCreate && !hs.PendingDelete && !hs.PendingTransfer && !hs.PendingUpdate && !hs.ClientDeleteProhibited && !hs.ClientUpdateProhibited && !hs.ServerDeleteProhibited && !hs.ServerUpdateProhibited {
					hs.OK = true
				}
				// Assign if any status was provided (or OK fallback applied)
				if !hs.IsNil() {
					cmds[i].Status = hs
				}
			}
		}

		if err := hostSvc.BulkCreate(ctx, cmds); err != nil {
			// If bulk fails (e.g., duplicates), try per-item create for idempotency
			for _, c := range cmds {
				if _, ierr := hostSvc.CreateHost(ctx, c); ierr != nil && !errors.Is(ierr, entities.ErrInvalidHost) {
					// collect error event and continue; keep import resilient
					*events = append(*events, ReportEvent{
						Level:     "error",
						Activity:  "ImportFromSQLite.hosts",
						Code:      "host_insert_failed",
						Message:   ierr.Error(),
						Object:    string(c.ClID),
						Context:   map[string]string{"name": c.Name},
						Timestamp: nowUTC(),
					})
					tallies["hosts_failed"]++
					continue
				}
				created++
			}
		} else {
			created += int64(len(cmds))
		}

		counts["hosts_imported"] = created
		activity.RecordHeartbeat(ctx, fmt.Sprintf("hosts imported: %d", created))

		if len(cmds) < pageSize {
			break
		}
		offset += pageSize
	}
	return nil
}

// importDomainsChunked reads domains from SQLite and bulk-creates them in Postgres.
func (a *EscrowImportActivities) importDomainsChunked(ctx context.Context, sqldb *sql.DB, tld string, domSvc *services.DomainService, counts map[string]int64, clidMap map[string]string, events *[]ReportEvent, tallies map[string]int64) error {
	const pageSize = 1000
	offset := 0
	created := int64(0)

	for {
		rows, err := sqldb.Query(`
			SELECT name, registrant, clid, crrr, crdate, exdate, uprr, uname, originalname
			FROM domains ORDER BY name LIMIT ? OFFSET ?
		`, pageSize, offset)
		if err != nil {
			return err
		}

		var cmds []*commands.CreateDomainCommand
		for rows.Next() {
			var name, registrant, clid, crrr, crdate, exdate, uprr, uname, original sql.NullString
			if err := rows.Scan(&name, &registrant, &clid, &crrr, &crdate, &exdate, &uprr, &uname, &original); err != nil {
				rows.Close()
				return err
			}

			cmd := &commands.CreateDomainCommand{
				Name:         name.String,
				ClID:         clid.String,
				AuthInfo:     "escr0W1mP*rt", // strong default authinfo
				OriginalName: original.String,
				UName:        uname.String,
			}
			// map clids using registrar mapping if available
			mapClid := func(raw string) string {
				r := strings.TrimSpace(raw)
				if r == "" {
					return raw
				}
				if v, ok := clidMap[r]; ok && strings.TrimSpace(v) != "" {
					return v
				}
				return raw
			}
			cmd.ClID = mapClid(cmd.ClID)
			if registrant.Valid {
				cmd.RegistrantID = registrant.String
			}
			if crrr.Valid {
				cmd.CrRr = mapClid(crrr.String)
			}
			if uprr.Valid {
				cmd.UpRr = mapClid(uprr.String)
			}
			if exdate.Valid {
				if ts, perr := parseBestEffortTime(exdate.String); perr == nil {
					cmd.ExpiryDate = ts
				}
			}
			if crdate.Valid {
				if ts, perr := parseBestEffortTime(crdate.String); perr == nil {
					cmd.CreatedAt = ts
				}
			}
			// Map domain/host statuses from SQLite status tables
			// Domain statuses
			if name.Valid {
				stRows, sErr := sqldb.Query(`SELECT status FROM domain_statuses WHERE domain_name = ?`, name.String)
				if sErr == nil {
					ds := entities.DomainStatus{}
					for stRows.Next() {
						var st string
						if err := stRows.Scan(&st); err == nil {
							switch strings.ToLower(strings.TrimSpace(st)) {
							case entities.DomainStatusOK:
								ds.OK = true
							case entities.DomainStatusInactive:
								ds.Inactive = true
							case entities.DomainStatusClientTransferProhibited:
								ds.ClientTransferProhibited = true
							case entities.DomainStatusClientUpdateProhibited:
								ds.ClientUpdateProhibited = true
							case entities.DomainStatusClientDeleteProhibited:
								ds.ClientDeleteProhibited = true
							case entities.DomainStatusClientRenewProhibited:
								ds.ClientRenewProhibited = true
							case entities.DomainStatusClientHold:
								ds.ClientHold = true
							case entities.DomainStatusServerTransferProhibited:
								ds.ServerTransferProhibited = true
							case entities.DomainStatusServerUpdateProhibited:
								ds.ServerUpdateProhibited = true
							case entities.DomainStatusServerDeleteProhibited:
								ds.ServerDeleteProhibited = true
							case entities.DomainStatusServerRenewProhibited:
								ds.ServerRenewProhibited = true
							case entities.DomainStatusServerHold:
								ds.ServerHold = true
							case entities.DomainStatusPendingCreate:
								ds.PendingCreate = true
							case entities.DomainStatusPendingRenew:
								ds.PendingRenew = true
							case entities.DomainStatusPendingTransfer:
								ds.PendingTransfer = true
							case entities.DomainStatusPendingUpdate:
								ds.PendingUpdate = true
							case entities.DomainStatusPendingRestore:
								ds.PendingRestore = true
							case entities.DomainStatusPendingDelete:
								ds.PendingDelete = true
							}
						}
					}
					stRows.Close()
					// Ensure OK is set when no prohibitions/pending are present (allowed with Inactive)
					if !ds.OK && !ds.HasProhibitions() && !ds.HasPendings() {
						ds.OK = true
					}
					if !ds.IsNil() {
						cmd.Status = ds
					}
				}
			}

			// TLD is computed from name in service; not set here
			cmds = append(cmds, cmd)
		}
		rows.Close()

		if len(cmds) == 0 {
			break
		}

		if err := domSvc.BulkCreate(ctx, cmds); err != nil {
			// On error (duplicates, etc.), try per-item to be idempotent
			for _, c := range cmds {
				if _, ierr := domSvc.Create(ctx, c); ierr != nil {
					// collect error and continue
					*events = append(*events, ReportEvent{
						Level:     "error",
						Activity:  "ImportFromSQLite.domains",
						Code:      "domain_insert_failed",
						Message:   ierr.Error(),
						Object:    c.Name,
						Context:   map[string]string{"clid": c.ClID},
						Timestamp: nowUTC(),
					})
					tallies["domains_failed"]++
					continue
				}
				created++
			}
		} else {
			created += int64(len(cmds))
		}

		counts["domains_imported"] = created
		activity.RecordHeartbeat(ctx, fmt.Sprintf("domains imported: %d", created))

		if len(cmds) < pageSize {
			break
		}
		offset += pageSize
	}
	return nil
}

// linkDomainHosts links domains to hosts based on domain_nameservers table using DomainService to enforce status updates.
func (a *EscrowImportActivities) linkDomainHosts(ctx context.Context, sqldb *sql.DB, domSvc *services.DomainService, counts map[string]int64, events *[]ReportEvent, tallies map[string]int64) error {
	const pageSize = 2000
	offset := 0
	linked := int64(0)

	for {
		rows, err := sqldb.Query(`
			SELECT domain_name, nameserver FROM domain_nameservers ORDER BY domain_name LIMIT ? OFFSET ?
		`, pageSize, offset)
		if err != nil {
			return err
		}

		type pair struct{ dom, ns string }
		pairs := make([]pair, 0, pageSize)
		for rows.Next() {
			var dom, ns sql.NullString
			if err := rows.Scan(&dom, &ns); err != nil {
				rows.Close()
				return err
			}
			if dom.Valid && ns.Valid {
				pairs = append(pairs, pair{dom: dom.String, ns: ns.String})
			}
		}
		rows.Close()

		if len(pairs) == 0 {
			break
		}

		for _, p := range pairs {
			// Use DomainService to add host by name; force=true to ignore update prohibitions during escrow import.
			if err := domSvc.AddHostToDomainByHostName(ctx, p.dom, p.ns, true); err == nil {
				linked++
			} else {
				*events = append(*events, ReportEvent{Level: "warn", Activity: "ImportFromSQLite.link", Code: "link_failed", Message: err.Error(), Object: p.dom, Context: map[string]string{"host": p.ns}, Timestamp: nowUTC()})
				tallies["links_skipped"]++
			}
		}

		counts["domain_hosts_linked"] = linked
		activity.RecordHeartbeat(ctx, fmt.Sprintf("domain-host links: %d", linked))

		if len(pairs) < pageSize {
			break
		}
		offset += pageSize
	}
	return nil
}

// importContactsChunked reads contacts from SQLite and creates them in Postgres before domains.
func (a *EscrowImportActivities) importContactsChunked(ctx context.Context, sqldb *sql.DB, contactSvc *services.ContactService, counts map[string]int64, clidMap map[string]string, events *[]ReportEvent, tallies map[string]int64) error {
	const pageSize = 1000
	offset := 0
	created := int64(0)

	for {
		rows, err := sqldb.Query(`
			SELECT id, roid, voice, fax, email, clid, crrr, crdate, uprr, "update"
			FROM contacts ORDER BY id LIMIT ? OFFSET ?
		`, pageSize, offset)
		if err != nil {
			return err
		}

		cmds := make([]*commands.CreateContactCommand, 0, pageSize)
		for rows.Next() {
			var id, roid, voice, fax, email, clid, crrr, crdate, uprr, upDate sql.NullString
			if err := rows.Scan(&id, &roid, &voice, &fax, &email, &clid, &crrr, &crdate, &uprr, &upDate); err != nil {
				rows.Close()
				return err
			}

			// map registrar clids using registrar mapping if available
			mapClid := func(raw string) string {
				r := strings.TrimSpace(raw)
				if r == "" {
					return raw
				}
				if v, ok := clidMap[r]; ok && strings.TrimSpace(v) != "" {
					return v
				}
				return raw
			}

			cmd := &commands.CreateContactCommand{
				ID:       id.String,
				RoID:     roid.String,
				Email:    email.String,
				AuthInfo: "escr0W1mP*rt",
				ClID:     mapClid(clid.String),
			}
			// If RoID is present but invalid or not a CONTACT RoID, clear it to auto-generate a valid one.
			if strings.TrimSpace(cmd.RoID) != "" {
				r := entities.RoidType(cmd.RoID)
				if err := r.Validate(); err != nil || r.ObjectIdentifier() != entities.CONTACT_ROID_ID {
					cmd.RoID = ""
				}
			}
			if voice.Valid {
				cmd.Voice = voice.String
			}
			if fax.Valid {
				cmd.Fax = fax.String
			}
			if crrr.Valid {
				cmd.CrRr = mapClid(crrr.String)
			}
			if uprr.Valid {
				cmd.UpRr = mapClid(uprr.String)
			}
			if crdate.Valid {
				if ts, perr := parseBestEffortTime(crdate.String); perr == nil {
					cmd.CreatedAt = ts
				}
			}
			if upDate.Valid {
				if ts, perr := parseBestEffortTime(upDate.String); perr == nil {
					cmd.UpdatedAt = ts
				}
			}

			// Populate Status from contact_statuses
			{
				srows, serr := sqldb.Query(`SELECT status FROM contact_statuses WHERE contact_id = ?`, id.String)
				if serr == nil {
					st := entities.ContactStatus{}
					for srows.Next() {
						var stval sql.NullString
						if err := srows.Scan(&stval); err == nil && stval.Valid {
							switch strings.ToLower(strings.TrimSpace(stval.String)) {
							case "ok":
								st.OK = true
							case "linked":
								st.Linked = true
							case "pendingcreate":
								st.PendingCreate = true
							case "pendingupdate":
								st.PendingUpdate = true
							case "pendingtransfer":
								st.PendingTransfer = true
							case "pendingdelete":
								st.PendingDelete = true
							case "clientdeleteprohibited":
								st.ClientDeleteProhibited = true
							case "clientupdateprohibited":
								st.ClientUpdateProhibited = true
							case "clienttransferprohibited":
								st.ClientTransferProhibited = true
							case "serverdeleteprohibited":
								st.ServerDeleteProhibited = true
							case "serverupdateprohibited":
								st.ServerUpdateProhibited = true
							case "servertransferprohibited":
								st.ServerTransferProhibited = true
							}
						}
					}
					srows.Close()
					cmd.Status = st
				}
			}

			// Populate PostalInfo from contact_postal_info
			{
				piRows, piErr := sqldb.Query(`
					SELECT type, name, org, street1, street2, street3, city, state_province, postal_code, country_code
					FROM contact_postal_info WHERE contact_id = ?
				`, id.String)
				if piErr == nil {
					// initialize array
					var postalInt, postalLoc *entities.ContactPostalInfo
					for piRows.Next() {
						var t, name, org, street1, street2, street3, city, sp, pc, cc sql.NullString
						if err := piRows.Scan(&t, &name, &org, &street1, &street2, &street3, &city, &sp, &pc, &cc); err != nil {
							continue
						}
						tval := strings.ToLower(strings.TrimSpace(t.String))
						// Build Address
						if strings.TrimSpace(city.String) == "" || strings.TrimSpace(cc.String) == "" {
							// invalid minimal address; skip
							continue
						}
						addr, aerr := entities.NewAddress(city.String, cc.String)
						if aerr != nil {
							continue
						}
						if street1.Valid {
							addr.Street1 = entities.OptPostalLineType(street1.String)
						}
						if street2.Valid {
							addr.Street2 = entities.OptPostalLineType(street2.String)
						}
						if street3.Valid {
							addr.Street3 = entities.OptPostalLineType(street3.String)
						}
						if sp.Valid {
							addr.StateProvince = entities.OptPostalLineType(sp.String)
						}
						if pc.Valid {
							if ppt, perr := entities.NewPCType(pc.String); perr == nil {
								addr.PostalCode = *ppt
							}
						}
						// Build ContactPostalInfo
						nameStr := strings.TrimSpace(name.String)
						if nameStr == "" {
							// name required by entity validation
							continue
						}
						cpi, perr := entities.NewContactPostalInfo(tval, nameStr, addr)
						if perr != nil {
							// best-effort: skip invalid postalinfo for this row
							continue
						}
						if org.Valid {
							cpi.Org = entities.OptPostalLineType(org.String)
						}
						if tval == string(entities.PostalInfoEnumTypeINT) {
							postalInt = cpi
						} else if tval == string(entities.PostalInfoEnumTypeLOC) {
							postalLoc = cpi
						}
					}
					piRows.Close()
					// Assign into fixed array order: [0]=int, [1]=loc
					cmd.PostalInfo[0] = postalInt
					cmd.PostalInfo[1] = postalLoc
				}
			}
			cmds = append(cmds, cmd)
		}
		rows.Close()

		if len(cmds) == 0 {
			break
		}

		if err := contactSvc.BulkCreate(ctx, cmds); err != nil {
			// try per-item for idempotency
			for _, c := range cmds {
				if _, ierr := contactSvc.CreateContact(ctx, c); ierr != nil {
					*events = append(*events, ReportEvent{
						Level:     "error",
						Activity:  "ImportFromSQLite.contacts",
						Code:      "contact_insert_failed",
						Message:   ierr.Error(),
						Object:    c.ID,
						Context:   map[string]string{"clid": c.ClID},
						Timestamp: nowUTC(),
					})
					tallies["contacts_failed"]++
					continue
				}
				created++
			}
		} else {
			created += int64(len(cmds))
		}

		counts["contacts_imported"] = created
		activity.RecordHeartbeat(ctx, fmt.Sprintf("contacts imported: %d", created))

		if len(cmds) < pageSize {
			break
		}
		offset += pageSize
	}
	return nil
}

func parseBestEffortTime(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, fmt.Errorf("empty time")
	}
	layouts := []string{
		time.RFC3339,
		"2006-01-02T15:04:05Z07:00",
		"2006-01-02 15:04:05",
		"2006-01-02",
		"2006-01-02 15:04:05.999",
	}
	var lastErr error
	for _, l := range layouts {
		if t, err := time.Parse(l, s); err == nil {
			return t, nil
		} else {
			lastErr = err
		}
	}
	// try unix seconds
	if sec, err := strconv.ParseInt(s, 10, 64); err == nil {
		return time.Unix(sec, 0).UTC(), nil
	}
	return time.Time{}, lastErr
}

// ---- run-report persistence ----

// PersistRunReportArgs for uploading a run-report.json artifact
type PersistRunReportArgs struct {
	TLD             string           `json:"tld"`
	RunPrefix       string           `json:"runPrefix"`
	WorkflowID      string           `json:"workflowId"`
	AnalysisErrors  []string         `json:"analysisErrors,omitempty"`
	MissingContacts []string         `json:"missingContacts,omitempty"`
	Events          []ReportEvent    `json:"events"`
	Tallies         map[string]int64 `json:"tallies"`
	Extra           map[string]any   `json:"extra,omitempty"`
}

type PersistRunReportResult struct{ Key string }

// PersistRunReport uploads the aggregated report as run-report.json under the runPrefix
func (a *EscrowImportActivities) PersistRunReport(ctx context.Context, args PersistRunReportArgs) (PersistRunReportResult, error) {
	if strings.TrimSpace(args.TLD) == "" || strings.TrimSpace(args.RunPrefix) == "" {
		return PersistRunReportResult{}, fmt.Errorf("tld and runPrefix are required")
	}

	payload := map[string]any{
		"tld":        args.TLD,
		"runPrefix":  args.RunPrefix,
		"workflowId": args.WorkflowID,
		"createdAt":  time.Now().UTC().Format(time.RFC3339),
		"analysis":   map[string]any{"errors": args.AnalysisErrors, "missingContacts": args.MissingContacts},
		"events":     args.Events,
		"tallies":    args.Tallies,
	}
	for k, v := range args.Extra {
		payload[k] = v
	}

	tmp, err := os.CreateTemp("", "escrow-run-report-*.json")
	if err != nil {
		return PersistRunReportResult{}, err
	}
	enc := json.NewEncoder(tmp)
	enc.SetIndent("", "  ")
	if err := enc.Encode(payload); err != nil {
		tmp.Close()
		return PersistRunReportResult{}, err
	}
	tmp.Close()

	s3c, err := storage.NewS3ClientFromEnv()
	if err != nil {
		return PersistRunReportResult{}, err
	}
	key := args.RunPrefix + "/run-report.json"
	if err := s3c.UploadFile(ctx, key, tmp.Name(), "application/json"); err != nil {
		return PersistRunReportResult{}, err
	}
	_ = os.Remove(tmp.Name())
	return PersistRunReportResult{Key: key}, nil
}
