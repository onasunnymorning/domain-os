// Package escrowkeys resolves which escrow key versions a run may use (issue
// #429, ADR-0009). It reads the key registry through repository ports only and
// holds no material, so both the escrow activities and the REST layer can use
// it without either depending on the other.
package escrowkeys

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/google/uuid"
	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
	"github.com/onasunnymorning/domain-os/pkg/domain/repositories"
)

// Resolver turns arrangements and key versions into a selection.
type Resolver struct {
	versions     repositories.EscrowKeyVersionRepository
	arrangements repositories.EscrowArrangementRepository
}

// NewResolver creates a resolver.
func NewResolver(versions repositories.EscrowKeyVersionRepository, arrangements repositories.EscrowArrangementRepository) *Resolver {
	return &Resolver{versions: versions, arrangements: arrangements}
}

// Effective resolves a TLD's inbound arrangement: TLD override, then operator
// default, then platform default, independently for each side.
func (r *Resolver) Effective(ctx context.Context, scope entities.OperatorID, tld string) (entities.EffectiveEscrowArrangement, error) {
	ks := entities.OperatorEscrowKeyScope(scope)
	get := func(level entities.EscrowArrangementLevel, tld string) (*entities.EscrowArrangement, error) {
		a, err := r.arrangements.GetLive(ctx, ks, level, tld, entities.EscrowArrangementInbound)
		if errors.Is(err, entities.ErrEscrowArrangementNotFound) {
			return nil, nil
		}
		return a, err
	}
	tldRow, err := get(entities.EscrowArrangementTLD, tld)
	if err != nil {
		return entities.EffectiveEscrowArrangement{}, fmt.Errorf("resolve arrangement (tld): %w", err)
	}
	opRow, err := get(entities.EscrowArrangementOperator, "")
	if err != nil {
		return entities.EffectiveEscrowArrangement{}, fmt.Errorf("resolve arrangement (operator): %w", err)
	}
	platformRow, err := get(entities.EscrowArrangementPlatform, "")
	if err != nil {
		return entities.EffectiveEscrowArrangement{}, fmt.Errorf("resolve arrangement (platform): %w", err)
	}
	return entities.ResolveEscrowArrangement(tldRow, opRow, platformRow), nil
}

// ResolveInbound selects the versions a validation of a deposit received at
// receivedAt may use.
//
//   - Verification: the effective depositor's verify-inbound versions usable
//     for the deposit. No depositor means no trusted key, which the pipeline
//     already reports as an untrusted signature.
//   - Decryption: the effective receiver's decrypt-inbound versions usable for
//     the deposit. No receiver means no decryption key, which the pipeline
//     reports as DECRYPT_KEY_UNAVAILABLE (ERROR, never a DVFN).
func (r *Resolver) ResolveInbound(ctx context.Context, scope entities.OperatorID, tld string, receivedAt time.Time) (entities.EscrowKeySelection, error) {
	eff, err := r.Effective(ctx, scope, tld)
	if err != nil {
		return entities.EscrowKeySelection{}, err
	}
	sel := entities.EscrowKeySelection{Arrangement: eff, Verify: []entities.EscrowKeyVersionRef{}, Decrypt: []entities.EscrowKeyVersionRef{}}
	if eff.Depositor != nil {
		if sel.Verify, err = r.usableForDeposit(ctx, scope, eff.Depositor.PartyID, entities.EscrowKeyPurposeVerifyInbound, receivedAt); err != nil {
			return entities.EscrowKeySelection{}, err
		}
	}
	if eff.Receiver != nil {
		if sel.Decrypt, err = r.usableForDeposit(ctx, scope, eff.Receiver.PartyID, entities.EscrowKeyPurposeDecryptInbound, receivedAt); err != nil {
			return entities.EscrowKeySelection{}, err
		}
	}
	return sel, nil
}

// ResolvePseudonymisation selects the receiver's active pseudonymisation
// version for a new derivative, or nil when the TLD has no receiver or the
// receiver has no active version.
func (r *Resolver) ResolvePseudonymisation(ctx context.Context, scope entities.OperatorID, tld string) (*entities.EscrowKeyVersionRef, entities.EffectiveEscrowArrangement, error) {
	eff, err := r.Effective(ctx, scope, tld)
	if err != nil || eff.Receiver == nil {
		return nil, eff, err
	}
	versions, err := r.versions.ListByParty(ctx, entities.OperatorEscrowKeyScope(scope), eff.Receiver.PartyID, entities.EscrowKeyPurposePseudonymise)
	if err != nil {
		return nil, eff, fmt.Errorf("resolve pseudonymisation key: %w", err)
	}
	for _, v := range sortNewestFirst(versions) {
		if v.UsableForNewWork() {
			ref := v.Ref()
			return &ref, eff, nil
		}
	}
	return nil, eff, nil
}

// Refs returns references for the visible versions among ids, in the order
// given, skipping ids that are missing or invisible.
func (r *Resolver) Refs(ctx context.Context, scope entities.OperatorID, ids ...uuid.UUID) ([]entities.EscrowKeyVersionRef, error) {
	versions, err := r.StillUsable(ctx, scope, idsAsRefs(ids))
	if err != nil {
		return nil, err
	}
	out := make([]entities.EscrowKeyVersionRef, len(versions))
	for i, v := range versions {
		out[i] = v.Ref()
	}
	return out, nil
}

// StillUsable re-reads the referenced versions and returns those a run that
// already selected them may still use, in the order given. A version revoked
// or destroyed since selection is dropped: emergency disablement wins over
// workflow history. A version whose fingerprint no longer matches the
// reference is dropped too.
func (r *Resolver) StillUsable(ctx context.Context, scope entities.OperatorID, refs []entities.EscrowKeyVersionRef) ([]*entities.EscrowKeyVersion, error) {
	if len(refs) == 0 {
		return nil, nil
	}
	ids := make([]uuid.UUID, len(refs))
	for i, ref := range refs {
		ids[i] = ref.ID
	}
	current, err := r.versions.GetByIDs(ctx, entities.OperatorEscrowKeyScope(scope), ids)
	if err != nil {
		return nil, fmt.Errorf("re-check key versions: %w", err)
	}
	byID := make(map[uuid.UUID]*entities.EscrowKeyVersion, len(current))
	for _, v := range current {
		byID[v.ID] = v
	}
	var out []*entities.EscrowKeyVersion
	for _, ref := range refs {
		v, ok := byID[ref.ID]
		if !ok || !v.UsableForRecordedRun() {
			continue
		}
		if ref.Fingerprint != "" && v.Fingerprint != ref.Fingerprint {
			continue
		}
		out = append(out, v)
	}
	return out, nil
}

func (r *Resolver) usableForDeposit(ctx context.Context, scope entities.OperatorID, partyID uuid.UUID, purpose entities.EscrowKeyPurpose, receivedAt time.Time) ([]entities.EscrowKeyVersionRef, error) {
	versions, err := r.versions.ListByParty(ctx, entities.OperatorEscrowKeyScope(scope), partyID, purpose)
	if err != nil {
		return nil, fmt.Errorf("resolve %s keys: %w", purpose, err)
	}
	out := []entities.EscrowKeyVersionRef{}
	// ACTIVE first, newest first; then HISTORICAL, newest first. A decryptor
	// tries keys in order, so the likeliest key goes first.
	for _, pass := range []entities.EscrowKeyVersionState{entities.EscrowKeyActive, entities.EscrowKeyHistorical} {
		for _, v := range sortNewestFirst(versions) {
			if v.State == pass && v.UsableForDeposit(receivedAt) {
				out = append(out, v.Ref())
			}
		}
	}
	return out, nil
}

func sortNewestFirst(vs []*entities.EscrowKeyVersion) []*entities.EscrowKeyVersion {
	out := append([]*entities.EscrowKeyVersion(nil), vs...)
	sort.SliceStable(out, func(i, j int) bool { return out[i].Version > out[j].Version })
	return out
}

func idsAsRefs(ids []uuid.UUID) []entities.EscrowKeyVersionRef {
	refs := make([]entities.EscrowKeyVersionRef, len(ids))
	for i, id := range ids {
		refs[i] = entities.EscrowKeyVersionRef{ID: id}
	}
	return refs
}
