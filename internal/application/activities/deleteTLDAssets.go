package activities

import (
	"context"
	"encoding/csv"
	"fmt"
	"io"

	"go.temporal.io/sdk/activity"
)

type DeleteTLDAssetsArgs struct {
	ManifestKey string
}

type DeleteTLDAssetsResult struct {
	DeletedCount int64
}

// DeleteTLDAssetsActivity reads the manifest CSV line by line and deletes the assets.
// Because the manifest is generated in FK-safe order (Domains -> Contacts -> Hosts -> Phases -> TLD),
// we can safely iter-delete chunk by chunk.
func (a *TLDCleanupActivities) DeleteTLDAssets(ctx context.Context, args DeleteTLDAssetsArgs) (DeleteTLDAssetsResult, error) {
	if args.ManifestKey == "" {
		return DeleteTLDAssetsResult{}, fmt.Errorf("manifest key is required")
	}

	s3c := a.S3Client
	db := a.DB

	manifestStream, err := s3c.DownloadStream(ctx, args.ManifestKey)
	if err != nil {
		return DeleteTLDAssetsResult{}, fmt.Errorf("failed to download manifest: %w", err)
	}
	defer manifestStream.Close()

	csvReader := csv.NewReader(manifestStream)
	// Skip header
	if _, err := csvReader.Read(); err != nil {
		return DeleteTLDAssetsResult{}, err
	}

	var count int64
	const batchSize = 1000

	var domainIDs []int64
	var contactIDs []string
	var hostIDs []int64
	var phaseIDs []int64

	flushDomains := func() error {
		if len(domainIDs) == 0 { return nil }

		// Delete dependent table records first to avoid foreign key constraints
		if err := db.Exec("DELETE FROM domain_hosts WHERE domain_ro_id IN ?", domainIDs).Error; err != nil {
			return err
		}

		if err := db.Exec("DELETE FROM domains WHERE ro_id IN ?", domainIDs).Error; err != nil {
			return err
		}
		count += int64(len(domainIDs))
		domainIDs = domainIDs[:0]
		activity.RecordHeartbeat(ctx, fmt.Sprintf("deleted: %d", count))
		return nil
	}

	flushContacts := func() error {
		if len(contactIDs) == 0 { return nil }

		if err := db.Exec("DELETE FROM contacts WHERE id IN ?", contactIDs).Error; err != nil {
			return err
		}
		count += int64(len(contactIDs))
		contactIDs = contactIDs[:0]
		activity.RecordHeartbeat(ctx, fmt.Sprintf("deleted: %d", count))
		return nil
	}

	flushHosts := func() error {
		if len(hostIDs) == 0 { return nil }
		
		// Delete from host dependent tables first
		if err := db.Exec("DELETE FROM host_addresses WHERE host_ro_id IN ?", hostIDs).Error; err != nil {
			return err
		}

		if err := db.Exec("DELETE FROM hosts WHERE ro_id IN ?", hostIDs).Error; err != nil {
			return err
		}
		count += int64(len(hostIDs))
		hostIDs = hostIDs[:0]
		activity.RecordHeartbeat(ctx, fmt.Sprintf("deleted: %d", count))
		return nil
	}

	flushPhases := func() error {
		if len(phaseIDs) == 0 { return nil }
		if err := db.Exec("DELETE FROM phases WHERE id IN ?", phaseIDs).Error; err != nil {
			return err
		}
		count += int64(len(phaseIDs))
		phaseIDs = phaseIDs[:0]
		activity.RecordHeartbeat(ctx, fmt.Sprintf("deleted: %d", count))
		return nil
	}

	for {
		record, err := csvReader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return DeleteTLDAssetsResult{}, err
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
				if err := flushDomains(); err != nil { return DeleteTLDAssetsResult{}, err }
			}
		case "Contact":
			// Before adding contacts to batch, make sure domains are flushed, because of FKs
			if err := flushDomains(); err != nil { return DeleteTLDAssetsResult{}, err }
			
			contactIDs = append(contactIDs, entityID)
			if len(contactIDs) >= batchSize {
				if err := flushContacts(); err != nil { return DeleteTLDAssetsResult{}, err }
			}
		case "Host":
			// Ensure domains are flushed
			if err := flushDomains(); err != nil { return DeleteTLDAssetsResult{}, err }
			
			var id int64
			fmt.Sscanf(entityID, "%d", &id)
			hostIDs = append(hostIDs, id)
			if len(hostIDs) >= batchSize {
				if err := flushHosts(); err != nil { return DeleteTLDAssetsResult{}, err }
			}
		case "Phase":
			// Ensure downstream dependency is flushed
			if err := flushDomains(); err != nil { return DeleteTLDAssetsResult{}, err }
			if err := flushContacts(); err != nil { return DeleteTLDAssetsResult{}, err }
			if err := flushHosts(); err != nil { return DeleteTLDAssetsResult{}, err }

			var id int64
			fmt.Sscanf(entityID, "%d", &id)
			phaseIDs = append(phaseIDs, id)
			if len(phaseIDs) >= batchSize {
				if err := flushPhases(); err != nil { return DeleteTLDAssetsResult{}, err }
			}
		case "TLD":
			// Ensure everything is flushed before TLD goes down
			if err := flushDomains(); err != nil { return DeleteTLDAssetsResult{}, err }
			if err := flushContacts(); err != nil { return DeleteTLDAssetsResult{}, err }
			if err := flushHosts(); err != nil { return DeleteTLDAssetsResult{}, err }
			if err := flushPhases(); err != nil { return DeleteTLDAssetsResult{}, err }

			if err := db.Exec("DELETE FROM tlds WHERE name = ?", entityID).Error; err != nil {
				return DeleteTLDAssetsResult{}, err
			}
			count++
		}
	}

	// Final flushes
	if err := flushDomains(); err != nil { return DeleteTLDAssetsResult{}, err }
	if err := flushContacts(); err != nil { return DeleteTLDAssetsResult{}, err }
	if err := flushHosts(); err != nil { return DeleteTLDAssetsResult{}, err }
	if err := flushPhases(); err != nil { return DeleteTLDAssetsResult{}, err }

	return DeleteTLDAssetsResult{DeletedCount: count}, nil
}
