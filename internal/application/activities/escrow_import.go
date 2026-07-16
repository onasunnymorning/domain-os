package activities

import (
	"compress/gzip"
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

	gopg "github.com/go-pg/pg/v10"
	"github.com/onasunnymorning/domain-os/internal/application/commands"
	"github.com/onasunnymorning/domain-os/internal/application/services"
	pg "github.com/onasunnymorning/domain-os/internal/infrastructure/db/postgres"
	"github.com/onasunnymorning/domain-os/internal/infrastructure/storage"
	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/temporal"
	"gorm.io/gorm"
	_ "modernc.org/sqlite"
)

// ValidateEscrowSourceArgs represents the basic input validation parameters
type ValidateEscrowSourceArgs struct {
	TLD       string
	ObjectKey string
}

// ValidateEscrowSourceResult returns the outcome of basic validation
type ValidateEscrowSourceResult struct {
	TLD       string
	ObjectKey string
	Exists    bool
}

// EscrowImportActivities groups escrow-related activity methods
type EscrowImportActivities struct{}

// ValidateEscrowSource checks that required inputs are present, the TLD allows
// escrow imports, and the source object exists in S3/MinIO.
func (a *EscrowImportActivities) ValidateEscrowSource(ctx context.Context, args ValidateEscrowSourceArgs) (ValidateEscrowSourceResult, error) {
	tld := strings.TrimSpace(args.TLD)
	if tld == "" {
		return ValidateEscrowSourceResult{}, fmt.Errorf("tld is required")
	}
	key := strings.TrimSpace(args.ObjectKey)
	if key == "" {
		return ValidateEscrowSourceResult{}, fmt.Errorf("objectKey is required")
	}

	// Guard: check TLD allows escrow imports via admin API
	apiBase := buildAdminAPIURL()
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := adminAPIGet(client, fmt.Sprintf("%s/tlds/%s", apiBase, tld))
	if err != nil {
		return ValidateEscrowSourceResult{}, fmt.Errorf("ValidateEscrowSource: failed to fetch TLD %q from API (%s): %w. Check that the admin-api is running and API_HOST/API_PORT are set", tld, apiBase, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return ValidateEscrowSourceResult{}, fmt.Errorf("ValidateEscrowSource: TLD %q not found in the system. Create the TLD first", tld)
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return ValidateEscrowSourceResult{}, fmt.Errorf("ValidateEscrowSource: failed to fetch TLD %q: HTTP %d: %s", tld, resp.StatusCode, string(body))
	}

	var tldInfo struct {
		AllowEscrowImport bool `json:"AllowEscrowImport"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tldInfo); err != nil {
		return ValidateEscrowSourceResult{}, fmt.Errorf("ValidateEscrowSource: failed to parse TLD response: %w", err)
	}
	if !tldInfo.AllowEscrowImport {
		return ValidateEscrowSourceResult{}, fmt.Errorf("ValidateEscrowSource: escrow import is disabled for TLD %q. Enable it via PUT /tlds/%s/status/AllowEscrowImport", tld, tld)
	}

	// Validate source file exists in S3
	s3c, err := storage.NewS3ClientFromEnv()
	if err != nil {
		return ValidateEscrowSourceResult{}, err
	}
	exists, err := s3c.Exists(ctx, key)
	if err != nil {
		return ValidateEscrowSourceResult{}, err
	}

	return ValidateEscrowSourceResult{TLD: tld, ObjectKey: key, Exists: exists}, nil
}

// StreamingAnalysisArgs parameters
type StreamingAnalysisArgs struct {
	TLD           string
	ObjectKey     string
	MapRegistrars bool
}

// StreamingAnalysisResult outcome
type StreamingAnalysisResult struct {
	RunPrefix    string            // flat: workflowId (e.g. escrow-import-best-20260625-001231)
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

	// Flat bucket layout: use workflow ID as the run prefix.
	// Falls back to a deterministic name if called outside a workflow.
	runPrefix := wfID
	if runPrefix == "" || runPrefix == "unknown" {
		runPrefix = fmt.Sprintf("escrow-legacy-%s-%s", tld, base)
	}

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
	token, err := GetBearerToken()
	if err != nil {
		return StreamingAnalysisResult{}, err
	}

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

	total, inserted, updated, skipped, err := importer.ImportContacts(ctx, db, clidMap, lastKey, heartbeat)
	if err != nil {
		return ImportFromSQLiteResult{}, err
	}

	counts["contacts_imported"] = total
	counts["contacts_inserted"] = inserted
	counts["contacts_updated"] = updated
	counts["contacts_skipped"] = skipped

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

	total, inserted, updated, err := importer.ImportHosts(ctx, db, clidMap, lastKey, heartbeat)
	if err != nil {
		activity.GetLogger(ctx).Error("Direct host import failed", "error", err)
		return ImportFromSQLiteResult{}, err
	}

	counts["hosts_imported"] = total
	counts["hosts_inserted"] = inserted
	counts["hosts_updated"] = updated

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

	total, inserted, updated, err := importer.ImportDomains(ctx, db, tld, clidMap, lastKey, heartbeat)
	if err != nil {
		return ImportFromSQLiteResult{}, err
	}

	counts["domains_imported"] = total
	counts["domains_inserted"] = inserted
	counts["domains_updated"] = updated

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

	total, lInserted, err := importer.LinkDomainHosts(ctx, db, lastKey, heartbeat)
	if err != nil {
		return ImportFromSQLiteResult{}, err
	}

	counts["domain_hosts_linked"] = total
	counts["domain_hosts_inserted"] = lInserted

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
	var gdb *gorm.DB
	var err error
	if dbURL := os.Getenv("DATABASE_URL"); dbURL != "" {
		gdb, err = pg.NewConnectionFromURL(dbURL, false)
	} else {
		pgCfg := pg.Config{
			User:        os.Getenv("DB_USER"),
			Pass:        os.Getenv("DB_PASS"),
			Host:        os.Getenv("DB_HOST"),
			Port:        os.Getenv("DB_PORT"),
			DBName:      os.Getenv("DB_NAME"),
			SSLmode:     defaultStr(os.Getenv("DB_SSLMODE"), "disable"),
			AutoMigrate: false,
		}
		gdb, err = pg.NewConnection(pgCfg)
	}
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
		if err := BulkCreateHosts(ctx, "escrow-import", createHostCmds); err != nil {
			// If bulk fails (e.g., duplicates), try per-item create for idempotency
			for i, c := range cmds {
				if i%100 == 0 {
					activity.RecordHeartbeat(ctx, "fallback-hosts", c.ClID)
				}
				if ierr := CreateHost(ctx, "escrow-import", *c); ierr != nil {
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
		if err := BulkCreateDomains(ctx, "escrow-import", createDomCmds); err != nil {
			// On error (duplicates, etc.), try per-item to be idempotent
			for i, c := range cmds {
				if i%100 == 0 {
					activity.RecordHeartbeat(ctx, "fallback-domains", c.Name)
				}
				if ierr := CreateDomain(ctx, "escrow-import", *c); ierr != nil {
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
			if err := AddHostToDomainByHostname(ctx, "escrow-import", p.d, p.n); err != nil {
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
		if err := BulkCreateContacts(ctx, "escrow-import", createContactCmds); err != nil {
			// try per-item for idempotency
			for i, c := range cmds {
				if i%100 == 0 {
					activity.RecordHeartbeat(ctx, "fallback-contacts", c.ID)
				}
				if ierr := CreateContact(ctx, "escrow-import", *c); ierr != nil {
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

type ParseAndExtractAssetsArgs struct {
	TLD       string
	ObjectKey string
	RunPrefix string
}

type ParseAndExtractAssetsResult struct {
	RunPrefix      string
	AssetKeys      map[string]string // filename -> s3 key
	HasIssues      bool
	AnalysisErrors []string
}

type BuildStagingDatabaseArgs struct {
	TLD          string
	RunPrefix    string
	AssetKeys    map[string]string
	BaseFilename string
}

type BuildStagingDatabaseResult struct {
	DBKey string
}

type ResolveRegistrarsArgs struct {
	TLD       string
	DBKey     string
	RunPrefix string
	Overrides map[string]string
}

// UnmappedRegistrar captures info about a registrar that couldn't be auto-mapped
// to a system registrar. Surfaced in the UI so operators can provide overrides.
// Suggestion fields are pre-populated from the escrow data so the operator can
// review and confirm without manual lookup.
type UnmappedRegistrar struct {
	EscrowID     string `json:"escrowId"`
	Name         string `json:"name"`
	GurID        int    `json:"gurId"`
	DomainCount  int    `json:"domainCount"`
	HostCount    int    `json:"hostCount"`
	ContactCount int    `json:"contactCount"`

	// Suggestion fields — pre-filled from escrow data for the inline create form.
	// Operators should review and correct before submitting.
	SuggestedEmail  string                       `json:"suggestedEmail,omitempty"`
	SuggestedVoice  string                       `json:"suggestedVoice,omitempty"`
	SuggestedURL    string                       `json:"suggestedUrl,omitempty"`
	SuggestedPostal []UnmappedRegistrarPostalInfo `json:"suggestedPostal,omitempty"`
}

// UnmappedRegistrarPostalInfo holds a single postal address suggestion from the escrow data.
type UnmappedRegistrarPostalInfo struct {
	Type          string `json:"type"`
	Street1       string `json:"street1,omitempty"`
	City          string `json:"city,omitempty"`
	StateProvince string `json:"stateProvince,omitempty"`
	PostalCode    string `json:"postalCode,omitempty"`
	CountryCode   string `json:"countryCode,omitempty"`
}

// AutoFixedRegistrar records a host-only registrar that was automatically resolved
// by tracing host→domain relationships and reassigning hosts to domain registrars.
type AutoFixedRegistrar struct {
	EscrowID        string `json:"escrowId"`
	Name            string `json:"name"`
	HostsReassigned int    `json:"hostsReassigned"` // Hosts updated to a single domain registrar
	HostsDuplicated int    `json:"hostsDuplicated"` // Hosts duplicated across multiple domain registrars
}

// RejectedOverride records an override that was provided but rejected during verification.
type RejectedOverride struct {
	Key      string `json:"key"`      // Override key (registrar name or GurID)
	TargetID string `json:"targetId"` // The system ClID that was provided
	Reason   string `json:"reason"`   // Why it was rejected
}

type ResolveRegistrarsResult struct {
	DBKey               string                // Updated db key
	HasIssues           bool                  // True if any active registrar is unmapped
	TotalRegistrars     int                   // Total registrars in escrow
	MappedCount         int                   // Successfully mapped
	UnmappedRegistrars  []UnmappedRegistrar   // Registrars that couldn't be auto-mapped
	AutoFixedRegistrars []AutoFixedRegistrar  // Host-only registrars auto-resolved
	RejectedOverrides   []RejectedOverride    // Overrides that were provided but rejected
}

// autoFixHostOnlyRegistrars resolves unmapped registrars that only manage hosts
// (no domains, no contacts) by tracing host→domain_nameservers→domain relationships.
//
// For each host from an unmapped registrar:
//   - Find which domains reference it (via domain_nameservers)
//   - Determine which registrar those domains belong to
//   - Reassign the host's clID to that domain's registrar
//   - If the host is used by domains from multiple registrars, duplicate the host record
//
// This modifies the DB in-place. Hosts not referenced by any domain are left unchanged.
func autoFixHostOnlyRegistrars(logger *log.Logger, db *sql.DB, hostOnly []UnmappedRegistrar) ([]AutoFixedRegistrar, error) {
	if len(hostOnly) == 0 {
		return nil, nil
	}

	// Build IN-clause for unmapped registrar IDs
	placeholders := make([]string, len(hostOnly))
	args := make([]interface{}, len(hostOnly))
	for i, r := range hostOnly {
		placeholders[i] = "?"
		args[i] = r.EscrowID
	}
	inClause := strings.Join(placeholders, ",")

	// Step 1: Compute host → target registrar assignments.
	// For each host from an unmapped registrar, find which mapped domain registrars
	// reference it through domain_nameservers.
	rows, err := db.Query(`
		SELECT h.name AS host_name, h.clID AS original_clid, d.clID AS target_registrar
		FROM hosts h
		JOIN domain_nameservers dn ON dn.nameserver = h.name
		JOIN domains d ON d.name = dn.domain_name
		WHERE h.clID IN (`+inClause+`)
		GROUP BY h.name, d.clID
	`, args...)
	if err != nil {
		return nil, fmt.Errorf("autoFixHostOnlyRegistrars: query assignments: %w", err)
	}

	type hostAssignment struct {
		HostName     string
		OriginalClID string
		TargetClID   string
	}

	// hostTargets maps hostname → list of target registrar escrow IDs
	hostTargets := map[string][]string{}
	// Track which original clID each host came from
	hostOriginal := map[string]string{}

	for rows.Next() {
		var a hostAssignment
		if err := rows.Scan(&a.HostName, &a.OriginalClID, &a.TargetClID); err != nil {
			rows.Close()
			return nil, fmt.Errorf("autoFixHostOnlyRegistrars: scan: %w", err)
		}
		hostTargets[a.HostName] = append(hostTargets[a.HostName], a.TargetClID)
		hostOriginal[a.HostName] = a.OriginalClID
	}
	rows.Close()

	if len(hostTargets) == 0 {
		logger.Printf("No host assignments found — all hosts orphaned (unmappedRegistrars=%d)", len(hostOnly))
		return nil, nil
	}

	// Step 2: Check if any hosts need duplication (used by multiple registrars)
	needsDuplication := false
	for _, targets := range hostTargets {
		if len(targets) > 1 {
			needsDuplication = true
			break
		}
	}

	// Step 3: Apply changes
	tx, err := db.Begin()
	if err != nil {
		return nil, fmt.Errorf("autoFixHostOnlyRegistrars: begin tx: %w", err)
	}
	defer tx.Rollback()

	if needsDuplication {
		// Recreate hosts table with composite PK (name, clid) to allow duplicates.
		// This is necessary because the original schema has name as sole PK.
		_, err = tx.Exec(`CREATE TABLE hosts_fixed (
			name TEXT NOT NULL,
			roid TEXT,
			clid TEXT NOT NULL,
			crrr TEXT,
			crdate TEXT,
			uprr TEXT,
			"update" TEXT,
			PRIMARY KEY (name, clid)
		)`)
		if err != nil {
			return nil, fmt.Errorf("autoFixHostOnlyRegistrars: create hosts_fixed: %w", err)
		}

		// Copy hosts NOT from unmapped registrars (unchanged)
		_, err = tx.Exec(`INSERT INTO hosts_fixed SELECT * FROM hosts WHERE clID NOT IN (`+inClause+`)`, args...)
		if err != nil {
			return nil, fmt.Errorf("autoFixHostOnlyRegistrars: copy unaffected hosts: %w", err)
		}

		// Insert reassigned/duplicated hosts
		stmt, err := tx.Prepare(`INSERT OR IGNORE INTO hosts_fixed (name, roid, clid, crrr, crdate, uprr, "update")
			SELECT name, roid, ?, crrr, crdate, uprr, "update" FROM hosts WHERE name = ?`)
		if err != nil {
			return nil, fmt.Errorf("autoFixHostOnlyRegistrars: prepare insert: %w", err)
		}
		for hostName, targets := range hostTargets {
			for _, targetClID := range targets {
				if _, err := stmt.Exec(targetClID, hostName); err != nil {
					logger.Printf("⚠️ Failed to insert reassigned host %s → %s: %v", hostName, targetClID, err)
				}
			}
		}
		stmt.Close()

		// Copy orphaned hosts (from unmapped registrars, not referenced by any domain) as-is
		_, err = tx.Exec(`INSERT OR IGNORE INTO hosts_fixed
			SELECT * FROM hosts WHERE clID IN (`+inClause+`)
			AND name NOT IN (SELECT DISTINCT nameserver FROM domain_nameservers)`, args...)
		if err != nil {
			logger.Printf("⚠️ Failed to copy orphaned hosts: %v", err)
		}

		// Swap tables
		if _, err = tx.Exec(`DROP TABLE hosts`); err != nil {
			return nil, fmt.Errorf("autoFixHostOnlyRegistrars: drop hosts: %w", err)
		}
		if _, err = tx.Exec(`ALTER TABLE hosts_fixed RENAME TO hosts`); err != nil {
			return nil, fmt.Errorf("autoFixHostOnlyRegistrars: rename hosts_fixed: %w", err)
		}
	} else {
		// Simple case: all hosts map to exactly one registrar, just UPDATE
		stmt, err := tx.Prepare(`UPDATE hosts SET clID = ? WHERE name = ?`)
		if err != nil {
			return nil, fmt.Errorf("autoFixHostOnlyRegistrars: prepare update: %w", err)
		}
		for hostName, targets := range hostTargets {
			if _, err := stmt.Exec(targets[0], hostName); err != nil {
				logger.Printf("⚠️ Failed to update host %s → %s: %v", hostName, targets[0], err)
			}
		}
		stmt.Close()
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("autoFixHostOnlyRegistrars: commit: %w", err)
	}

	// Step 4: Build tracking data per unmapped registrar
	perRegistrar := map[string]*AutoFixedRegistrar{}
	for _, r := range hostOnly {
		perRegistrar[r.EscrowID] = &AutoFixedRegistrar{
			EscrowID: r.EscrowID,
			Name:     r.Name,
		}
	}
	for hostName, targets := range hostTargets {
		origClID := hostOriginal[hostName]
		if af, ok := perRegistrar[origClID]; ok {
			if len(targets) == 1 {
				af.HostsReassigned++
			} else {
				af.HostsDuplicated++
			}
		}
	}

	var result []AutoFixedRegistrar
	for _, af := range perRegistrar {
		if af.HostsReassigned > 0 || af.HostsDuplicated > 0 {
			logger.Printf("✅ Auto-fixed host-only registrar %s (%s): reassigned=%d duplicated=%d",
				af.EscrowID, af.Name, af.HostsReassigned, af.HostsDuplicated)
			result = append(result, *af)
		}
	}

	return result, nil
}

type ApplyRegistrarMappingsArgs struct {
	TLD   string
	DBKey string
}

type ApplyRegistrarMappingsResult struct {
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

func (a *EscrowImportActivities) ParseAndExtractAssets(ctx context.Context, args ParseAndExtractAssetsArgs) (ParseAndExtractAssetsResult, error) {
	tld := strings.TrimSpace(args.TLD)
	if tld == "" {
		return ParseAndExtractAssetsResult{}, fmt.Errorf("tld is required")
	}
	key := strings.TrimSpace(args.ObjectKey)
	if key == "" {
		return ParseAndExtractAssetsResult{}, fmt.Errorf("objectKey is required")
	}

	s3c, err := storage.NewS3ClientFromEnv()
	if err != nil {
		return ParseAndExtractAssetsResult{}, err
	}

	base := filepath.Base(key)
	base = strings.TrimSuffix(base, ".gz")
	base = strings.TrimSuffix(base, ".xml")
	var runPrefix string
	if args.RunPrefix != "" {
		runPrefix = args.RunPrefix
	} else {
		// Fallback: use activity workflow ID (flat bucket layout)
		wfID := activity.GetInfo(ctx).WorkflowExecution.ID
		if wfID != "" {
			runPrefix = wfID
		} else {
			runPrefix = fmt.Sprintf("escrow-legacy-%s-%s", tld, base)
		}
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

		return ParseAndExtractAssetsResult{
			RunPrefix:      runPrefix,
			AssetKeys:      assets,
			HasIssues:      len(envelope.Analysis.Errors) > 0 || len(envelope.Analysis.MissingContacts) > 0,
			AnalysisErrors: envelope.Analysis.Errors,
		}, nil
	}

	// ── Stream S3 → gzip → XML decoder (no temp files for source XML) ──
	isGzip := strings.HasSuffix(strings.ToLower(key), ".gz")

	stream, fileSize, err := s3c.GetObjectStream(ctx, key)
	if err != nil {
		return ParseAndExtractAssetsResult{}, fmt.Errorf("S3 stream failed: %w", err)
	}
	defer stream.Close()

	// Build the reader chain: S3 stream → [gzip decoder] → XML decoder
	var xmlReader io.Reader = stream
	var gzReader *gzip.Reader
	if isGzip {
		activity.GetLogger(ctx).Info("Streaming gzip-compressed escrow", "key", key, "compressedSize", fileSize)
		gzReader, err = gzip.NewReader(stream)
		if err != nil {
			return ParseAndExtractAssetsResult{}, fmt.Errorf("gzip reader init failed: %w", err)
		}
		defer gzReader.Close()
		xmlReader = gzReader
	} else {
		activity.GetLogger(ctx).Info("Streaming uncompressed escrow", "key", key, "size", fileSize)
	}

	// Signal liveness before XML parsing begins — covers the S3 buffering gap
	activity.RecordHeartbeat(ctx, "stream opened, starting parse")

	// CSVs still need a temp directory for output
	outputDir, err := os.MkdirTemp("", "escrow-csv-*")
	if err != nil {
		return ParseAndExtractAssetsResult{}, fmt.Errorf("temp dir creation failed: %w", err)
	}
	defer os.RemoveAll(outputDir)

	svc, err := services.NewStreamingXMLEscrowServiceFromReader(xmlReader, outputDir, base, fileSize)
	if err != nil {
		return ParseAndExtractAssetsResult{}, fmt.Errorf("service init failed: %w", err)
	}

	heartbeat := func(details ...interface{}) {
		activity.RecordHeartbeat(ctx, details...)
	}

	// Run analysis (generates local CSVs in outputDir)
	token, err := GetBearerToken()
	if err != nil {
		return ParseAndExtractAssetsResult{}, err
	}
	if err := svc.StreamAnalyze(false, token, heartbeat); err != nil {
		return ParseAndExtractAssetsResult{}, fmt.Errorf("stream analyze failed: %w", err)
	}

	// Upload Artifacts — CSVs are at outputDir/base-*.csv
	tempBase := filepath.Join(outputDir, base)
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
				return ParseAndExtractAssetsResult{}, fmt.Errorf("upload %s failed: %w", targetName, err)
			}
			assets[targetName] = objKey
			os.Remove(localFile)
		}
	}

	// Check for analysis errors directly from service (no re-download needed)
	hasIssues := len(svc.Analysis.Errors) > 0 || len(svc.Analysis.MissingContacts) > 0

	return ParseAndExtractAssetsResult{
		RunPrefix:      runPrefix,
		AssetKeys:      assets,
		HasIssues:      hasIssues,
		AnalysisErrors: svc.Analysis.Errors,
	}, nil
}

func (a *EscrowImportActivities) BuildStagingDatabase(ctx context.Context, args BuildStagingDatabaseArgs) (BuildStagingDatabaseResult, error) {
	tld := strings.TrimSpace(args.TLD)
	if tld == "" {
		return BuildStagingDatabaseResult{}, fmt.Errorf("tld is required")
	}
	runPrefix := strings.TrimSpace(args.RunPrefix)
	if runPrefix == "" {
		return BuildStagingDatabaseResult{}, fmt.Errorf("runPrefix is required")
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
		return BuildStagingDatabaseResult{}, err
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
					return BuildStagingDatabaseResult{DBKey: manifest.DBKey}, nil
				}
			}
			_ = os.Remove(tmp)
		}
	}

	workDir, err := os.MkdirTemp("", "escrow-sqlite-*")
	if err != nil {
		return BuildStagingDatabaseResult{}, err
	}
	defer os.RemoveAll(workDir)

	for filename, key := range args.AssetKeys {
		dst := filepath.Join(workDir, filename)
		tmpPath, err := s3c.DownloadToFile(ctx, key)
		if err != nil {
			return BuildStagingDatabaseResult{}, fmt.Errorf("download asset %s failed: %w", filename, err)
		}
		if err := moveFile(tmpPath, dst); err != nil {
			return BuildStagingDatabaseResult{}, fmt.Errorf("move asset %s to workdir failed: %w", filename, err)
		}
	}

	// Validate Critical Assets
	// ParseAndExtractAssets should have produced these. If not, we must fail.
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
				return BuildStagingDatabaseResult{}, fmt.Errorf("critical asset missing: %s", req)
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
		return BuildStagingDatabaseResult{}, fmt.Errorf("csv to sqlite failed for base %s: %w", base, err)
	}

	// NOTE: Registrar object counts (domain_count, host_count, contact_count) are now
	// computed directly from the imported data tables by CSVToSQLiteService.enrichRegistrarCounts(),
	// and ResolveRegistrars independently computes counts via LEFT JOIN against the data tables.
	// The analysis.json enrichment was removed because the analysis data was pre-computed
	// externally and often had stale or incomplete counts.

	dbKey := runPrefix + "/" + filepath.Base(dbPath)
	if err := s3c.UploadFile(ctx, dbKey, dbPath, "application/octet-stream"); err != nil {
		return BuildStagingDatabaseResult{}, fmt.Errorf("upload db failed: %w", err)
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

	return BuildStagingDatabaseResult{DBKey: dbKey}, nil
}

func (a *EscrowImportActivities) ResolveRegistrars(ctx context.Context, args ResolveRegistrarsArgs) (ResolveRegistrarsResult, error) {
	if args.DBKey == "" {
		return ResolveRegistrarsResult{}, fmt.Errorf("dbKey is required")
	}

	s3c, err := storage.NewS3ClientFromEnv()
	if err != nil {
		return ResolveRegistrarsResult{}, err
	}

	// Idempotency Check
	// Since we modify the DB in place, we use a sidecar manifest to track completion.
	// SKIP when overrides are provided — the caller explicitly wants a re-run
	// with new mappings. The manifest will be overwritten at the end of this run.
	manifestKey := args.DBKey + ".mapping-manifest.json"
	if len(args.Overrides) > 0 {
		activity.GetLogger(ctx).Info("Overrides provided — bypassing idempotency check",
			"manifestKey", manifestKey, "overrideCount", len(args.Overrides))
	} else if exists, _ := s3c.Exists(ctx, manifestKey); exists {
		activity.GetLogger(ctx).Info("Resuming: Registrar mapping manifest found", "manifestKey", manifestKey)
		return ResolveRegistrarsResult{DBKey: args.DBKey, HasIssues: false}, nil
	}

	// Download DB
	dbPath, err := s3c.DownloadToFile(ctx, args.DBKey)
	if err != nil {
		return ResolveRegistrarsResult{}, fmt.Errorf("download db failed: %w", err)
	}
	defer os.Remove(dbPath)

	// Open SQLite
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return ResolveRegistrarsResult{}, fmt.Errorf("open sqlite failed: %w", err)
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
		return ResolveRegistrarsResult{}, fmt.Errorf("create mapping table failed: %w", err)
	}

	// Read Registrars with actual object counts computed from data tables.
	// The registrars table may have stale/missing count columns, so we derive
	// counts by joining against domains, hosts, and contacts directly.
	type regRow struct {
		ClID         string
		Name         string
		GurID        int
		DomainCount  int
		HostCount    int
		ContactCount int
		Email        string
		Voice        string
		URL          string
	}
	var registrars []regRow

	query := `
		SELECT r.ID, r.name, r.gurID,
			COALESCE(dc.cnt, 0) AS domain_count,
			COALESCE(hc.cnt, 0) AS host_count,
			COALESCE(cc.cnt, 0) AS contact_count,
			COALESCE(r.email, '') AS email,
			COALESCE(r.voice, '') AS voice,
			COALESCE(r.url, '')   AS url
		FROM registrars r
		LEFT JOIN (SELECT clID, COUNT(*) AS cnt FROM domains GROUP BY clID) dc ON dc.clID = r.ID
		LEFT JOIN (SELECT clID, COUNT(*) AS cnt FROM hosts GROUP BY clID) hc ON hc.clID = r.ID
		LEFT JOIN (SELECT clID, COUNT(*) AS cnt FROM contacts GROUP BY clID) cc ON cc.clID = r.ID
	`
	rows, err := db.Query(query)
	if err != nil {
		// Fallback: domains/hosts/contacts tables may not exist yet (thin TLD)
		activity.GetLogger(ctx).Warn("Could not query with object counts, falling back", "error", err)
		rows, err = db.Query(`SELECT ID, name, gurID, 0 AS domain_count, 0 AS host_count, 0 AS contact_count, COALESCE(email,'') AS email, COALESCE(voice,'') AS voice, COALESCE(url,'') AS url FROM registrars`)
	}

	if err != nil {
		activity.GetLogger(ctx).Warn("Could not query registrars table", "error", err)
		return ResolveRegistrarsResult{}, temporal.NewNonRetryableApplicationError(
			fmt.Sprintf("failed to query registrars table: %v", err),
			"RegistrarQueryFailed",
			nil,
		)
	}
	defer rows.Close()

	for rows.Next() {
		var r regRow
		if err := rows.Scan(&r.ClID, &r.Name, &r.GurID, &r.DomainCount, &r.HostCount, &r.ContactCount, &r.Email, &r.Voice, &r.URL); err == nil {
			registrars = append(registrars, r)
		}
	}

	// Mapping Logic
	token, err := GetBearerToken()
	if err != nil {
		return ResolveRegistrarsResult{}, err
	}
	client := &http.Client{Timeout: 10 * time.Second}
	baseURL := buildAdminAPIURL()

	activity.GetLogger(ctx).Info("ResolveRegistrars: starting mapping",
		"apiBaseURL", baseURL,
		"overrideCount", len(args.Overrides),
		"tokenPrefix", func() string {
			if len(token) > 15 { return token[:15] + "..." }
			return token
		}(),
	)

	tx, err := db.Begin()
	if err != nil {
		return ResolveRegistrarsResult{}, err
	}

	mappedCount := 0
	var unmapped []UnmappedRegistrar
	var rejected []RejectedOverride

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
				if ok, reason := verifyRegistrarExists(client, baseURL, token, mappedID); !ok {
					activity.GetLogger(ctx).Error("Override registrar verification FAILED",
						"overrideKey", sourceName,
						"targetClID", mappedID,
						"reason", reason,
						"apiURL", baseURL+"/registrars/"+mappedID,
					)
					rejected = append(rejected, RejectedOverride{
						Key:      sourceName,
						TargetID: mappedID,
						Reason:   reason,
					})
					mappedID = "" // Invalid
				} else {
					activity.GetLogger(ctx).Info("Override registrar verified OK",
						"overrideKey", sourceName,
						"targetClID", mappedID,
					)
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

		// Insert mapping (mapped or NULL for unmapped)
		if mappedID != "" {
			_, err := tx.Exec(`INSERT OR REPLACE INTO registrar_mapping (escrow_id, registrar_clid, name, gurid) VALUES (?, ?, ?, ?)`,
				r.ClID, mappedID, r.Name, r.GurID)
			if err != nil {
				activity.GetLogger(ctx).Error("Failed to insert mapping", "err", err)
				return ResolveRegistrarsResult{}, fmt.Errorf("failed to insert mapping: %w", err)
			}
			mappedCount++
		} else {
			// Do NOT insert a row into registrar_mapping for unmapped registrars.
			// The absence of a mapping row tells ApplyRegistrarMappings.copyAndUpdate
			// to preserve the original clID value (via WHERE EXISTS). Inserting a NULL
			// row would cause EXISTS to match and set clID = NULL.

			// Track unmapped registrars that actually manage objects
			if r.DomainCount > 0 || r.HostCount > 0 || r.ContactCount > 0 {
				activity.GetLogger(ctx).Warn("Registrar unmapped", "name", r.Name, "clid", r.ClID, "gurid", r.GurID, "domain_count", r.DomainCount, "host_count", r.HostCount, "contact_count", r.ContactCount)

				// Enrich with postal info from the already-open SQLite DB so the
				// operator doesn't have to look it up manually in the form.
				var suggestedPostal []UnmappedRegistrarPostalInfo
				piRows, piErr := db.Query(
					`SELECT COALESCE(type,'int'), COALESCE(street1,''), COALESCE(city,''), COALESCE(state_province,''), COALESCE(postal_code,''), COALESCE(country_code,'') FROM registrar_postal_info WHERE registrar_id = ?`,
					r.ClID,
				)
				if piErr == nil {
					for piRows.Next() {
						var pi UnmappedRegistrarPostalInfo
						if scanErr := piRows.Scan(&pi.Type, &pi.Street1, &pi.City, &pi.StateProvince, &pi.PostalCode, &pi.CountryCode); scanErr == nil {
							suggestedPostal = append(suggestedPostal, pi)
						}
					}
					piRows.Close()
				}

				unmapped = append(unmapped, UnmappedRegistrar{
					EscrowID:        r.ClID,
					Name:            r.Name,
					GurID:           r.GurID,
					DomainCount:     r.DomainCount,
					HostCount:       r.HostCount,
					ContactCount:    r.ContactCount,
					SuggestedEmail:  r.Email,
					SuggestedVoice:  r.Voice,
					SuggestedURL:    r.URL,
					SuggestedPostal: suggestedPostal,
				})
			} else {
				activity.GetLogger(ctx).Warn("Ignoring unmapped empty registrar", "name", r.Name, "clid", r.ClID, "gurid", r.GurID)
			}
		}
	}

	if err := tx.Commit(); err != nil {
		return ResolveRegistrarsResult{}, fmt.Errorf("commit failed: %w", err)
	}

	// Auto-fix: resolve unmapped registrars that only have hosts (no domains, no contacts)
	// by tracing host→domain_nameservers→domain relationships.
	var hostOnly, remaining []UnmappedRegistrar
	for _, u := range unmapped {
		if u.DomainCount == 0 && u.ContactCount == 0 && u.HostCount > 0 {
			hostOnly = append(hostOnly, u)
		} else {
			remaining = append(remaining, u)
		}
	}

	var autoFixed []AutoFixedRegistrar
	if len(hostOnly) > 0 {
		activity.GetLogger(ctx).Info("Auto-fixing host-only unmapped registrars",
			"count", len(hostOnly), "totalHosts", func() int {
				n := 0
				for _, r := range hostOnly {
					n += r.HostCount
				}
				return n
			}())

		autoFixed, err = autoFixHostOnlyRegistrars(
			log.New(os.Stderr, "[autofix] ", log.LstdFlags), db, hostOnly)
		if err != nil {
			activity.GetLogger(ctx).Error("Auto-fix failed, continuing with gaps", "err", err)
			// Fall back: keep them as unmapped
			remaining = append(remaining, hostOnly...)
		} else {
			activity.GetLogger(ctx).Info("Auto-fix complete",
				"registrarsFixed", len(autoFixed),
				"hostsProcessed", func() int {
					n := 0
					for _, af := range autoFixed {
						n += af.HostsReassigned + af.HostsDuplicated
					}
					return n
				}())
		}
	}

	// Use remaining (non-fixable) as the final unmapped list
	unmapped = remaining

	if len(unmapped) > 0 {
		activity.GetLogger(ctx).Warn("Registrar mapping incomplete — gaps will surface in QA",
			"total", len(registrars), "mapped", mappedCount, "unmapped", len(unmapped))
	} else {
		activity.GetLogger(ctx).Info("Mapped registrars", "total", len(registrars), "mapped", mappedCount,
			"autoFixed", len(autoFixed))
	}

	// Close the db to ensure WAL is checkpointed and flushed to the main db file before uploading
	db.Close()

	// Upload Updated DB
	if err := s3c.UploadFile(ctx, args.DBKey, dbPath, "application/octet-stream"); err != nil {
		return ResolveRegistrarsResult{}, fmt.Errorf("upload db failed: %w", err)
	}

	// Write Manifest to mark completion
	manifest := map[string]any{
		"dbKey":       args.DBKey,
		"completedAt": time.Now().UTC().Format(time.RFC3339),
		"mappedCount": mappedCount,
		"unmapped":    len(unmapped),
		"autoFixed":   len(autoFixed),
	}
	if tmp, err := os.CreateTemp("", "map-manifest-*.json"); err == nil {
		json.NewEncoder(tmp).Encode(manifest)
		tmp.Close()
		s3c.UploadFile(ctx, manifestKey, tmp.Name(), "application/json")
		os.Remove(tmp.Name())
	}

	return ResolveRegistrarsResult{
		DBKey:               args.DBKey,
		HasIssues:           len(unmapped) > 0,
		TotalRegistrars:     len(registrars),
		MappedCount:         mappedCount,
		UnmappedRegistrars:  unmapped,
		AutoFixedRegistrars: autoFixed,
		RejectedOverrides:   rejected,
	}, nil
}

func verifyRegistrarExists(client *http.Client, baseURL, token, id string) (bool, string) {
	url := baseURL + "/registrars/" + id
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return false, fmt.Sprintf("failed to build request: %v", err)
	}
	req.Header.Set("Authorization", token)
	resp, err := client.Do(req)
	if err != nil {
		return false, fmt.Sprintf("HTTP request failed (url=%s): %v", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == 200 {
		return true, ""
	}
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
	return false, fmt.Sprintf("GET %s returned HTTP %d: %s", url, resp.StatusCode, strings.TrimSpace(string(body)))
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

func (a *EscrowImportActivities) ApplyRegistrarMappings(ctx context.Context, args ApplyRegistrarMappingsArgs) (ApplyRegistrarMappingsResult, error) {
	if args.DBKey == "" {
		return ApplyRegistrarMappingsResult{}, fmt.Errorf("dbKey is required")
	}

	s3c, err := storage.NewS3ClientFromEnv()
	if err != nil {
		return ApplyRegistrarMappingsResult{}, err
	}

	// Idempotency: Check if staged DB already exists
	// We derive stagedKey from DBKey logic below
	srcDir := filepath.Dir(args.DBKey)
	srcBase := filepath.Base(args.DBKey)
	stagedBase := "staged-" + srcBase
	stagedKey := srcDir + "/" + stagedBase

	if exists, _ := s3c.Exists(ctx, stagedKey); exists {
		activity.GetLogger(ctx).Info("Resuming: Staged DB found", "key", stagedKey)
		return ApplyRegistrarMappingsResult{StagedDBKey: stagedKey}, nil
	}

	// Download Source DB
	srcPath, err := s3c.DownloadToFile(ctx, args.DBKey)
	if err != nil {
		return ApplyRegistrarMappingsResult{}, fmt.Errorf("download db failed: %w", err)
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
		return ApplyRegistrarMappingsResult{}, fmt.Errorf("open staged db failed: %w", err)
	}
	defer db.Close()

	// Attach Source DB
	if _, err := db.Exec(fmt.Sprintf("ATTACH DATABASE '%s' AS src", srcPath)); err != nil {
		return ApplyRegistrarMappingsResult{}, fmt.Errorf("attach failed: %w", err)
	}

	// Build a pre-normalized mapping lookup table (once).
	// This eliminates the TRIM()/COLLATE NOCASE overhead that was defeating index
	// usage on the PRIMARY KEY. ~500 rows, instant to build.
	if _, err := db.Exec(`
		CREATE TEMP TABLE _mapping AS 
		SELECT TRIM(LOWER(escrow_id)) AS eid, registrar_clid 
		FROM src.registrar_mapping
		WHERE registrar_clid IS NOT NULL AND registrar_clid != ''
	`); err != nil {
		return ApplyRegistrarMappingsResult{}, fmt.Errorf("build mapping lookup failed: %w", err)
	}
	if _, err := db.Exec(`CREATE INDEX _idx_mapping_eid ON _mapping(eid)`); err != nil {
		return ApplyRegistrarMappingsResult{}, fmt.Errorf("index mapping lookup failed: %w", err)
	}

	// stageTable copies a table from src and applies registrar mappings in a single UPDATE.
	// - strictCols (clID): preserved as-is if unmapped (COALESCE fallback)
	// - nullableCols (crRr, upRr): set to NULL if unmapped
	stageTable := func(table string, strictCols []string, nullableCols []string) error {
		_, err := db.Exec(fmt.Sprintf("CREATE TABLE %s AS SELECT * FROM src.%s", table, table))
		if err != nil {
			if strings.Contains(err.Error(), "no such table") {
				return nil
			}
			return err
		}

		// Build a single UPDATE that maps all columns at once.
		var setClauses []string
		for _, col := range strictCols {
			// Strict: preserve original value if no mapping found
			setClauses = append(setClauses, fmt.Sprintf(
				`%s = COALESCE((SELECT registrar_clid FROM _mapping WHERE eid = TRIM(LOWER(%s.%s))), %s.%s)`,
				col, table, col, table, col))
		}
		for _, col := range nullableCols {
			// Nullable: set to NULL if no mapping found
			setClauses = append(setClauses, fmt.Sprintf(
				`%s = (SELECT registrar_clid FROM _mapping WHERE eid = TRIM(LOWER(%s.%s)))`,
				col, table, col))
		}

		if len(setClauses) > 0 {
			query := fmt.Sprintf("UPDATE %s SET %s", table, strings.Join(setClauses, ", "))
			if res, err := db.Exec(query); err != nil {
				activity.GetLogger(ctx).Warn("ApplyRegistrarMappings: UPDATE failed", "table", table, "error", err)
			} else {
				af, _ := res.RowsAffected()
				activity.GetLogger(ctx).Info("ApplyRegistrarMappings: mapped registrar IDs", "table", table, "affected", af)
			}
		}

		// Add clID index on staged table — benefits downstream QA queries
		for _, col := range strictCols {
			db.Exec(fmt.Sprintf("CREATE INDEX IF NOT EXISTS idx_staged_%s_%s ON %s(%s)", table, col, table, col))
		}

		return nil
	}

	// Stage entity tables (single indexed-lookup UPDATE each)
	if err := stageTable("contacts", []string{"clID"}, []string{"crRr", "upRr"}); err != nil {
		return ApplyRegistrarMappingsResult{}, fmt.Errorf("stage contacts failed: %w", err)
	}
	if err := stageTable("hosts", []string{"clID"}, []string{"crRr", "upRr"}); err != nil {
		return ApplyRegistrarMappingsResult{}, fmt.Errorf("stage hosts failed: %w", err)
	}
	if err := stageTable("domains", []string{"clID"}, []string{"crRr", "upRr"}); err != nil {
		return ApplyRegistrarMappingsResult{}, fmt.Errorf("stage domains failed: %w", err)
	}

	// Copy auxiliary tables without updates
	knownTables := []string{"host_addresses", "domain_hosts", "domain_statuses", "contact_statuses", "host_statuses", "domain_nameservers", "domain_rgp_statuses", "nndns", "registrars", "registrar_mapping", "registrar_postal_info"}
	for _, t := range knownTables {
		if _, err := db.Exec(fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s AS SELECT * FROM src.%s", t, t)); err != nil {
			if strings.Contains(err.Error(), "no such table") {
				activity.GetLogger(ctx).Warn("⚠️ ApplyRegistrarMappings: missing aux table in source, skipping", "table", t)
				continue
			}
			return ApplyRegistrarMappingsResult{}, fmt.Errorf("stage aux table %s failed: %w", t, err)
		}
	}

	// Detach
	db.Exec("DETACH DATABASE src")

	// Close the DB to ensure WAL is flushed to the main file
	db.Close()

	// Upload Staged DB
	if err := s3c.UploadFile(ctx, stagedKey, stagedPath, "application/octet-stream"); err != nil {
		return ApplyRegistrarMappingsResult{}, fmt.Errorf("upload staged db failed: %w", err)
	}

	return ApplyRegistrarMappingsResult{StagedDBKey: stagedKey}, nil
}

// IngestContactsArgs parameters
type IngestContactsArgs struct {
	StagedDBKey string
}

// IngestContactsResult outcome
type IngestContactsResult struct {
	Total   int64
	Inserted int64
	Updated  int64
	Skipped  int64 // contacts present in staged DB but excluded (unmapped CLID / RoID failure)
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

	total, inserted, updated, skipped, err := destImporter.ImportContacts(ctx, db, clidMap, lastKey, heartbeat)
	if err != nil {
		return IngestContactsResult{Total: total, Inserted: inserted, Updated: updated, Skipped: skipped}, err
	}

	return IngestContactsResult{Total: total, Inserted: inserted, Updated: updated, Skipped: skipped}, nil
}

// ValidateRegistrantRefsArgs parameters
type ValidateRegistrantRefsArgs struct {
	StagedDBKey string
}

// ValidateRegistrantRefsResult outcome
type ValidateRegistrantRefsResult struct {
	// RegistrantMissing is the count of domain registrant IDs not found in Postgres contacts.
	RegistrantMissing int
	// AdminMissing is the count of domain admin IDs not found in Postgres contacts.
	AdminMissing int
	// TechMissing is the count of domain tech IDs not found in Postgres contacts.
	TechMissing int
	// BillingMissing is the count of domain billing IDs not found in Postgres contacts.
	BillingMissing int
	// TotalMissing is the total count of missing contact references across all roles.
	TotalMissing int
	// SampledMissing contains up to 50 sampled violations for operator triage.
	SampledMissing []MissingContactRef
}

// MissingContactRef describes a single domain contact reference that could not be resolved
// in Postgres after IngestContacts completed.
type MissingContactRef struct {
	Domain    string `json:"domain"`
	Role      string `json:"role"`      // "registrant", "admin", "tech", "billing"
	ContactID string `json:"contactId"` // the ID that is missing from contacts table
}

// ValidateRegistrantRefs cross-checks all domain contact references (registrant, admin, tech,
// billing) in the staged SQLite DB against the live Postgres contacts table. It must run AFTER
// IngestContacts and BEFORE IngestDomains — any missing reference would cause a FK violation.
//
// Failures are NOT retryable: if contacts are absent from Postgres, a retry of this activity
// cannot fix that. The operator must investigate the skipped contacts and re-run the import.
func (a *EscrowImportActivities) ValidateRegistrantRefs(ctx context.Context, args ValidateRegistrantRefsArgs) (ValidateRegistrantRefsResult, error) {
	if args.StagedDBKey == "" {
		return ValidateRegistrantRefsResult{}, temporal.NewNonRetryableApplicationError(
			"ValidateRegistrantRefs: stagedDBKey is required",
			"ValidationError", nil,
		)
	}

	s3c, err := storage.NewS3ClientFromEnv()
	if err != nil {
		return ValidateRegistrantRefsResult{}, fmt.Errorf("ValidateRegistrantRefs: init S3 client: %w", err)
	}

	dbPath, err := s3c.DownloadToFile(ctx, args.StagedDBKey)
	if err != nil {
		return ValidateRegistrantRefsResult{}, fmt.Errorf("ValidateRegistrantRefs: download staged DB (key=%s): %w", args.StagedDBKey, err)
	}
	defer os.Remove(dbPath)

	sqliteDB, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return ValidateRegistrantRefsResult{}, fmt.Errorf("ValidateRegistrantRefs: open staged DB: %w", err)
	}
	defer sqliteDB.Close()

	// Open Postgres connection for live contact lookup.
	destImporter, err := services.NewDirectDBImporter()
	if err != nil {
		return ValidateRegistrantRefsResult{}, fmt.Errorf("ValidateRegistrantRefs: init PG importer: %w", err)
	}
	defer destImporter.PG.Close()

	// Collect all distinct, non-empty contact IDs from the staged DB for each role.
	type roleQuery struct {
		role   string
		column string // column name in the staged SQLite domains table
	}
	roles := []roleQuery{
		{"registrant", "registrant"},
		{"admin", "admin"},
		{"tech", "tech"},
		{"billing", "billing"},
	}

	result := ValidateRegistrantRefsResult{}
	const sampleCap = 50

	for _, rq := range roles {
		// Collect all distinct non-empty contact IDs for this role.
		query := fmt.Sprintf(
			`SELECT DISTINCT TRIM(%s) FROM domains WHERE %s IS NOT NULL AND TRIM(%s) != ''`,
			rq.column, rq.column, rq.column,
		)
		rows, err := sqliteDB.QueryContext(ctx, query)
		if err != nil {
			// If the column doesn't exist in this escrow schema, skip gracefully.
			activity.GetLogger(ctx).Warn("ValidateRegistrantRefs: column query failed, skipping role",
				"role", rq.role, "error", err)
			continue
		}

		var contactIDs []string
		for rows.Next() {
			var id string
			if rows.Scan(&id) == nil && id != "" {
				contactIDs = append(contactIDs, id)
			}
		}
		rows.Close()

		if len(contactIDs) == 0 {
			continue
		}

		// Batch-query Postgres to find which IDs exist.
		// We query in chunks of 1000 to avoid very long IN clauses.
		const chunkSize = 1000
		for i := 0; i < len(contactIDs); i += chunkSize {
			end := i + chunkSize
			if end > len(contactIDs) {
				end = len(contactIDs)
			}
			chunk := contactIDs[i:end]

			// Use go-pg's Model API with pg.In to query which contact IDs from this
			// chunk actually exist in Postgres. We scan into a slice of structs.
			type contactIDRow struct {
				ID string `pg:"id"`
			}
			var found []contactIDRow
			if _, err := destImporter.PG.QueryContext(
				ctx,
				&found,
				`SELECT id FROM contacts WHERE id IN (?)`,
				gopg.In(chunk),
			); err != nil {
				return ValidateRegistrantRefsResult{}, fmt.Errorf(
					"ValidateRegistrantRefs: PG lookup for role=%s chunk=%d: %w — check that Postgres is reachable and the contacts table exists",
					rq.role, i/chunkSize, err,
				)
			}

			foundIDs := make(map[string]bool, len(found))
			for _, row := range found {
				foundIDs[row.ID] = true
			}

			// Any ID in the chunk not found in Postgres is a violation.
			for _, id := range chunk {
				if !foundIDs[id] {
					switch rq.role {
					case "registrant":
						result.RegistrantMissing++
					case "admin":
						result.AdminMissing++
					case "tech":
						result.TechMissing++
					case "billing":
						result.BillingMissing++
					}
					result.TotalMissing++

					// Sample up to sampleCap violations for operator triage.
					if len(result.SampledMissing) < sampleCap {
						// Find a domain referencing this contact ID for context.
						var domainName string
						sampleQ := fmt.Sprintf(
							`SELECT name FROM domains WHERE TRIM(%s) = ? LIMIT 1`,
							rq.column,
						)
						_ = sqliteDB.QueryRowContext(ctx, sampleQ, id).Scan(&domainName)
						result.SampledMissing = append(result.SampledMissing, MissingContactRef{
							Domain:    domainName,
							Role:      rq.role,
							ContactID: id,
						})
					}
				}
			}
		}

		activity.GetLogger(ctx).Info("ValidateRegistrantRefs: role checked",
			"role", rq.role,
			"totalRefsChecked", len(contactIDs),
		)
	}

	if result.TotalMissing > 0 {
		// Non-retryable: missing contacts cannot appear in Postgres by retrying this activity.
		// The operator must investigate why IngestContacts skipped these contacts (check for
		// unmapped CLIDs in registrar_mapping, or CLID override mismatches), fix the mapping,
		// and re-run the import from scratch.
		return result, temporal.NewNonRetryableApplicationError(
			fmt.Sprintf(
				"ValidateRegistrantRefs: %d domain contact references are missing from Postgres contacts table "+
					"(registrant=%d, admin=%d, tech=%d, billing=%d). "+
					"These contacts were likely skipped during IngestContacts due to unmapped CLIDs. "+
					"Check registrar_mapping entries and registrar overrides, then re-run the import. "+
					"See SampledMissing in the result for up to %d examples.",
				result.TotalMissing,
				result.RegistrantMissing, result.AdminMissing, result.TechMissing, result.BillingMissing,
				sampleCap,
			),
			"MissingContactRefs", nil,
		)
	}

	activity.GetLogger(ctx).Info("ValidateRegistrantRefs: all domain contact references resolved in Postgres",
		"rolesChecked", len(roles),
	)
	return result, nil
}


// IngestHostsArgs parameters
type IngestHostsArgs struct {
	StagedDBKey string
}

type IngestHostsResult struct {
	Total    int64
	Inserted int64
	Updated  int64
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
	total, inserted, updated, err := destImporter.ImportHosts(ctx, db, clidMap, lastKey, heartbeat)
	if err != nil {
		return IngestHostsResult{Total: total, Inserted: inserted, Updated: updated}, err
	}

	return IngestHostsResult{Total: total, Inserted: inserted, Updated: updated}, nil
}

// IngestDomainsArgs parameters
type IngestDomainsArgs struct {
	StagedDBKey string
	TLD         string
}

type IngestDomainsResult struct {
	Total    int64
	Inserted int64
	Updated  int64
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
	total, inserted, updated, err := destImporter.ImportDomains(ctx, db, args.TLD, clidMap, lastKey, heartbeat)
	if err != nil {
		return IngestDomainsResult{Total: total, Inserted: inserted, Updated: updated}, err
	}

	return IngestDomainsResult{Total: total, Inserted: inserted, Updated: updated}, nil
}

// IngestNNDNsArgs parameters
type IngestNNDNsArgs struct {
	StagedDBKey string
	TLD         string
}

// IngestNNDNsResult outcome
type IngestNNDNsResult struct {
	Total    int64
	Inserted int64
	Updated  int64
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

	total, inserted, updated, err := destImporter.ImportNNDNs(ctx, db, args.TLD, lastKey, heartbeat)
	if err != nil {
		return IngestNNDNsResult{Total: total, Inserted: inserted, Updated: updated}, err
	}

	return IngestNNDNsResult{Total: total, Inserted: inserted, Updated: updated}, nil
}

// LinkDomainHostsArgs parameters
type LinkDomainHostsArgs struct {
	StagedDBKey string
	TLD         string
}

// LinkDomainHostsResult outcome
type LinkDomainHostsResult struct {
	Total    int64
	Inserted int64
	Cleaned  int64 // stale links removed before re-linking
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

	// Clean up stale domain_hosts links for this TLD before re-linking.
	// Only on first attempt (no resume cursor) — the escrow is the source of truth.
	var cleaned int64
	if lastKey == "" && args.TLD != "" {
		res, err := destImporter.PG.Exec(`
			DELETE FROM domain_hosts
			WHERE domain_ro_id IN (
				SELECT ro_id FROM domains WHERE tld_name = ?
			)
		`, args.TLD)
		if err != nil {
			activity.GetLogger(ctx).Warn("LinkDomainHosts: failed to clean stale links (non-fatal)", "tld", args.TLD, "error", err)
		} else {
			cleaned = int64(res.RowsAffected())
			activity.GetLogger(ctx).Info("LinkDomainHosts: cleaned stale links", "tld", args.TLD, "removed", cleaned)
		}
	}

	total, linked, err := destImporter.LinkDomainHosts(ctx, db, lastKey, heartbeat)
	if err != nil {
		return LinkDomainHostsResult{Total: total, Inserted: linked, Cleaned: cleaned}, err
	}

	return LinkDomainHostsResult{Total: total, Inserted: linked, Cleaned: cleaned}, nil
}

// AccreditRegistrarsArgs parameters
type AccreditRegistrarsArgs struct {
	StagedDBKey string
	TLD         string
}

// AccreditRegistrarsResult outcome
type AccreditRegistrarsResult struct {
	Total int64
}

// AccreditRegistrars accredits registrars based on the escrow file
func (a *EscrowImportActivities) AccreditRegistrars(ctx context.Context, args AccreditRegistrarsArgs) (AccreditRegistrarsResult, error) {
	s3c, err := storage.NewS3ClientFromEnv()
	if err != nil {
		return AccreditRegistrarsResult{}, err
	}

	// Resumability: Check if we have a heartbeat detail to resume from
	// actually we don't have lastKey logic in the new importer method, it just retries all because it's idempotent

	// Download Staged DB
	dbPath, err := s3c.DownloadToFile(ctx, args.StagedDBKey)
	if err != nil {
		return AccreditRegistrarsResult{}, fmt.Errorf("download db failed: %w", err)
	}
	defer os.Remove(dbPath)

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return AccreditRegistrarsResult{}, fmt.Errorf("open sqlite failed: %w", err)
	}
	defer db.Close()

	destImporter, err := services.NewDirectDBImporter()
	if err != nil {
		return AccreditRegistrarsResult{}, fmt.Errorf("failed to init importer: %w", err)
	}
	defer destImporter.PG.Close()

	heartbeat := func(processed string) {
		activity.RecordHeartbeat(ctx, processed)
	}

	total, err := destImporter.AccreditRegistrars(ctx, db, args.TLD, heartbeat)
	if err != nil {
		return AccreditRegistrarsResult{Total: total}, err
	}

	return AccreditRegistrarsResult{Total: total}, nil
}

// --- QA Staged Database ---

// QACheck represents a single quality check result
type QACheck struct {
	Rule          string      `json:"rule"`
	Description   string      `json:"description"`
	Severity      string      `json:"severity"` // "error", "warning", "info"
	Passed        bool        `json:"passed"`
	AffectedCount int         `json:"affectedCount"`
	Message       string      `json:"message"`
	Detail        interface{} `json:"detail,omitempty"`
	SampledItems  interface{} `json:"sampledItems,omitempty"`
}

// QAReport is the structured QA report for a staged database
type QAReport struct {
	Version   string            `json:"version"`
	Timestamp time.Time         `json:"timestamp"`
	Pipeline  string            `json:"pipeline"`
	Context   map[string]string `json:"context"`
	SourceKey string            `json:"sourceKey"`
	Passed    bool              `json:"passed"`
	Summary   map[string]int64  `json:"summary"`
	Checks    []QACheck         `json:"checks"`
}

// CleanOrphanedContactsArgs input for the orphan cleanup activity
type CleanOrphanedContactsArgs struct {
	TLD       string
	DBKey     string
	RunPrefix string
}

// CleanedRegistrar records what was cleaned for a specific dead registrar
type CleanedRegistrar struct {
	EscrowID     string `json:"escrowId"`
	Name         string `json:"name"`
	Reassigned   int    `json:"reassigned"`   // Contacts reassigned to domain's registrar
	Deleted      int    `json:"deleted"`       // Contacts deleted (unreferenced)
}

// CleanOrphanedContactsResult output of the orphan cleanup activity
type CleanOrphanedContactsResult struct {
	DeletedContacts    int                `json:"deletedContacts"`
	ReassignedContacts int                `json:"reassignedContacts"`
	DeletedStatuses    int                `json:"deletedStatuses"`
	CleanedRegistrars  []CleanedRegistrar `json:"cleanedRegistrars,omitempty"`
	ReportKey          string             `json:"reportKey,omitempty"`
}

// CleanOrphanedContacts identifies registrars in the escrow that have zero domains
// and zero hosts (dead/terminated registrars) and cleans up their contact objects:
//   - Contacts referenced by domains as registrant: reassigned to the domain's registrar
//   - Contacts not referenced by any domain: deleted
//
// This runs on the BASE DB before ResolveRegistrars, ensuring dead registrars
// have zero objects and won't appear as unmapped.
func (a *EscrowImportActivities) CleanOrphanedContacts(ctx context.Context, args CleanOrphanedContactsArgs) (CleanOrphanedContactsResult, error) {
	if args.DBKey == "" {
		return CleanOrphanedContactsResult{}, fmt.Errorf("dbKey is required")
	}

	s3c, err := storage.NewS3ClientFromEnv()
	if err != nil {
		return CleanOrphanedContactsResult{}, err
	}

	dbPath, err := s3c.DownloadToFile(ctx, args.DBKey)
	if err != nil {
		return CleanOrphanedContactsResult{}, fmt.Errorf("download db failed: %w", err)
	}
	defer os.Remove(dbPath)

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return CleanOrphanedContactsResult{}, fmt.Errorf("open db failed: %w", err)
	}
	defer db.Close()

	// Identify dead registrars: registrars with 0 domains AND 0 hosts.
	// These registrars only have contacts (which may or may not be referenced by domains
	// from other registrars).
	deadRows, err := db.QueryContext(ctx, `
		SELECT r.ID, r.name
		FROM registrars r
		WHERE r.ID NOT IN (SELECT DISTINCT clID FROM domains WHERE clID IS NOT NULL)
		  AND r.ID NOT IN (SELECT DISTINCT clID FROM hosts WHERE clID IS NOT NULL)
		  AND r.ID IN (SELECT DISTINCT clID FROM contacts WHERE clID IS NOT NULL)
	`)
	if err != nil {
		return CleanOrphanedContactsResult{}, fmt.Errorf("query dead registrars: %w", err)
	}

	type deadReg struct{ ID, Name string }
	var deadRegs []deadReg
	for deadRows.Next() {
		var d deadReg
		if err := deadRows.Scan(&d.ID, &d.Name); err != nil {
			deadRows.Close()
			return CleanOrphanedContactsResult{}, fmt.Errorf("scan dead registrar: %w", err)
		}
		deadRegs = append(deadRegs, d)
	}
	deadRows.Close()

	if len(deadRegs) == 0 {
		activity.GetLogger(ctx).Info("No dead registrars with orphaned contacts found")
		return CleanOrphanedContactsResult{}, nil
	}

	activity.GetLogger(ctx).Info("Found dead registrars with contact-only objects",
		"count", len(deadRegs))

	var result CleanOrphanedContactsResult

	for _, dr := range deadRegs {
		var cleaned CleanedRegistrar
		cleaned.EscrowID = dr.ID
		cleaned.Name = dr.Name

		// Reassign contacts from this dead registrar that ARE referenced by domains
		reassignRes, err := db.ExecContext(ctx, `
			UPDATE contacts SET clID = (
				SELECT d.clID FROM domains d WHERE d.registrant = contacts.id LIMIT 1
			)
			WHERE contacts.clID = ?
			  AND contacts.id IN (SELECT registrant FROM domains WHERE registrant IS NOT NULL)
		`, dr.ID)
		if err != nil {
			activity.GetLogger(ctx).Warn("Failed to reassign contacts", "registrar", dr.ID, "error", err)
		} else if af, _ := reassignRes.RowsAffected(); af > 0 {
			cleaned.Reassigned = int(af)
			result.ReassignedContacts += int(af)
		}

		// Delete contacts from this dead registrar that are NOT referenced by any domain
		deleteRes, err := db.ExecContext(ctx, `
			DELETE FROM contacts
			WHERE clID = ?
			  AND id NOT IN (SELECT registrant FROM domains WHERE registrant IS NOT NULL)
		`, dr.ID)
		if err != nil {
			activity.GetLogger(ctx).Warn("Failed to delete orphaned contacts", "registrar", dr.ID, "error", err)
		} else if af, _ := deleteRes.RowsAffected(); af > 0 {
			cleaned.Deleted = int(af)
			result.DeletedContacts += int(af)
		}

		if cleaned.Reassigned > 0 || cleaned.Deleted > 0 {
			result.CleanedRegistrars = append(result.CleanedRegistrars, cleaned)
			activity.GetLogger(ctx).Info("Cleaned dead registrar",
				"id", dr.ID, "name", dr.Name,
				"reassigned", cleaned.Reassigned, "deleted", cleaned.Deleted)
		}
	}

	// Clean up contact_statuses for deleted contacts
	statusRes, err := db.ExecContext(ctx, `
		DELETE FROM contact_statuses
		WHERE contactID NOT IN (SELECT id FROM contacts)
	`)
	if err != nil {
		activity.GetLogger(ctx).Warn("Failed to clean orphaned contact_statuses", "error", err)
	} else if af, _ := statusRes.RowsAffected(); af > 0 {
		result.DeletedStatuses = int(af)
	}

	activity.GetLogger(ctx).Info("CleanOrphanedContacts complete",
		"deadRegistrars", len(deadRegs),
		"reassigned", result.ReassignedContacts,
		"deleted", result.DeletedContacts,
		"statusesDeleted", result.DeletedStatuses,
	)

	// Upload cleanup report as a separate artifact
	reportKey := args.RunPrefix + "/cleanup-report.json"
	reportData, _ := json.MarshalIndent(result, "", "  ")
	if tmp, err := os.CreateTemp("", "cleanup-report-*.json"); err == nil {
		tmp.Write(reportData)
		tmp.Close()
		if err := s3c.UploadFile(ctx, reportKey, tmp.Name(), "application/json"); err != nil {
			activity.GetLogger(ctx).Warn("Failed to upload cleanup report", "error", err)
		}
		os.Remove(tmp.Name())
	}
	result.ReportKey = reportKey

	// Close DB to flush WAL before uploading
	db.Close()

	// Re-upload the cleaned base DB
	if err := s3c.UploadFile(ctx, args.DBKey, dbPath, "application/octet-stream"); err != nil {
		return CleanOrphanedContactsResult{}, fmt.Errorf("re-upload db failed: %w", err)
	}

	return result, nil
}

// AddCheck adds a check to the report and updates the overall passed status
func (r *QAReport) AddCheck(check QACheck) {
	r.Checks = append(r.Checks, check)
	if !check.Passed && check.Severity == "error" {
		r.Passed = false
	}
}


// QAStagedDatabaseArgs input for the QA activity
type QAStagedDatabaseArgs struct {
	TLD         string
	StagedDBKey string
	RunPrefix   string
}

// QAStagedDatabaseResult output of the QA activity
type QAStagedDatabaseResult struct {
	Passed      bool   `json:"passed"`
	QAReportKey string `json:"qaReportKey"`
}

// QAStagedDatabase validates the staged database and produces a QA report
func (a *EscrowImportActivities) QAStagedDatabase(ctx context.Context, args QAStagedDatabaseArgs) (QAStagedDatabaseResult, error) {
	if args.StagedDBKey == "" {
		return QAStagedDatabaseResult{}, fmt.Errorf("stagedDBKey is required")
	}

	s3c, err := storage.NewS3ClientFromEnv()
	if err != nil {
		return QAStagedDatabaseResult{}, err
	}

	// Download Staged DB
	dbPath, err := s3c.DownloadToFile(ctx, args.StagedDBKey)
	if err != nil {
		return QAStagedDatabaseResult{}, fmt.Errorf("download staged db failed: %w", err)
	}
	defer os.Remove(dbPath)
	defer os.Remove(dbPath + "-wal")
	defer os.Remove(dbPath + "-shm")

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return QAStagedDatabaseResult{}, fmt.Errorf("open sqlite failed: %w", err)
	}
	defer db.Close()

	report := &QAReport{
		Version:   "1.0",
		Timestamp: time.Now().UTC(),
		Pipeline:  "escrow-staging",
		Context: map[string]string{
			"tld":       args.TLD,
			"runPrefix": args.RunPrefix,
		},
		SourceKey: args.StagedDBKey,
		Passed:    true, // Optimistic — AddCheck flips to false on error-severity failures
		Summary:   make(map[string]int64),
		Checks:    []QACheck{},
	}

	// --- Collect counts for summary ---
	countTable := func(table string) int64 {
		var c int64
		if err := db.QueryRow(fmt.Sprintf("SELECT COUNT(*) FROM %s", table)).Scan(&c); err != nil {
			return 0
		}
		return c
	}

	report.Summary["contacts"] = countTable("contacts")
	report.Summary["hosts"] = countTable("hosts")
	report.Summary["domains"] = countTable("domains")
	report.Summary["nndns"] = countTable("nndns")
	report.Summary["domain_hosts"] = countTable("domain_nameservers")
	report.Summary["registrar_mappings"] = countTable("registrar_mapping")

	// --- Check 1: Unmapped primary CLIDs ---
	// Any clID in contacts/hosts/domains that doesn't match a registrar_clid in registrar_mapping
	{
		query := `
			SELECT COUNT(*) FROM (
				SELECT clID FROM contacts WHERE clID IS NOT NULL AND clID != '' AND TRIM(clID) NOT IN (SELECT TRIM(registrar_clid) FROM registrar_mapping WHERE registrar_clid IS NOT NULL)
				UNION ALL
				SELECT clID FROM hosts WHERE clID IS NOT NULL AND clID != '' AND TRIM(clID) NOT IN (SELECT TRIM(registrar_clid) FROM registrar_mapping WHERE registrar_clid IS NOT NULL)
				UNION ALL
				SELECT clID FROM domains WHERE clID IS NOT NULL AND clID != '' AND TRIM(clID) NOT IN (SELECT TRIM(registrar_clid) FROM registrar_mapping WHERE registrar_clid IS NOT NULL)
			)`
		var count int
		if err := db.QueryRow(query).Scan(&count); err != nil {
			activity.GetLogger(ctx).Warn("QA: unmapped_primary_clids query failed", "error", err)
			count = -1
		}

		check := QACheck{
			Rule:          "unmapped_primary_clids",
			Description:   "Every clID in contacts, hosts, and domains must resolve to a registrar_mapping entry",
			Severity:      "error",
			Passed:        count == 0,
			AffectedCount: count,
		}
		if count == 0 {
			total := report.Summary["contacts"] + report.Summary["hosts"] + report.Summary["domains"]
			check.Message = fmt.Sprintf("All %d primary CLIDs are mapped", total)
		} else if count < 0 {
			check.Message = "Query failed — unable to verify"
			check.Passed = false
		} else {
			check.Message = fmt.Sprintf("%d primary CLIDs are not mapped to any registrar", count)
			// Sample up to 50
			sampleQuery := `
				SELECT 'contacts' as entity, clID FROM contacts WHERE clID IS NOT NULL AND clID != '' AND TRIM(clID) NOT IN (SELECT TRIM(registrar_clid) FROM registrar_mapping WHERE registrar_clid IS NOT NULL)
				UNION ALL
				SELECT 'hosts', clID FROM hosts WHERE clID IS NOT NULL AND clID != '' AND TRIM(clID) NOT IN (SELECT TRIM(registrar_clid) FROM registrar_mapping WHERE registrar_clid IS NOT NULL)
				UNION ALL
				SELECT 'domains', clID FROM domains WHERE clID IS NOT NULL AND clID != '' AND TRIM(clID) NOT IN (SELECT TRIM(registrar_clid) FROM registrar_mapping WHERE registrar_clid IS NOT NULL)
				LIMIT 50`
			if rows, err := db.Query(sampleQuery); err == nil {
				var samples []map[string]string
				for rows.Next() {
					var entity, clid string
					if rows.Scan(&entity, &clid) == nil {
						samples = append(samples, map[string]string{"entity": entity, "clid": clid})
					}
				}
				rows.Close()
				check.SampledItems = samples
			}
		}
		report.AddCheck(check)
	}

	// --- Check 2: NULL primary CLIDs ---
	{
		query := `
			SELECT COUNT(*) FROM (
				SELECT 1 FROM contacts WHERE clID IS NULL OR TRIM(clID) = ''
				UNION ALL
				SELECT 1 FROM hosts WHERE clID IS NULL OR TRIM(clID) = ''
				UNION ALL
				SELECT 1 FROM domains WHERE clID IS NULL OR TRIM(clID) = ''
			)`
		var count int
		if err := db.QueryRow(query).Scan(&count); err != nil {
			count = -1
		}
		check := QACheck{
			Rule:          "null_primary_clids",
			Description:   "No contact, host, or domain should have a NULL or empty clID after staging",
			Severity:      "error",
			Passed:        count == 0,
			AffectedCount: count,
		}
		if count == 0 {
			check.Message = "0 NULL primary CLIDs"
		} else if count < 0 {
			check.Message = "Query failed — unable to verify"
			check.Passed = false
		} else {
			check.Message = fmt.Sprintf("%d records have NULL or empty clID", count)
			// Sample failing records for troubleshooting
			sampleQuery := `
				SELECT 'contact' as entity, id as identifier FROM contacts WHERE clID IS NULL OR TRIM(clID) = ''
				UNION ALL
				SELECT 'host', name FROM hosts WHERE clID IS NULL OR TRIM(clID) = ''
				UNION ALL
				SELECT 'domain', name FROM domains WHERE clID IS NULL OR TRIM(clID) = ''
				LIMIT 50`
			if rows, err := db.Query(sampleQuery); err == nil {
				defer rows.Close()
				var samples []map[string]string
				for rows.Next() {
					var entity, identifier string
					if rows.Scan(&entity, &identifier) == nil {
						samples = append(samples, map[string]string{"entity": entity, "identifier": identifier})
					}
				}
				check.SampledItems = samples
			}
		}
		report.AddCheck(check)
	}

	// --- Check 3: Registrar mapping completeness ---
	// Every distinct CLID value across all tables exists in registrar_mapping
	{
		query := `
			SELECT COUNT(DISTINCT clid) FROM (
				SELECT TRIM(clID) as clid FROM contacts WHERE clID IS NOT NULL AND clID != ''
				UNION
				SELECT TRIM(clID) FROM hosts WHERE clID IS NOT NULL AND clID != ''
				UNION
				SELECT TRIM(clID) FROM domains WHERE clID IS NOT NULL AND clID != ''
			) WHERE clid NOT IN (SELECT TRIM(registrar_clid) FROM registrar_mapping WHERE registrar_clid IS NOT NULL)`
		var count int
		if err := db.QueryRow(query).Scan(&count); err != nil {
			count = -1
		}
		// Count total distinct CLIDs
		var totalDistinct int
		db.QueryRow(`SELECT COUNT(DISTINCT clid) FROM (
			SELECT TRIM(clID) as clid FROM contacts WHERE clID IS NOT NULL AND clID != ''
			UNION
			SELECT TRIM(clID) FROM hosts WHERE clID IS NOT NULL AND clID != ''
			UNION
			SELECT TRIM(clID) FROM domains WHERE clID IS NOT NULL AND clID != ''
		)`).Scan(&totalDistinct)

		check := QACheck{
			Rule:          "registrar_mapping_completeness",
			Description:   "Every distinct CLID value appearing anywhere in the staged data exists in registrar_mapping",
			Severity:      "error",
			Passed:        count == 0,
			AffectedCount: count,
		}
		if count == 0 {
			check.Message = fmt.Sprintf("All %d distinct CLIDs are mapped", totalDistinct)
		} else {
			check.Message = fmt.Sprintf("%d distinct CLIDs are unmapped (of %d total)", count, totalDistinct)
			// Sample unmapped CLIDs for troubleshooting
			sampleQuery := `
				SELECT DISTINCT clid FROM (
					SELECT TRIM(clID) as clid FROM contacts WHERE clID IS NOT NULL AND clID != ''
					UNION
					SELECT TRIM(clID) FROM hosts WHERE clID IS NOT NULL AND clID != ''
					UNION
					SELECT TRIM(clID) FROM domains WHERE clID IS NOT NULL AND clID != ''
				) WHERE clid NOT IN (SELECT TRIM(registrar_clid) FROM registrar_mapping WHERE registrar_clid IS NOT NULL)
				LIMIT 50`
			if rows, err := db.Query(sampleQuery); err == nil {
				defer rows.Close()
				var samples []map[string]string
				for rows.Next() {
					var clid string
					if rows.Scan(&clid) == nil {
						samples = append(samples, map[string]string{"unmappedClid": clid})
					}
				}
				check.SampledItems = samples
			}
		}
		report.AddCheck(check)
	}

	// --- Check 4: Entity count consistency ---
	// Cross-check staged counts with registrar_mapping domain_count sums
	{
		var mappingDomainSum int64
		db.QueryRow(`SELECT COALESCE(SUM(domain_count), 0) FROM registrars WHERE domain_count IS NOT NULL`).Scan(&mappingDomainSum)

		stagedDomains := report.Summary["domains"]
		delta := stagedDomains - mappingDomainSum
		if delta < 0 {
			delta = -delta
		}

		// Tolerance: within 1% or exact match
		tolerance := int64(float64(mappingDomainSum) * 0.01)
		if tolerance < 1 {
			tolerance = 1
		}
		passed := delta <= tolerance || mappingDomainSum == 0 // Skip check if no source counts

		check := QACheck{
			Rule:          "entity_count_consistency",
			Description:   "Entity counts in staged DB are consistent with source analysis counts",
			Severity:      "warning",
			Passed:        passed,
			AffectedCount: int(delta),
			Detail: map[string]int64{
				"staged_domains":  stagedDomains,
				"source_domains":  mappingDomainSum,
				"delta":           delta,
				"staged_contacts": report.Summary["contacts"],
				"staged_hosts":    report.Summary["hosts"],
			},
		}
		if passed {
			check.Message = fmt.Sprintf("Domain count consistent: %d staged (source: %d)", stagedDomains, mappingDomainSum)
		} else {
			check.Message = fmt.Sprintf("Domain count mismatch: %d staged vs %d from source (delta: %d)", stagedDomains, mappingDomainSum, delta)
		}
		report.AddCheck(check)
	}

	// --- Check 5: Referential contacts ---
	// Domains referencing registrant IDs that don't exist in contacts table
	{
		query := `SELECT COUNT(*) FROM domains WHERE registrant IS NOT NULL AND TRIM(registrant) != '' AND TRIM(registrant) NOT IN (SELECT TRIM(id) FROM contacts)`
		var count int
		if err := db.QueryRow(query).Scan(&count); err != nil {
			count = 0 // Non-fatal if column doesn't exist
		}
		check := QACheck{
			Rule:          "referential_contacts",
			Description:   "Domain registrant IDs reference contacts that exist in the contacts table",
			Severity:      "error", // FK violation if contacts are missing; always blocks ingestion
			Passed:        count == 0,
			AffectedCount: count,
		}
		if count == 0 {
			check.Message = "All domain contact references are valid"
		} else {
			check.Message = fmt.Sprintf("%d domains reference contacts not in the contacts table", count)
			// Sample
			sampleQuery := `SELECT name, registrant FROM domains WHERE registrant IS NOT NULL AND TRIM(registrant) != '' AND TRIM(registrant) NOT IN (SELECT TRIM(id) FROM contacts) LIMIT 50`
			if rows, err := db.Query(sampleQuery); err == nil {
				var samples []map[string]string
				for rows.Next() {
					var domain, registrant string
					if rows.Scan(&domain, &registrant) == nil {
						samples = append(samples, map[string]string{"domain": domain, "missingContactId": registrant})
					}
				}
				rows.Close()
				check.SampledItems = samples
			}
		}
		report.AddCheck(check)
	}

	// --- Check 6: Referential hosts ---
	// domain_nameservers referencing hosts not in the hosts table
	{
		query := `SELECT COUNT(*) FROM domain_nameservers WHERE TRIM(nameserver) NOT IN (SELECT TRIM(name) FROM hosts)`
		var count int
		if err := db.QueryRow(query).Scan(&count); err != nil {
			count = 0 // Non-fatal if table doesn't exist
		}
		totalNS := report.Summary["domain_hosts"]
		check := QACheck{
			Rule:          "referential_hosts",
			Description:   "domain_nameservers entries reference hosts that exist in the hosts table",
			Severity:      "warning",
			Passed:        count == 0,
			AffectedCount: count,
		}
		if count == 0 {
			check.Message = fmt.Sprintf("All %d nameserver references are valid", totalNS)
		} else {
			check.Message = fmt.Sprintf("%d nameserver references point to missing hosts", count)
			sampleQuery := `SELECT domain_name, nameserver FROM domain_nameservers WHERE TRIM(nameserver) NOT IN (SELECT TRIM(name) FROM hosts) LIMIT 50`
			if rows, err := db.Query(sampleQuery); err == nil {
				var samples []map[string]string
				for rows.Next() {
					var domain, ns string
					if rows.Scan(&domain, &ns) == nil {
						samples = append(samples, map[string]string{"domain": domain, "missingHost": ns})
					}
				}
				rows.Close()
				check.SampledItems = samples
			}
		}
		report.AddCheck(check)
	}

	// --- Check 7a: Expiry date far in the future (warning) ---
	{
		now := time.Now().UTC()
		futureDate := now.AddDate(10, 0, 0).Format("2006-01-02")

		var count int
		query := fmt.Sprintf(`SELECT COUNT(*) FROM domains WHERE exdate IS NOT NULL AND exdate != '' AND exdate > '%s'`, futureDate)
		if err := db.QueryRow(query).Scan(&count); err != nil {
			count = 0 // Non-fatal
		}
		check := QACheck{
			Rule:          "expiry_date_far_future",
			Description:   "Domain expiry dates should not be more than 10 years in the future",
			Severity:      "warning",
			Passed:        count == 0,
			AffectedCount: count,
		}
		if count == 0 {
			check.Message = "No domains have expiry dates more than 10 years in the future"
		} else {
			check.Message = fmt.Sprintf("%d domains have expiry dates more than 10 years in the future", count)
			sampleQuery := fmt.Sprintf(`SELECT name, exdate FROM domains WHERE exdate IS NOT NULL AND exdate != '' AND exdate > '%s' ORDER BY exdate DESC LIMIT 50`, futureDate)
			if rows, err := db.Query(sampleQuery); err == nil {
				var samples []map[string]string
				for rows.Next() {
					var domain, exdate string
					if rows.Scan(&domain, &exdate) == nil {
						samples = append(samples, map[string]string{"domain": domain, "expiryDate": exdate})
					}
				}
				rows.Close()
				check.SampledItems = samples
			}
		}
		report.AddCheck(check)
	}

	// --- Check 7b: Expiry dates in the past (info) ---
	{
		now := time.Now().UTC()
		pastDate := now.Format("2006-01-02")

		var count int
		query := fmt.Sprintf(`SELECT COUNT(*) FROM domains WHERE exdate IS NOT NULL AND exdate != '' AND exdate < '%s'`, pastDate)
		if err := db.QueryRow(query).Scan(&count); err != nil {
			count = 0 // Non-fatal
		}
		check := QACheck{
			Rule:          "expiry_date_in_past",
			Description:   "Domains whose expiry date has already passed — expected for recently-expired domains pending deletion",
			Severity:      "info",
			Passed:        true, // past expiry dates are informational, not a failure
			AffectedCount: count,
		}
		if count == 0 {
			check.Message = "No domains have expiry dates in the past"
		} else {
			check.Message = fmt.Sprintf("%d domains have expiry dates in the past", count)
			sampleQuery := fmt.Sprintf(`SELECT name, exdate FROM domains WHERE exdate IS NOT NULL AND exdate != '' AND exdate < '%s' ORDER BY exdate ASC LIMIT 50`, pastDate)
			if rows, err := db.Query(sampleQuery); err == nil {
				var samples []map[string]string
				for rows.Next() {
					var domain, exdate string
					if rows.Scan(&domain, &exdate) == nil {
						samples = append(samples, map[string]string{"domain": domain, "expiryDate": exdate})
					}
				}
				rows.Close()
				check.SampledItems = samples
			}
		}
		report.AddCheck(check)
	}

	// --- Serialize and upload QA report ---
	reportJSON, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return QAStagedDatabaseResult{}, fmt.Errorf("marshal qa report failed: %w", err)
	}

	qaReportKey := filepath.Dir(args.StagedDBKey) + "/qa-report.json"
	tmpFile, err := os.CreateTemp("", "qa-report-*.json")
	if err != nil {
		return QAStagedDatabaseResult{}, fmt.Errorf("create temp file failed: %w", err)
	}
	tmpFile.Write(reportJSON)
	tmpFile.Close()
	defer os.Remove(tmpFile.Name())

	if err := s3c.UploadFile(ctx, qaReportKey, tmpFile.Name(), "application/json"); err != nil {
		return QAStagedDatabaseResult{}, fmt.Errorf("upload qa report failed: %w", err)
	}

	activity.GetLogger(ctx).Info("QA Staged Database complete",
		"passed", report.Passed,
		"checks_run", len(report.Checks),
		"qa_report_key", qaReportKey,
	)

	return QAStagedDatabaseResult{
		Passed:      report.Passed,
		QAReportKey: qaReportKey,
	}, nil
}

type PersistImportSummaryArgs struct {
	TLD            string           `json:"tld"`
	RunPrefix      string           `json:"runPrefix"`
	WorkflowID     string           `json:"workflowId"`
	QAPassed       bool             `json:"qaPassed"`
	QAReportKey    string           `json:"qaReportKey"`
	IngestedCounts map[string]int64 `json:"ingestedCounts"`
}

type PersistImportSummaryResult struct {
	SummaryKey string `json:"summaryKey"`
}

func (a *EscrowImportActivities) PersistImportSummary(ctx context.Context, args PersistImportSummaryArgs) (PersistImportSummaryResult, error) {
	if strings.TrimSpace(args.TLD) == "" || strings.TrimSpace(args.RunPrefix) == "" {
		return PersistImportSummaryResult{}, fmt.Errorf("tld and runPrefix are required")
	}

	payload := map[string]any{
		"tld":            args.TLD,
		"runPrefix":      args.RunPrefix,
		"workflowId":     args.WorkflowID,
		"completedAt":    time.Now().UTC().Format(time.RFC3339),
		"qaPassed":       args.QAPassed,
		"qaReportKey":    args.QAReportKey,
		"ingestedCounts": args.IngestedCounts,
	}

	tmp, err := os.CreateTemp("", "escrow-summary-*.json")
	if err != nil {
		return PersistImportSummaryResult{}, fmt.Errorf("create temp file failed: %w", err)
	}
	defer os.Remove(tmp.Name())

	enc := json.NewEncoder(tmp)
	enc.SetIndent("", "  ")
	if err := enc.Encode(payload); err != nil {
		tmp.Close()
		return PersistImportSummaryResult{}, fmt.Errorf("encode summary JSON failed: %w", err)
	}
	tmp.Close()

	s3c, err := storage.NewS3ClientFromEnv()
	if err != nil {
		return PersistImportSummaryResult{}, fmt.Errorf("init S3 client failed: %w", err)
	}

	key := args.RunPrefix + "/summary.json"
	if err := s3c.UploadFile(ctx, key, tmp.Name(), "application/json"); err != nil {
		return PersistImportSummaryResult{}, fmt.Errorf("upload summary to S3 failed: %w", err)
	}

	return PersistImportSummaryResult{SummaryKey: key}, nil
}

// CopySourceToRunFolderArgs represents the input for the CopySourceToRunFolder activity
type CopySourceToRunFolderArgs struct {
	SourceKey string `json:"sourceKey"`
	RunPrefix string `json:"runPrefix"`
}

// CopySourceToRunFolderResult represents the output
type CopySourceToRunFolderResult struct {
	DestKey string `json:"destKey"`
}

// CopySourceToRunFolder copies the original upload into the run folder via server-side S3 copy (no download roundtrip).
func (a *EscrowImportActivities) CopySourceToRunFolder(ctx context.Context, args CopySourceToRunFolderArgs) (CopySourceToRunFolderResult, error) {
	if args.SourceKey == "" || args.RunPrefix == "" {
		return CopySourceToRunFolderResult{}, fmt.Errorf("sourceKey and runPrefix are required")
	}

	s3c, err := storage.NewS3ClientFromEnv()
	if err != nil {
		return CopySourceToRunFolderResult{}, fmt.Errorf("CopySourceToRunFolder: s3 client: %w", err)
	}

	destKey := args.RunPrefix + "/" + filepath.Base(args.SourceKey)

	// Idempotency: skip if already copied
	if exists, _ := s3c.Exists(ctx, destKey); exists {
		activity.GetLogger(ctx).Info("Source file already in run folder", "key", destKey)
		return CopySourceToRunFolderResult{DestKey: destKey}, nil
	}

	if err := s3c.CopyObject(ctx, args.SourceKey, destKey); err != nil {
		return CopySourceToRunFolderResult{}, fmt.Errorf("CopySourceToRunFolder(src=%s, dst=%s): %w", args.SourceKey, destKey, err)
	}

	activity.GetLogger(ctx).Info("Copied source file to run folder", "src", args.SourceKey, "dst", destKey)
	return CopySourceToRunFolderResult{DestKey: destKey}, nil
}

func decompressGzipFile(src string) (string, error) {
	f, err := os.Open(src)
	if err != nil {
		return "", err
	}
	defer f.Close()

	gr, err := gzip.NewReader(f)
	if err != nil {
		return "", err
	}
	defer gr.Close()

	// Keep extension as .xml so standard parsing functions detect it correctly
	tmp, err := os.CreateTemp("", "escrow-decompressed-*.xml")
	if err != nil {
		return "", err
	}
	defer tmp.Close()

	if _, err := io.Copy(tmp, gr); err != nil {
		os.Remove(tmp.Name())
		return "", err
	}

	return tmp.Name(), nil
}

// moveFile moves src to dst. It tries os.Rename first (fast, same-device).
// If Rename fails (e.g., cross-device, permission error), it falls back to
// copy+delete so the caller always gets the file at dst.
func moveFile(src, dst string) error {
	if err := os.Rename(src, dst); err == nil {
		return nil
	}

	// Fallback: copy content then remove source
	in, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("open source %s: %w", src, err)
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("create dest %s: %w", dst, err)
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		os.Remove(dst)
		return fmt.Errorf("copy %s -> %s: %w", src, dst, err)
	}

	// Ensure data is flushed to disk
	if err := out.Sync(); err != nil {
		return fmt.Errorf("sync %s: %w", dst, err)
	}

	os.Remove(src)
	return nil
}
