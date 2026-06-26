package activities

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/onasunnymorning/domain-os/internal/infrastructure/storage"
	"go.temporal.io/sdk/activity"
)

// VerifyIngestionArgs contains the inputs for the post-ingestion verification activity.
type VerifyIngestionArgs struct {
	TLD         string `json:"tld"`
	StagedDBKey string `json:"stagedDbKey"`
	RunPrefix   string `json:"runPrefix"`
}

// VerifyIngestionResult contains the verification report.
type VerifyIngestionResult struct {
	Passed    bool                `json:"passed"`
	ReportKey string              `json:"reportKey,omitempty"`
	Checks    []VerificationCheck `json:"checks"`
}

// VerificationCheck is a single verification check result.
type VerificationCheck struct {
	Rule          string              `json:"rule"`
	Description   string              `json:"description"`
	Severity      string              `json:"severity"` // "error" | "warning" | "info"
	Passed        bool                `json:"passed"`
	Message       string              `json:"message"`
	Expected      int64               `json:"expected,omitempty"`
	Actual        int64               `json:"actual,omitempty"`
	AffectedCount int                 `json:"affectedCount,omitempty"`
	SampledItems  []map[string]string `json:"sampledItems,omitempty"`
}

// buildAdminAPIURL constructs the admin API base URL. Prefers API_URL if set
// (for deployed environments), otherwise falls back to API_HOST/API_PORT
// env vars (set in docker-compose/Tiltfile), defaulting to http://localhost:8080.
func buildAdminAPIURL() string {
	if u := os.Getenv("API_URL"); u != "" {
		return strings.TrimRight(u, "/")
	}
	host := os.Getenv("API_HOST")
	port := os.Getenv("API_PORT")
	if host == "" {
		host = "localhost"
	}
	if port == "" {
		port = "8080"
	}
	return fmt.Sprintf("http://%s:%s", host, port)
}

// adminAPIGet performs an authenticated GET request against the admin API.
// Uses the shared TokenManager (Auth0 M2M) for authentication, falling back
// to ADMIN_TOKEN when Auth0 is not configured.
func adminAPIGet(client *http.Client, url string) (*http.Response, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", GetBearerToken())
	return client.Do(req)
}

// VerifyIngestion runs post-ingestion verification checks comparing the staged
// SQLite database against the live system via the admin API.
// Verification failures are informational — they do not fail the workflow.
func (a *EscrowImportActivities) VerifyIngestion(ctx context.Context, args VerifyIngestionArgs) (VerifyIngestionResult, error) {
	logger := activity.GetLogger(ctx)
	logger.Info("Starting post-ingestion verification", "tld", args.TLD)

	// Download staged DB
	s3c, err := storage.NewS3ClientFromEnv()
	if err != nil {
		return VerifyIngestionResult{}, fmt.Errorf("VerifyIngestion: create S3 client: %w", err)
	}

	dbPath, err := s3c.DownloadToFile(ctx, args.StagedDBKey)
	if err != nil {
		return VerifyIngestionResult{}, fmt.Errorf("VerifyIngestion: download staged DB (key=%s): %w", args.StagedDBKey, err)
	}
	defer os.Remove(dbPath)
	defer os.Remove(dbPath + "-wal")
	defer os.Remove(dbPath + "-shm")

	sqliteDB, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return VerifyIngestionResult{}, fmt.Errorf("VerifyIngestion: open SQLite (path=%s): %w", dbPath, err)
	}
	defer sqliteDB.Close()

	apiBase := buildAdminAPIURL()
	client := &http.Client{Timeout: 30 * time.Second}

	var checks []VerificationCheck

	// =========================================================================
	// Check 1: Domain count reconciliation via API
	// =========================================================================
	{
		var stagedCount int64
		if err := sqliteDB.QueryRow("SELECT COUNT(*) FROM domains").Scan(&stagedCount); err != nil {
			checks = append(checks, VerificationCheck{
				Rule: "domain_count", Description: "Domain count in staged DB matches live system",
				Severity: "error", Passed: false,
				Message: fmt.Sprintf("Failed to count staged domains: %v", err),
			})
		} else {
			liveCount, apiErr := apiCount(client, apiBase, fmt.Sprintf("/domains/count?tld_equals=%s", args.TLD))
			if apiErr != nil {
				checks = append(checks, VerificationCheck{
					Rule: "domain_count", Description: "Domain count in staged DB matches live system",
					Severity: "error", Passed: false,
					Message: fmt.Sprintf("Failed to query API for domain count: %v", apiErr),
				})
			} else {
				passed := stagedCount == liveCount
				msg := fmt.Sprintf("Staged: %d, Live: %d", stagedCount, liveCount)
				if !passed {
					msg = fmt.Sprintf("Count mismatch — Staged: %d, Live: %d, Delta: %d", stagedCount, liveCount, liveCount-stagedCount)
				}
				checks = append(checks, VerificationCheck{
					Rule: "domain_count", Description: "Domain count in staged DB matches live system",
					Severity: "error", Passed: passed,
					Expected: stagedCount, Actual: liveCount, Message: msg,
				})
			}
		}
	}

	// =========================================================================
	// Check 2: Contact count (staged only — contacts are not TLD-scoped in API)
	// =========================================================================
	{
		var stagedCount int64
		if err := sqliteDB.QueryRow("SELECT COUNT(*) FROM contacts").Scan(&stagedCount); err == nil {
			checks = append(checks, VerificationCheck{
				Rule: "contact_count", Description: "Contact count in staged DB recorded for reference",
				Severity: "info", Passed: true,
				Expected: stagedCount, Actual: stagedCount,
				Message: fmt.Sprintf("%d contacts staged (cross-TLD, live comparison skipped)", stagedCount),
			})
		}
	}

	// =========================================================================
	// Check 3: Host count (staged only — hosts are not TLD-scoped)
	// =========================================================================
	{
		var stagedCount int64
		if err := sqliteDB.QueryRow("SELECT COUNT(*) FROM hosts").Scan(&stagedCount); err == nil {
			checks = append(checks, VerificationCheck{
				Rule: "host_count", Description: "Host count in staged DB recorded for reference",
				Severity: "info", Passed: true,
				Expected: stagedCount, Actual: stagedCount,
				Message: fmt.Sprintf("%d hosts staged (cross-TLD, live comparison skipped)", stagedCount),
			})
		}
	}

	// =========================================================================
	// Check 4: NNDN count
	// =========================================================================
	{
		var stagedCount int64
		if err := sqliteDB.QueryRow("SELECT COUNT(*) FROM nndns").Scan(&stagedCount); err == nil {
			checks = append(checks, VerificationCheck{
				Rule: "nndn_count", Description: "NNDN count in staged DB recorded for reference",
				Severity: "info", Passed: true,
				Expected: stagedCount, Actual: stagedCount,
				Message: fmt.Sprintf("%d NNDNs staged", stagedCount),
			})
		}
	}

	// =========================================================================
	// Check 5: Sample domain verification via API
	// Pick N random domains from staged DB, fetch via GET /domains/:name,
	// and verify they exist in the live system.
	// =========================================================================
	{
		const sampleSize = 20

		var totalDomains int64
		_ = sqliteDB.QueryRow("SELECT COUNT(*) FROM domains").Scan(&totalDomains)

		var sampleNames []string
		if totalDomains > 0 {
			rng := rand.New(rand.NewSource(time.Now().UnixNano())) //nolint:gosec // not security-sensitive
			seen := make(map[int64]bool)
			for len(sampleNames) < sampleSize && len(sampleNames) < int(totalDomains) {
				offset := rng.Int63n(totalDomains)
				if seen[offset] {
					continue
				}
				seen[offset] = true

				var name string
				if err := sqliteDB.QueryRow("SELECT name FROM domains LIMIT 1 OFFSET ?", offset).Scan(&name); err == nil {
					sampleNames = append(sampleNames, name)
				}
			}
		}

		mismatches := 0
		var sampledItems []map[string]string

		for _, domainName := range sampleNames {
			// Get staged fields
			var sClID, sRegistrant, sCrDate, sExDate string
			err := sqliteDB.QueryRow(
				"SELECT COALESCE(clID,''), COALESCE(registrant,''), COALESCE(crDate,''), COALESCE(exDate,'') FROM domains WHERE name = ?",
				domainName,
			).Scan(&sClID, &sRegistrant, &sCrDate, &sExDate)
			if err != nil {
				continue
			}

			// Fetch from API
			resp, err := adminAPIGet(client, fmt.Sprintf("%s/domains/%s", apiBase, domainName))
			if err != nil {
				sampledItems = append(sampledItems, map[string]string{
					"domain": domainName, "status": "api_error", "detail": err.Error(),
				})
				mismatches++
				continue
			}

			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()

			if resp.StatusCode == http.StatusNotFound {
				sampledItems = append(sampledItems, map[string]string{
					"domain": domainName, "status": "missing", "detail": "Domain exists in staged DB but not found via API",
				})
				mismatches++
				continue
			}

			if resp.StatusCode != http.StatusOK {
				sampledItems = append(sampledItems, map[string]string{
					"domain": domainName, "status": "api_error", "detail": fmt.Sprintf("HTTP %d: %s", resp.StatusCode, string(body)),
				})
				mismatches++
				continue
			}

			var apiDomain map[string]interface{}
			if err := json.Unmarshal(body, &apiDomain); err != nil {
				sampledItems = append(sampledItems, map[string]string{
					"domain": domainName, "status": "parse_error", "detail": err.Error(),
				})
				mismatches++
				continue
			}

			// Compare registrar (clID) field — try common JSON key variants
			apiClID := jsonStringField(apiDomain, "ClID", "clId", "clID", "cl_id")
			if apiClID != "" && apiClID != sClID {
				sampledItems = append(sampledItems, map[string]string{
					"domain": domainName, "field": "clID",
					"staged": sClID, "live": apiClID,
				})
				mismatches++
			}
		}

		passed := mismatches == 0
		msg := fmt.Sprintf("%d/%d sample domains verified via API", len(sampleNames)-mismatches, len(sampleNames))
		if !passed {
			msg = fmt.Sprintf("%d mismatches in %d sample domains", mismatches, len(sampleNames))
		}

		check := VerificationCheck{
			Rule:          "api_sample_domains",
			Description:   "Random sample of domains verified via admin API (full stack test)",
			Severity:      "error",
			Passed:        passed,
			Message:       msg,
			AffectedCount: mismatches,
		}
		if len(sampledItems) > 0 {
			check.SampledItems = sampledItems
		}
		checks = append(checks, check)
	}

	// =========================================================================
	// Build overall result
	// =========================================================================
	allPassed := true
	for _, c := range checks {
		if !c.Passed && c.Severity == "error" {
			allPassed = false
			break
		}
	}

	result := VerifyIngestionResult{
		Passed: allPassed,
		Checks: checks,
	}

	// Upload verification report to S3
	reportJSON, _ := json.MarshalIndent(map[string]interface{}{
		"version":   "1.0",
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"pipeline":  "escrow-verification",
		"context": map[string]string{
			"runPrefix": args.RunPrefix,
			"tld":       args.TLD,
		},
		"passed": allPassed,
		"checks": checks,
	}, "", "  ")

	reportKey := args.RunPrefix + "/verification-report.json"
	if err := s3c.UploadString(ctx, reportKey, string(reportJSON)); err != nil {
		logger.Warn("Failed to upload verification report", "error", err)
	} else {
		result.ReportKey = reportKey
	}

	logger.Info("Post-ingestion verification complete", "passed", allPassed, "checks", len(checks))

	return result, nil
}

// apiCount fetches a count from the admin API's count endpoint.
// Expected response: { "count": N, ... }
func apiCount(client *http.Client, baseURL, path string) (int64, error) {
	resp, err := adminAPIGet(client, baseURL+path)
	if err != nil {
		return 0, fmt.Errorf("GET %s: %w", path, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return 0, fmt.Errorf("GET %s returned HTTP %d: %s", path, resp.StatusCode, string(body))
	}

	var result struct {
		Count int64 `json:"count"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return 0, fmt.Errorf("parse response from %s: %w", path, err)
	}
	return result.Count, nil
}

// jsonStringField extracts a string value from a JSON map, trying multiple key names.
func jsonStringField(m map[string]interface{}, keys ...string) string {
	for _, k := range keys {
		if v, ok := m[k]; ok {
			if s, ok := v.(string); ok {
				return s
			}
			return fmt.Sprintf("%v", v)
		}
	}
	return ""
}
