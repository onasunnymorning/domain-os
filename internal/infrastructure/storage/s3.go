package storage

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type S3Client struct {
	client *minio.Client
	bucket string
	// presignClient is configured with the public endpoint so that presigned URLs
	// have correct S3 V4 signatures for the host the browser will actually use.
	// If no public endpoint is set, this falls back to client.
	presignClient *minio.Client
}

func NewS3ClientFromEnv() (*S3Client, error) {
	endpoint := os.Getenv("MINIO_ENDPOINT")
	accessKey := os.Getenv("MINIO_ACCESS_KEY")
	secretKey := os.Getenv("MINIO_SECRET_KEY")
	useSSL, _ := strconv.ParseBool(os.Getenv("MINIO_USE_SSL"))
	bucket := os.Getenv("ESCROW_BUCKET")
	if bucket == "" {
		bucket = "escrow"
	}
	public := strings.TrimSpace(os.Getenv("MINIO_PUBLIC_ENDPOINT"))

	// Allow self-signed in dev when not using SSL or custom certs
	tr := &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}
	httpClient := &http.Client{Transport: tr}

	cli, err := minio.New(endpoint, &minio.Options{
		Creds:     credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure:    useSSL,
		Transport: httpClient.Transport,
	})
	if err != nil {
		return nil, err
	}

	// Build a second client for presigning that uses the public endpoint.
	// S3 V4 signatures include the Host header, so presigned URLs must be
	// signed against the host the browser will actually reach.
	presignCli := cli
	if public != "" {
		pubEndpoint, pubSSL := parseEndpointURL(public)
		if pubEndpoint != "" {
			pc, err := minio.New(pubEndpoint, &minio.Options{
				Creds:     credentials.NewStaticV4(accessKey, secretKey, ""),
				Secure:    pubSSL,
				Region:    "us-east-1", // Avoids bucket location lookup (unreachable from Docker)
				Transport: httpClient.Transport,
			})
			if err != nil {
				return nil, fmt.Errorf("creating presign client for %s: %w", pubEndpoint, err)
			}
			presignCli = pc
		}
	}

	return &S3Client{client: cli, bucket: bucket, presignClient: presignCli}, nil
}

// parseEndpointURL extracts host:port and SSL flag from a URL string.
// Accepts "http://localhost:9000", "https://s3.example.com", or "localhost:9000".
// Strips literal quotes (common docker-compose YAML mistake).
func parseEndpointURL(raw string) (endpoint string, useSSL bool) {
	raw = strings.Trim(raw, `"'`)
	u, err := url.Parse(raw)
	if err != nil {
		return raw, false
	}
	useSSL = u.Scheme == "https"
	if u.Host != "" {
		return u.Host, useSSL
	}
	// url.Parse("localhost:9000") puts it in Path with empty Host
	if u.Path != "" {
		return strings.TrimPrefix(u.Path, "/"), false
	}
	return raw, false
}

// PresignPut returns a presigned PUT URL for the specified key.
// Uses the presign client so the V4 signature matches the public host.
func (s *S3Client) PresignPut(ctx context.Context, key string, expiry time.Duration) (string, error) {
	u, err := s.presignClient.PresignedPutObject(ctx, s.bucket, key, expiry)
	if err != nil {
		return "", fmt.Errorf("PresignPut(key=%s): %w", key, err)
	}
	return u.String(), nil
}

// PresignGet returns a presigned GET URL for the specified key.
// Uses the presign client so the V4 signature matches the public host.
func (s *S3Client) PresignGet(ctx context.Context, key string, expiry time.Duration) (string, error) {
	u, err := s.presignClient.PresignedGetObject(ctx, s.bucket, key, expiry, nil)
	if err != nil {
		return "", fmt.Errorf("PresignGet(key=%s): %w", key, err)
	}
	return u.String(), nil
}

func (s *S3Client) Exists(ctx context.Context, key string) (bool, error) {
	_, err := s.client.StatObject(ctx, s.bucket, key, minio.StatObjectOptions{})
	if err != nil {
		// Not found
		if minio.ToErrorResponse(err).Code == "NoSuchKey" {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// DownloadToFile downloads object to a local temp file and returns path
func (s *S3Client) DownloadToFile(ctx context.Context, key string) (string, error) {
	// Use the filename tail if present
	base := filepath.Base(strings.TrimSpace(key))
	if base == "." || base == "/" || base == "" {
		base = "escrow"
	}
	dstPath := filepath.Join(os.TempDir(), strconv.FormatInt(time.Now().UnixNano(), 10)+"-"+base)
	err := s.client.FGetObject(ctx, s.bucket, key, dstPath, minio.GetObjectOptions{})
	if err != nil {
		return "", err
	}
	return dstPath, nil
}

// GetObjectStream returns a streaming reader for the object.
// The caller MUST close the returned ReadCloser when done.
// This avoids writing the entire object to disk (unlike DownloadToFile).
func (s *S3Client) GetObjectStream(ctx context.Context, key string) (io.ReadCloser, int64, error) {
	obj, err := s.client.GetObject(ctx, s.bucket, key, minio.GetObjectOptions{})
	if err != nil {
		return nil, 0, fmt.Errorf("GetObjectStream(%s): %w", key, err)
	}
	info, err := obj.Stat()
	if err != nil {
		obj.Close()
		return nil, 0, fmt.Errorf("GetObjectStream(%s) stat: %w", key, err)
	}
	return obj, info.Size, nil
}

// UploadFile uploads a local file to the bucket at the given key
func (s *S3Client) UploadFile(ctx context.Context, key, path, contentType string) error {
	_, err := s.client.FPutObject(ctx, s.bucket, key, path, minio.PutObjectOptions{ContentType: contentType})
	return err
}

// CopyObject performs a server-side copy of srcKey to dstKey within the same bucket.
// No data is downloaded — the copy happens entirely on the S3/MinIO server.
func (s *S3Client) CopyObject(ctx context.Context, srcKey, dstKey string) error {
	_, err := s.client.CopyObject(ctx,
		minio.CopyDestOptions{Bucket: s.bucket, Object: dstKey},
		minio.CopySrcOptions{Bucket: s.bucket, Object: srcKey},
	)
	if err != nil {
		return fmt.Errorf("CopyObject(src=%s, dst=%s): %w", srcKey, dstKey, err)
	}
	return nil
}

// ListObjectKeys lists object keys under a given prefix. If recursive is true, it descends into sub-prefixes.
// Set maxKeys to a positive integer to limit the number of keys returned; pass 0 or negative for no explicit cap.
func (s *S3Client) ListObjectKeys(ctx context.Context, prefix string, recursive bool, maxKeys int) ([]string, error) {
	opts := minio.ListObjectsOptions{Prefix: prefix, Recursive: recursive}
	ch := s.client.ListObjects(ctx, s.bucket, opts)
	keys := []string{}
	for obj := range ch {
		if obj.Err != nil {
			return nil, obj.Err
		}
		keys = append(keys, obj.Key)
		if maxKeys > 0 && len(keys) >= maxKeys {
			break
		}
	}
	return keys, nil
}

// DownloadToString downloads an object's content and returns it as a string
func (s *S3Client) DownloadToString(ctx context.Context, key string) (string, error) {
	obj, err := s.client.GetObject(ctx, s.bucket, key, minio.GetObjectOptions{})
	if err != nil {
		return "", err
	}
	defer obj.Close()

	buf := new(strings.Builder)
	if _, err := io.Copy(buf, obj); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// UploadString uploads a string content to the bucket at the given key
func (s *S3Client) UploadString(ctx context.Context, key, content string) error {
	reader := strings.NewReader(content)
	_, err := s.client.PutObject(ctx, s.bucket, key, reader, int64(reader.Len()), minio.PutObjectOptions{ContentType: "text/plain"})
	return err
}

// UploadStream uploads an io.Reader stream to the bucket at the given key.
// It uses multipart upload since the size is unknown (-1).
func (s *S3Client) UploadStream(ctx context.Context, key string, reader io.Reader, contentType string) error {
	// minio-go requires part size when total size is -1
	opts := minio.PutObjectOptions{
		ContentType: contentType,
		PartSize:    10 * 1024 * 1024, // 10MB parts
	}
	_, err := s.client.PutObject(ctx, s.bucket, key, reader, -1, opts)
	return err
}

// DownloadStream returns an io.ReadCloser for the given key's object data.
func (s *S3Client) DownloadStream(ctx context.Context, key string) (io.ReadCloser, error) {
	obj, err := s.client.GetObject(ctx, s.bucket, key, minio.GetObjectOptions{})
	if err != nil {
		return nil, err
	}
	// Note: It's the caller's responsibility to close the returned io.ReadCloser
	return obj, nil
}

// =============================================================================
// Multipart Upload — browser-direct large file uploads (S3-compatible)
// =============================================================================

// MultipartCompletePart identifies a successfully uploaded part by its number and ETag.
type MultipartCompletePart struct {
	PartNumber int
	ETag       string
}

// InitMultipartUpload starts a new multipart upload and returns the upload ID.
func (s *S3Client) InitMultipartUpload(ctx context.Context, key string) (string, error) {
	core := minio.Core{Client: s.client}
	uploadID, err := core.NewMultipartUpload(ctx, s.bucket, key, minio.PutObjectOptions{
		ContentType: "application/octet-stream",
	})
	if err != nil {
		return "", fmt.Errorf("InitMultipartUpload: %w", err)
	}
	return uploadID, nil
}

// PresignUploadPart returns a presigned PUT URL for uploading a single part.
// The browser PUTs the chunk data directly to this URL.
// Uses the presign client so the V4 signature matches the public host.
func (s *S3Client) PresignUploadPart(ctx context.Context, key, uploadID string, partNumber int, expiry time.Duration) (string, error) {
	params := url.Values{
		"partNumber": {strconv.Itoa(partNumber)},
		"uploadId":   {uploadID},
	}
	u, err := s.presignClient.Presign(ctx, "PUT", s.bucket, key, expiry, params)
	if err != nil {
		return "", fmt.Errorf("PresignUploadPart(key=%s, part=%d): %w", key, partNumber, err)
	}
	return u.String(), nil
}

// CompleteMultipartUpload finalizes a multipart upload by assembling all parts.
func (s *S3Client) CompleteMultipartUpload(ctx context.Context, key, uploadID string, parts []MultipartCompletePart) error {
	core := minio.Core{Client: s.client}
	completeParts := make([]minio.CompletePart, len(parts))
	for i, p := range parts {
		completeParts[i] = minio.CompletePart{
			PartNumber: p.PartNumber,
			ETag:       p.ETag,
		}
	}
	_, err := core.CompleteMultipartUpload(ctx, s.bucket, key, uploadID, completeParts, minio.PutObjectOptions{})
	if err != nil {
		return fmt.Errorf("CompleteMultipartUpload: %w", err)
	}
	return nil
}

// AbortMultipartUpload cancels an in-progress multipart upload and cleans up parts.
func (s *S3Client) AbortMultipartUpload(ctx context.Context, key, uploadID string) error {
	core := minio.Core{Client: s.client}
	if err := core.AbortMultipartUpload(ctx, s.bucket, key, uploadID); err != nil {
		return fmt.Errorf("AbortMultipartUpload: %w", err)
	}
	return nil
}
