package interfaces

import (
	"context"

	"github.com/ProtonMail/go-crypto/openpgp"
	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
)

// EscrowSecretStore is custody for escrow key material (issue #429, ADR-0009).
// It stores and returns opaque bytes under a name; what the bytes mean, who
// may use them and when belongs to the key registry, not to the store.
//
// The first implementation is AWS Secrets Manager. Deployments split the
// methods across identities: the API may Put and ScheduleDelete but never Get,
// and workers may only Get.
//
// Implementations must never log or return material except from Get, and
// their errors must wrap the fixed-text entities sentinels
// (ErrEscrowSecretNotFound, ErrEscrowSecretAlreadyExists,
// ErrEscrowKeyStoreUnavailable) rather than echo backend messages.
type EscrowSecretStore interface {
	// Put stores a new secret. It never overwrites: an existing name fails
	// with entities.ErrEscrowSecretAlreadyExists.
	Put(ctx context.Context, name string, tags map[string]string, value []byte) (entities.EscrowSecretRef, error)
	// Get returns the value at the reference's backend version, or the
	// current value when the reference carries none.
	Get(ctx context.Context, ref entities.EscrowSecretRef) ([]byte, error)
	// ScheduleDelete asks the backend to destroy the secret after its recovery
	// window. Deleting a secret that does not exist is not an error.
	ScheduleDelete(ctx context.Context, ref entities.EscrowSecretRef) error
}

// EscrowKeyMaterialLoader turns a key version reference into usable material
// on a worker. It fetches from the EscrowSecretStore, unlocks or decodes, and
// refuses material whose fingerprint differs from the one recorded on the
// version, so a swapped backend value fails closed instead of silently
// changing which key a run uses.
type EscrowKeyMaterialLoader interface {
	// OpenPGPPrivateKey returns the unlocked private key of an OpenPGP version.
	OpenPGPPrivateKey(ctx context.Context, ref entities.EscrowKeyVersionRef) (*openpgp.Entity, error)
	// SymmetricKey returns the raw key of a symmetric version. Callers must not
	// retain it beyond the run.
	SymmetricKey(ctx context.Context, ref entities.EscrowKeyVersionRef) ([]byte, error)
}
