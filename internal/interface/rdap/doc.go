// Package rdap is the registry's RDAP server (RFC 9082, RFC 9083), a
// read-only view of registry state for the gTLD RDAP Profile.
//
// It is being built in phases; docs/rdap/ARCHITECTURE.md says what exists
// today and what each phase adds. The decisions are in ADR 0020 (server),
// ADR 0021 (publication and redaction) and ADR 0022 (access policies).
package rdap
