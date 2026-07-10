package activities

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/onasunnymorning/domain-os/internal/infrastructure/db/postgres"
	"go.temporal.io/sdk/activity"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// --- ValidateSnapshot ---

// ValidateSnapshotArgs contains the arguments for ValidateSnapshot.
type ValidateSnapshotArgs struct {
	SnapshotKey string // S3 key prefix, e.g. "snapshot-pre-migration-20260625-080000"
}

// ValidateSnapshotResult is returned by the ValidateSnapshot activity.
type ValidateSnapshotResult struct {
	ManifestKey string           `json:"manifestKey"`
	TableCounts map[string]int64 `json:"tableCounts"`
	TotalRows   int64            `json:"totalRows"`
	Label       string           `json:"label"`
	Note        string           `json:"note,omitempty"`
	IsValid     bool             `json:"isValid"`
	Error       string           `json:"error,omitempty"`
}

// ValidateSnapshot downloads the manifest for a snapshot, verifies the JSONL exists, and
// returns the table counts for operator review before seeding.
func (a *SnapshotActivities) ValidateSnapshot(ctx context.Context, args ValidateSnapshotArgs) (ValidateSnapshotResult, error) {
	if args.SnapshotKey == "" {
		return ValidateSnapshotResult{}, fmt.Errorf("ValidateSnapshot: snapshotKey is required")
	}

	s3c := a.S3Client

	manifestKey := args.SnapshotKey + "/manifest.json"
	snapshotKey := args.SnapshotKey + "/snapshot.jsonl"

	// Download manifest
	manifestStream, err := s3c.DownloadStream(ctx, manifestKey)
	if err != nil {
		return ValidateSnapshotResult{
			IsValid: false,
			Error:   fmt.Sprintf("failed to download manifest at %s: %v", manifestKey, err),
		}, fmt.Errorf("ValidateSnapshot: download manifest at %s: %w", manifestKey, err)
	}
	defer manifestStream.Close()

	var manifest SnapshotManifest
	if err := json.NewDecoder(manifestStream).Decode(&manifest); err != nil {
		return ValidateSnapshotResult{
			IsValid: false,
			Error:   fmt.Sprintf("failed to parse manifest: %v", err),
		}, fmt.Errorf("ValidateSnapshot: parse manifest: %w", err)
	}

	// Verify the JSONL file exists (lightweight check via download start)
	jsonlStream, err := s3c.DownloadStream(ctx, snapshotKey)
	if err != nil {
		return ValidateSnapshotResult{
			ManifestKey: manifestKey,
			IsValid:     false,
			Error:       fmt.Sprintf("snapshot JSONL not found at %s: %v", snapshotKey, err),
		}, fmt.Errorf("ValidateSnapshot: snapshot JSONL not found at %s: %w", snapshotKey, err)
	}
	jsonlStream.Close()

	return ValidateSnapshotResult{
		ManifestKey: manifestKey,
		TableCounts: manifest.TableCounts,
		TotalRows:   manifest.TotalRows,
		Label:       manifest.Label,
		Note:        manifest.Note,
		IsValid:     true,
	}, nil
}

// --- SeedFromSnapshot ---

// SeedFromSnapshotArgs contains the arguments for SeedFromSnapshot.
type SeedFromSnapshotArgs struct {
	SnapshotKey string // S3 key prefix, e.g. "snapshot-pre-migration-20260625-080000"
	WorkflowID  string
}

// SeedFromSnapshotResult is returned by the SeedFromSnapshot activity.
type SeedFromSnapshotResult struct {
	InsertedCounts map[string]int64 `json:"insertedCounts"`
	SkippedCounts  map[string]int64 `json:"skippedCounts"`
	TotalInserted  int64            `json:"totalInserted"`
	TotalSkipped   int64            `json:"totalSkipped"`
}

// SeedFromSnapshot streams a JSONL snapshot from S3 and inserts rows into Postgres.
//
// It uses ON CONFLICT DO NOTHING for idempotent, gap-filling inserts — existing
// rows are preserved and skipped. This makes the operation safe to retry and
// suitable for populating an empty or partially-populated database.
//
// The JSONL file is already in FK-safe order (parents before children), so rows
// are inserted in the order they appear in the stream.
//
// The domain_events table is not included in snapshots and will not be seeded.
func (a *SnapshotActivities) SeedFromSnapshot(ctx context.Context, args SeedFromSnapshotArgs) (SeedFromSnapshotResult, error) {
	if args.SnapshotKey == "" {
		return SeedFromSnapshotResult{}, fmt.Errorf("SeedFromSnapshot: snapshotKey is required")
	}

	s3c := a.S3Client
	db := a.DB

	snapshotKey := args.SnapshotKey + "/snapshot.jsonl"

	stream, err := s3c.DownloadStream(ctx, snapshotKey)
	if err != nil {
		return SeedFromSnapshotResult{}, fmt.Errorf("SeedFromSnapshot: download snapshot at %s: %w", snapshotKey, err)
	}
	defer stream.Close()

	inserted := make(map[string]int64)
	skipped := make(map[string]int64)
	var totalInserted, totalSkipped int64

	// Buffer for batch inserts keyed by table name
	type batchEntry struct {
		table string
		rows  []json.RawMessage
	}

	scanner := bufio.NewScanner(stream)
	// Allow large lines (some rows can be big)
	scanner.Buffer(make([]byte, 0, 1024*1024), 10*1024*1024)

	var currentTable string
	var currentBatch []json.RawMessage
	lineNum := 0

	flushBatch := func(table string, batch []json.RawMessage) error {
		if len(batch) == 0 {
			return nil
		}
		ins, skip, err := insertBatch(db, table, batch)
		if err != nil {
			return fmt.Errorf("SeedFromSnapshot: insert batch for %s (line ~%d): %w", table, lineNum, err)
		}
		inserted[table] += ins
		skipped[table] += skip
		totalInserted += ins
		totalSkipped += skip
		activity.RecordHeartbeat(ctx, fmt.Sprintf("seeding %s: inserted=%d, skipped=%d, total_inserted=%d",
			table, inserted[table], skipped[table], totalInserted))
		return nil
	}

	for scanner.Scan() {
		lineNum++
		line := scanner.Bytes()

		// Parse just the table name first
		var partial struct {
			Table string          `json:"table"`
			Data  json.RawMessage `json:"data"`
		}
		if err := json.Unmarshal(line, &partial); err != nil {
			return SeedFromSnapshotResult{}, fmt.Errorf("SeedFromSnapshot: parse JSONL line %d: %w", lineNum, err)
		}

		// If table changed, flush the previous batch
		if partial.Table != currentTable && currentTable != "" {
			if err := flushBatch(currentTable, currentBatch); err != nil {
				return SeedFromSnapshotResult{}, err
			}
			currentBatch = currentBatch[:0]
		}
		currentTable = partial.Table
		currentBatch = append(currentBatch, partial.Data)

		// Flush every snapshotBatchSize rows
		if len(currentBatch) >= snapshotBatchSize {
			if err := flushBatch(currentTable, currentBatch); err != nil {
				return SeedFromSnapshotResult{}, err
			}
			currentBatch = currentBatch[:0]
		}
	}

	// Flush remaining batch
	if len(currentBatch) > 0 {
		if err := flushBatch(currentTable, currentBatch); err != nil {
			return SeedFromSnapshotResult{}, err
		}
	}

	if err := scanner.Err(); err != nil {
		return SeedFromSnapshotResult{}, fmt.Errorf("SeedFromSnapshot: scanner error: %w", err)
	}

	return SeedFromSnapshotResult{
		InsertedCounts: inserted,
		SkippedCounts:  skipped,
		TotalInserted:  totalInserted,
		TotalSkipped:   totalSkipped,
	}, nil
}

// insertBatch decodes a batch of JSON rows and inserts them into the appropriate
// table using GORM with ON CONFLICT DO NOTHING. Returns (inserted, skipped, error).
func insertBatch(db *gorm.DB, table string, batch []json.RawMessage) (int64, int64, error) {
	if len(batch) == 0 {
		return 0, 0, nil
	}

	// Decode based on table name
	switch table {
	case "iana_registrars":
		return insertTyped[postgres.IANARegistrar](db, batch)
	case "spec5_labels":
		return insertTyped[postgres.Spec5Label](db, batch)
	case "registry_operators":
		return insertTypedOmit[postgres.RegistryOperator](db, batch, "TLDs", "PremiumLists")
	case "tlds":
		return insertTypedOmit[postgres.TLD](db, batch, "Phases", "Registrars", "DNSRecord")
	case "phases":
		return insertTypedOmit[postgres.Phase](db, batch, "Prices", "Fees", "PremiumList")
	case "phase_prices":
		return insertTyped[postgres.Price](db, batch)
	case "phase_fees":
		return insertTyped[postgres.Fee](db, batch)
	case "nndns":
		return insertTypedOmit[postgres.NNDN](db, batch, "TLD")
	case "registrars":
		return insertTypedOmit[postgres.Registrar](db, batch,
			"Contacts", "ContactsCreated", "ContactsUpdated",
			"Hosts", "HostsCreated", "HostsUpdated",
			"Domains", "DomainsCreated", "DomainsUpdated", "TLDs")
	case "accreditations":
		return insertTyped[Accreditation](db, batch)
	case "contacts":
		return insertTypedOmit[postgres.Contact](db, batch,
			"DomsWhereRegistrant", "DomsWhereAdmin", "DomsWhereTech", "DomsWhereBilling")
	case "hosts":
		return insertTypedOmit[postgres.Host](db, batch, "Addresses")
	case "host_addresses":
		return insertTyped[postgres.HostAddress](db, batch)
	case "domains":
		return insertTypedOmit[postgres.Domain](db, batch, "Hosts", "TLD")
	case "domain_hosts":
		return insertTyped[DomainHost](db, batch)
	case "premium_lists":
		return insertTypedOmit[postgres.PremiumList](db, batch, "PremiumLabels")
	case "premium_labels":
		return insertTyped[postgres.PremiumLabel](db, batch)
	case "fx":
		return insertTyped[postgres.FX](db, batch)
	case "tld_dns_records":
		return insertTyped[postgres.TLDDNSRecord](db, batch)
	default:
		return 0, 0, fmt.Errorf("unknown table: %s", table)
	}
}

// insertTyped decodes a batch of JSON into the typed model and inserts with ON CONFLICT DO NOTHING.
func insertTyped[T any](db *gorm.DB, batch []json.RawMessage) (int64, int64, error) {
	rows := make([]T, 0, len(batch))
	for _, raw := range batch {
		var row T
		if err := json.Unmarshal(raw, &row); err != nil {
			return 0, 0, fmt.Errorf("unmarshal %T: %w", row, err)
		}
		rows = append(rows, row)
	}

	result := db.Clauses(clause.OnConflict{DoNothing: true}).Create(&rows)
	if result.Error != nil {
		return 0, 0, result.Error
	}

	inserted := result.RowsAffected
	skipped := int64(len(rows)) - inserted
	return inserted, skipped, nil
}

// insertTypedOmit is like insertTyped but omits specific relation columns to avoid
// GORM trying to cascade-insert related records.
func insertTypedOmit[T any](db *gorm.DB, batch []json.RawMessage, omit ...string) (int64, int64, error) {
	rows := make([]T, 0, len(batch))
	for _, raw := range batch {
		var row T
		if err := json.Unmarshal(raw, &row); err != nil {
			return 0, 0, fmt.Errorf("unmarshal %T: %w", row, err)
		}
		rows = append(rows, row)
	}

	result := db.Omit(omit...).Clauses(clause.OnConflict{DoNothing: true}).Create(&rows)
	if result.Error != nil {
		return 0, 0, result.Error
	}

	inserted := result.RowsAffected
	skipped := int64(len(rows)) - inserted
	return inserted, skipped, nil
}

// --- ListSnapshots ---

// SnapshotListItem represents a snapshot in the list returned by ListSnapshots.
type SnapshotListItem struct {
	Key         string           `json:"key"`       // S3 prefix key
	Label       string           `json:"label"`
	Note        string           `json:"note,omitempty"`
	CreatedAt   string           `json:"createdAt"` // ISO 8601 timestamp
	TotalRows   int64            `json:"totalRows"`
	TableCounts map[string]int64 `json:"tableCounts,omitempty"`
}

// ListSnapshotsResult is returned by the ListSnapshots activity.
type ListSnapshotsResult struct {
	Snapshots []SnapshotListItem `json:"snapshots"`
}

// ListSnapshots lists all available snapshots by scanning S3 for manifest.json files
// under the "snapshot-" prefix. Returns metadata from each manifest.
func (a *SnapshotActivities) ListSnapshots(ctx context.Context) (ListSnapshotsResult, error) {
	s3c := a.S3Client

	// List all keys under the "snapshot-" prefix to find manifest files
	keys, err := s3c.ListObjectKeys(ctx, "snapshot-", true, 500)
	if err != nil {
		return ListSnapshotsResult{}, fmt.Errorf("ListSnapshots: list S3 keys: %w", err)
	}

	var snapshots []SnapshotListItem

	for _, key := range keys {
		if !strings.HasSuffix(key, "/manifest.json") {
			continue
		}

		// Download and parse the manifest
		manifestStream, err := s3c.DownloadStream(ctx, key)
		if err != nil {
			continue // Skip unreadable manifests
		}

		var manifest SnapshotManifest
		decodeErr := json.NewDecoder(manifestStream).Decode(&manifest)
		manifestStream.Close()
		if decodeErr != nil {
			continue // Skip malformed manifests
		}

		// Derive the snapshot key prefix from the manifest key
		// e.g., "snapshot-pre-migration-20260625-080000/manifest.json" → "snapshot-pre-migration-20260625-080000"
		snapshotPrefix := strings.TrimSuffix(key, "/manifest.json")

		snapshots = append(snapshots, SnapshotListItem{
			Key:         snapshotPrefix,
			Label:       manifest.Label,
			Note:        manifest.Note,
			CreatedAt:   manifest.CreatedAt.Format("2006-01-02T15:04:05Z"),
			TotalRows:   manifest.TotalRows,
			TableCounts: manifest.TableCounts,
		})
	}

	return ListSnapshotsResult{Snapshots: snapshots}, nil
}
