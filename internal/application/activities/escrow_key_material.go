package activities

import (
	"context"
	"errors"
	"fmt"

	"github.com/ProtonMail/go-crypto/openpgp"
	"github.com/google/uuid"
	"github.com/onasunnymorning/domain-os/internal/application/escrowkeys"
	"github.com/onasunnymorning/domain-os/internal/application/interfaces"
	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
	"go.temporal.io/sdk/activity"
)

// escrowKeyMaterial turns a key selection into usable material for the escrow
// validation and sanitisation activities (issue #429, ADR-0009). It is the one
// place both pipelines apply the same rules:
//
//   - every selected version is re-checked before use, so a revocation reaches
//     a run already in flight;
//   - a key store outage is an error the activity returns for Temporal to
//     retry, while unusable material is skipped and, if nothing usable is
//     left, surfaces through the pipeline's own "key unavailable" code.
type escrowKeyMaterial struct {
	resolver *escrowkeys.Resolver
	loader   interfaces.EscrowKeyMaterialLoader
}

// trustedKeys returns the armored public keys of the still-usable
// verification versions, in selection order.
func (m escrowKeyMaterial) trustedKeys(ctx context.Context, scope entities.OperatorID, sel *entities.EscrowKeySelection) ([]string, error) {
	versions, err := m.resolver.StillUsable(ctx, scope, sel.Verify)
	if err != nil {
		return nil, err
	}
	armored := make([]string, 0, len(versions))
	for _, v := range versions {
		armored = append(armored, v.ArmoredPublicKey)
	}
	return armored, nil
}

// decryptionRing returns the keyring the selection allows. An empty ring is
// not an error: the pipeline reports it as DECRYPT_KEY_UNAVAILABLE.
func (m escrowKeyMaterial) decryptionRing(ctx context.Context, scope entities.OperatorID, sel *entities.EscrowKeySelection) (openpgp.EntityList, error) {
	logger := activity.GetLogger(ctx)
	versions, err := m.resolver.StillUsable(ctx, scope, sel.Decrypt)
	if err != nil {
		return nil, err
	}
	var ring openpgp.EntityList
	for _, v := range versions {
		e, err := m.loader.OpenPGPPrivateKey(ctx, v.Ref())
		switch {
		case err == nil:
			ring = append(ring, e)
		case errors.Is(err, entities.ErrEscrowKeyStoreUnavailable):
			return nil, fmt.Errorf("escrow decryption key %s: %w", v.ID, err)
		default:
			logger.Warn("escrow keys: decryption key version not usable; skipped",
				"key_version_id", v.ID.String(), "fingerprint", v.Fingerprint, "reason", probeCodeFor(err))
		}
	}
	return ring, nil
}

// tokenKey returns the pseudonymisation master key the selection allows and
// the version it came from. ok is false when no usable key exists; err is set
// only for a retryable store outage.
func (m escrowKeyMaterial) tokenKey(ctx context.Context, scope entities.OperatorID, sel *entities.EscrowKeySelection) (key []byte, versionID *uuid.UUID, ok bool, err error) {
	if sel.Pseudonymise == nil {
		return nil, nil, false, nil
	}
	versions, err := m.resolver.StillUsable(ctx, scope, []entities.EscrowKeyVersionRef{*sel.Pseudonymise})
	if err != nil {
		return nil, nil, false, err
	}
	if len(versions) == 0 {
		return nil, nil, false, nil
	}
	k, err := m.loader.SymmetricKey(ctx, versions[0].Ref())
	switch {
	case err == nil:
		id := versions[0].ID
		return k, &id, true, nil
	case errors.Is(err, entities.ErrEscrowKeyStoreUnavailable):
		return nil, nil, false, fmt.Errorf("escrow pseudonymisation key %s: %w", versions[0].ID, err)
	default:
		activity.GetLogger(ctx).Warn("escrow keys: pseudonymisation key version not usable",
			"key_version_id", versions[0].ID.String(), "reason", probeCodeFor(err))
		return nil, nil, false, nil
	}
}
