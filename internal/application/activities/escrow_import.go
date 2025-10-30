package activities

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
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

	// Upload DB to S3 under runPrefix/base.db
	dbKey := runPrefix + "/" + filepath.Base(dbPath)
	if err := s3c.UploadFile(ctx, dbKey, dbPath, "application/octet-stream"); err != nil {
		return ConvertToSQLiteResult{}, fmt.Errorf("upload db failed: %w", err)
	}

	return ConvertToSQLiteResult{DBKey: dbKey, RunPrefix: runPrefix, BaseFilename: base}, nil
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
}

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
	domSvc := services.NewDomainService(domRepo, hostRepo, *roidSvc, nndnRepo, tldRepo, phaseRepo, premLabelRepo, fxRepo, rarRepo)

	counts := map[string]int64{}

	// 1) Import hosts in chunks
	if err := a.importHostsChunked(ctx, db, hostSvc, counts); err != nil {
		return ImportFromSQLiteResult{}, fmt.Errorf("host import failed: %w", err)
	}

	// 2) Import domains in chunks (without host associations yet)
	if err := a.importDomainsChunked(ctx, db, tld, domSvc, counts); err != nil {
		return ImportFromSQLiteResult{}, fmt.Errorf("domain import failed: %w", err)
	}

	// 3) Link domain nameservers → domain_hosts
	if err := a.linkDomainHosts(ctx, db, domRepo, hostRepo, counts); err != nil {
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

	return ImportFromSQLiteResult{DBKey: dbKey, Counts: counts}, nil
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
func (a *EscrowImportActivities) importHostsChunked(ctx context.Context, sqldb *sql.DB, hostSvc *services.HostService, counts map[string]int64) error {
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
			cmd := &commands.CreateHostCommand{
				Name: name.String,
				ClID: entities.ClIDType(clid.String),
			}
			if crrr.Valid {
				cmd.CrRr = entities.ClIDType(crrr.String)
			}
			if uprr.Valid {
				cmd.UpRr = entities.ClIDType(uprr.String)
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
		}

		if err := hostSvc.BulkCreate(ctx, cmds); err != nil {
			// If bulk fails (e.g., duplicates), try per-item create for idempotency
			for _, c := range cmds {
				if _, ierr := hostSvc.CreateHost(ctx, c); ierr != nil && !errors.Is(ierr, entities.ErrInvalidHost) {
					// log and continue; keep import resilient
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
func (a *EscrowImportActivities) importDomainsChunked(ctx context.Context, sqldb *sql.DB, tld string, domSvc *services.DomainService, counts map[string]int64) error {
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
			if registrant.Valid {
				cmd.RegistrantID = registrant.String
			}
			if crrr.Valid {
				cmd.CrRr = crrr.String
			}
			if uprr.Valid {
				cmd.UpRr = uprr.String
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
					// tolerate duplicates
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

// linkDomainHosts links domains to hosts based on domain_nameservers table.
func (a *EscrowImportActivities) linkDomainHosts(ctx context.Context, sqldb *sql.DB, domRepo *pg.DomainRepository, hostRepo *pg.HostRepository, counts map[string]int64) error {
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
			// find domain
			d, derr := domRepo.GetDomainByName(ctx, p.dom, false)
			if derr != nil || d == nil {
				continue
			}
			droid, derr := d.RoID.Int64()
			if derr != nil {
				continue
			}
			// find host clid from sqlite hosts
			var clid sql.NullString
			_ = sqldb.QueryRow(`SELECT clid FROM hosts WHERE name = ?`, p.ns).Scan(&clid)
			if !clid.Valid {
				continue
			}
			h, herr := hostRepo.GetHostByNameAndClID(ctx, p.ns, clid.String)
			if herr != nil || h == nil {
				continue
			}
			hroid, herr := h.RoID.Int64()
			if herr != nil {
				continue
			}
			if err := domRepo.AddHostToDomain(ctx, droid, hroid); err == nil {
				linked++
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
