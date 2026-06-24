package storage

import (
	"testing"
)

func TestParseEndpointURL(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantHost string
		wantSSL  bool
	}{
		{
			name:     "http with port",
			input:    "http://localhost:9000",
			wantHost: "localhost:9000",
			wantSSL:  false,
		},
		{
			name:     "https with port",
			input:    "https://s3.example.com:9000",
			wantHost: "s3.example.com:9000",
			wantSSL:  true,
		},
		{
			name:     "https without port",
			input:    "https://s3.amazonaws.com",
			wantHost: "s3.amazonaws.com",
			wantSSL:  true,
		},
		{
			name:     "bare host:port (no scheme)",
			input:    "localhost:9000",
			wantHost: "localhost:9000",
			wantSSL:  false,
		},
		{
			name:     "http without port",
			input:    "http://minio.local",
			wantHost: "minio.local",
			wantSSL:  false,
		},
		{
			name:     "ip address with port",
			input:    "http://192.168.1.100:9000",
			wantHost: "192.168.1.100:9000",
			wantSSL:  false,
		},
		{
			name:     "quoted value (common docker-compose mistake)",
			input:    `"http://localhost:9000"`,
			wantHost: "localhost:9000",
			wantSSL:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotHost, gotSSL := parseEndpointURL(tt.input)
			if gotHost != tt.wantHost {
				t.Errorf("parseEndpointURL(%q) host = %q, want %q", tt.input, gotHost, tt.wantHost)
			}
			if gotSSL != tt.wantSSL {
				t.Errorf("parseEndpointURL(%q) ssl = %v, want %v", tt.input, gotSSL, tt.wantSSL)
			}
		})
	}
}

func TestNewS3ClientFromEnv_PresignClientFallback(t *testing.T) {
	// When MINIO_PUBLIC_ENDPOINT is empty, presignClient should be the same as client
	t.Setenv("MINIO_ENDPOINT", "localhost:9000")
	t.Setenv("MINIO_ACCESS_KEY", "test")
	t.Setenv("MINIO_SECRET_KEY", "test")
	t.Setenv("MINIO_USE_SSL", "false")
	t.Setenv("MINIO_PUBLIC_ENDPOINT", "")
	t.Setenv("ESCROW_BUCKET", "test-bucket")

	s3c, err := NewS3ClientFromEnv()
	if err != nil {
		t.Fatalf("NewS3ClientFromEnv() error: %v", err)
	}

	if s3c.client != s3c.presignClient {
		t.Error("Expected presignClient to be the same as client when MINIO_PUBLIC_ENDPOINT is empty")
	}
	if s3c.bucket != "test-bucket" {
		t.Errorf("Expected bucket 'test-bucket', got %q", s3c.bucket)
	}
}

func TestNewS3ClientFromEnv_PresignClientSeparate(t *testing.T) {
	// When MINIO_PUBLIC_ENDPOINT is set, presignClient should be a different instance
	t.Setenv("MINIO_ENDPOINT", "minio:9000")
	t.Setenv("MINIO_ACCESS_KEY", "minioadmin")
	t.Setenv("MINIO_SECRET_KEY", "minioadmin")
	t.Setenv("MINIO_USE_SSL", "false")
	t.Setenv("MINIO_PUBLIC_ENDPOINT", "http://localhost:9000")
	t.Setenv("ESCROW_BUCKET", "escrow")

	s3c, err := NewS3ClientFromEnv()
	if err != nil {
		t.Fatalf("NewS3ClientFromEnv() error: %v", err)
	}

	if s3c.client == s3c.presignClient {
		t.Error("Expected presignClient to be a DIFFERENT instance when MINIO_PUBLIC_ENDPOINT is set")
	}
}

func TestNewS3ClientFromEnv_DefaultBucket(t *testing.T) {
	t.Setenv("MINIO_ENDPOINT", "localhost:9000")
	t.Setenv("MINIO_ACCESS_KEY", "test")
	t.Setenv("MINIO_SECRET_KEY", "test")
	t.Setenv("MINIO_USE_SSL", "false")
	t.Setenv("MINIO_PUBLIC_ENDPOINT", "")
	t.Setenv("ESCROW_BUCKET", "")

	s3c, err := NewS3ClientFromEnv()
	if err != nil {
		t.Fatalf("NewS3ClientFromEnv() error: %v", err)
	}

	if s3c.bucket != "escrow" {
		t.Errorf("Expected default bucket 'escrow', got %q", s3c.bucket)
	}
}

func TestNewS3ClientFromEnv_HTTPSPublicEndpoint(t *testing.T) {
	t.Setenv("MINIO_ENDPOINT", "minio:9000")
	t.Setenv("MINIO_ACCESS_KEY", "test")
	t.Setenv("MINIO_SECRET_KEY", "test")
	t.Setenv("MINIO_USE_SSL", "false")
	t.Setenv("MINIO_PUBLIC_ENDPOINT", "https://s3.production.example.com")

	s3c, err := NewS3ClientFromEnv()
	if err != nil {
		t.Fatalf("NewS3ClientFromEnv() error: %v", err)
	}

	// Presign client should be separate
	if s3c.client == s3c.presignClient {
		t.Error("Expected presignClient to be separate for HTTPS public endpoint")
	}
}

func TestNewS3ClientFromEnv_WhitespacePublicEndpoint(t *testing.T) {
	// Whitespace-only should be treated as empty
	t.Setenv("MINIO_ENDPOINT", "localhost:9000")
	t.Setenv("MINIO_ACCESS_KEY", "test")
	t.Setenv("MINIO_SECRET_KEY", "test")
	t.Setenv("MINIO_USE_SSL", "false")
	t.Setenv("MINIO_PUBLIC_ENDPOINT", "   ")

	s3c, err := NewS3ClientFromEnv()
	if err != nil {
		t.Fatalf("NewS3ClientFromEnv() error: %v", err)
	}

	if s3c.client != s3c.presignClient {
		t.Error("Expected presignClient to fallback when MINIO_PUBLIC_ENDPOINT is whitespace-only")
	}
}
