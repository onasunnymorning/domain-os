package activities

import (
	"context"
	"io"
	"os"

	"github.com/onasunnymorning/domain-os/internal/infrastructure/db/postgres"
	"github.com/onasunnymorning/domain-os/internal/infrastructure/storage"
	"gorm.io/gorm"
)

// SnapshotStorageAPI extends StorageAPI with operations needed by snapshot activities.
// The S3Client already satisfies this interface.
type SnapshotStorageAPI interface {
	UploadStream(ctx context.Context, key string, reader io.Reader, contentType string) error
	DownloadStream(ctx context.Context, key string) (io.ReadCloser, error)
	ListObjectKeys(ctx context.Context, prefix string, recursive bool, maxKeys int) ([]string, error)
}

// SnapshotActivities holds dependencies for snapshot-related activities.
// Follows the same struct-method pattern as TLDCleanupActivities.
type SnapshotActivities struct {
	DB       *gorm.DB
	S3Client SnapshotStorageAPI
}

// NewSnapshotActivities creates a new SnapshotActivities with initialized DB and S3 dependencies.
func NewSnapshotActivities() (*SnapshotActivities, error) {
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
	return &SnapshotActivities{
		DB:       db,
		S3Client: s3c,
	}, nil
}
