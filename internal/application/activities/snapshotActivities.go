package activities

import (
	"context"
	"io"

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

// NewSnapshotActivities creates a new SnapshotActivities on db and the temp bucket.
func NewSnapshotActivities(db *gorm.DB) (*SnapshotActivities, error) {
	s3c, err := storage.NewTempS3Client()
	if err != nil {
		return nil, err
	}
	return &SnapshotActivities{
		DB:       db,
		S3Client: s3c,
	}, nil
}
