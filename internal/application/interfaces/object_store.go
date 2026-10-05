package interfaces

import (
	"context"
	"io"
	"time"
)

// ObjectStore is the subset of object-storage operations the escrow
// validation activities need. *storage.S3Client satisfies it; tests use an
// in-memory fake. Keeping the surface this small is what lets the activities
// be tested without MinIO.
type ObjectStore interface {
	Exists(ctx context.Context, key string) (bool, error)
	GetObjectStream(ctx context.Context, key string) (io.ReadCloser, int64, error)
	CopyObject(ctx context.Context, srcKey, dstKey string) error
	UploadStream(ctx context.Context, key string, reader io.Reader, contentType string) error
}

// ObjectInfo is what a listing says about one object: enough to tell two
// uploads under the same key apart without reading either.
type ObjectInfo struct {
	Key          string
	Size         int64
	ETag         string
	LastModified time.Time
}

// IntakeObjectStore is what the sFTP intake sweep needs from the escrow
// bucket: list what has arrived, move it by copy-then-delete, and remove it
// once it is settled. It is a separate port from ObjectStore so that the
// validation and sanitize activities never gain a way to delete objects.
type IntakeObjectStore interface {
	ListObjectsInfo(ctx context.Context, prefix string, limit int) ([]ObjectInfo, error)
	Exists(ctx context.Context, key string) (bool, error)
	CopyObject(ctx context.Context, srcKey, dstKey string) error
	RemoveObject(ctx context.Context, key string) error
}
