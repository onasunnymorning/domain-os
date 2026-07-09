package storage

import (
	"context"
	"fmt"
	"log"

	"github.com/minio/minio-go/v7"
)

// EnsureBucket creates the client's target bucket if it doesn't already
// exist. Safe to call on every startup — MakeBucket is a no-op once the
// bucket is present. Intended for local/dev MinIO; in production (R2/S3)
// buckets are expected to be provisioned out-of-band and this simply
// confirms they exist.
func (s *S3Client) EnsureBucket(ctx context.Context) error {
	exists, err := s.client.BucketExists(ctx, s.bucket)
	if err != nil {
		return fmt.Errorf("EnsureBucket(%s): checking existence: %w", s.bucket, err)
	}
	if exists {
		return nil
	}
	if err := s.client.MakeBucket(ctx, s.bucket, minio.MakeBucketOptions{}); err != nil {
		return fmt.Errorf("EnsureBucket(%s): %w", s.bucket, err)
	}
	return nil
}

// EnsureBuckets constructs a client for each of the four storage buckets
// (escrow, event logs, reports, temp artifacts) and ensures each exists.
// Failures are logged and skipped rather than returned, matching the
// self-healing startup pattern used by bootstrap.EnsureTemporalInfrastructure —
// a missing bucket shouldn't prevent the service from starting.
func EnsureBuckets(ctx context.Context) {
	constructors := map[string]func() (*S3Client, error){
		"escrow":     NewS3ClientFromEnv,
		"event-logs": NewEventLogsS3Client,
		"reports":    NewReportsS3Client,
		"temp":       NewTempS3Client,
	}
	for name, newClient := range constructors {
		s3c, err := newClient()
		if err != nil {
			log.Printf("[storage] WARNING: failed to init %s bucket client: %v", name, err)
			continue
		}
		if err := s3c.EnsureBucket(ctx); err != nil {
			log.Printf("[storage] WARNING: failed to ensure %s bucket exists: %v", name, err)
		}
	}
}
