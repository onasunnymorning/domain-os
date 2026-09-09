package interfaces

import (
	"context"

	"github.com/ProtonMail/go-crypto/openpgp"
)

// EscrowDecryptionKeyProvider is the key-management boundary for escrow
// validation (issue #412; ADR-0007).
//
// It is keyring-shaped on purpose. Registries re-key on their own schedule,
// so during a rollover deposits arrive encrypted to the previous service
// public key and to the new one; the decryptor must hold every non-retired
// private key and try them all. Which key actually decrypted a run is
// recorded on the run (fingerprint) for audit.
//
// Implementations own custody: they must never log, return as text, or
// otherwise expose private key material. The env-backed adapter is the first
// implementation; a secrets-service adapter replaces it behind this same
// interface without touching validation code.
type EscrowDecryptionKeyProvider interface {
	// DecryptionKeyring returns every unlocked private key that may be used
	// to decrypt an inbound deposit, most recent first.
	DecryptionKeyring(ctx context.Context) (openpgp.EntityList, error)
}
