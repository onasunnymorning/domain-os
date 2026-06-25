package activities

import (
	"bytes"
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/onasunnymorning/domain-os/internal/infrastructure/db/postgres"
	"github.com/onasunnymorning/domain-os/internal/infrastructure/storage"
	"go.temporal.io/sdk/activity"
	"gorm.io/gorm"
)

// Spec5SweepArgs contains parameters for the Spec5 sweep activity.
type Spec5SweepArgs struct {
	TLD        string   `json:"tld"`
	TLDs       []string `json:"tlds"`
	AllTLDs    bool     `json:"allTlds"`
	WorkflowID string   `json:"workflowId"`
}

// Spec5SweepResult contains the results of the sweep.
type Spec5SweepResult struct {
	TLDResults map[string]Spec5SweepTLDResult `json:"tldResults"`
}

// Spec5SweepTLDResult holds matching counts and download details for a specific TLD.
type Spec5SweepTLDResult struct {
	Count       int64  `json:"count"`
	ArtifactKey string `json:"artifactKey,omitempty"`
	DownloadURL string `json:"downloadUrl,omitempty"`
}

// SweepStorageAPI defines the S3 operations needed by the sweep activity.
type SweepStorageAPI interface {
	UploadStream(ctx context.Context, key string, reader io.Reader, contentType string) error
	PresignGet(ctx context.Context, key string, expiry time.Duration) (string, error)
}

// Spec5SweepActivities holds GORM and S3 dependencies.
type Spec5SweepActivities struct {
	DB       *gorm.DB
	S3Client SweepStorageAPI
}

// NewSpec5SweepActivities initializes dependencies from environment variables.
func NewSpec5SweepActivities() (*Spec5SweepActivities, error) {
	s3c, err := storage.NewS3ClientFromEnv()
	if err != nil {
		return nil, fmt.Errorf("NewSpec5SweepActivities: failed to initialize S3 client: %w. Check that MinIO is running and MinIO credentials are set", err)
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
		return nil, fmt.Errorf("NewSpec5SweepActivities: failed to connect to database: %w. Check that Postgres is running and DB connection variables are configured", err)
	}

	return &Spec5SweepActivities{
		DB:       db,
		S3Client: s3c,
	}, nil
}

type SweepRow struct {
	TldName string `gorm:"column:tld_name"`
	Label   string `gorm:"column:label"`
	Type    string `gorm:"column:type"`
	Name    string `gorm:"column:name"`
}

// SweepSpec5Labels performs a sweep of domain names matching ICANN Spec5 reserved labels.
func (a *Spec5SweepActivities) SweepSpec5Labels(ctx context.Context, args Spec5SweepArgs) (Spec5SweepResult, error) {
	if args.WorkflowID == "" {
		return Spec5SweepResult{}, fmt.Errorf("SweepSpec5Labels: workflowId is required")
	}

	activity.RecordHeartbeat(ctx, "initializing sweep TLD targets")

	var tldNames []string
	if args.AllTLDs {
		// Fetch all TLD names in the system
		err := a.DB.Table("tlds").Pluck("name", &tldNames).Error
		if err != nil {
			return Spec5SweepResult{}, fmt.Errorf("SweepSpec5Labels: failed to retrieve TLDs: %w. Check that postgres is running and the tlds table exists", err)
		}
	} else {
		if args.TLD != "" {
			tldNames = append(tldNames, args.TLD)
		}
		for _, t := range args.TLDs {
			if t != "" {
				// Deduplicate
				found := false
				for _, existing := range tldNames {
					if existing == t {
						found = true
						break
					}
				}
				if !found {
					tldNames = append(tldNames, t)
				}
			}
		}
	}

	if len(tldNames) == 0 {
		return Spec5SweepResult{}, fmt.Errorf("SweepSpec5Labels: no TLDs specified for sweep. Ensure at least one TLD or allTlds=true is provided")
	}

	activity.RecordHeartbeat(ctx, "executing CTE query to match domains against Spec5 labels")

	// CTE query optimized for slow DB connections.
	// Cross joins reference list (spec5_labels) with target TLDs in memory on the DB server,
	// then performs an index seek join on domains.name (uniquely indexed).
	query := `
WITH candidates AS (
	SELECT s.label || '.' || t.name AS name, s.label, s.type, t.name AS tld_name
	FROM spec5_labels s
	CROSS JOIN tlds t
	WHERE t.name IN ?
)
SELECT c.tld_name, c.label, c.type, d.name
FROM domains d
JOIN candidates c ON d.name = c.name
`

	var rows []SweepRow
	err := a.DB.Raw(query, tldNames).Scan(&rows).Error
	if err != nil {
		return Spec5SweepResult{}, fmt.Errorf("SweepSpec5Labels query execution: %w. Check database connection health and index on domains(name)", err)
	}

	activity.RecordHeartbeat(ctx, "grouping matching records by TLD")

	byTLD := make(map[string][]SweepRow)
	for _, r := range rows {
		byTLD[r.TldName] = append(byTLD[r.TldName], r)
	}

	tldResults := make(map[string]Spec5SweepTLDResult)
	for _, tld := range tldNames {
		// Default to empty/0 results
		tldResults[tld] = Spec5SweepTLDResult{Count: 0}
	}

	for tld, matches := range byTLD {
		activity.RecordHeartbeat(ctx, fmt.Sprintf("generating CSV artifact for %s", tld))

		var buf bytes.Buffer
		w := csv.NewWriter(&buf)

		if err := w.Write([]string{"Domain", "Label", "Type"}); err != nil {
			return Spec5SweepResult{}, fmt.Errorf("SweepSpec5Labels: failed to write CSV header: %w", err)
		}

		for _, m := range matches {
			if err := w.Write([]string{m.Name, m.Label, m.Type}); err != nil {
				return Spec5SweepResult{}, fmt.Errorf("SweepSpec5Labels: failed to write CSV row: %w", err)
			}
		}

		w.Flush()
		if err := w.Error(); err != nil {
			return Spec5SweepResult{}, fmt.Errorf("SweepSpec5Labels: failed to flush CSV writer: %w", err)
		}

		artifactKey := fmt.Sprintf("spec5-sweep/%s/%s-matching-spec5.csv", args.WorkflowID, tld)

		activity.RecordHeartbeat(ctx, fmt.Sprintf("uploading CSV artifact for %s to S3", tld))
		err = a.S3Client.UploadStream(ctx, artifactKey, bytes.NewReader(buf.Bytes()), "text/csv")
		if err != nil {
			return Spec5SweepResult{}, fmt.Errorf("SweepSpec5Labels: failed to upload CSV artifact to S3 at key %s: %w. Ensure MinIO is running and ESCROW_BUCKET is configured", artifactKey, err)
		}

		activity.RecordHeartbeat(ctx, fmt.Sprintf("presigning download URL for %s", tld))
		downloadURL, err := a.S3Client.PresignGet(ctx, artifactKey, 7*24*time.Hour)
		if err != nil {
			return Spec5SweepResult{}, fmt.Errorf("SweepSpec5Labels: failed to generate presigned download URL for S3 key %s: %w", artifactKey, err)
		}

		tldResults[tld] = Spec5SweepTLDResult{
			Count:       int64(len(matches)),
			ArtifactKey: artifactKey,
			DownloadURL: downloadURL,
		}
	}

	return Spec5SweepResult{TLDResults: tldResults}, nil
}
