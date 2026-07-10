package storage

import "testing"

// TestStorageEnvDeprecatedFallback pins the one-release migration contract:
// STORAGE_* wins, the legacy MINIO_* name still works, and neither being set
// yields an empty string rather than a panic.
func TestStorageEnvDeprecatedFallback(t *testing.T) {
	t.Run("prefers the new name", func(t *testing.T) {
		t.Setenv(EnvEndpoint, "new:9000")
		t.Setenv("MINIO_ENDPOINT", "old:9000")

		if got := storageEnv(EnvEndpoint); got != "new:9000" {
			t.Fatalf("expected the STORAGE_* value to win, got %q", got)
		}
	})

	t.Run("falls back to the deprecated name", func(t *testing.T) {
		t.Setenv(EnvEndpoint, "")
		t.Setenv("MINIO_ENDPOINT", "old:9000")

		if got := storageEnv(EnvEndpoint); got != "old:9000" {
			t.Fatalf("expected fallback to MINIO_ENDPOINT, got %q", got)
		}
	})

	t.Run("empty when neither is set", func(t *testing.T) {
		t.Setenv(EnvEndpoint, "")
		t.Setenv("MINIO_ENDPOINT", "")

		if got := storageEnv(EnvEndpoint); got != "" {
			t.Fatalf("expected empty string, got %q", got)
		}
	})

	t.Run("every storage var has an alias and resolves", func(t *testing.T) {
		for newName, oldName := range deprecatedAliases {
			t.Setenv(newName, "")
			t.Setenv(oldName, "via-"+oldName)

			if got := storageEnv(newName); got != "via-"+oldName {
				t.Errorf("%s did not fall back to %s: got %q", newName, oldName, got)
			}
		}
	})

	t.Run("unaliased name has no fallback", func(t *testing.T) {
		t.Setenv("STORAGE_REGION", "")
		if got := storageEnv("STORAGE_REGION"); got != "" {
			t.Fatalf("expected no alias lookup for STORAGE_REGION, got %q", got)
		}
	})
}

// TestPublicEndpointHonoursFallback covers the escrow controller's direct-link
// path, which reads the public endpoint through this helper.
func TestPublicEndpointHonoursFallback(t *testing.T) {
	t.Setenv(EnvPublicEndpoint, "")
	t.Setenv("MINIO_PUBLIC_ENDPOINT", "http://localhost:9000")

	if got := PublicEndpoint(); got != "http://localhost:9000" {
		t.Fatalf("PublicEndpoint() = %q, want the deprecated fallback value", got)
	}
}
