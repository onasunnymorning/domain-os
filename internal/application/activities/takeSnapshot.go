package activities

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/onasunnymorning/domain-os/internal/infrastructure/db/postgres"
	"go.temporal.io/sdk/activity"
	"gorm.io/gorm"
)

// TakeSnapshotArgs contains the arguments for the TakeSnapshot activity.
type TakeSnapshotArgs struct {
	Label      string // User-provided label, e.g. "pre-migration-2026-06-25"
	Note       string // Free-text description of the snapshot's intent
	WorkflowID string // Temporal workflow ID, used as the S3 folder prefix
}

// SnapshotManifest is persisted as manifest.json alongside the snapshot.
type SnapshotManifest struct {
	Label       string           `json:"label"`
	Note        string           `json:"note,omitempty"`
	CreatedAt   time.Time        `json:"createdAt"`
	TableCounts map[string]int64 `json:"tableCounts"`
	TotalRows   int64            `json:"totalRows"`
	// Note: domain_events table is intentionally excluded from snapshots.
	// It is append-only event data that can grow very large and is not
	// needed for seeding a new environment.
	ExcludedTables []string `json:"excludedTables"`
}

// TakeSnapshotResult is returned by the TakeSnapshot activity.
type TakeSnapshotResult struct {
	SnapshotKey string           `json:"snapshotKey"` // S3 key of the JSONL file
	ManifestKey string           `json:"manifestKey"` // S3 key of the manifest JSON
	TableCounts map[string]int64 `json:"tableCounts"` // Per-table row counts
	TotalRows   int64            `json:"totalRows"`
}

// SnapshotLine represents a single line in the JSONL snapshot file.
type SnapshotLine struct {
	Table string      `json:"table"`
	Data  interface{} `json:"data"`
}

// tableExporter defines how to export a single table in batches.
type tableExporter struct {
	name    string
	exportFn func(db *gorm.DB, encoder *json.Encoder, ctx context.Context) (int64, error)
}

// TakeSnapshot exports all database tables as a JSONL stream to S3.
// Tables are exported in FK-safe order (parents before children).
//
// The domain_events table is intentionally excluded — it is append-only
// event data that can grow very large and is not needed for seeding a
// new environment.
func (a *SnapshotActivities) TakeSnapshot(ctx context.Context, args TakeSnapshotArgs) (TakeSnapshotResult, error) {
	if args.WorkflowID == "" {
		return TakeSnapshotResult{}, fmt.Errorf("TakeSnapshot: workflowID is required")
	}

	db := a.DB
	s3c := a.S3Client

	snapshotKey := fmt.Sprintf("%s/snapshot.jsonl", args.WorkflowID)
	manifestKey := fmt.Sprintf("%s/manifest.json", args.WorkflowID)

	// Define tables in FK-safe order (parents first, children last).
	// Join tables (accreditations, domain_hosts) are exported after their
	// parent tables to preserve referential integrity on import.
	tables := buildTableExporters()

	tableCounts := make(map[string]int64, len(tables))
	var totalRows int64

	// Stream JSONL to S3 via io.Pipe
	pr, pw := io.Pipe()

	errChan := make(chan error, 1)
	go func() {
		defer close(errChan)
		errChan <- s3c.UploadStream(context.Background(), snapshotKey, pr, "application/jsonl")
	}()

	// Writer goroutine: iterate tables, query DB, encode JSONL
	go func() {
		var writeErr error
		defer func() {
			pw.CloseWithError(writeErr)
		}()

		encoder := json.NewEncoder(pw)

		for _, te := range tables {
			activity.RecordHeartbeat(ctx, fmt.Sprintf("exporting: %s", te.name))

			count, err := te.exportFn(db, encoder, ctx)
			if err != nil {
				writeErr = fmt.Errorf("TakeSnapshot: export table %s: %w", te.name, err)
				return
			}
			tableCounts[te.name] = count
			totalRows += count

			activity.RecordHeartbeat(ctx, fmt.Sprintf("exported: %s (%d rows)", te.name, count))
		}
	}()

	// Wait for S3 upload to complete
	if uploadErr := <-errChan; uploadErr != nil {
		return TakeSnapshotResult{}, fmt.Errorf("TakeSnapshot: S3 JSONL upload failed: %w", uploadErr)
	}

	// Upload manifest
	manifest := SnapshotManifest{
		Label:          args.Label,
		Note:           args.Note,
		CreatedAt:      time.Now().UTC(),
		TableCounts:    tableCounts,
		TotalRows:      totalRows,
		ExcludedTables: []string{"domain_events"},
	}
	manifestJSON, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return TakeSnapshotResult{}, fmt.Errorf("TakeSnapshot: marshal manifest: %w", err)
	}
	if err := s3c.UploadStream(ctx, manifestKey, strings.NewReader(string(manifestJSON)), "application/json"); err != nil {
		return TakeSnapshotResult{}, fmt.Errorf("TakeSnapshot: upload manifest: %w", err)
	}

	return TakeSnapshotResult{
		SnapshotKey: snapshotKey,
		ManifestKey: manifestKey,
		TableCounts: tableCounts,
		TotalRows:   totalRows,
	}, nil
}

// buildTableExporters returns the ordered list of table exporters in FK-safe order.
func buildTableExporters() []tableExporter {
	return []tableExporter{
		{"iana_registrars", makeTypedExporter[postgres.IANARegistrar]("iana_registrars")},
		{"spec5_labels", makeTypedExporter[postgres.Spec5Label]("spec5_labels")},
		{"registry_operators", exportRegistryOperators},
		{"tlds", exportTLDs},
		{"phases", exportPhases},
		{"phase_prices", makeTypedExporter[postgres.Price]("phase_prices")},
		{"phase_fees", makeTypedExporter[postgres.Fee]("phase_fees")},
		{"nndns", exportNNDNs},
		{"registrars", exportRegistrars},
		{"accreditations", exportAccreditations},
		{"contacts", exportContacts},
		{"hosts", exportHosts},
		{"host_addresses", makeTypedExporter[postgres.HostAddress]("host_addresses")},
		{"domains", exportDomains},
		{"domain_hosts", exportDomainHosts},
		{"premium_lists", exportPremiumLists},
		{"premium_labels", makeTypedExporter[postgres.PremiumLabel]("premium_labels")},
		{"fx", makeTypedExporter[postgres.FX]("fx")},
		{"tld_dns_records", makeTypedExporter[postgres.TLDDNSRecord]("tld_dns_records")},
	}
}

const snapshotBatchSize = 1000

// makeTypedExporter returns an export function for a GORM model table.
// Uses Offset/Limit pagination instead of FindInBatches, because FindInBatches
// requires a single auto-increment primary key and fails on composite-PK tables
// (fx, host_addresses, phase_prices, phase_fees).
func makeTypedExporter[T any](tableName string) func(db *gorm.DB, encoder *json.Encoder, ctx context.Context) (int64, error) {
	return func(db *gorm.DB, encoder *json.Encoder, ctx context.Context) (int64, error) {
		var count int64
		offset := 0

		for {
			var batch []T
			result := db.Offset(offset).Limit(snapshotBatchSize).Find(&batch)
			if result.Error != nil {
				return count, fmt.Errorf("query %s at offset %d: %w", tableName, offset, result.Error)
			}
			if len(batch) == 0 {
				break
			}

			for i := range batch {
				if err := encoder.Encode(SnapshotLine{Table: tableName, Data: batch[i]}); err != nil {
					return count, fmt.Errorf("encode row: %w", err)
				}
				count++
			}

			batchNum := (offset / snapshotBatchSize) + 1
			activity.RecordHeartbeat(ctx, fmt.Sprintf("%s batch %d, rows: %d", tableName, batchNum, count))

			if len(batch) < snapshotBatchSize {
				break // last batch
			}
			offset += snapshotBatchSize
		}
		return count, nil
	}
}

// exportRegistryOperators exports registry_operators without preloading relations.
func exportRegistryOperators(db *gorm.DB, encoder *json.Encoder, ctx context.Context) (int64, error) {
	var count int64
	var batch []postgres.RegistryOperator

	result := db.Omit("TLDs", "PremiumLists").FindInBatches(&batch, snapshotBatchSize, func(tx *gorm.DB, batchNum int) error {
		for i := range batch {
			batch[i].TLDs = nil
			batch[i].PremiumLists = nil
			if err := encoder.Encode(SnapshotLine{Table: "registry_operators", Data: batch[i]}); err != nil {
				return err
			}
			count++
		}
		activity.RecordHeartbeat(ctx, fmt.Sprintf("registry_operators batch %d, rows: %d", batchNum, count))
		return nil
	})
	return count, result.Error
}

// exportTLDs exports TLDs without preloading Phases, Registrars, or DNSRecords.
func exportTLDs(db *gorm.DB, encoder *json.Encoder, ctx context.Context) (int64, error) {
	var count int64
	var batch []postgres.TLD

	result := db.Omit("Phases", "Registrars", "DNSRecord").FindInBatches(&batch, snapshotBatchSize, func(tx *gorm.DB, batchNum int) error {
		for i := range batch {
			batch[i].Phases = nil
			batch[i].Registrars = nil
			batch[i].DNSRecord = nil
			if err := encoder.Encode(SnapshotLine{Table: "tlds", Data: batch[i]}); err != nil {
				return err
			}
			count++
		}
		activity.RecordHeartbeat(ctx, fmt.Sprintf("tlds batch %d, rows: %d", batchNum, count))
		return nil
	})
	return count, result.Error
}

// exportPhases exports phases without preloading Prices, Fees, or PremiumList.
func exportPhases(db *gorm.DB, encoder *json.Encoder, ctx context.Context) (int64, error) {
	var count int64
	var batch []postgres.Phase

	result := db.Omit("Prices", "Fees", "PremiumList").FindInBatches(&batch, snapshotBatchSize, func(tx *gorm.DB, batchNum int) error {
		for i := range batch {
			batch[i].Prices = nil
			batch[i].Fees = nil
			batch[i].PremiumList = nil
			if err := encoder.Encode(SnapshotLine{Table: "phases", Data: batch[i]}); err != nil {
				return err
			}
			count++
		}
		activity.RecordHeartbeat(ctx, fmt.Sprintf("phases batch %d, rows: %d", batchNum, count))
		return nil
	})
	return count, result.Error
}

// exportNNDNs exports NNDNs without preloading the TLD relation.
func exportNNDNs(db *gorm.DB, encoder *json.Encoder, ctx context.Context) (int64, error) {
	var count int64
	var batch []postgres.NNDN

	result := db.Omit("TLD").FindInBatches(&batch, snapshotBatchSize, func(tx *gorm.DB, batchNum int) error {
		for i := range batch {
			if err := encoder.Encode(SnapshotLine{Table: "nndns", Data: batch[i]}); err != nil {
				return err
			}
			count++
		}
		activity.RecordHeartbeat(ctx, fmt.Sprintf("nndns batch %d, rows: %d", batchNum, count))
		return nil
	})
	return count, result.Error
}

// exportRegistrars exports registrars without preloading any relations.
func exportRegistrars(db *gorm.DB, encoder *json.Encoder, ctx context.Context) (int64, error) {
	var count int64
	var batch []postgres.Registrar

	result := db.Omit("Contacts", "ContactsCreated", "ContactsUpdated", "Hosts", "HostsCreated", "HostsUpdated", "Domains", "DomainsCreated", "DomainsUpdated", "TLDs").
		FindInBatches(&batch, snapshotBatchSize, func(tx *gorm.DB, batchNum int) error {
			for i := range batch {
				batch[i].Contacts = nil
				batch[i].ContactsCreated = nil
				batch[i].ContactsUpdated = nil
				batch[i].Hosts = nil
				batch[i].HostsCreated = nil
				batch[i].HostsUpdated = nil
				batch[i].Domains = nil
				batch[i].DomainsCreated = nil
				batch[i].DomainsUpdated = nil
				batch[i].TLDs = nil
				if err := encoder.Encode(SnapshotLine{Table: "registrars", Data: batch[i]}); err != nil {
					return err
				}
				count++
			}
			activity.RecordHeartbeat(ctx, fmt.Sprintf("registrars batch %d, rows: %d", batchNum, count))
			return nil
		})
	return count, result.Error
}

// Accreditation represents a row in the accreditations join table (TLD ↔ Registrar M2M).
type Accreditation struct {
	TLDName      string `json:"tld_name" gorm:"column:tld_name"`
	RegistrarClID string `json:"registrar_cl_id" gorm:"column:registrar_cl_id"`
}

func (Accreditation) TableName() string { return "accreditations" }

// exportAccreditations exports the accreditations join table using raw queries.
func exportAccreditations(db *gorm.DB, encoder *json.Encoder, ctx context.Context) (int64, error) {
	var count int64
	var batch []Accreditation

	result := db.Table("accreditations").FindInBatches(&batch, snapshotBatchSize, func(tx *gorm.DB, batchNum int) error {
		for i := range batch {
			if err := encoder.Encode(SnapshotLine{Table: "accreditations", Data: batch[i]}); err != nil {
				return err
			}
			count++
		}
		activity.RecordHeartbeat(ctx, fmt.Sprintf("accreditations batch %d, rows: %d", batchNum, count))
		return nil
	})
	return count, result.Error
}

// exportContacts exports contacts without preloading domain relations.
func exportContacts(db *gorm.DB, encoder *json.Encoder, ctx context.Context) (int64, error) {
	var count int64
	var batch []postgres.Contact

	result := db.Omit("DomsWhereRegistrant", "DomsWhereAdmin", "DomsWhereTech", "DomsWhereBilling").
		FindInBatches(&batch, snapshotBatchSize, func(tx *gorm.DB, batchNum int) error {
			for i := range batch {
				batch[i].DomsWhereRegistrant = nil
				batch[i].DomsWhereAdmin = nil
				batch[i].DomsWhereTech = nil
				batch[i].DomsWhereBilling = nil
				if err := encoder.Encode(SnapshotLine{Table: "contacts", Data: batch[i]}); err != nil {
					return err
				}
				count++
			}
			activity.RecordHeartbeat(ctx, fmt.Sprintf("contacts batch %d, rows: %d", batchNum, count))
			return nil
		})
	return count, result.Error
}

// exportHosts exports hosts without preloading addresses (exported separately).
func exportHosts(db *gorm.DB, encoder *json.Encoder, ctx context.Context) (int64, error) {
	var count int64
	var batch []postgres.Host

	result := db.Omit("Addresses").FindInBatches(&batch, snapshotBatchSize, func(tx *gorm.DB, batchNum int) error {
		for i := range batch {
			batch[i].Addresses = nil
			if err := encoder.Encode(SnapshotLine{Table: "hosts", Data: batch[i]}); err != nil {
				return err
			}
			count++
		}
		activity.RecordHeartbeat(ctx, fmt.Sprintf("hosts batch %d, rows: %d", batchNum, count))
		return nil
	})
	return count, result.Error
}

// exportDomains exports domains without preloading hosts (exported via join table).
func exportDomains(db *gorm.DB, encoder *json.Encoder, ctx context.Context) (int64, error) {
	var count int64
	var batch []postgres.Domain

	result := db.Omit("Hosts", "TLD").FindInBatches(&batch, snapshotBatchSize, func(tx *gorm.DB, batchNum int) error {
		for i := range batch {
			batch[i].Hosts = nil
			if err := encoder.Encode(SnapshotLine{Table: "domains", Data: batch[i]}); err != nil {
				return err
			}
			count++
		}
		activity.RecordHeartbeat(ctx, fmt.Sprintf("domains batch %d, rows: %d", batchNum, count))
		return nil
	})
	return count, result.Error
}

// DomainHost represents a row in the domain_hosts join table (Domain ↔ Host M2M).
type DomainHost struct {
	DomainRoID int64 `json:"domain_ro_id" gorm:"column:domain_ro_id"`
	HostRoID   int64 `json:"host_ro_id" gorm:"column:host_ro_id"`
}

func (DomainHost) TableName() string { return "domain_hosts" }

// exportDomainHosts exports the domain_hosts join table.
func exportDomainHosts(db *gorm.DB, encoder *json.Encoder, ctx context.Context) (int64, error) {
	var count int64
	var batch []DomainHost

	result := db.Table("domain_hosts").FindInBatches(&batch, snapshotBatchSize, func(tx *gorm.DB, batchNum int) error {
		for i := range batch {
			if err := encoder.Encode(SnapshotLine{Table: "domain_hosts", Data: batch[i]}); err != nil {
				return err
			}
			count++
		}
		activity.RecordHeartbeat(ctx, fmt.Sprintf("domain_hosts batch %d, rows: %d", batchNum, count))
		return nil
	})
	return count, result.Error
}

// exportPremiumLists exports premium_lists without preloading labels.
func exportPremiumLists(db *gorm.DB, encoder *json.Encoder, ctx context.Context) (int64, error) {
	var count int64
	var batch []postgres.PremiumList

	result := db.Omit("PremiumLabels").FindInBatches(&batch, snapshotBatchSize, func(tx *gorm.DB, batchNum int) error {
		for i := range batch {
			batch[i].PremiumLabels = nil
			if err := encoder.Encode(SnapshotLine{Table: "premium_lists", Data: batch[i]}); err != nil {
				return err
			}
			count++
		}
		activity.RecordHeartbeat(ctx, fmt.Sprintf("premium_lists batch %d, rows: %d", batchNum, count))
		return nil
	})
	return count, result.Error
}
