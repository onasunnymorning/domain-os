package activities

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	reqUrl "net/url"
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
	"github.com/onasunnymorning/domain-os/internal/infrastructure/storage"
	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/temporal"
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

	// Determine base filename early for stable path generation
	base := strings.TrimSuffix(filepath.Base(key), filepath.Ext(key))

	// Stable Run Prefix: escrow/<tld>/processed/<base>
	// This usage of a deterministic path (vs random WorkflowID) allows resumption by finding existing artifacts.
	runPrefix := filepath.ToSlash(filepath.Join("escrow", tld, "processed", base))

	// Resumption Check: If analysis.json exists, we assume this step was already done.
	// We download the analysis JSON to populate the result struct correctly.
	analysisKey := runPrefix + "/" + base + "-analysis.json"
	if exists, _ := s3c.Exists(ctx, analysisKey); exists {
		activity.GetLogger(ctx).Info("Resuming: Analysis artifacts found, skipping processing", "prefix", runPrefix)

		// Download analysis.json to recover state (errors, missing contacts)
		var analysisIssues struct {
			Errors          []string `json:"errors"`
			MissingContacts []string `json:"missingContacts"`
		}
		// Wrap in expected envelope structure from file
		envelope := struct {
			Analysis struct {
				Errors          []string `json:"errors"`
				MissingContacts []string `json:"missingContacts"`
			} `json:"analysis"`
		}{}

		if tmp, err := s3c.DownloadToFile(ctx, analysisKey); err == nil {
			if data, rerr := os.ReadFile(tmp); rerr == nil {
				_ = json.Unmarshal(data, &envelope)
				analysisIssues.Errors = envelope.Analysis.Errors
				analysisIssues.MissingContacts = envelope.Analysis.MissingContacts
			}
			_ = os.Remove(tmp)
		}

		// Reconstruct expected artifact keys
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
			base + "-analysis.json",
			base + "-registrarMapping.json",
			base + "-registrar-map.json",
		}
		artifacts := map[string]string{}
		for _, f := range candidates {
			// We assume if analysis.json exists, the CSVs that were generated also exist.
			// Ideally we'd verify each, but for resumption speed we'll assume consistency or checks downstream.
			// We only include them in the map if they actually exist to avoid 404s later.
			objKey := runPrefix + "/" + f
			if ex, _ := s3c.Exists(ctx, objKey); ex {
				artifacts[f] = objKey
			}
		}

		return StreamingAnalysisResult{
			RunPrefix:       runPrefix,
			BaseFilename:    base,
			ArtifactKeys:    artifacts,
			Counts:          map[string]int64{}, // Counts lost on resume (best effort), or could parse summary
			HasIssues:       len(analysisIssues.Errors) > 0 || len(analysisIssues.MissingContacts) > 0,
			AnalysisErrors:  analysisIssues.Errors,
			MissingContacts: analysisIssues.MissingContacts,
		}, nil
	}

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
	// Fetch fresh token for MapRegistrars
	token := GetBearerToken()

	// Create a heartbeat wrapper
	heartbeat := func(details ...interface{}) {
		activity.RecordHeartbeat(ctx, details...)
	}

	// NOTE: We set MapRegistrars to false because we moved it to a separate activity
	if err := streamingSvc.StreamAnalyze(false, token, heartbeat); err != nil {
		return StreamingAnalysisResult{}, fmt.Errorf("stream analyze failed: %w", err)
	}

	// Determine temp base filename from the downloaded file (what the service produced)
	tempBase := strings.TrimSuffix(xmlPath, filepath.Ext(xmlPath))
	// Deterministic base is 'base' (calculated from S3 key at start)

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
	// Local analysis check uses tempBase
	analysisPath := tempBase + "-analysis.json"
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

	// Artifact suffixes to look for
	suffixes := []string{
		"-domains.csv",
		"-domainStatuses.csv",
		"-domainNameservers.csv",
		"-DomainDnssec.csv",
		"-domainTransfers.csv",
		"-domainRgpStatus.csv",
		"-contacts.csv",
		"-contactStatuses.csv",
		"-contactPostalInfo.csv",
		"-uniqueDomainContactIDs.csv",
		"-hosts.csv",
		"-hostAddresses.csv",
		"-hostStatuses.csv",
		"-registrars.csv",
		"-registrarPostalInfo.csv",
		"-registrarMapping.csv",
		"-nndns.csv",
		// Helpful troubleshooting artifacts
		"-analysis.json",
		"-registrarMapping.json",
		"-registrar-map.json",
	}
	artifacts := map[string]string{}
	counts := map[string]int64{}

	for _, suffix := range suffixes {
		localFile := tempBase + suffix
		targetName := base + suffix // Deterministic name for S3

		if _, err := os.Stat(localFile); err == nil {
			// Upload to S3 under runPrefix targetName
			objKey := runPrefix + "/" + targetName
			// Choose content type based on file extension
			ctype := "text/csv"
			switch strings.ToLower(filepath.Ext(targetName)) {
			case ".json":
				ctype = "application/json"
			case ".csv":
				ctype = "text/csv"
			default:
				ctype = "application/octet-stream"
			}
			if upErr := s3c.UploadFile(ctx, objKey, localFile, ctype); upErr != nil {
				return StreamingAnalysisResult{}, fmt.Errorf("upload %s failed: %w", targetName, upErr)
			}
			artifacts[targetName] = objKey
			// best-effort count lines (minus header)
			if strings.HasSuffix(strings.ToLower(targetName), ".csv") {
				if c, cerr := countCSVLines(localFile); cerr == nil && c > 0 {
					// crude mapping for some expected types
					name := strings.ToLower(targetName)
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
			_ = os.Remove(localFile)
		}
	}

	return StreamingAnalysisResult{
		RunPrefix:       runPrefix,
		BaseFilename:    base, // Return deterministic base
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

	// Resumption Check: Check for success manifest instead of just the DB file.
	// This ensures we don't resume from a partial/corrupt upload.
	manifestKey := runPrefix + "/" + base + ".db.manifest.json"
	if exists, _ := s3c.Exists(ctx, manifestKey); exists {
		activity.GetLogger(ctx).Info("Resuming: SQLite manifest found, skipping conversion", "manifestKey", manifestKey)

		var manifest struct {
			DBKey                    string `json:"dbKey"`
			CompletedAt              string `json:"completedAt"`
			RegistrarMappingRowCount int    `json:"registrarMappingRowCount"`
		}

		if tmp, err := s3c.DownloadToFile(ctx, manifestKey); err == nil {
			if data, rerr := os.ReadFile(tmp); rerr == nil {
				if jerr := json.Unmarshal(data, &manifest); jerr == nil {
					_ = os.Remove(tmp)
					return ConvertToSQLiteResult{
						DBKey:                    manifest.DBKey,
						RunPrefix:                runPrefix,
						BaseFilename:             base,
						RegistrarMappingRowCount: manifest.RegistrarMappingRowCount,
					}, nil
				}
			}
			_ = os.Remove(tmp)
		}
		// If manifest download/parse fails, fall through to re-run
		activity.GetLogger(ctx).Warn("Resuming: Manifest found but failed to read, re-running conversion")
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

	// Create heartbeat closure
	heartbeat := func(details ...interface{}) {
		activity.RecordHeartbeat(ctx, details...)
	}

	if err := svc.ConvertToSQLite(dbPath, heartbeat); err != nil {
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

	// Upload Success Manifest
	// This marks the step as fully complete and provides metadata for resumption.
	manifest := map[string]any{
		"dbKey":                    dbKey,
		"completedAt":              time.Now().UTC().Format(time.RFC3339),
		"registrarMappingRowCount": mappingRows,
	}
	if tmp, err := os.CreateTemp("", "escrow-sqlite-manifest-*.json"); err == nil {
		enc := json.NewEncoder(tmp)
		enc.SetIndent("", "  ")
		if err := enc.Encode(manifest); err == nil {
			tmp.Close()
			if err := s3c.UploadFile(ctx, manifestKey, tmp.Name(), "application/json"); err != nil {
				activity.GetLogger(ctx).Warn("Failed to upload SQLite manifest (resumption may not work next time)", "error", err)
			}
			_ = os.Remove(tmp.Name())
		} else {
			tmp.Close()
			_ = os.Remove(tmp.Name())
		}
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

	// Initialize compact counters and runtime events early
	counts := map[string]int64{}
	events := []ReportEvent{}
	tallies := map[string]int64{}

	// counts, events, tallies are initialized above

	// 0) Import contacts in chunks (must exist before domains due to FK on registrant)
	cursorKeyContacts := path.Dir(dbKey) + "/cursor_contacts.txt"
	if err := a.importContactsChunked(ctx, db, counts, clidMap, &events, tallies, s3c, cursorKeyContacts); err != nil {
		return ImportFromSQLiteResult{}, fmt.Errorf("contact import failed: %w", err)
	}

	// 1) Import hosts in chunks
	cursorKeyHosts := path.Dir(dbKey) + "/cursor_hosts.txt"
	if err := a.importHostsChunked(ctx, db, counts, clidMap, &events, tallies, s3c, cursorKeyHosts); err != nil {
		return ImportFromSQLiteResult{}, fmt.Errorf("host import failed: %w", err)
	}

	// 2) Import domains in chunks (without host associations yet)
	cursorKeyDomains := path.Dir(dbKey) + "/cursor_domains.txt"
	if err := a.importDomainsChunked(ctx, db, tld, counts, clidMap, &events, tallies, s3c, cursorKeyDomains); err != nil {
		return ImportFromSQLiteResult{}, fmt.Errorf("domain import failed: %w", err)
	}

	// 3) Link domain nameservers → domain_hosts via DomainService to ensure statuses update
	cursorKeyLinks := path.Dir(dbKey) + "/cursor_links.txt"
	if err := a.linkDomainHosts(ctx, db, counts, &events, tallies, s3c, cursorKeyLinks); err != nil {
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

// ImportContactsFromSQLite imports contacts only using the admin API bulk endpoint.
func (a *EscrowImportActivities) ImportContactsFromSQLite(ctx context.Context, args ImportFromSQLiteArgs) (ImportFromSQLiteResult, error) {
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
	dbPath, err := s3c.DownloadToFile(ctx, dbKey)
	if err != nil {
		return ImportFromSQLiteResult{}, fmt.Errorf("download db failed: %w", err)
	}
	defer os.Remove(dbPath)
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

	counts := map[string]int64{}
	events := []ReportEvent{}
	tallies := map[string]int64{}

	cursorKey := path.Dir(dbKey) + "/cursor_contacts.txt"
	if err := a.importContactsChunked(ctx, db, counts, clidMap, &events, tallies, s3c, cursorKey); err != nil {
		return ImportFromSQLiteResult{}, fmt.Errorf("contact import failed: %w", err)
	}

	// Persist events JSON
	runPrefix := path.Dir(dbKey)
	if len(events) > 0 {
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

// ImportContactsDirect using DirectDBImporter
func (a *EscrowImportActivities) ImportContactsDirect(ctx context.Context, args ImportFromSQLiteArgs) (ImportFromSQLiteResult, error) {
	dbKey := strings.TrimSpace(args.DBKey)
	if dbKey == "" {
		return ImportFromSQLiteResult{}, fmt.Errorf("dbKey required")
	}

	importer, err := services.NewDirectDBImporter()
	if err != nil {
		return ImportFromSQLiteResult{}, err
	}
	defer importer.PG.Close()

	s3c, err := storage.NewS3ClientFromEnv()
	if err != nil {
		return ImportFromSQLiteResult{}, err
	}

	dbPath, err := s3c.DownloadToFile(ctx, dbKey)
	if err != nil {
		return ImportFromSQLiteResult{}, err
	}
	defer os.Remove(dbPath)

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return ImportFromSQLiteResult{}, err
	}
	defer db.Close()

	clidMap := a.loadRegistrarMapping(db)

	counts := map[string]int64{}
	tallies := map[string]int64{}
	// Direct import resume
	cursorKey := path.Dir(dbKey) + "/cursor_contacts_direct.txt"
	var lastKey string
	if content, err := s3c.DownloadToString(ctx, cursorKey); err == nil {
		lastKey = strings.TrimSpace(content)
	}

	heartbeat := func(k string) {
		activity.RecordHeartbeat(ctx, k)
		_ = s3c.UploadString(ctx, cursorKey, k)
	}

	total, skipped, err := importer.ImportContacts(ctx, db, clidMap, lastKey, heartbeat)
	if err != nil {
		return ImportFromSQLiteResult{}, err
	}

	counts["contacts_imported"] = total
	tallies["contacts_skipped_unmapped"] = skipped

	return ImportFromSQLiteResult{DBKey: dbKey, Counts: counts, Tallies: tallies}, nil
}

// Helper for mapping reuse
func (a *EscrowImportActivities) loadRegistrarMapping(db *sql.DB) map[string]string {
	clidMap := map[string]string{}
	if rows, qerr := db.Query(`SELECT escrow_id, registrar_clid FROM registrar_mapping`); qerr == nil {
		defer rows.Close()
		for rows.Next() {
			var escrowID, mapped sql.NullString
			if err := rows.Scan(&escrowID, &mapped); err == nil {
				if escrowID.Valid && mapped.Valid && strings.TrimSpace(mapped.String) != "" {
					clidMap[escrowID.String] = mapped.String
				}
			}
		}
	}
	return clidMap
}

// ImportHostsFromSQLite imports hosts only using the admin API bulk endpoint.
func (a *EscrowImportActivities) ImportHostsFromSQLite(ctx context.Context, args ImportFromSQLiteArgs) (ImportFromSQLiteResult, error) {
	tld := strings.TrimSpace(args.TLD)
	if tld == "" { // keep arg signature uniform
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
	dbPath, err := s3c.DownloadToFile(ctx, dbKey)
	if err != nil {
		return ImportFromSQLiteResult{}, fmt.Errorf("download db failed: %w", err)
	}
	defer os.Remove(dbPath)
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return ImportFromSQLiteResult{}, fmt.Errorf("open sqlite failed: %w", err)
	}
	defer db.Close()

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

	counts := map[string]int64{}
	events := []ReportEvent{}
	tallies := map[string]int64{}
	// 2) Import hosts in chunks
	cursorKey := path.Dir(dbKey) + "/cursor_hosts.txt"
	if err := a.importHostsChunked(ctx, db, counts, clidMap, &events, tallies, s3c, cursorKey); err != nil {
		return ImportFromSQLiteResult{}, fmt.Errorf("host import failed: %w", err)
	}
	runPrefix := path.Dir(dbKey)
	if len(events) > 0 {
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

// ImportHostsDirect imports hosts using DirectDBImporter
func (a *EscrowImportActivities) ImportHostsDirect(ctx context.Context, args ImportFromSQLiteArgs) (ImportFromSQLiteResult, error) {
	tld := strings.TrimSpace(args.TLD)
	dbKey := strings.TrimSpace(args.DBKey)
	if tld == "" || dbKey == "" {
		return ImportFromSQLiteResult{}, fmt.Errorf("tld and dbKey required")
	}

	importer, err := services.NewDirectDBImporter()
	if err != nil {
		return ImportFromSQLiteResult{}, err
	}
	defer importer.PG.Close()

	s3c, err := storage.NewS3ClientFromEnv()
	if err != nil {
		return ImportFromSQLiteResult{}, err
	}
	dbPath, err := s3c.DownloadToFile(ctx, dbKey)
	if err != nil {
		return ImportFromSQLiteResult{}, err
	}
	defer os.Remove(dbPath)

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return ImportFromSQLiteResult{}, err
	}
	defer db.Close()

	clidMap := a.loadRegistrarMapping(db) // Helper reuse

	counts := map[string]int64{}
	tallies := map[string]int64{}

	cursorKey := path.Dir(dbKey) + "/cursor_hosts_direct.txt"
	var lastKey string
	if content, err := s3c.DownloadToString(ctx, cursorKey); err == nil {
		lastKey = strings.TrimSpace(content)
	}

	heartbeat := func(k string) {
		activity.RecordHeartbeat(ctx, k)
		_ = s3c.UploadString(ctx, cursorKey, k)
	}

	total, skipped, err := importer.ImportHosts(ctx, db, clidMap, lastKey, heartbeat)
	if err != nil {
		activity.GetLogger(ctx).Error("Direct host import failed", "error", err)
		return ImportFromSQLiteResult{}, err
	}

	counts["hosts_imported"] = total
	tallies["hosts_skipped_unmapped"] = skipped

	return ImportFromSQLiteResult{DBKey: dbKey, Counts: counts, Tallies: tallies}, nil
}

// ImportDomainsDirect using DirectDBImporter
func (a *EscrowImportActivities) ImportDomainsDirect(ctx context.Context, args ImportFromSQLiteArgs) (ImportFromSQLiteResult, error) {
	tld := strings.TrimSpace(args.TLD)
	dbKey := strings.TrimSpace(args.DBKey)
	if tld == "" || dbKey == "" {
		return ImportFromSQLiteResult{}, fmt.Errorf("tld and dbKey required")
	}

	importer, err := services.NewDirectDBImporter()
	if err != nil {
		return ImportFromSQLiteResult{}, err
	}
	defer importer.PG.Close()

	s3c, err := storage.NewS3ClientFromEnv()
	if err != nil {
		return ImportFromSQLiteResult{}, err
	}
	dbPath, err := s3c.DownloadToFile(ctx, dbKey)
	if err != nil {
		return ImportFromSQLiteResult{}, err
	}
	defer os.Remove(dbPath)

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return ImportFromSQLiteResult{}, err
	}
	defer db.Close()

	clidMap := a.loadRegistrarMapping(db)

	counts := map[string]int64{}
	tallies := map[string]int64{}

	cursorKey := path.Dir(dbKey) + "/cursor_domains_direct.txt"
	var lastKey string
	if content, err := s3c.DownloadToString(ctx, cursorKey); err == nil {
		lastKey = strings.TrimSpace(content)
	}

	heartbeat := func(k string) {
		activity.RecordHeartbeat(ctx, k)
		_ = s3c.UploadString(ctx, cursorKey, k)
	}

	total, skipped, err := importer.ImportDomains(ctx, db, tld, clidMap, lastKey, heartbeat)
	if err != nil {
		return ImportFromSQLiteResult{}, err
	}

	counts["domains_imported"] = total
	tallies["domains_skipped_unmapped"] = skipped

	return ImportFromSQLiteResult{DBKey: dbKey, Counts: counts, Tallies: tallies}, nil
}

// ImportDomainsFromSQLite imports domains only using the admin API bulk endpoint.
func (a *EscrowImportActivities) ImportDomainsFromSQLite(ctx context.Context, args ImportFromSQLiteArgs) (ImportFromSQLiteResult, error) {
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
	dbPath, err := s3c.DownloadToFile(ctx, dbKey)
	if err != nil {
		return ImportFromSQLiteResult{}, fmt.Errorf("download db failed: %w", err)
	}
	defer os.Remove(dbPath)
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return ImportFromSQLiteResult{}, fmt.Errorf("open sqlite failed: %w", err)
	}
	defer db.Close()

	clidMap := a.loadRegistrarMapping(db)

	counts := map[string]int64{}
	events := []ReportEvent{}
	tallies := map[string]int64{}
	// 2) Import domains in chunks
	cursorKeyDomains := path.Dir(dbKey) + "/cursor_domains.txt"
	if err := a.importDomainsChunked(ctx, db, tld, counts, clidMap, &events, tallies, s3c, cursorKeyDomains); err != nil {
		return ImportFromSQLiteResult{}, fmt.Errorf("domain import failed: %w", err)
	}
	runPrefix := path.Dir(dbKey)
	if len(events) > 0 {
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

// LinkDomainHostsDirect using DirectDBImporter
func (a *EscrowImportActivities) LinkDomainHostsDirect(ctx context.Context, args ImportFromSQLiteArgs) (ImportFromSQLiteResult, error) {
	tld := strings.TrimSpace(args.TLD)
	dbKey := strings.TrimSpace(args.DBKey)
	if tld == "" || dbKey == "" {
		return ImportFromSQLiteResult{}, fmt.Errorf("tld and dbKey required")
	}

	importer, err := services.NewDirectDBImporter()
	if err != nil {
		return ImportFromSQLiteResult{}, err
	}
	defer importer.PG.Close()

	s3c, err := storage.NewS3ClientFromEnv()
	if err != nil {
		return ImportFromSQLiteResult{}, err
	}
	dbPath, err := s3c.DownloadToFile(ctx, dbKey)
	if err != nil {
		return ImportFromSQLiteResult{}, err
	}
	defer os.Remove(dbPath)

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return ImportFromSQLiteResult{}, err
	}
	defer db.Close()

	counts := map[string]int64{}
	tallies := map[string]int64{}

	cursorKey := path.Dir(dbKey) + "/cursor_links_direct.txt"
	var lastKey string
	if content, err := s3c.DownloadToString(ctx, cursorKey); err == nil {
		lastKey = strings.TrimSpace(content)
	}

	heartbeat := func(k string) {
		activity.RecordHeartbeat(ctx, k)
		_ = s3c.UploadString(ctx, cursorKey, k)
	}

	total, err := importer.LinkDomainHosts(ctx, db, lastKey, heartbeat)
	if err != nil {
		return ImportFromSQLiteResult{}, err
	}

	counts["domain_hosts_linked"] = total

	return ImportFromSQLiteResult{DBKey: dbKey, Counts: counts, Tallies: tallies}, nil
}

// LinkDomainHostsFromSQLite links domain NS from SQLite using admin API.
func (a *EscrowImportActivities) LinkDomainHostsFromSQLite(ctx context.Context, args ImportFromSQLiteArgs) (ImportFromSQLiteResult, error) {
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
	dbPath, err := s3c.DownloadToFile(ctx, dbKey)
	if err != nil {
		return ImportFromSQLiteResult{}, fmt.Errorf("download db failed: %w", err)
	}
	defer os.Remove(dbPath)
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return ImportFromSQLiteResult{}, fmt.Errorf("open sqlite failed: %w", err)
	}
	defer db.Close()

	counts := map[string]int64{}
	events := []ReportEvent{}
	tallies := map[string]int64{}

	cursorKey := path.Dir(dbKey) + "/cursor_links.txt"
	if err := a.linkDomainHosts(ctx, db, counts, &events, tallies, s3c, cursorKey); err != nil {
		return ImportFromSQLiteResult{}, fmt.Errorf("domain-host linking failed: %w", err)
	}
	runPrefix := path.Dir(dbKey)
	if len(events) > 0 {
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
	// Domains for this TLD with any RGP field set (non-zero timestamps)
	row = cRow{}
	if r := gdb.Raw(`
		SELECT COUNT(1) AS cnt
		FROM domains
		WHERE tld_name = ?
		AND (
			add_period_end > '0001-01-01' OR
			renew_period_end > '0001-01-01' OR
			auto_renew_period_end > '0001-01-01' OR
			redemption_period_end > '0001-01-01' OR
			purge_date > '0001-01-01'
		)
	`, tld).Scan(&row); r.Error == nil {
		pgCounts["postgres_domains_with_rgp_for_tld"] = row.Cnt
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

// importHostsChunked reads hosts and addresses from SQLite and bulk-creates them via Admin API.
func (a *EscrowImportActivities) importHostsChunked(ctx context.Context, sqldb *sql.DB, counts map[string]int64, clidMap map[string]string, events *[]ReportEvent, tallies map[string]int64, s3c *storage.S3Client, cursorKey string) error {
	const pageSize = 2000
	created := int64(0)
	var lastKey string

	// 1. Try Heartbeat (Intra-run resume)
	if activity.HasHeartbeatDetails(ctx) {
		var savedKey string
		if err := activity.GetHeartbeatDetails(ctx, &savedKey); err == nil {
			lastKey = savedKey
			activity.GetLogger(ctx).Info("Resuming host import from heartbeat", "cursor", lastKey)
		}
	}

	// 2. Try S3 (Inter-run resume)
	if lastKey == "" && s3c != nil && cursorKey != "" {
		if content, err := s3c.DownloadToString(ctx, cursorKey); err == nil {
			k := strings.TrimSpace(content)
			if k != "" {
				lastKey = k
				activity.GetLogger(ctx).Info("Resuming host import from S3", "cursor", lastKey)
			}
		}
	}

	for {
		rows, err := sqldb.Query(`
			SELECT name, clid, crrr, uprr FROM hosts WHERE name > ? ORDER BY name LIMIT ?
		`, lastKey, pageSize)
		if err != nil {
			return err
		}

		cmds := make([]*commands.CreateHostCommand, 0, pageSize)
		for rows.Next() {
			var name, clid, crrr, uprr sql.NullString
			if err := rows.Scan(&name, &clid, &crrr, &uprr); err != nil {
				rows.Close()
				return err
			}
			// map clids using registrar mapping if available
			// Returns (mappedValue, valid)
			mapClid := func(raw string) (string, bool) {
				r := strings.TrimSpace(raw)
				if r == "" {
					return raw, true
				}
				if v, ok := clidMap[r]; ok && strings.TrimSpace(v) != "" {
					return v, true
				}
				// Not found in map -> Invalid
				return "", false
			}

			mappedClID, ok := mapClid(clid.String)
			if !ok {
				// Record skipped event
				*events = append(*events, ReportEvent{
					Level:     "warning",
					Activity:  "ImportFromSQLite.hosts",
					Code:      "skipped_unmapped_registrar",
					Message:   fmt.Sprintf("skipped host %s due to unmapped registrar ID: %s", name.String, clid.String),
					Object:    clid.String,
					Context:   map[string]string{"name": name.String},
					Timestamp: nowUTC(),
				})
				tallies["hosts_skipped_unmapped"]++
				continue
			}

			cmd := &commands.CreateHostCommand{
				Name: name.String,
				ClID: entities.ClIDType(mappedClID),
			}
			if crrr.Valid {
				if m, ok := mapClid(crrr.String); ok {
					cmd.CrRr = entities.ClIDType(m)
				}
			}
			if uprr.Valid {
				if m, ok := mapClid(uprr.String); ok {
					cmd.UpRr = entities.ClIDType(m)
				}
			}
			cmds = append(cmds, cmd)
		}
		rows.Close()

		if len(cmds) == 0 {
			break
		}

		// Batch fetch associations
		firstName := cmds[0].Name
		lastName := cmds[len(cmds)-1].Name

		// 1. Host Addresses
		addrMap := make(map[string][]string)
		{
			addrRows, err := sqldb.Query(`SELECT host_name, ip_address FROM host_addresses WHERE host_name >= ? AND host_name <= ?`, firstName, lastName)
			if err == nil {
				for addrRows.Next() {
					var hn, ip sql.NullString
					if err := addrRows.Scan(&hn, &ip); err == nil && hn.Valid && ip.Valid && ip.String != "" {
						addrMap[hn.String] = append(addrMap[hn.String], ip.String)
					}
				}
				addrRows.Close()
			} else {
				// if query errors (e.g. no table), just skip addresses
			}
		}

		// 2. Host Statuses
		statusMap := make(map[string]entities.HostStatus)
		{
			stRows, err := sqldb.Query(`SELECT host_name, status FROM host_statuses WHERE host_name >= ? AND host_name <= ?`, firstName, lastName)
			if err == nil {
				for stRows.Next() {
					var hn, stval sql.NullString
					if err := stRows.Scan(&hn, &stval); err == nil && hn.Valid && stval.Valid {
						hs := statusMap[hn.String]
						switch strings.ToLower(strings.TrimSpace(stval.String)) {
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
						statusMap[hn.String] = hs
					}
				}
				stRows.Close()
			}
		}

		// Fill commands
		for _, cmd := range cmds {
			if addrs, ok := addrMap[cmd.Name]; ok {
				cmd.Addresses = addrs
			}
			hs, ok := statusMap[cmd.Name]
			if ok {
				// Ensure OK is set when no prohibitions/pending are present (per RFC and our validation rules)
				if !hs.OK && !hs.PendingCreate && !hs.PendingDelete && !hs.PendingTransfer && !hs.PendingUpdate && !hs.ClientDeleteProhibited && !hs.ClientUpdateProhibited && !hs.ServerDeleteProhibited && !hs.ServerUpdateProhibited {
					hs.OK = true
				}
				if !hs.IsNil() {
					cmd.Status = hs
				}
			}
		}

		// Try bulk first via API, then per-item fallback for idempotency
		createHostCmds := make([]commands.CreateHostCommand, 0, len(cmds))
		for _, c := range cmds {
			createHostCmds = append(createHostCmds, *c)
		}
		if err := BulkCreateHosts("escrow-import", createHostCmds); err != nil {
			// If bulk fails (e.g., duplicates), try per-item create for idempotency
			for i, c := range cmds {
				if i%100 == 0 {
					activity.RecordHeartbeat(ctx, "fallback-hosts", c.ClID)
				}
				if ierr := CreateHost("escrow-import", *c); ierr != nil {
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

		// Update cursor
		lastKey = cmds[len(cmds)-1].Name
		activity.RecordHeartbeat(ctx, lastKey)

		// Persist to S3 (best effort)
		if s3c != nil && cursorKey != "" {
			_ = s3c.UploadString(ctx, cursorKey, lastKey)
		}

		if len(cmds) < pageSize {
			break
		}
	}
	return nil
}

// importDomainsChunked reads domains from SQLite and bulk-creates them via Admin API.
func (a *EscrowImportActivities) importDomainsChunked(ctx context.Context, sqldb *sql.DB, tld string, counts map[string]int64, clidMap map[string]string, events *[]ReportEvent, tallies map[string]int64, s3c *storage.S3Client, cursorKey string) error {
	const pageSize = 2000
	created := int64(0)
	var lastKey string

	// 1. Try Heartbeat (Intra-run resume)
	if activity.HasHeartbeatDetails(ctx) {
		var savedKey string
		if err := activity.GetHeartbeatDetails(ctx, &savedKey); err == nil {
			lastKey = savedKey
			activity.GetLogger(ctx).Info("Resuming domain import from heartbeat", "cursor", lastKey)
		}
	}

	// 2. Try S3 (Inter-run resume)
	if lastKey == "" && s3c != nil && cursorKey != "" {
		if content, err := s3c.DownloadToString(ctx, cursorKey); err == nil {
			k := strings.TrimSpace(content)
			if k != "" {
				lastKey = k
				activity.GetLogger(ctx).Info("Resuming domain import from S3", "cursor", lastKey)
			}
		}
	}

	for {
		rows, err := sqldb.Query(`
			SELECT name, registrant, clid, crrr, crdate, exdate, uprr, uname, originalname
			FROM domains WHERE name > ? ORDER BY name LIMIT ?
		`, lastKey, pageSize)
		if err != nil {
			return err
		}

		cmds := make([]*commands.CreateDomainCommand, 0, pageSize)
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
			mapClid := func(raw string) (string, bool) {
				r := strings.TrimSpace(raw)
				if r == "" {
					return raw, true
				}
				if v, ok := clidMap[r]; ok && strings.TrimSpace(v) != "" {
					return v, true
				}
				return "", false
			}
			mappedClID, ok := mapClid(cmd.ClID)
			if !ok {
				*events = append(*events, ReportEvent{
					Level:     "warning",
					Activity:  "ImportFromSQLite.domains",
					Code:      "skipped_unmapped_registrar",
					Message:   fmt.Sprintf("skipped domain %s due to unmapped registrar ID: %s", cmd.Name, cmd.ClID),
					Object:    cmd.ClID,
					Context:   map[string]string{"name": cmd.Name},
					Timestamp: nowUTC(),
				})
				tallies["domains_skipped_unmapped"]++
				continue
			}
			cmd.ClID = mappedClID

			if registrant.Valid {
				cmd.RegistrantID = registrant.String
			}
			if crrr.Valid {
				if m, ok := mapClid(crrr.String); ok {
					cmd.CrRr = m
				}
			}
			if uprr.Valid {
				if m, ok := mapClid(uprr.String); ok {
					cmd.UpRr = m
				}
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
			cmds = append(cmds, cmd)
		}
		rows.Close()

		if len(cmds) == 0 {
			break
		}

		// Batch fetch associations

		// Since case differences between `domains` and `domain_statuses` can cause ASCII range queries (>= and <=) 
		// to miss rows, we build a query using an IN clause.
		// We chunk the IN clause to avoid SQLite parameter limits.
		statusMap := make(map[string]entities.DomainStatus)
		rgpMap := make(map[string][]string)
		
		var names []interface{}
		var placeholders []string
		for _, cmd := range cmds {
			names = append(names, strings.ToLower(cmd.Name))
			placeholders = append(placeholders, "?")
		}

		// Process in chunks of 500 to be safe with SQLite limits
		chunkSize := 500
		for i := 0; i < len(names); i += chunkSize {
			end := i + chunkSize
			if end > len(names) {
				end = len(names)
			}
			chunkNames := names[i:end]
			chunkPlaceholders := strings.Join(placeholders[i:end], ",")

			// 1. Domain Statuses
			stQuery := fmt.Sprintf(`SELECT domain_name, status FROM domain_statuses WHERE LOWER(domain_name) IN (%s)`, chunkPlaceholders)
			stRows, sErr := sqldb.Query(stQuery, chunkNames...)
			if sErr == nil {
				for stRows.Next() {
					var dn, stval sql.NullString
					if err := stRows.Scan(&dn, &stval); err == nil && dn.Valid && stval.Valid {
						normalizedDN := strings.ToLower(strings.TrimSpace(dn.String))
						ds := statusMap[normalizedDN]
						switch strings.ToLower(strings.TrimSpace(stval.String)) {
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
							ds.PendingRestore = true
						case "pendingdelete":
							ds.PendingDelete = true
						}
						statusMap[normalizedDN] = ds
					}
				}
				stRows.Close()
			}

			// 2. RGP Statuses
			rgpQuery := fmt.Sprintf(`SELECT domain_name, rgp_status FROM domain_rgp_statuses WHERE LOWER(domain_name) IN (%s)`, chunkPlaceholders)
			rgpRows, rErr := sqldb.Query(rgpQuery, chunkNames...)
			if rErr == nil {
				for rgpRows.Next() {
					var dn, st sql.NullString
					if err := rgpRows.Scan(&dn, &st); err == nil && dn.Valid && st.Valid {
						normalizedDN := strings.ToLower(strings.TrimSpace(dn.String))
						rgpMap[normalizedDN] = append(rgpMap[normalizedDN], st.String)
					}
				}
				rgpRows.Close()
			}
		}

		log.Printf("DEBUG importDomainsChunked: fetched %d statuses and %d RGP statuses from SQLite chunk", len(statusMap), len(rgpMap))

		// Fill commands
		now := time.Now().UTC()
		for _, cmd := range cmds {
			normalizedCmdName := strings.ToLower(strings.TrimSpace(cmd.Name))
			// Status
			if ds, ok := statusMap[normalizedCmdName]; ok {
				// Ensure OK is set when no prohibitions/pending are present (allowed with Inactive)
				if !ds.OK && !ds.HasProhibitions() && !ds.HasPendings() {
					ds.OK = true
				}
				if !ds.IsNil() {
					cmd.Status = ds
				}
			}
			// RGP
			if rgps, ok := rgpMap[normalizedCmdName]; ok {
				for _, st := range rgps {
					switch strings.ToLower(strings.TrimSpace(st)) {
					case "addperiod":
						// Use CreatedAt + 5 days when available, otherwise fallback to now + 5 days
						base := cmd.CreatedAt
						if base.IsZero() {
							base = now
						}
						cmd.RGPStatus.AddPeriodEnd = base.AddDate(0, 0, 5)
					case "renewperiod":
						// Keep default of now + 5 days
						cmd.RGPStatus.RenewPeriodEnd = now.AddDate(0, 0, 5)
					case "autorenewperiod":
						// Use the last past expiry anniversary + 45 days
						base := cmd.ExpiryDate
						if base.IsZero() {
							base = now
						}
						// If base is not in the past, keep subtracting one year until it is
						// guard with a sane max to avoid accidental infinite loops
						safety := 0
						for !base.Before(now) && safety < 100 {
							base = base.AddDate(-1, 0, 0)
							safety++
						}
						cmd.RGPStatus.AutoRenewPeriodEnd = base.AddDate(0, 0, 45)
					case "redemptiongraceperiod":
						// Now + 30 days
						cmd.RGPStatus.RedemptionPeriodEnd = now.AddDate(0, 0, 30)
					case "pendingdeletegraceperiod":
						// Pending delete GP -> set purge date to now + 5 days
						cmd.RGPStatus.PurgeDate = now.AddDate(0, 0, 5)
					}
				}
			}
		}

		// Bulk create via API, then fallback per-item
		createDomCmds := make([]commands.CreateDomainCommand, 0, len(cmds))
		for _, c := range cmds {
			createDomCmds = append(createDomCmds, *c)
		}
		if err := BulkCreateDomains("escrow-import", createDomCmds); err != nil {
			// On error (duplicates, etc.), try per-item to be idempotent
			for i, c := range cmds {
				if i%100 == 0 {
					activity.RecordHeartbeat(ctx, "fallback-domains", c.Name)
				}
				if ierr := CreateDomain("escrow-import", *c); ierr != nil {
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

		// Update cursor
		lastKey = cmds[len(cmds)-1].Name
		activity.RecordHeartbeat(ctx, lastKey)

		// Persist to S3 (best effort)
		if s3c != nil && cursorKey != "" {
			_ = s3c.UploadString(ctx, cursorKey, lastKey)
		}

		if len(cmds) < pageSize {
			break
		}
	}
	return nil
}

// linkDomainHosts links domains to hosts based on domain_nameservers via Admin API.
func (a *EscrowImportActivities) linkDomainHosts(ctx context.Context, sqldb *sql.DB, counts map[string]int64, events *[]ReportEvent, tallies map[string]int64, s3c *storage.S3Client, cursorKey string) error {
	const pageSize = 2000
	linked := int64(0)
	var lastDomain, lastNS string

	// Parse helper for composite cursor "domain|ns"
	parseCursor := func(s string) {
		parts := strings.SplitN(s, "|", 2)
		if len(parts) == 2 {
			lastDomain, lastNS = parts[0], parts[1]
		}
	}

	// 1. Try Heartbeat
	if activity.HasHeartbeatDetails(ctx) {
		var savedKey string
		if err := activity.GetHeartbeatDetails(ctx, &savedKey); err == nil {
			parseCursor(savedKey)
			activity.GetLogger(ctx).Info("Resuming linkDomainHosts from heartbeat", "domain", lastDomain, "ns", lastNS)
		}
	}

	// 2. Try S3
	if lastDomain == "" && s3c != nil && cursorKey != "" {
		if content, err := s3c.DownloadToString(ctx, cursorKey); err == nil {
			k := strings.TrimSpace(content)
			if k != "" {
				parseCursor(k)
				activity.GetLogger(ctx).Info("Resuming linkDomainHosts from S3", "domain", lastDomain, "ns", lastNS)
			}
		}
	}

	for {
		// Composite keyset pagination:
		// WHERE (d > lastD) OR (d = lastD AND ns > lastNS)
		rows, err := sqldb.Query(`
			SELECT domain_name, nameserver 
			FROM domain_nameservers 
			WHERE (domain_name > ?) OR (domain_name = ? AND nameserver > ?)
			ORDER BY domain_name, nameserver 
			LIMIT ?
		`, lastDomain, lastDomain, lastNS, pageSize)
		if err != nil {
			return err
		}

		type pair struct {
			d, n string
		}
		pairs := make([]pair, 0, pageSize)

		for rows.Next() {
			var d, n sql.NullString
			if err := rows.Scan(&d, &n); err != nil {
				rows.Close()
				return err
			}
			if d.Valid && n.Valid {
				pairs = append(pairs, pair{d.String, n.String})
			}
		}
		rows.Close()

		if len(pairs) == 0 {
			break
		}

		for i, p := range pairs {
			if i%100 == 0 {
				activity.RecordHeartbeat(ctx, "link-domains", p.d)
			}
			// AddHostToDomainByHostname is idempotent (safe to retry)
			if err := AddHostToDomainByHostname("escrow-import", p.d, p.n); err != nil {
				*events = append(*events, ReportEvent{
					Level:     "error",
					Activity:  "ImportFromSQLite.linkDomainHosts",
					Code:      "link_failed",
					Message:   err.Error(),
					Object:    p.d,
					Context:   map[string]string{"host": p.n},
					Timestamp: nowUTC(),
				})
				tallies["links_failed"]++
			} else {
				linked++
			}
		}
		counts["domain_hosts_linked"] = linked

		// Update composite cursor
		lastP := pairs[len(pairs)-1]
		lastDomain = lastP.d
		lastNS = lastP.n

		cursorVal := fmt.Sprintf("%s|%s", lastDomain, lastNS)
		activity.RecordHeartbeat(ctx, cursorVal)

		if s3c != nil && cursorKey != "" {
			_ = s3c.UploadString(ctx, cursorKey, cursorVal)
		}

		if len(pairs) < pageSize {
			break
		}
	}
	return nil
}

// importContactsChunked reads contacts from SQLite and creates them via Admin API before domains.
func (a *EscrowImportActivities) importContactsChunked(ctx context.Context, sqldb *sql.DB, counts map[string]int64, clidMap map[string]string, events *[]ReportEvent, tallies map[string]int64, s3c *storage.S3Client, cursorKey string) error {
	const pageSize = 2000
	created := int64(0)
	var lastKey string

	// 1. Try Heartbeat (Intra-run resume)
	if activity.HasHeartbeatDetails(ctx) {
		var savedKey string
		if err := activity.GetHeartbeatDetails(ctx, &savedKey); err == nil {
			lastKey = savedKey
			activity.GetLogger(ctx).Info("Resuming contact import from heartbeat", "cursor", lastKey)
		}
	}

	// 2. Try S3 (Inter-run resume)
	if lastKey == "" && s3c != nil && cursorKey != "" {
		if content, err := s3c.DownloadToString(ctx, cursorKey); err == nil {
			k := strings.TrimSpace(content)
			if k != "" {
				lastKey = k
				activity.GetLogger(ctx).Info("Resuming contact import from S3", "cursor", lastKey)
			}
		}
	}

	for {
		rows, err := sqldb.Query(`
			SELECT id, roid, voice, fax, email, clid, crrr, crdate, uprr, "update"
			FROM contacts WHERE id > ? ORDER BY id LIMIT ?
		`, lastKey, pageSize)
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
			mapClid := func(raw string) (string, bool) {
				r := strings.TrimSpace(raw)
				if r == "" {
					return raw, true
				}
				if v, ok := clidMap[r]; ok && strings.TrimSpace(v) != "" {
					return v, true
				}
				return "", false
			}

			mappedClID, ok := mapClid(clid.String)
			if !ok {
				*events = append(*events, ReportEvent{
					Level:     "warning",
					Activity:  "ImportFromSQLite.contacts",
					Code:      "skipped_unmapped_registrar",
					Message:   fmt.Sprintf("skipped contact %s due to unmapped registrar ID: %s", id.String, clid.String),
					Object:    clid.String,
					Context:   map[string]string{"id": id.String},
					Timestamp: nowUTC(),
				})
				tallies["contacts_skipped_unmapped"]++
				continue
			}

			cmd := &commands.CreateContactCommand{
				ID:       id.String,
				RoID:     roid.String,
				Email:    email.String,
				AuthInfo: "escr0W1mP*rt",
				ClID:     mappedClID,
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
				if m, ok := mapClid(crrr.String); ok {
					cmd.CrRr = m
				}
			}
			if uprr.Valid {
				if m, ok := mapClid(uprr.String); ok {
					cmd.UpRr = m
				}
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

			cmds = append(cmds, cmd)
		}
		rows.Close()

		if len(cmds) == 0 {
			break
		}

		// Batch fetch associations
		firstID := cmds[0].ID
		lastID := cmds[len(cmds)-1].ID

		// 1. Statuses
		statusMap := make(map[string]entities.ContactStatus)
		{
			srows, serr := sqldb.Query(`SELECT contact_id, status FROM contact_statuses WHERE contact_id >= ? AND contact_id <= ?`, firstID, lastID)
			if serr == nil {
				for srows.Next() {
					var cid, stval sql.NullString
					if err := srows.Scan(&cid, &stval); err == nil && cid.Valid && stval.Valid {
						st := statusMap[cid.String]
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
						statusMap[cid.String] = st
					}
				}
				srows.Close()
			}
		}

		// 2. Postal Info
		postalMap := make(map[string]*[2]*entities.ContactPostalInfo)
		{
			piRows, piErr := sqldb.Query(`
				SELECT contact_id, type, name, org, street1, street2, street3, city, state_province, postal_code, country_code
				FROM contact_postal_info WHERE contact_id >= ? AND contact_id <= ?
			`, firstID, lastID)
			if piErr == nil {
				for piRows.Next() {
					var cid, t, name, org, street1, street2, street3, city, sp, pc, cc sql.NullString
					if err := piRows.Scan(&cid, &t, &name, &org, &street1, &street2, &street3, &city, &sp, &pc, &cc); err != nil || !cid.Valid {
						continue
					}
					tval := strings.ToLower(strings.TrimSpace(t.String))
					if strings.TrimSpace(city.String) == "" || strings.TrimSpace(cc.String) == "" {
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
					nameStr := strings.TrimSpace(name.String)
					if nameStr == "" {
						continue
					}
					cpi, perr := entities.NewContactPostalInfo(tval, nameStr, addr)
					if perr != nil {
						continue
					}
					if org.Valid {
						cpi.Org = entities.OptPostalLineType(org.String)
					}

					arr, exists := postalMap[cid.String]
					if !exists {
						arr = &[2]*entities.ContactPostalInfo{}
						postalMap[cid.String] = arr
					}
					if tval == string(entities.PostalInfoEnumTypeINT) {
						arr[0] = cpi
					} else if tval == string(entities.PostalInfoEnumTypeLOC) {
						arr[1] = cpi
					}
				}
				piRows.Close()
			}
		}

		// Assign associations to commands
		for _, c := range cmds {
			if st, ok := statusMap[c.ID]; ok {
				c.Status = st
			}
			if pi, ok := postalMap[c.ID]; ok {
				c.PostalInfo = *pi
			}
		}

		// Bulk via API, fallback per-item
		createContactCmds := make([]commands.CreateContactCommand, 0, len(cmds))
		for _, c := range cmds {
			createContactCmds = append(createContactCmds, *c)
		}
		if err := BulkCreateContacts("escrow-import", createContactCmds); err != nil {
			// try per-item for idempotency
			for i, c := range cmds {
				if i%100 == 0 {
					activity.RecordHeartbeat(ctx, "fallback-contacts", c.ID)
				}
				if ierr := CreateContact("escrow-import", *c); ierr != nil {
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

		// Update cursor
		lastKey = cmds[len(cmds)-1].ID
		activity.RecordHeartbeat(ctx, lastKey)

		// Persist to S3 (best effort)
		if s3c != nil && cursorKey != "" {
			_ = s3c.UploadString(ctx, cursorKey, lastKey)
		}

		if len(cmds) < pageSize {
			break
		}
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

// --- Refactored 5-Stage Pipeline Structs ---

type ParseAndAssetizeArgs struct {
	TLD       string
	ObjectKey string
	RunPrefix string
}

type ParseAndAssetizeResult struct {
	RunPrefix      string
	AssetKeys      map[string]string // filename -> s3 key
	HasIssues      bool
	AnalysisErrors []string
}

type CollateAssetsArgs struct {
	TLD          string
	RunPrefix    string
	AssetKeys    map[string]string
	BaseFilename string
}

type CollateAssetsResult struct {
	DBKey string
}

type RegistrarMapArgs struct {
	TLD       string
	DBKey     string
	RunPrefix string
	Overrides map[string]string
}

type RegistrarMapResult struct {
	DBKey     string // Updated db key
	HasIssues bool
}

type StageImportArgs struct {
	TLD   string
	DBKey string
}

type StageImportResult struct {
	StagedDBKey string
}

type ExecuteImportArgs struct {
	TLD         string
	StagedDBKey string
}

type ExecuteImportResult struct {
	BaseImportCounts map[string]int64
	Success          bool
}

// --- Refactored Activity Stubs ---

func (a *EscrowImportActivities) ParseAndAssetize(ctx context.Context, args ParseAndAssetizeArgs) (ParseAndAssetizeResult, error) {
	tld := strings.TrimSpace(args.TLD)
	if tld == "" {
		return ParseAndAssetizeResult{}, fmt.Errorf("tld is required")
	}
	key := strings.TrimSpace(args.ObjectKey)
	if key == "" {
		return ParseAndAssetizeResult{}, fmt.Errorf("objectKey is required")
	}

	s3c, err := storage.NewS3ClientFromEnv()
	if err != nil {
		return ParseAndAssetizeResult{}, err
	}

	base := strings.TrimSuffix(filepath.Base(key), filepath.Ext(key))
	var runPrefix string
	if args.RunPrefix != "" {
		runPrefix = args.RunPrefix
	} else {
		// Fallback to legacy behavior
		runPrefix = filepath.ToSlash(filepath.Join("escrow", tld, "processed", base))
	}

	// Resumption Check using analysis.json
	analysisKey := runPrefix + "/" + base + "-analysis.json"
	if exists, _ := s3c.Exists(ctx, analysisKey); exists {
		activity.GetLogger(ctx).Info("Resuming: Analysis artifacts found", "prefix", runPrefix)

		var envelope struct {
			Analysis struct {
				Errors          []string `json:"errors"`
				MissingContacts []string `json:"missingContacts"`
			} `json:"analysis"`
		}
		if tmp, err := s3c.DownloadToFile(ctx, analysisKey); err == nil {
			if data, rerr := os.ReadFile(tmp); rerr == nil {
				_ = json.Unmarshal(data, &envelope)
			}
			_ = os.Remove(tmp)
		}

		// Reconstruct asset keys logic (simplified for now to match StreamingAnalysis)
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
			base + "-analysis.json",
		}
		assets := map[string]string{}
		for _, f := range candidates {
			objKey := runPrefix + "/" + f
			if ex, _ := s3c.Exists(ctx, objKey); ex {
				assets[f] = objKey
			}
		}

		return ParseAndAssetizeResult{
			RunPrefix:      runPrefix,
			AssetKeys:      assets,
			HasIssues:      len(envelope.Analysis.Errors) > 0 || len(envelope.Analysis.MissingContacts) > 0,
			AnalysisErrors: envelope.Analysis.Errors,
		}, nil
	}

	// Download and Process
	xmlPath, err := s3c.DownloadToFile(ctx, key)
	if err != nil {
		return ParseAndAssetizeResult{}, fmt.Errorf("download failed: %w", err)
	}
	defer os.Remove(xmlPath)

	svc, err := services.NewStreamingXMLEscrowService(xmlPath)
	if err != nil {
		return ParseAndAssetizeResult{}, fmt.Errorf("service init failed: %w", err)
	}

	heartbeat := func(details ...interface{}) {
		activity.RecordHeartbeat(ctx, details...)
	}

	// Run analysis (generates local CSVs)
	// We pass false for mapRegistrars as that is now a separate step
	if err := svc.StreamAnalyze(false, GetBearerToken(), heartbeat); err != nil {
		return ParseAndAssetizeResult{}, fmt.Errorf("stream analyze failed: %w", err)
	}

	// Upload Artifacts
	tempBase := strings.TrimSuffix(xmlPath, filepath.Ext(xmlPath))
	suffixes := []string{
		"-domains.csv", "-domainStatuses.csv", "-domainNameservers.csv", "-DomainDnssec.csv",
		"-domainTransfers.csv", "-domainRgpStatus.csv",
		"-contacts.csv", "-contactStatuses.csv", "-contactPostalInfo.csv", "-uniqueDomainContactIDs.csv",
		"-hosts.csv", "-hostAddresses.csv", "-hostStatuses.csv",
		"-registrars.csv", "-registrarPostalInfo.csv", "-registrarMapping.csv",
		"-nndns.csv", "-analysis.json",
	}

	assets := map[string]string{}
	for _, suffix := range suffixes {
		localFile := tempBase + suffix
		targetName := base + suffix
		if _, err := os.Stat(localFile); err == nil {
			objKey := runPrefix + "/" + targetName
			ctype := "text/csv"
			if strings.HasSuffix(targetName, ".json") {
				ctype = "application/json"
			}
			if err := s3c.UploadFile(ctx, objKey, localFile, ctype); err != nil {
				return ParseAndAssetizeResult{}, fmt.Errorf("upload %s failed: %w", targetName, err)
			}
			assets[targetName] = objKey
			os.Remove(localFile)
		}
	}

	// Check for analysis errors
	hasIssues := false
	var errors []string

	// Re-download analysis.json to check for errors (cleanest way since we uploaded it)
	if analysisKey, ok := assets[base+"-analysis.json"]; ok {
		if tmp, err := s3c.DownloadToFile(ctx, analysisKey); err == nil {
			data, _ := os.ReadFile(tmp)
			var envelope struct {
				Analysis struct {
					Errors          []string `json:"errors"`
					MissingContacts []string `json:"missingContacts"`
				} `json:"analysis"`
			}
			json.Unmarshal(data, &envelope)
			errors = envelope.Analysis.Errors
			if len(errors) > 0 || len(envelope.Analysis.MissingContacts) > 0 {
				hasIssues = true
			}
			os.Remove(tmp)
		}
	}

	return ParseAndAssetizeResult{
		RunPrefix:      runPrefix,
		AssetKeys:      assets,
		HasIssues:      hasIssues,
		AnalysisErrors: errors,
	}, nil
}

func (a *EscrowImportActivities) CollateAssets(ctx context.Context, args CollateAssetsArgs) (CollateAssetsResult, error) {
	tld := strings.TrimSpace(args.TLD)
	if tld == "" {
		return CollateAssetsResult{}, fmt.Errorf("tld is required")
	}
	runPrefix := strings.TrimSpace(args.RunPrefix)
	if runPrefix == "" {
		return CollateAssetsResult{}, fmt.Errorf("runPrefix is required")
	}
	base := strings.TrimSpace(args.BaseFilename)
	if base == "" {
		// Fallback: try to infer from first asset key
		// Asset keys are filename -> s3key.
		// Filenames are typically base-suffix.csv
		// If fails, we error or fallback.
		// Let's assume user passes it. If not, we might be in trouble.
		// Try to infer from runPrefix last part if it was legacy
		parts := strings.Split(runPrefix, "/")
		base = parts[len(parts)-1]
	}

	s3c, err := storage.NewS3ClientFromEnv()
	if err != nil {
		return CollateAssetsResult{}, err
	}

	manifestKey := runPrefix + "/" + base + ".db.manifest.json"
	if exists, _ := s3c.Exists(ctx, manifestKey); exists {
		activity.GetLogger(ctx).Info("Resuming: SQLite manifest found", "manifestKey", manifestKey)
		var manifest struct {
			DBKey string `json:"dbKey"`
		}
		if tmp, err := s3c.DownloadToFile(ctx, manifestKey); err == nil {
			if data, rerr := os.ReadFile(tmp); rerr == nil {
				if jerr := json.Unmarshal(data, &manifest); jerr == nil {
					_ = os.Remove(tmp)
					return CollateAssetsResult{DBKey: manifest.DBKey}, nil
				}
			}
			_ = os.Remove(tmp)
		}
	}

	workDir, err := os.MkdirTemp("", "escrow-sqlite-*")
	if err != nil {
		return CollateAssetsResult{}, err
	}
	defer os.RemoveAll(workDir)

	for filename, key := range args.AssetKeys {
		dst := filepath.Join(workDir, filename)
		tmpPath, err := s3c.DownloadToFile(ctx, key)
		if err != nil {
			return CollateAssetsResult{}, fmt.Errorf("download asset %s failed: %w", filename, err)
		}
		os.Rename(tmpPath, dst)
	}

	// Validate Critical Assets
	// ParseAndAssetize should have produced these. If not, we must fail.
	requiredAssets := []string{
		base + "-contacts.csv",
		base + "-hosts.csv",
		base + "-domains.csv",
		base + "-registrars.csv",
	}
	for _, req := range requiredAssets {
		if _, ok := args.AssetKeys[req]; !ok {
			// Also check if file exists on disk (maybe it wasn't in keys but somehow got there? Unlikely)
			if _, err := os.Stat(filepath.Join(workDir, req)); os.IsNotExist(err) {
				return CollateAssetsResult{}, fmt.Errorf("critical asset missing: %s", req)
			}
		}
	}

	basePath := filepath.Join(workDir, base)
	dbPath := basePath + ".db"
	svc := services.NewCSVToSQLiteService(basePath)

	heartbeat := func(details ...interface{}) {
		activity.RecordHeartbeat(ctx, details...)
	}

	// This converts all CSVs found in basePath* to the SQLite DB
	if err := svc.ConvertToSQLite(dbPath, heartbeat); err != nil {
		return CollateAssetsResult{}, fmt.Errorf("csv to sqlite failed for base %s: %w", base, err)
	}

	// Data Enrichment: Ingest domain counts from analysis.json
	if analysisKey, ok := args.AssetKeys[base+"-analysis.json"]; ok {
		activity.GetLogger(ctx).Info("Enriching DB with analysis data", "key", analysisKey)
		if tmp, err := s3c.DownloadToFile(ctx, analysisKey); err == nil {
			var analysis struct {
				RegistrarMapping map[string]struct {
					RecClID      string `json:"registrarClID"`
					DomainCount  int    `json:"domainCount"`
					HostCount    int    `json:"hostCount"`
					ContactCount int    `json:"contactCount"`
				} `json:"registrarMapping"`
			}
			data, _ := os.ReadFile(tmp)
			if err := json.Unmarshal(data, &analysis); err == nil {
				// Update DB
				if db, err := sql.Open("sqlite", dbPath); err == nil {
					// Add column (ignore error if exists)
					_, _ = db.Exec(`ALTER TABLE registrars ADD COLUMN host_count INTEGER DEFAULT 0`)
					_, _ = db.Exec(`ALTER TABLE registrars ADD COLUMN contact_count INTEGER DEFAULT 0`)

					tx, _ := db.Begin()
					stmt, _ := tx.Prepare(`UPDATE registrars SET domain_count = ?, host_count = ?, contact_count = ? WHERE ID = ?`)
					// Try lower case ID if first fails? Usually sensitive.
					// Actually the DB schema might be "id" or "ID".
					// We'll rely on the fact that we just created it from CSV.
					// If CSV header was "ClID", column is "ClID". Only checks ID for now.
					// Let's try both to be safe or inspect.
					// Simple update:
					for clID, reg := range analysis.RegistrarMapping {
						// Use the map key as the ID, as RecClID might be empty
						if _, err := stmt.Exec(reg.DomainCount, reg.HostCount, reg.ContactCount, clID); err != nil {
							// If ID column is lowercase "id" or something, this might fail if we assumed ID.
							// But previously we query `SELECT ID...`.
						}
					}
					stmt.Close()
					tx.Commit()
					db.Close()
					activity.GetLogger(ctx).Info("Updated registrar domain counts", "count", len(analysis.RegistrarMapping))
				}
			}
			os.Remove(tmp)
		}
	}

	dbKey := runPrefix + "/" + filepath.Base(dbPath)
	if err := s3c.UploadFile(ctx, dbKey, dbPath, "application/octet-stream"); err != nil {
		return CollateAssetsResult{}, fmt.Errorf("upload db failed: %w", err)
	}

	// Write Manifest
	manifest := map[string]any{
		"dbKey":       dbKey,
		"completedAt": time.Now().UTC().Format(time.RFC3339),
	}
	if tmp, err := os.CreateTemp("", "manifest-*.json"); err == nil {
		json.NewEncoder(tmp).Encode(manifest)
		tmp.Close()
		s3c.UploadFile(ctx, manifestKey, tmp.Name(), "application/json")
		os.Remove(tmp.Name())
	}

	return CollateAssetsResult{DBKey: dbKey}, nil
}

func (a *EscrowImportActivities) RegistrarMap(ctx context.Context, args RegistrarMapArgs) (RegistrarMapResult, error) {
	if args.DBKey == "" {
		return RegistrarMapResult{}, fmt.Errorf("dbKey is required")
	}

	s3c, err := storage.NewS3ClientFromEnv()
	if err != nil {
		return RegistrarMapResult{}, err
	}

	// Idempotency Check
	// Since we modify the DB in place, we use a sidecar manifest to track completion.
	manifestKey := args.DBKey + ".mapping-manifest.json"
	if exists, _ := s3c.Exists(ctx, manifestKey); exists {
		activity.GetLogger(ctx).Info("Resuming: Registrar mapping manifest found", "manifestKey", manifestKey)
		// We assume if manifest exists, the DB at args.DBKey is already mapped.
		// We could verify content, but for now trust the artifact.
		return RegistrarMapResult{DBKey: args.DBKey, HasIssues: false}, nil
	}

	// Download DB
	dbPath, err := s3c.DownloadToFile(ctx, args.DBKey)
	if err != nil {
		return RegistrarMapResult{}, fmt.Errorf("download db failed: %w", err)
	}
	defer os.Remove(dbPath)

	// Open SQLite
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return RegistrarMapResult{}, fmt.Errorf("open sqlite failed: %w", err)
	}
	defer db.Close()

	// Ensure registrar_mapping table exists
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS registrar_mapping (
		escrow_id TEXT PRIMARY KEY,
		registrar_clid TEXT,
		name TEXT,
		gurid INTEGER
	)`)
	if err != nil {
		return RegistrarMapResult{}, fmt.Errorf("create mapping table failed: %w", err)
	}

	// Read Registrars
	type regRow struct {
		ClID         string
		Name         string
		GurID        int
		DomainCount  int
		HostCount    int
		ContactCount int
	}
	var registrars []regRow

	// Try to query registrars table
	// We check for domain_count column existence or handle missing
	// Try to query registrars table
	// We check for domain_count, host_count, and contact_count column existence or handle missing
	query := `SELECT ID, name, gurID, domain_count, host_count, contact_count FROM registrars`
	rows, err := db.Query(query)
	if err != nil {
		// Fallback: DB might not have domain_count if enrichment failed or old run
		// Try without domain_count
		rows, err = db.Query(`SELECT ID, name, gurID, 0 as domain_count, 0 as host_count, 0 as contact_count FROM registrars`)
		if err != nil {
			// Try lowercase
			rows, err = db.Query(`SELECT id, name, gurid, domain_count FROM registrars`)
			if err != nil {
				rows, err = db.Query(`SELECT id, name, gurid, 0 as domain_count FROM registrars`)
			}
		}
	}

	if err != nil {
		activity.GetLogger(ctx).Warn("Could not query registrars table", "error", err)
		return RegistrarMapResult{}, temporal.NewNonRetryableApplicationError(
			fmt.Sprintf("failed to query registrars table: %v", err),
			"RegistrarQueryFailed",
			nil,
		)
	}
	defer rows.Close()

	for rows.Next() {
		var r regRow
		if err := rows.Scan(&r.ClID, &r.Name, &r.GurID, &r.DomainCount, &r.HostCount, &r.ContactCount); err == nil {
			registrars = append(registrars, r)
		}
	}

	// Mapping Logic
	// ... (rest is same context)

	// ... inside the loop ...
	// Instead of showing the whole file, I will just replace the loop body logic where "missing" is handled.
	// But ReplaceFileContent needs context. I will fetch the file content to be sure.

	// Mapping Logic
	token := GetBearerToken()
	client := &http.Client{Timeout: 10 * time.Second}
	baseURL := "http://" + os.Getenv("API_HOST") + ":" + os.Getenv("API_PORT")

	tx, err := db.Begin()
	if err != nil {
		return RegistrarMapResult{}, err
	}

	mappedCount := 0
	var missing []string

	for _, r := range registrars {
		mappedID := ""
		sourceName := strings.TrimSpace(r.Name)

		// 1. Check override
		if args.Overrides != nil {
			if v, ok := args.Overrides[sourceName]; ok {
				mappedID = v
			} else if v, ok := args.Overrides[strconv.Itoa(r.GurID)]; ok {
				mappedID = v
			}

			// Verify override validity
			if mappedID != "" {
				if !verifyRegistrarExists(client, baseURL, token, mappedID) {
					activity.GetLogger(ctx).Warn("Override registrar not found", "id", mappedID)
					mappedID = "" // Invalid
				}
			}
		}

		// 2. Auto-map by GURID (Primary Strategy - "90% accurate")
		if mappedID == "" && r.GurID > 0 {
			rid, err := fetchRegistrarByGurID(client, baseURL, token, r.GurID)
			if err == nil && rid != "" {
				mappedID = rid
				activity.GetLogger(ctx).Info("Auto-mapped by GURID", "gurid", r.GurID, "id", rid)
			} else {
				activity.GetLogger(ctx).Info("Failed GURID lookup", "gurid", r.GurID, "error", err, "rid", rid)
			}
		}

		// 3. Auto-map by Name (Secondary Strategy - "Name Like")
		if mappedID == "" {
			rid, err := fetchRegistrarByNameLike(client, baseURL, token, sourceName)
			if err == nil && rid != "" {
				mappedID = rid
				activity.GetLogger(ctx).Info("Auto-mapped by Name", "name", sourceName, "id", rid)
			} else {
				activity.GetLogger(ctx).Info("Failed Name lookup", "name", sourceName, "error", err, "rid", rid)
			}
		}

		// Insert mapping or mark missing
		if mappedID != "" {
			_, err := tx.Exec(`INSERT OR REPLACE INTO registrar_mapping (escrow_id, registrar_clid, name, gurid) VALUES (?, ?, ?, ?)`,
				r.ClID, mappedID, r.Name, r.GurID)
			if err != nil {
				activity.GetLogger(ctx).Error("Failed to insert mapping", "err", err)
				return RegistrarMapResult{}, fmt.Errorf("failed to insert mapping: %w", err)
			}
			mappedCount++
		} else {
			// Smart Validation: Only fail if the registrar actually manages domains OR hosts OR contacts
			if r.DomainCount > 0 || r.HostCount > 0 || r.ContactCount > 0 {
				activity.GetLogger(ctx).Warn("Registrar unmapped", "name", r.Name, "clid", r.ClID, "gurid", r.GurID, "domain_count", r.DomainCount, "host_count", r.HostCount, "contact_count", r.ContactCount)
				missing = append(missing, fmt.Sprintf("%s (ClID: %s, GurID: %d, Domains: %d, Hosts: %d, Contacts: %d)", r.Name, r.ClID, r.GurID, r.DomainCount, r.HostCount, r.ContactCount))
			} else {
				activity.GetLogger(ctx).Warn("Ignoring unmapped empty registrar", "name", r.Name, "clid", r.ClID, "gurid", r.GurID)
			}
		}
	}

	if len(missing) > 0 {
		tx.Rollback()
		activity.GetLogger(ctx).Error("Registrar mapping incomplete", "missing_count", len(missing), "missing", missing)
		return RegistrarMapResult{DBKey: args.DBKey, HasIssues: true}, temporal.NewNonRetryableApplicationError(
			fmt.Sprintf("mapping failed: %d registrars unmapped. Please provide overrides for: %s", len(missing), strings.Join(missing, ", ")),
			"RegistrarMappingIncomplete",
			nil,
		)
	}

	tx.Commit()

	activity.GetLogger(ctx).Info("Mapped registrars", "total", len(registrars), "mapped", mappedCount)

	// Upload Updated DB
	if err := s3c.UploadFile(ctx, args.DBKey, dbPath, "application/octet-stream"); err != nil {
		return RegistrarMapResult{}, fmt.Errorf("upload db failed: %w", err)
	}

	// Write Manifest to mark completion
	manifest := map[string]any{
		"dbKey":       args.DBKey,
		"completedAt": time.Now().UTC().Format(time.RFC3339),
		"mappedCount": mappedCount,
	}
	if tmp, err := os.CreateTemp("", "map-manifest-*.json"); err == nil {
		json.NewEncoder(tmp).Encode(manifest)
		tmp.Close()
		s3c.UploadFile(ctx, manifestKey, tmp.Name(), "application/json")
		os.Remove(tmp.Name())
	}

	return RegistrarMapResult{DBKey: args.DBKey, HasIssues: false}, nil
}

func verifyRegistrarExists(client *http.Client, baseURL, token, id string) bool {
	req, _ := http.NewRequest("GET", baseURL+"/registrars/"+id, nil)
	req.Header.Set("Authorization", token)
	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == 200
}

// fetchRegistrarByGurID returns ID if found for GURID
func fetchRegistrarByGurID(client *http.Client, baseURL, token string, gurid int) (string, error) {
	req, _ := http.NewRequest("GET", fmt.Sprintf("%s/registrars/gurid/%d", baseURL, gurid), nil)
	req.Header.Set("Authorization", token)
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		if resp.StatusCode == 404 {
			return "", nil
		}
		return "", fmt.Errorf("api status %d", resp.StatusCode)
	}

	var res struct {
		ID string `json:"ClID"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return "", err
	}
	return res.ID, nil
}

// fetchRegistrarByNameLike returns ID if found using name filter
func fetchRegistrarByNameLike(client *http.Client, baseURL, token, name string) (string, error) {
	// Encode params
	q := reqUrl.Values{}
	q.Add("name_like", name)

	// We construct URL carefully
	u := baseURL + "/registrars?" + q.Encode()
	req, _ := http.NewRequest("GET", u, nil)
	req.Header.Set("Authorization", token)

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		if resp.StatusCode == 404 {
			return "", nil
		}
		return "", fmt.Errorf("api status %d", resp.StatusCode)
	}

	var res struct {
		Data []struct {
			ID   string `json:"ClID"`
			Name string `json:"Name"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return "", err
	}

	if len(res.Data) > 0 {
		// Prefer exact match if possible, otherwise first
		lowerName := strings.ToLower(name)
		for _, r := range res.Data {
			if strings.ToLower(r.Name) == lowerName {
				return r.ID, nil
			}
		}
		// Fallback to first
		return res.Data[0].ID, nil
	}
	return "", nil
}

func (a *EscrowImportActivities) StageImport(ctx context.Context, args StageImportArgs) (StageImportResult, error) {
	if args.DBKey == "" {
		return StageImportResult{}, fmt.Errorf("dbKey is required")
	}

	s3c, err := storage.NewS3ClientFromEnv()
	if err != nil {
		return StageImportResult{}, err
	}

	// Idempotency: Check if staged DB already exists
	// We derive stagedKey from DBKey logic below
	srcDir := filepath.Dir(args.DBKey)
	srcBase := filepath.Base(args.DBKey)
	stagedBase := "staged-" + srcBase
	stagedKey := srcDir + "/" + stagedBase

	if exists, _ := s3c.Exists(ctx, stagedKey); exists {
		activity.GetLogger(ctx).Info("Resuming: Staged DB found", "key", stagedKey)
		return StageImportResult{StagedDBKey: stagedKey}, nil
	}

	// Download Source DB
	srcPath, err := s3c.DownloadToFile(ctx, args.DBKey)
	if err != nil {
		return StageImportResult{}, fmt.Errorf("download db failed: %w", err)
	}
	defer os.Remove(srcPath)
	defer os.Remove(srcPath + "-wal")
	defer os.Remove(srcPath + "-shm")

	// Create Staging DB path
	workDir := filepath.Dir(srcPath)
	stagedPath := filepath.Join(workDir, stagedBase)
	
	// Ensure we start fresh, especially if local storage emulator keeps stale files
	_ = os.Remove(stagedPath)

	db, err := sql.Open("sqlite", stagedPath)
	if err != nil {
		return StageImportResult{}, fmt.Errorf("open staged db failed: %w", err)
	}
	defer db.Close()

	// Attach Source DB
	if _, err := db.Exec(fmt.Sprintf("ATTACH DATABASE '%s' AS src", srcPath)); err != nil {
		return StageImportResult{}, fmt.Errorf("attach failed: %w", err)
	}

	// Helper to copy and update (Strict: Preserves original if unmapped)
	copyAndUpdate := func(table string, clidCols ...string) error {
		// Copy structure and data (Only CREATE if not exists, but we usually want fresh table? Logic is CREATE AS SELECT)
		// The original code tried DROP TABLE IF EXISTS or relied on fresh staged.db?
		// We are opening a fresh file usually (defer os.Remove(srcPath) implies we work on copies).
		_, err := db.Exec(fmt.Sprintf("CREATE TABLE %s AS SELECT * FROM src.%s", table, table))
		if err != nil {
			if strings.Contains(err.Error(), "no such table") {
				return nil
			}
			return err
		}

		// Update CLIDs - Strict Mode (Preserve Value if Unmapped - triggers FK error later if invalid)
		for _, col := range clidCols {
			query := fmt.Sprintf(`UPDATE %s SET %s = (
				SELECT registrar_clid FROM src.registrar_mapping 
				WHERE TRIM(escrow_id) = TRIM(%s.%s) COLLATE NOCASE
			) WHERE EXISTS (
				SELECT 1 FROM src.registrar_mapping 
				WHERE TRIM(escrow_id) = TRIM(%s.%s) COLLATE NOCASE
			)`, table, col, table, col, table, col)

			if res, err := db.Exec(query); err != nil {
				activity.GetLogger(ctx).Warn("Failed to update column in staged db", "table", table, "col", col, "error", err)
			} else {
				af, _ := res.RowsAffected()
				activity.GetLogger(ctx).Info("Updated CLIDs in staged db (Strict)", "table", table, "col", col, "affected", af)
			}
		}
		return nil
	}

	// Helper to update columns to NULL if unmapped (Nullable Mode)
	// We run this AFTER copyAndUpdate created the table.
	updateNullable := func(table string, clidCols ...string) error {
		for _, col := range clidCols {
			// Update to NULL if no mapping found (Removing WHERE EXISTS clause)
			query := fmt.Sprintf(`UPDATE %s SET %s = (
				SELECT registrar_clid FROM src.registrar_mapping 
				WHERE TRIM(escrow_id) = TRIM(%s.%s) COLLATE NOCASE
			)`, table, col, table, col)

			if res, err := db.Exec(query); err != nil {
				activity.GetLogger(ctx).Warn("Failed to update nullable column in staged db", "table", table, "col", col, "error", err)
			} else {
				af, _ := res.RowsAffected()
				activity.GetLogger(ctx).Info("Updated CLIDs in staged db (Nullable)", "table", table, "col", col, "affected", af)
			}
		}
		return nil
	}

	// Contacts
	if err := copyAndUpdate("contacts", "clID"); err != nil {
		return StageImportResult{}, fmt.Errorf("stage contacts failed: %w", err)
	}
	updateNullable("contacts", "crRr", "upRr")

	// Hosts
	if err := copyAndUpdate("hosts", "clID"); err != nil {
		return StageImportResult{}, fmt.Errorf("stage hosts failed: %w", err)
	}
	updateNullable("hosts", "crRr", "upRr")

	// Domains
	if err := copyAndUpdate("domains", "clID"); err != nil {
		return StageImportResult{}, fmt.Errorf("stage domains failed: %w", err)
	}
	updateNullable("domains", "crRr", "upRr")

	// Copy auxiliary tables without updates
	knownTables := []string{"host_addresses", "domain_hosts", "domain_statuses", "contact_statuses", "host_statuses", "domain_nameservers", "domain_rgp_statuses", "nndns"}
	for _, t := range knownTables {
		if _, err := db.Exec(fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s AS SELECT * FROM src.%s", t, t)); err != nil {
			if strings.Contains(err.Error(), "no such table") {
				activity.GetLogger(ctx).Warn("⚠️ StageImport: missing aux table in source, skipping", "table", t)
				continue
			}
			return StageImportResult{}, fmt.Errorf("stage aux table %s failed: %w", t, err)
		}
	}

	// Detach
	db.Exec("DETACH DATABASE src")

	// Upload Staged DB
	if err := s3c.UploadFile(ctx, stagedKey, stagedPath, "application/octet-stream"); err != nil {
		return StageImportResult{}, fmt.Errorf("upload staged db failed: %w", err)
	}

	return StageImportResult{StagedDBKey: stagedKey}, nil
}

// IngestContactsArgs parameters
type IngestContactsArgs struct {
	StagedDBKey string
}

// IngestContactsResult outcome
type IngestContactsResult struct {
	Total   int64
	Skipped int64
}

// IngestContacts imports contacts from the staged DB into the registry
func (a *EscrowImportActivities) IngestContacts(ctx context.Context, args IngestContactsArgs) (IngestContactsResult, error) {
	s3c, err := storage.NewS3ClientFromEnv()
	if err != nil {
		return IngestContactsResult{}, err
	}

	// Resumability: Check if we have a heartbeat detail to resume from
	var lastKey string
	if activity.HasHeartbeatDetails(ctx) {
		var d string
		if err := activity.GetHeartbeatDetails(ctx, &d); err == nil {
			lastKey = d
		}
	}

	// Download Staged DB
	dbPath, err := s3c.DownloadToFile(ctx, args.StagedDBKey)
	if err != nil {
		return IngestContactsResult{}, fmt.Errorf("download db failed: %w", err)
	}
	defer os.Remove(dbPath)

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return IngestContactsResult{}, fmt.Errorf("open sqlite failed: %w", err)
	}
	defer db.Close()

	destImporter, err := services.NewDirectDBImporter()
	if err != nil {
		return IngestContactsResult{}, fmt.Errorf("failed to init importer: %w", err)
	}
	defer destImporter.PG.Close()

	// Identity mapping (staged DB has correct IDs)
	// We pass nil to indicate trust/passthrough mode
	var clidMap map[string]string = nil

	heartbeat := func(processed string) {
		activity.RecordHeartbeat(ctx, processed)
	}

	total, skipped, err := destImporter.ImportContacts(ctx, db, clidMap, lastKey, heartbeat)
	if err != nil {
		return IngestContactsResult{Total: total, Skipped: skipped}, err
	}

	return IngestContactsResult{Total: total, Skipped: skipped}, nil
}

// IngestHostsArgs parameters
type IngestHostsArgs struct {
	StagedDBKey string
}

type IngestHostsResult struct {
	Total   int64
	Skipped int64
}

// IngestHosts imports hosts
func (a *EscrowImportActivities) IngestHosts(ctx context.Context, args IngestHostsArgs) (IngestHostsResult, error) {
	s3c, err := storage.NewS3ClientFromEnv()
	if err != nil {
		return IngestHostsResult{}, err
	}

	var lastKey string
	if activity.HasHeartbeatDetails(ctx) {
		var d string
		if err := activity.GetHeartbeatDetails(ctx, &d); err == nil {
			lastKey = d
		}
	}

	dbPath, err := s3c.DownloadToFile(ctx, args.StagedDBKey)
	if err != nil {
		return IngestHostsResult{}, err
	}
	defer os.Remove(dbPath)

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return IngestHostsResult{}, err
	}
	defer db.Close()

	destImporter, err := services.NewDirectDBImporter()
	if err != nil {
		return IngestHostsResult{}, err
	}
	defer destImporter.PG.Close()

	heartbeat := func(processed string) {
		activity.RecordHeartbeat(ctx, processed)
	}

	// Identity mapping (staged DB has correct IDs)
	var clidMap map[string]string = nil
	total, skipped, err := destImporter.ImportHosts(ctx, db, clidMap, lastKey, heartbeat)
	if err != nil {
		return IngestHostsResult{Total: total, Skipped: skipped}, err
	}

	return IngestHostsResult{Total: total, Skipped: skipped}, nil
}

// IngestDomainsArgs parameters
type IngestDomainsArgs struct {
	StagedDBKey string
	TLD         string
}

type IngestDomainsResult struct {
	Total   int64
	Skipped int64
}

// IngestDomains imports domains
func (a *EscrowImportActivities) IngestDomains(ctx context.Context, args IngestDomainsArgs) (IngestDomainsResult, error) {
	s3c, err := storage.NewS3ClientFromEnv()
	if err != nil {
		return IngestDomainsResult{}, err
	}

	var lastKey string
	if activity.HasHeartbeatDetails(ctx) {
		var d string
		if err := activity.GetHeartbeatDetails(ctx, &d); err == nil {
			lastKey = d
		}
	}

	dbPath, err := s3c.DownloadToFile(ctx, args.StagedDBKey)
	if err != nil {
		return IngestDomainsResult{}, err
	}
	defer os.Remove(dbPath)
	defer os.Remove(dbPath + "-wal")
	defer os.Remove(dbPath + "-shm")

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return IngestDomainsResult{}, err
	}
	defer db.Close()

	destImporter, err := services.NewDirectDBImporter()
	if err != nil {
		return IngestDomainsResult{}, err
	}
	defer destImporter.PG.Close()

	heartbeat := func(processed string) {
		activity.RecordHeartbeat(ctx, processed)
	}

	// Identity mapping (staged DB has correct IDs)
	var clidMap map[string]string = nil
	total, skipped, err := destImporter.ImportDomains(ctx, db, args.TLD, clidMap, lastKey, heartbeat)
	if err != nil {
		return IngestDomainsResult{Total: total, Skipped: skipped}, err
	}

	return IngestDomainsResult{Total: total, Skipped: skipped}, nil
}

// IngestNNDNsArgs parameters
type IngestNNDNsArgs struct {
	StagedDBKey string
	TLD         string
}

// IngestNNDNsResult outcome
type IngestNNDNsResult struct {
	Total   int64
	Skipped int64
}

// IngestNNDNs imports NNDNs from the staged DB into the registry
func (a *EscrowImportActivities) IngestNNDNs(ctx context.Context, args IngestNNDNsArgs) (IngestNNDNsResult, error) {
	s3c, err := storage.NewS3ClientFromEnv()
	if err != nil {
		return IngestNNDNsResult{}, err
	}

	var lastKey string
	if activity.HasHeartbeatDetails(ctx) {
		var d string
		if err := activity.GetHeartbeatDetails(ctx, &d); err == nil {
			lastKey = d
		}
	}

	dbPath, err := s3c.DownloadToFile(ctx, args.StagedDBKey)
	if err != nil {
		return IngestNNDNsResult{}, err
	}
	defer os.Remove(dbPath)
	defer os.Remove(dbPath + "-wal")
	defer os.Remove(dbPath + "-shm")

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return IngestNNDNsResult{}, err
	}
	defer db.Close()

	destImporter, err := services.NewDirectDBImporter()
	if err != nil {
		return IngestNNDNsResult{}, err
	}
	defer destImporter.PG.Close()

	heartbeat := func(processed string) {
		activity.RecordHeartbeat(ctx, processed)
	}

	total, skipped, err := destImporter.ImportNNDNs(ctx, db, args.TLD, lastKey, heartbeat)
	if err != nil {
		return IngestNNDNsResult{Total: total, Skipped: skipped}, err
	}

	return IngestNNDNsResult{Total: total, Skipped: skipped}, nil
}

// LinkDomainHostsArgs parameters
type LinkDomainHostsArgs struct {
	StagedDBKey string
}

// LinkDomainHostsResult outcome
type LinkDomainHostsResult struct {
	Total int64
}

// LinkDomainHosts links domains to hosts
func (a *EscrowImportActivities) LinkDomainHosts(ctx context.Context, args LinkDomainHostsArgs) (LinkDomainHostsResult, error) {
	s3c, err := storage.NewS3ClientFromEnv()
	if err != nil {
		return LinkDomainHostsResult{}, err
	}

	var lastKey string
	if activity.HasHeartbeatDetails(ctx) {
		var d string
		if err := activity.GetHeartbeatDetails(ctx, &d); err == nil {
			lastKey = d
		}
	}

	dbPath, err := s3c.DownloadToFile(ctx, args.StagedDBKey)
	if err != nil {
		return LinkDomainHostsResult{}, err
	}
	defer os.Remove(dbPath)
	defer os.Remove(dbPath + "-wal")
	defer os.Remove(dbPath + "-shm")

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return LinkDomainHostsResult{}, err
	}
	defer db.Close()

	destImporter, err := services.NewDirectDBImporter()
	if err != nil {
		return LinkDomainHostsResult{}, err
	}
	defer destImporter.PG.Close()

	heartbeat := func(processed string) {
		activity.RecordHeartbeat(ctx, processed)
	}

	total, err := destImporter.LinkDomainHosts(ctx, db, lastKey, heartbeat)
	if err != nil {
		return LinkDomainHostsResult{Total: total}, err
	}

	return LinkDomainHostsResult{Total: total}, nil
}
