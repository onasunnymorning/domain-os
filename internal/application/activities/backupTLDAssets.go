package activities

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"

	"github.com/onasunnymorning/domain-os/internal/infrastructure/db/postgres"
	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
	"go.temporal.io/sdk/activity"
)

type BackupTLDAssetsArgs struct {
	ManifestKey string
	WorkflowID  string
	TLD         string
}

type BackupTLDAssetsResult struct {
	BackupKey     string
	EntitiesSaved int64
}

type BackupItem struct {
	Type   string      `json:"type"`
	Entity interface{} `json:"entity"`
}

// BackupTLDAssetsActivity reads the manifest CSV line by line, fetches the full entity payloads
// from Postgres in chunks, and streams them as JSONL to S3.
func (a *TLDCleanupActivities) BackupTLDAssets(ctx context.Context, args BackupTLDAssetsArgs) (BackupTLDAssetsResult, error) {
	if args.ManifestKey == "" {
		return BackupTLDAssetsResult{}, fmt.Errorf("manifest key is required")
	}

	s3c := a.S3Client
	db := a.DB

	// 1. Download Manifest Stream
	manifestStream, err := s3c.DownloadStream(ctx, args.ManifestKey)
	if err != nil {
		return BackupTLDAssetsResult{}, fmt.Errorf("failed to download manifest: %w", err)
	}
	defer manifestStream.Close()

	backupKey := fmt.Sprintf("%s/backup.jsonl", args.WorkflowID)
	pr, pw := io.Pipe()

	errChan := make(chan error, 1)
	go func() {
		defer close(errChan)
		errChan <- s3c.UploadStream(context.Background(), backupKey, pr, "application/jsonl")
	}()

	var count int64

	// Writer goroutine to stream from DB to pipe
	go func() {
		var writeErr error
		defer func() {
			pw.CloseWithError(writeErr)
		}()

		csvReader := csv.NewReader(manifestStream)
		// Skip header
		if _, err := csvReader.Read(); err != nil {
			writeErr = err
			return
		}

		encoder := json.NewEncoder(pw)

		// We will accumulate IDs to batch fetch
		const batchSize = 1000
		var domainIDs []int64
		var contactIDs []string
		var hostIDs []int64
		// tld name is handled directly

		flushDomains := func() error {
			if len(domainIDs) == 0 {
				return nil
			}
			var dbDomains []postgres.Domain
			if err := db.Preload("Hosts").Where("ro_id IN ?", domainIDs).Find(&dbDomains).Error; err != nil {
				return err
			}
			for _, d := range dbDomains {
				count++
				if err := encoder.Encode(BackupItem{Type: "Domain", Entity: postgres.ToDomain(&d)}); err != nil {
					return err
				}
			}
			domainIDs = domainIDs[:0]
			activity.RecordHeartbeat(ctx, fmt.Sprintf("backed up: %d", count))
			return nil
		}

		flushContacts := func() error {
			if len(contactIDs) == 0 {
				return nil
			}
			var dbContacts []postgres.Contact
			if err := db.Where("id IN ?", contactIDs).Find(&dbContacts).Error; err != nil {
				return err
			}
			for _, c := range dbContacts {
				count++
				if err := encoder.Encode(BackupItem{Type: "Contact", Entity: postgres.FromDBContact(&c)}); err != nil {
					return err
				}
			}
			contactIDs = contactIDs[:0]
			activity.RecordHeartbeat(ctx, fmt.Sprintf("backed up: %d", count))
			return nil
		}

		flushHosts := func() error {
			if len(hostIDs) == 0 {
				return nil
			}
			var dbHosts []postgres.Host
			if err := db.Preload("Addresses").Where("ro_id IN ?", hostIDs).Find(&dbHosts).Error; err != nil {
				return err
			}
			for _, h := range dbHosts {
				count++
				if err := encoder.Encode(BackupItem{Type: "Host", Entity: postgres.ToHost(&h)}); err != nil {
					return err
				}
			}
			hostIDs = hostIDs[:0]
			activity.RecordHeartbeat(ctx, fmt.Sprintf("backed up: %d", count))
			return nil
		}

		for {
			record, err := csvReader.Read()
			if err == io.EOF {
				break
			}
			if err != nil {
				writeErr = err
				return
			}

			if len(record) < 2 {
				continue
			}
			entityType := record[0]
			entityID := record[1]

			switch entityType {
			case "Domain":
				var id int64
				fmt.Sscanf(entityID, "%d", &id)
				domainIDs = append(domainIDs, id)
				if len(domainIDs) >= batchSize {
					if err := flushDomains(); err != nil {
						writeErr = err
						return
					}
				}
			case "Contact":
				contactIDs = append(contactIDs, entityID)
				if len(contactIDs) >= batchSize {
					if err := flushContacts(); err != nil {
						writeErr = err
						return
					}
				}
			case "Host":
				var id int64
				fmt.Sscanf(entityID, "%d", &id)
				hostIDs = append(hostIDs, id)
				if len(hostIDs) >= batchSize {
					if err := flushHosts(); err != nil {
						writeErr = err
						return
					}
				}
			case "Phase":
				var dbP postgres.Phase
				if err := db.Where("id = ?", entityID).First(&dbP).Error; err == nil {
					count++
					encoder.Encode(BackupItem{Type: "Phase", Entity: dbP.ToEntity()})
				}
			case "TLD":
				var dbT postgres.TLD
				if err := db.Where("name = ?", entityID).First(&dbT).Error; err == nil {
					count++
					var ent *entities.TLD = postgres.FromDBTLD(&dbT)
					encoder.Encode(BackupItem{Type: "TLD", Entity: ent})
				}
			}
		}

		// Flush remaining
		if err := flushDomains(); err != nil {
			writeErr = err
			return
		}
		if err := flushContacts(); err != nil {
			writeErr = err
			return
		}
		if err := flushHosts(); err != nil {
			writeErr = err
			return
		}

	}()

	if uploadErr := <-errChan; uploadErr != nil {
		return BackupTLDAssetsResult{}, fmt.Errorf("s3 jsonl upload failed: %w", uploadErr)
	}

	return BackupTLDAssetsResult{BackupKey: backupKey, EntitiesSaved: count}, nil
}
