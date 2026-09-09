package interfaces

import (
	"context"
	"io"
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
