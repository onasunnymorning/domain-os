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
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	gopg "github.com/go-pg/pg/v10"
	"github.com/onasunnymorning/domain-os/internal/application/dataqa"
	"github.com/onasunnymorning/domain-os/internal/application/services"
	"github.com/onasunnymorning/domain-os/internal/infrastructure/storage"
	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/temporal"
	"golang.org/x/net/idna"
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
type EscrowImportActivities struct {
	// RepositoryID is the repository ID imported objects' new ROIDs are
	// minted with (ADR 0017).
	RepositoryID entities.RepositoryID
	// ImportPool is the one pool every ingestion activity's importer shares:
	// the importer's driver (go-pg) has a pool apart from the worker's GORM
	// pool, so the worker opens this one out of the same budget
	// (config.Database.MaxOpenConns), bounded once for the process however
	// many imports run at once.
	ImportPool *services.ImportPool
}

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
	SuggestedEmail  string                        `json:"suggestedEmail,omitempty"`
	SuggestedVoice  string                        `json:"suggestedVoice,omitempty"`
	SuggestedURL    string                        `json:"suggestedUrl,omitempty"`
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
	DBKey               string               // Updated db key
	HasIssues           bool                 // True if any active registrar is unmapped
	TotalRegistrars     int                  // Total registrars in escrow
	MappedCount         int                  // Successfully mapped
	UnmappedRegistrars  []UnmappedRegistrar  // Registrars that couldn't be auto-mapped
	AutoFixedRegistrars []AutoFixedRegistrar // Host-only registrars auto-resolved
	RejectedOverrides   []RejectedOverride   // Overrides that were provided but rejected
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
	// #nosec G202 -- the concatenated text is a run of "?" placeholders built in this function; every value is bound as a parameter
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
		_, err = tx.Exec(`INSERT INTO hosts_fixed SELECT * FROM hosts WHERE clID NOT IN (`+inClause+`)`, args...) // #nosec G202 -- the concatenated text is a run of "?" placeholders built in this function; every value is bound as a parameter
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
		// #nosec G202 -- the concatenated text is a run of "?" placeholders built in this function; every value is bound as a parameter
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
	if err := svc.StreamAnalyze(heartbeat); err != nil {
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
		if encErr := json.NewEncoder(tmp).Encode(manifest); encErr != nil {
			activity.GetLogger(ctx).Warn("Failed to encode manifest", "error", encErr)
		}
		tmp.Close()
		if upErr := s3c.UploadFile(ctx, manifestKey, tmp.Name(), "application/json"); upErr != nil {
			activity.GetLogger(ctx).Warn("Failed to upload manifest", "key", manifestKey, "error", upErr)
		}
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
		// Fallback: domains/hosts/contacts tables may not exist yet (e.g. registrar-only escrow)
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
			if len(token) > 15 {
				return token[:15] + "..."
			}
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
		if encErr := json.NewEncoder(tmp).Encode(manifest); encErr != nil {
			activity.GetLogger(ctx).Warn("Failed to encode manifest", "error", encErr)
		}
		tmp.Close()
		if upErr := s3c.UploadFile(ctx, manifestKey, tmp.Name(), "application/json"); upErr != nil {
			activity.GetLogger(ctx).Warn("Failed to upload manifest", "key", manifestKey, "error", upErr)
		}
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
		// #nosec G201 -- the interpolated text is a compile-time literal (table/column names, or a run of "?" placeholders); every value is bound as a parameter
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
			// #nosec G201 -- table and col are compile-time literals from the stageTable call sites
			if _, err := db.Exec(fmt.Sprintf("CREATE INDEX IF NOT EXISTS idx_staged_%s_%s ON %s(%s)", table, col, table, col)); err != nil {
				activity.GetLogger(ctx).Warn("ApplyRegistrarMappings: index creation failed", "table", table, "column", col, "error", err)
			}
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
	_, _ = db.Exec("DETACH DATABASE src") // best-effort cleanup of a scratch object

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
	Total    int64
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

	// Resumability: carry on after the batch the previous attempt last committed
	cursor := ingestCursor(ctx, services.PhaseContacts)

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

	// Wait for a connection of the shared pool, heartbeating meanwhile.
	release, err := a.ImportPool.Acquire(ctx, func() { activity.RecordHeartbeat(ctx, cursor) })
	if err != nil {
		return IngestContactsResult{}, err
	}
	defer release()
	destImporter, err := services.NewDirectDBImporterWith(a.RepositoryID, a.ImportPool)
	if err != nil {
		return IngestContactsResult{}, fmt.Errorf("failed to init importer: %w", err)
	}

	// Identity mapping (staged DB has correct IDs)
	// We pass nil to indicate trust/passthrough mode
	var clidMap map[string]string = nil

	heartbeat := func(c services.ImportCursor) {
		activity.RecordHeartbeat(ctx, c)
	}

	total, inserted, updated, skipped, err := destImporter.ImportContacts(ctx, db, clidMap, cursor, heartbeat)
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
	// Wait for a connection of the shared pool, heartbeating meanwhile.
	release, err := a.ImportPool.Acquire(ctx, func() { activity.RecordHeartbeat(ctx, "waiting for an import connection") })
	if err != nil {
		return ValidateRegistrantRefsResult{}, err
	}
	defer release()
	destImporter, err := services.NewDirectDBImporterWith(a.RepositoryID, a.ImportPool)
	if err != nil {
		return ValidateRegistrantRefsResult{}, fmt.Errorf("ValidateRegistrantRefs: init PG importer: %w", err)
	}

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
		// #nosec G201 -- the interpolated text is a compile-time literal (table/column names, or a program-generated date); every caller-supplied value is bound as a parameter
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
						// #nosec G201 -- the interpolated text is a compile-time literal (table/column names, or a program-generated date); every caller-supplied value is bound as a parameter
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
	RunPrefix   string // where the importer report is saved; see importerReportKey
}

type IngestHostsResult struct {
	Total        int64
	Inserted     int64
	Updated      int64
	SkippedHosts int64
	ReportKey    string // the importer report, "" if it could not be saved
}

// IngestHosts imports hosts
func (a *EscrowImportActivities) IngestHosts(ctx context.Context, args IngestHostsArgs) (IngestHostsResult, error) {
	s3c, err := storage.NewS3ClientFromEnv()
	if err != nil {
		return IngestHostsResult{}, err
	}

	cursor := ingestCursor(ctx, services.PhaseHosts)

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

	// Wait for a connection of the shared pool, heartbeating meanwhile.
	release, err := a.ImportPool.Acquire(ctx, func() { activity.RecordHeartbeat(ctx, cursor) })
	if err != nil {
		return IngestHostsResult{}, err
	}
	defer release()
	destImporter, err := services.NewDirectDBImporterWith(a.RepositoryID, a.ImportPool)
	if err != nil {
		return IngestHostsResult{}, err
	}

	heartbeat := func(c services.ImportCursor) {
		activity.RecordHeartbeat(ctx, c)
	}

	// Identity mapping (staged DB has correct IDs)
	var clidMap map[string]string = nil
	var res IngestHostsResult
	reportKey := importerReportKey(args.RunPrefix, args.StagedDBKey, "hosts")
	res.ReportKey, err = runWithImporterReport(ctx, s3c, reportKey, cursor.LastKey != "", destImporter.Report, activity.GetLogger(ctx).Warn, func() error {
		var err error
		res.Total, res.Inserted, res.Updated, err = destImporter.ImportHosts(ctx, db, clidMap, cursor, heartbeat)
		return err
	})
	res.SkippedHosts = int64(len(destImporter.Report.SkippedHosts))
	return res, err
}

// IngestDomainsArgs parameters
type IngestDomainsArgs struct {
	StagedDBKey string
	TLD         string
	RunPrefix   string // where the importer report is saved; see importerReportKey
}

type IngestDomainsResult struct {
	Total            int64
	Inserted         int64
	Updated          int64
	ModifiedDomains  int64
	SkippedDSRecords int64
	ReportKey        string // the importer report, "" if it could not be saved
}

// IngestDomains imports domains
func (a *EscrowImportActivities) IngestDomains(ctx context.Context, args IngestDomainsArgs) (IngestDomainsResult, error) {
	s3c, err := storage.NewS3ClientFromEnv()
	if err != nil {
		return IngestDomainsResult{}, err
	}

	cursor := ingestCursor(ctx, services.PhaseDomains)

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

	// Wait for a connection of the shared pool, heartbeating meanwhile.
	release, err := a.ImportPool.Acquire(ctx, func() { activity.RecordHeartbeat(ctx, cursor) })
	if err != nil {
		return IngestDomainsResult{}, err
	}
	defer release()
	destImporter, err := services.NewDirectDBImporterWith(a.RepositoryID, a.ImportPool)
	if err != nil {
		return IngestDomainsResult{}, err
	}

	heartbeat := func(c services.ImportCursor) {
		activity.RecordHeartbeat(ctx, c)
	}

	// Identity mapping (staged DB has correct IDs)
	var clidMap map[string]string = nil
	var res IngestDomainsResult
	reportKey := importerReportKey(args.RunPrefix, args.StagedDBKey, "domains")
	res.ReportKey, err = runWithImporterReport(ctx, s3c, reportKey, cursor.LastKey != "", destImporter.Report, activity.GetLogger(ctx).Warn, func() error {
		var err error
		res.Total, res.Inserted, res.Updated, err = destImporter.ImportDomains(ctx, db, args.TLD, clidMap, cursor, heartbeat)
		return err
	})
	res.ModifiedDomains = int64(len(destImporter.Report.ModifiedDomains))
	res.SkippedDSRecords = int64(len(destImporter.Report.SkippedDSRecords))
	return res, err
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

	cursor := ingestCursor(ctx, services.PhaseNNDNs)

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

	// Wait for a connection of the shared pool, heartbeating meanwhile.
	release, err := a.ImportPool.Acquire(ctx, func() { activity.RecordHeartbeat(ctx, cursor) })
	if err != nil {
		return IngestNNDNsResult{}, err
	}
	defer release()
	destImporter, err := services.NewDirectDBImporterWith(a.RepositoryID, a.ImportPool)
	if err != nil {
		return IngestNNDNsResult{}, err
	}

	heartbeat := func(c services.ImportCursor) {
		activity.RecordHeartbeat(ctx, c)
	}

	total, inserted, updated, err := destImporter.ImportNNDNs(ctx, db, args.TLD, cursor, heartbeat)
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

	// Wait for a connection of the shared pool, heartbeating meanwhile.
	release, err := a.ImportPool.Acquire(ctx, func() { activity.RecordHeartbeat(ctx, lastKey) })
	if err != nil {
		return LinkDomainHostsResult{}, err
	}
	defer release()
	destImporter, err := services.NewDirectDBImporterWith(a.RepositoryID, a.ImportPool)
	if err != nil {
		return LinkDomainHostsResult{}, err
	}

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

	// Wait for a connection of the shared pool, heartbeating meanwhile.
	release, err := a.ImportPool.Acquire(ctx, func() { activity.RecordHeartbeat(ctx, "") })
	if err != nil {
		return AccreditRegistrarsResult{}, err
	}
	defer release()
	destImporter, err := services.NewDirectDBImporterWith(a.RepositoryID, a.ImportPool)
	if err != nil {
		return AccreditRegistrarsResult{}, fmt.Errorf("failed to init importer: %w", err)
	}

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

// QACheck and QAReport are the project-wide staged-data QA schema, shared with the other pipelines
// (see internal/application/dataqa). They stay exported here as aliases so this package's callers and
// its JSON report are unchanged.
type (
	// QACheck represents a single quality check result
	QACheck = dataqa.QACheck
	// QAReport is the structured QA report for a staged database
	QAReport = dataqa.QAReport
)

// CleanOrphanedContactsArgs input for the orphan cleanup activity
type CleanOrphanedContactsArgs struct {
	TLD       string
	DBKey     string
	RunPrefix string
}

// CleanedRegistrar records what was cleaned for a specific dead registrar
type CleanedRegistrar struct {
	EscrowID   string `json:"escrowId"`
	Name       string `json:"name"`
	Reassigned int    `json:"reassigned"` // Contacts reassigned to domain's registrar
	Deleted    int    `json:"deleted"`    // Contacts deleted (unreferenced)
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
		if _, wErr := tmp.Write(reportData); wErr != nil {
			activity.GetLogger(ctx).Warn("Failed to write cleanup report", "error", wErr)
		}
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

// tldForms returns the distinct lower-cased spellings of a TLD that a staged
// domain name may legitimately end in: the value as given, its A-label and its
// U-label. Conversions are best-effort — an ASCII TLD converts to itself.
func tldForms(tld string) []string {
	base := strings.ToLower(strings.Trim(strings.TrimSpace(tld), "."))
	if base == "" {
		return nil
	}
	forms := []string{base}
	for _, conv := range []func(string) (string, error){idna.ToASCII, idna.ToUnicode} {
		if f, err := conv(base); err == nil && f != "" && !slices.Contains(forms, f) {
			forms = append(forms, f)
		}
	}
	return forms
}

// checkDomainsWithinTLD fails when the staged database holds a domain that is
// not under the TLD being imported. Ingestion upserts domains by name and
// stamps the import's TLD on the row, so a deposit whose names carry another
// suffix — e.g. a `.paco` derivative imported as `gza` — would re-assign every
// existing domain of that name to the wrong TLD rather than add new ones.
func checkDomainsWithinTLD(db *sql.DB, tld string) QACheck {
	check := QACheck{
		Rule:        "domains_within_tld",
		Description: "Every staged domain name is under the TLD being imported",
		Severity:    "error",
	}
	forms := tldForms(tld)
	if forms == nil {
		check.Message = "No TLD given — unable to verify"
		return check
	}

	const nameExpr = "rtrim(lower(trim(name)), '.')"
	var conds []string
	var args []any
	for _, f := range forms {
		conds = append(conds, nameExpr+" <> ? AND "+nameExpr+" NOT LIKE ?")
		args = append(args, f, "%."+f)
	}
	where := "(" + strings.Join(conds, ") AND (") + ")"

	var outside, total int
	if err := db.QueryRow("SELECT COUNT(*) FROM domains WHERE "+where, args...).Scan(&outside); err != nil {
		check.AffectedCount = -1
		check.Message = "Query failed — unable to verify"
		return check
	}
	if err := db.QueryRow("SELECT COUNT(*) FROM domains").Scan(&total); err != nil {
		total = 0 // Non-fatal: only used in the message
	}
	check.AffectedCount = outside
	if outside == 0 {
		check.Passed = true
		check.Message = fmt.Sprintf("All %d domains are under .%s", total, forms[0])
		return check
	}

	check.Message = fmt.Sprintf("%d of %d domains are not under .%s. Importing them as .%s would re-assign any existing domain with the same name to .%s. "+
		"The deposit was probably sanitized or renamed to a different suffix than the TLD it is being imported as", outside, total, forms[0], forms[0], forms[0])
	if rows, err := db.Query("SELECT name FROM domains WHERE "+where+" LIMIT 50", args...); err == nil { // #nosec G202 -- where is built only from a constant column expression and "?" placeholders; the TLD forms are bound as parameters
		defer rows.Close()
		var samples []map[string]string
		for rows.Next() {
			var name string
			if rows.Scan(&name) == nil {
				samples = append(samples, map[string]string{"domain": name})
			}
		}
		check.SampledItems = samples
	}
	return check
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

	// --- Check 2b: Domains belong to the TLD being imported ---
	report.AddCheck(checkDomainsWithinTLD(db, args.TLD))

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
		if err := db.QueryRow(`SELECT COUNT(DISTINCT clid) FROM (
			SELECT TRIM(clID) as clid FROM contacts WHERE clID IS NOT NULL AND clID != ''
			UNION
			SELECT TRIM(clID) FROM hosts WHERE clID IS NOT NULL AND clID != ''
			UNION
			SELECT TRIM(clID) FROM domains WHERE clID IS NOT NULL AND clID != ''
		)`).Scan(&totalDistinct); err != nil {
			totalDistinct = 0 // Non-fatal: the message below reports on what was read
		}

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
		if err := db.QueryRow(`SELECT COALESCE(SUM(domain_count), 0) FROM registrars WHERE domain_count IS NOT NULL`).Scan(&mappingDomainSum); err != nil {
			mappingDomainSum = 0 // Non-fatal: the delta below reports on what was read
		}

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
		query := fmt.Sprintf(`SELECT COUNT(*) FROM domains WHERE exdate IS NOT NULL AND exdate != '' AND exdate > '%s'`, futureDate) // #nosec G201 -- the interpolated text is a compile-time literal (table/column names, or a run of "?" placeholders); every value is bound as a parameter
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
			sampleQuery := fmt.Sprintf(`SELECT name, exdate FROM domains WHERE exdate IS NOT NULL AND exdate != '' AND exdate > '%s' ORDER BY exdate DESC LIMIT 50`, futureDate) // #nosec G201 -- the interpolated text is a compile-time literal (table/column names, or a run of "?" placeholders); every value is bound as a parameter
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
		query := fmt.Sprintf(`SELECT COUNT(*) FROM domains WHERE exdate IS NOT NULL AND exdate != '' AND exdate < '%s'`, pastDate) // #nosec G201 -- the interpolated text is a compile-time literal (table/column names, or a run of "?" placeholders); every value is bound as a parameter
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
			sampleQuery := fmt.Sprintf(`SELECT name, exdate FROM domains WHERE exdate IS NOT NULL AND exdate != '' AND exdate < '%s' ORDER BY exdate ASC LIMIT 50`, pastDate) // #nosec G201 -- the interpolated text is a compile-time literal (table/column names, or a run of "?" placeholders); every value is bound as a parameter
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
	if _, wErr := tmpFile.Write(reportJSON); wErr != nil {
		tmpFile.Close()
		return QAStagedDatabaseResult{}, fmt.Errorf("write qa report failed: %w", wErr)
	}
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
	// ImporterReportKeys names each ingest phase's importer report (what it
	// skipped or changed), by phase.
	ImporterReportKeys map[string]string `json:"importerReportKeys,omitempty"`
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
	if len(args.ImporterReportKeys) > 0 {
		payload["importerReportKeys"] = args.ImporterReportKeys
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

	// Backstop against a gzip bomb filling the worker's disk. This is not a
	// business limit on deposit size — raise maxDecompressedEscrowBytes if a
	// legitimate deposit ever approaches it.
	written, err := io.Copy(tmp, io.LimitReader(gr, maxDecompressedEscrowBytes+1))
	if err != nil {
		os.Remove(tmp.Name())
		return "", err
	}
	if written > maxDecompressedEscrowBytes {
		os.Remove(tmp.Name())
		return "", fmt.Errorf("decompressed escrow exceeds %d bytes; refusing to continue", maxDecompressedEscrowBytes)
	}

	return tmp.Name(), nil
}

// maxDecompressedEscrowBytes bounds gzip expansion when decompressing a
// deposit. Escrow files for large TLDs run to tens of GB uncompressed, so this
// is set far above any real deposit; it exists only so a crafted archive cannot
// expand without limit.
const maxDecompressedEscrowBytes int64 = 2 << 40 // 2 TiB

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
