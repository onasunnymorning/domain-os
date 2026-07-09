package activities

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"os"

	"github.com/onasunnymorning/domain-os/internal/infrastructure/db/postgres"
	"github.com/onasunnymorning/domain-os/internal/infrastructure/storage"
	"go.temporal.io/sdk/activity"
	"gorm.io/gorm"
)

type PlanTLDCleanupArgs struct {
	TLD              string
	WorkflowID       string
	KeepTLDAndPhases bool // Not strictly required for planning, but useful to store in manifest
}

type PlanTLDCleanupResult struct {
	ManifestKey  string // The S3 key for the returned manifest CSV
	DomainCount  int64
	HostCount    int64
	ContactCount int64
}

// StorageAPI defines the subset of S3 operations used by cleanup
type StorageAPI interface {
	UploadStream(ctx context.Context, key string, reader io.Reader, contentType string) error
	DownloadStream(ctx context.Context, key string) (io.ReadCloser, error)
}

type TLDCleanupActivities struct {
	DB       *gorm.DB
	S3Client StorageAPI
}

// NewTLDCleanupActivities creates a new activities struct with initialized dependencies
func NewTLDCleanupActivities() (*TLDCleanupActivities, error) {
	s3c, err := storage.NewTempS3Client()
	if err != nil {
		return nil, err
	}
	var db *gorm.DB
	if dbURL := os.Getenv("DATABASE_URL"); dbURL != "" {
		db, err = postgres.NewConnectionFromURL(dbURL, false)
	} else {
		dbCfg := postgres.Config{
			User:    os.Getenv("DB_USER"),
			Pass:    os.Getenv("DB_PASS"),
			Host:    os.Getenv("DB_HOST"),
			Port:    os.Getenv("DB_PORT"),
			DBName:  os.Getenv("DB_NAME"),
			SSLmode: os.Getenv("DB_SSLMODE"),
		}
		db, err = postgres.NewConnection(dbCfg)
	}
	if err != nil {
		return nil, err
	}
	return &TLDCleanupActivities{
		DB:       db,
		S3Client: s3c,
	}, nil
}

// PlanTLDCleanup figures out exactly which entities will be removed.
// It finds orphaned contacts and orphaned hosts (i.e., those ONLY associated with the target TLD).
// It streams this list as a CSV directly to S3.
func (a *TLDCleanupActivities) PlanTLDCleanup(ctx context.Context, args PlanTLDCleanupArgs) (PlanTLDCleanupResult, error) {
	if args.TLD == "" {
		return PlanTLDCleanupResult{}, fmt.Errorf("tld is required")
	}

	s3c := a.S3Client
	db := a.DB

	manifestKey := fmt.Sprintf("%s/manifest.csv", args.WorkflowID)

	pr, pw := io.Pipe()

	var result PlanTLDCleanupResult
	result.ManifestKey = manifestKey

	// Fire up the S3 uploader in a goroutine
	errChan := make(chan error, 1)
	go func() {
		defer close(errChan)
		// Upload stream
		errChan <- s3c.UploadStream(context.Background(), manifestKey, pr, "text/csv")
	}()

	// Stream writer goroutine (writes lines to the pipe)
	go func() {
		var writeErr error
		defer func() {
			pw.CloseWithError(writeErr)
		}()

		writeLine := func(line string) error {
			_, err := pw.Write([]byte(line + "\n"))
			return err
		}

		// Write Header
		if err := writeLine("Entity,ID,Name"); err != nil {
			writeErr = err
			return
		}

		// 3. Find and Stream DOMAINS First to satisfy FKs downstream
		rowsDomains, err := db.Raw("SELECT ro_id, name FROM domains WHERE tld_name = ?", args.TLD).Rows()
		if err != nil {
			writeErr = err
			return
		}
		defer rowsDomains.Close()

		for rowsDomains.Next() {
			var dID int64
			var dName string
			if err := rowsDomains.Scan(&dID, &dName); err != nil {
				writeErr = err
				return
			}
			result.DomainCount++
			if err := writeLine(fmt.Sprintf("Domain,%d,%s", dID, dName)); err != nil {
				writeErr = err
				return
			}
			activity.RecordHeartbeat(ctx, fmt.Sprintf("domains: %d", result.DomainCount))
		}

		// 4. Find Orphaned CONTACTS
		orphanContactQuery := `
			SELECT c.id, c.name_int
			FROM contacts c
			WHERE EXISTS (
				SELECT 1 FROM domains d 
				WHERE d.tld_name = ? AND (d.registrant_id = c.id OR d.admin_id = c.id OR d.tech_id = c.id OR d.billing_id = c.id)
			)
			AND NOT EXISTS (
				SELECT 1 FROM domains d2 
				WHERE d2.tld_name != ? AND (d2.registrant_id = c.id OR d2.admin_id = c.id OR d2.tech_id = c.id OR d2.billing_id = c.id)
			)
		`
		rowsContacts, err := db.Raw(orphanContactQuery, args.TLD, args.TLD).Rows()
		if err != nil {
			writeErr = err
			return
		}
		defer rowsContacts.Close()

		for rowsContacts.Next() {
			var cID string
			var cName sql.NullString
			if err := rowsContacts.Scan(&cID, &cName); err != nil {
				writeErr = err
				return
			}
			result.ContactCount++
			if err := writeLine(fmt.Sprintf("Contact,%s,%s", cID, cName.String)); err != nil {
				writeErr = err
				return
			}
			activity.RecordHeartbeat(ctx, fmt.Sprintf("contacts: %d", result.ContactCount))
		}

		// 5. Find Orphaned HOSTS
		orphanHostsQuery := `
			SELECT h.ro_id, h.name
			FROM hosts h
			WHERE EXISTS (
				SELECT 1 FROM domain_hosts dh 
				JOIN domains d ON dh.domain_ro_id = d.ro_id 
				WHERE dh.host_ro_id = h.ro_id AND d.tld_name = ?
			)
			AND NOT EXISTS (
				SELECT 1 FROM domain_hosts dh2 
				JOIN domains d2 ON dh2.domain_ro_id = d2.ro_id 
				WHERE dh2.host_ro_id = h.ro_id AND d2.tld_name != ?
			)
		`
		rowsHosts, err := db.Raw(orphanHostsQuery, args.TLD, args.TLD).Rows()
		if err != nil {
			writeErr = err
			return
		}
		defer rowsHosts.Close()

		for rowsHosts.Next() {
			var hID int64
			var hName string
			if err := rowsHosts.Scan(&hID, &hName); err != nil {
				writeErr = err
				return
			}
			result.HostCount++
			if err := writeLine(fmt.Sprintf("Host,%d,%s", hID, hName)); err != nil {
				writeErr = err
				return
			}
			activity.RecordHeartbeat(ctx, fmt.Sprintf("hosts: %d", result.HostCount))
		}

		// 6. Write Phases & TLD (If not kept)
		if !args.KeepTLDAndPhases {
			// Write Phases
			var phases []struct {
				ID   int64
				Name string
			}
			db.Raw("SELECT id, name FROM phases WHERE tld_name = ?", args.TLD).Scan(&phases)
			for _, p := range phases {
				if err := writeLine(fmt.Sprintf("Phase,%d,%s", p.ID, p.Name)); err != nil {
					writeErr = err
					return
				}
			}
			// Write the TLD entry
			if err := writeLine(fmt.Sprintf("TLD,%s,%s", args.TLD, args.TLD)); err != nil {
				writeErr = err
				return
			}
		}

	}()

	// Wait for S3 upload to finish
	if uploadErr := <-errChan; uploadErr != nil {
		return PlanTLDCleanupResult{}, fmt.Errorf("s3 upload failed: %w", uploadErr)
	}

	return result, nil
}
