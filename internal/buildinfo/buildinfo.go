// Package buildinfo holds build-time metadata injected via -ldflags.
// All binaries in the domain-os monorepo import this package to expose
// a consistent version, git SHA, and build timestamp.
//
// Example ldflags:
//
//	go build -ldflags="-X github.com/onasunnymorning/domain-os/internal/buildinfo.Version=0.7.0 \
//	  -X github.com/onasunnymorning/domain-os/internal/buildinfo.GitSHA=abc123 \
//	  -X github.com/onasunnymorning/domain-os/internal/buildinfo.BuildDate=2025-04-12T00:00:00Z"
package buildinfo

// Version is the semantic version set at build time (e.g., "0.7.0").
// Defaults to "dev" when not injected.
var Version = "dev"

// GitSHA is the full git commit hash set at build time.
var GitSHA = "unknown"

// BuildDate is the UTC ISO 8601 timestamp of when the binary was built.
var BuildDate = "unknown"
