package interfaces

import "context"

// SanitizationTokenKeyProvider supplies the master HMAC key the escrow
// sanitizer derives its pseudonymisation tokens from (issue #415).
//
// The split is the one ADR-0007 drew for the escrow decryption keyring:
// custody of the value — storage, access control, audit, versioning — belongs
// to the secrets service, while what the key *means* belongs to this
// application. Here that meaning is the tenant-and-purpose scoping and the
// fingerprint recorded on every derivative, neither of which a secrets service
// can know about.
//
// Implementations must never log, persist or return the key anywhere except
// through this method, and their errors must be fixed text that cannot echo
// key material.
type SanitizationTokenKeyProvider interface {
	// TokenKey returns the master key. Callers derive per-tenant subkeys from
	// it and must not retain it beyond the run.
	TokenKey(ctx context.Context) ([]byte, error)
}
