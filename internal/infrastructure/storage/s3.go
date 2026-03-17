package storage

import (
	"context"
	"crypto/tls"
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
	// publicEndpoint is the externally reachable endpoint for presigned URLs (e.g., http://localhost:9000)
	// If empty, the SDK's internal endpoint host will be used as-is.
	publicEndpoint string
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

	return &S3Client{client: cli, bucket: bucket, publicEndpoint: public}, nil
}

// PresignPut returns a presigned PUT URL for the specified key
func (s *S3Client) PresignPut(ctx context.Context, key string, expiry time.Duration) (string, error) {
	u, err := s.client.PresignedPutObject(ctx, s.bucket, key, expiry)
	if err != nil {
		return "", err
	}
	// If a public endpoint is provided, rewrite the URL's scheme/host to be externally reachable
	if s.publicEndpoint != "" {
		if pub, err := url.Parse(s.publicEndpoint); err == nil {
			if pub.Scheme != "" {
				u.Scheme = pub.Scheme
			}
			// If MINIO_PUBLIC_ENDPOINT may be provided without scheme (host:port), set default http
			if u.Scheme == "" {
				u.Scheme = "http"
			}
			if pub.Host != "" {
				u.Host = pub.Host
			} else if pub.Path != "" {
				// Handle values like "localhost:9000" which end up in Path when scheme is missing
				u.Host = strings.TrimPrefix(pub.Path, "/")
			}
		}
	} else {
		// Dev-friendly fallback: if the SDK host points to an internal docker name like "minio",
		// rewrite it to localhost:9000 so the browser can reach it directly.
		// This avoids common misconfig where MINIO_PUBLIC_ENDPOINT isn't set in dev.
		if strings.HasPrefix(strings.ToLower(u.Host), "minio") || strings.HasSuffix(u.Host, ":9000") {
			u.Scheme = "http"
			// Preserve port when present; default to 9000
			host := u.Host
			if strings.Contains(host, ":") {
				parts := strings.Split(host, ":")
				port := parts[len(parts)-1]
				if port == "" {
					port = "9000"
				}
				u.Host = "localhost:" + port
			} else {
				u.Host = "localhost:9000"
			}
		}
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
	dstPath := filepath.Join(os.TempDir(), base)
	err := s.client.FGetObject(ctx, s.bucket, key, dstPath, minio.GetObjectOptions{})
	if err != nil {
		return "", err
	}
	return dstPath, nil
}

// UploadFile uploads a local file to the bucket at the given key
func (s *S3Client) UploadFile(ctx context.Context, key, path, contentType string) error {
	_, err := s.client.FPutObject(ctx, s.bucket, key, path, minio.PutObjectOptions{ContentType: contentType})
	return err
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
