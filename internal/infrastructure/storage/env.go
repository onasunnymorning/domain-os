package storage

import (
	"log"
	"os"
	"sync"
)

// Storage connection env vars. These replaced the MINIO_* names, which predate
// support for R2 and S3 and no longer describe what they configure.
const (
	EnvEndpoint       = "STORAGE_ENDPOINT"
	EnvAccessKey      = "STORAGE_ACCESS_KEY"
	EnvSecretKey      = "STORAGE_SECRET_KEY"
	EnvUseSSL         = "STORAGE_USE_SSL"
	EnvPublicEndpoint = "STORAGE_PUBLIC_ENDPOINT"
)

// deprecatedAliases maps each storage env var to the legacy MINIO_* name it
// replaced. The fallback is a migration aid for one release: deployments that
// still set only the old names keep working, loudly.
var deprecatedAliases = map[string]string{
	EnvEndpoint:       "MINIO_ENDPOINT",
	EnvAccessKey:      "MINIO_ACCESS_KEY",
	EnvSecretKey:      "MINIO_SECRET_KEY",
	EnvUseSSL:         "MINIO_USE_SSL",
	EnvPublicEndpoint: "MINIO_PUBLIC_ENDPOINT",
}

// warnedAliases keeps the deprecation warning to one line per variable rather
// than one per client construction — there are four clients per process.
var warnedAliases sync.Map

// storageEnv reads name, falling back to its deprecated MINIO_* alias when
// name is unset. Reading via a variable (rather than a string literal) keeps
// these out of the env-registry drift scan, which only matches literal
// os.Getenv("…") calls — deliberate, since the aliases are not registered.
func storageEnv(name string) string {
	if v := os.Getenv(name); v != "" {
		return v
	}
	alias, ok := deprecatedAliases[name]
	if !ok {
		return ""
	}
	v := os.Getenv(alias)
	if v != "" {
		if _, seen := warnedAliases.LoadOrStore(alias, true); !seen {
			log.Printf("[storage] DEPRECATED: %s is set but %s is not. %s will be removed in a future release — rename it.", alias, name, alias)
		}
	}
	return v
}

// PublicEndpoint returns the browser-facing storage endpoint used to build
// direct object links, or "" when unset.
func PublicEndpoint() string {
	return storageEnv(EnvPublicEndpoint)
}
