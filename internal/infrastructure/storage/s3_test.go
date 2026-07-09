package storage

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestStorageCredentials(t *testing.T) {
	t.Run("defaults to static mode", func(t *testing.T) {
		t.Setenv("STORAGE_AUTH_MODE", "")
		t.Setenv("STORAGE_ACCESS_KEY", "ak")
		t.Setenv("STORAGE_SECRET_KEY", "sk")

		creds, err := storageCredentials()
		if err != nil {
			t.Fatalf("storageCredentials: %v", err)
		}
		v, err := creds.Get()
		if err != nil {
			t.Fatalf("creds.Get: %v", err)
		}
		if v.AccessKeyID != "ak" || v.SecretAccessKey != "sk" {
			t.Fatalf("expected static keys to be used, got %q/%q", v.AccessKeyID, v.SecretAccessKey)
		}
	})

	t.Run("static mode fails fast without keys", func(t *testing.T) {
		t.Setenv("STORAGE_AUTH_MODE", "static")
		t.Setenv("STORAGE_ACCESS_KEY", "")
		t.Setenv("STORAGE_SECRET_KEY", "")

		if _, err := storageCredentials(); err == nil {
			t.Fatal("expected an error when static mode is missing credentials")
		}
	})

	t.Run("iam mode needs no static keys", func(t *testing.T) {
		t.Setenv("STORAGE_AUTH_MODE", "iam")
		t.Setenv("STORAGE_ACCESS_KEY", "")
		t.Setenv("STORAGE_SECRET_KEY", "")

		if _, err := storageCredentials(); err != nil {
			t.Fatalf("iam mode should not require static keys, got: %v", err)
		}
	})

	t.Run("mode is case and whitespace insensitive", func(t *testing.T) {
		t.Setenv("STORAGE_AUTH_MODE", "  IAM ")
		t.Setenv("STORAGE_ACCESS_KEY", "")
		t.Setenv("STORAGE_SECRET_KEY", "")

		if _, err := storageCredentials(); err != nil {
			t.Fatalf("expected \"  IAM \" to resolve to iam mode, got: %v", err)
		}
	})

	t.Run("unknown mode is rejected", func(t *testing.T) {
		t.Setenv("STORAGE_AUTH_MODE", "assume-role")
		t.Setenv("STORAGE_ACCESS_KEY", "ak")
		t.Setenv("STORAGE_SECRET_KEY", "sk")

		if _, err := storageCredentials(); err == nil {
			t.Fatal("expected an unknown STORAGE_AUTH_MODE to be rejected rather than silently defaulting")
		}
	})
}

// TestTLSVerificationEnforced pins the security contract of STORAGE_TLS_SKIP_VERIFY:
// certificate verification is on unless explicitly disabled. httptest.NewTLSServer
// serves a self-signed cert, so a verifying client must reject it.
func TestTLSVerificationEnforced(t *testing.T) {
	srv := httptest.NewTLSServer(nil)
	defer srv.Close()
	host := strings.TrimPrefix(srv.URL, "https://")

	setEnv := func(t *testing.T, skipVerify string) *S3Client {
		t.Helper()
		t.Setenv("STORAGE_ENDPOINT", host)
		t.Setenv("STORAGE_ACCESS_KEY", "test")
		t.Setenv("STORAGE_SECRET_KEY", "test")
		t.Setenv("STORAGE_USE_SSL", "true")
		t.Setenv("STORAGE_PUBLIC_ENDPOINT", "")
		t.Setenv("STORAGE_TLS_SKIP_VERIFY", skipVerify)
		c, err := NewS3ClientForBucket("STORAGE_TEST_BUCKET", "test")
		if err != nil {
			t.Fatalf("NewS3ClientForBucket: %v", err)
		}
		return c
	}

	t.Run("rejects self-signed cert by default", func(t *testing.T) {
		c := setEnv(t, "")
		_, err := c.Exists(context.Background(), "some-key")
		if err == nil {
			t.Fatal("expected TLS verification to reject the self-signed cert, got nil error")
		}
		if !strings.Contains(err.Error(), "x509") && !strings.Contains(err.Error(), "certificate") {
			t.Fatalf("expected a certificate verification error, got: %v", err)
		}
	})

	t.Run("accepts self-signed cert when explicitly skipped", func(t *testing.T) {
		c := setEnv(t, "true")
		// The bare TLS server is not S3, so the request fails on the response —
		// but it must get past the TLS handshake, i.e. no certificate error.
		_, err := c.Exists(context.Background(), "some-key")
		if err != nil && (strings.Contains(err.Error(), "x509") || strings.Contains(err.Error(), "certificate")) {
			t.Fatalf("STORAGE_TLS_SKIP_VERIFY=true should bypass cert verification, got: %v", err)
		}
	})
}

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
	// When STORAGE_PUBLIC_ENDPOINT is empty, presignClient should be the same as client
	t.Setenv("STORAGE_ENDPOINT", "localhost:9000")
	t.Setenv("STORAGE_ACCESS_KEY", "test")
	t.Setenv("STORAGE_SECRET_KEY", "test")
	t.Setenv("STORAGE_USE_SSL", "false")
	t.Setenv("STORAGE_PUBLIC_ENDPOINT", "")
	t.Setenv("STORAGE_ESCROW_BUCKET", "test-bucket")

	s3c, err := NewS3ClientFromEnv()
	if err != nil {
		t.Fatalf("NewS3ClientFromEnv() error: %v", err)
	}

	if s3c.client != s3c.presignClient {
		t.Error("Expected presignClient to be the same as client when STORAGE_PUBLIC_ENDPOINT is empty")
	}
	if s3c.bucket != "test-bucket" {
		t.Errorf("Expected bucket 'test-bucket', got %q", s3c.bucket)
	}
}

func TestNewS3ClientFromEnv_PresignClientSeparate(t *testing.T) {
	// When STORAGE_PUBLIC_ENDPOINT is set, presignClient should be a different instance
	t.Setenv("STORAGE_ENDPOINT", "minio:9000")
	t.Setenv("STORAGE_ACCESS_KEY", "minioadmin")
	t.Setenv("STORAGE_SECRET_KEY", "minioadmin")
	t.Setenv("STORAGE_USE_SSL", "false")
	t.Setenv("STORAGE_PUBLIC_ENDPOINT", "http://localhost:9000")
	t.Setenv("STORAGE_ESCROW_BUCKET", "escrow")

	s3c, err := NewS3ClientFromEnv()
	if err != nil {
		t.Fatalf("NewS3ClientFromEnv() error: %v", err)
	}

	if s3c.client == s3c.presignClient {
		t.Error("Expected presignClient to be a DIFFERENT instance when STORAGE_PUBLIC_ENDPOINT is set")
	}
}

func TestNewS3ClientFromEnv_DefaultBucket(t *testing.T) {
	t.Setenv("STORAGE_ENDPOINT", "localhost:9000")
	t.Setenv("STORAGE_ACCESS_KEY", "test")
	t.Setenv("STORAGE_SECRET_KEY", "test")
	t.Setenv("STORAGE_USE_SSL", "false")
	t.Setenv("STORAGE_PUBLIC_ENDPOINT", "")
	t.Setenv("STORAGE_ESCROW_BUCKET", "")

	s3c, err := NewS3ClientFromEnv()
	if err != nil {
		t.Fatalf("NewS3ClientFromEnv() error: %v", err)
	}

	if s3c.bucket != "escrow" {
		t.Errorf("Expected default bucket 'escrow', got %q", s3c.bucket)
	}
}

func TestNewS3ClientFromEnv_HTTPSPublicEndpoint(t *testing.T) {
	t.Setenv("STORAGE_ENDPOINT", "minio:9000")
	t.Setenv("STORAGE_ACCESS_KEY", "test")
	t.Setenv("STORAGE_SECRET_KEY", "test")
	t.Setenv("STORAGE_USE_SSL", "false")
	t.Setenv("STORAGE_PUBLIC_ENDPOINT", "https://s3.production.example.com")

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
	t.Setenv("STORAGE_ENDPOINT", "localhost:9000")
	t.Setenv("STORAGE_ACCESS_KEY", "test")
	t.Setenv("STORAGE_SECRET_KEY", "test")
	t.Setenv("STORAGE_USE_SSL", "false")
	t.Setenv("STORAGE_PUBLIC_ENDPOINT", "   ")

	s3c, err := NewS3ClientFromEnv()
	if err != nil {
		t.Fatalf("NewS3ClientFromEnv() error: %v", err)
	}

	if s3c.client != s3c.presignClient {
		t.Error("Expected presignClient to fallback when STORAGE_PUBLIC_ENDPOINT is whitespace-only")
	}
}
