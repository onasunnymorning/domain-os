// Package escrowkeystest provides in-memory key-registry repositories with the
// same scope and conditional-update semantics as the Postgres ones, for tests
// that exercise the escrow pipelines without a database.
package escrowkeystest

import (
	"context"
	"slices"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
)

// Versions is an in-memory EscrowKeyVersionRepository.
type Versions struct {
	mu     sync.Mutex
	rows   map[uuid.UUID]*entities.EscrowKeyVersion
	Audits []*entities.EscrowKeyAuditEvent
}

// NewVersions creates the repository holding copies of vs.
func NewVersions(vs ...*entities.EscrowKeyVersion) *Versions {
	r := &Versions{rows: map[uuid.UUID]*entities.EscrowKeyVersion{}}
	for _, v := range vs {
		r.rows[v.ID] = v.Clone()
	}
	return r
}

// Put stores or replaces a version directly (test setup, no audit).
func (r *Versions) Put(v *entities.EscrowKeyVersion) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.rows[v.ID] = v.Clone()
}

// Create implements the repository.
func (r *Versions) Create(_ context.Context, v *entities.EscrowKeyVersion, a *entities.EscrowKeyAuditEvent) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, existing := range r.rows {
		if existing.PartyID == v.PartyID && existing.Purpose == v.Purpose {
			if existing.Fingerprint == v.Fingerprint {
				return entities.ErrEscrowKeyDuplicateFingerprint
			}
			if existing.Version == v.Version {
				return entities.ErrEscrowKeyVersionConflict
			}
		}
	}
	r.rows[v.ID] = v.Clone()
	r.Audits = append(r.Audits, a)
	return nil
}

// GetByID implements the repository.
func (r *Versions) GetByID(_ context.Context, scope entities.EscrowKeyScope, id uuid.UUID) (*entities.EscrowKeyVersion, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	v, ok := r.rows[id]
	if !ok || !scope.CanSee(v.Owner) {
		return nil, entities.ErrEscrowKeyVersionNotFound
	}
	return v.Clone(), nil
}

// GetByIDs implements the repository.
func (r *Versions) GetByIDs(ctx context.Context, scope entities.EscrowKeyScope, ids []uuid.UUID) ([]*entities.EscrowKeyVersion, error) {
	var out []*entities.EscrowKeyVersion
	for _, id := range ids {
		if v, err := r.GetByID(ctx, scope, id); err == nil {
			out = append(out, v)
		}
	}
	return out, nil
}

// ListByParty implements the repository.
func (r *Versions) ListByParty(_ context.Context, scope entities.EscrowKeyScope, partyID uuid.UUID, purpose entities.EscrowKeyPurpose) ([]*entities.EscrowKeyVersion, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []*entities.EscrowKeyVersion
	for _, v := range r.rows {
		if v.PartyID == partyID && scope.CanSee(v.Owner) && (purpose == "" || v.Purpose == purpose) {
			out = append(out, v.Clone())
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Purpose != out[j].Purpose {
			return out[i].Purpose < out[j].Purpose
		}
		return out[i].Version > out[j].Version
	})
	return out, nil
}

// ActivePurposesByParty implements the repository.
func (r *Versions) ActivePurposesByParty(_ context.Context, scope entities.EscrowKeyScope, partyIDs []uuid.UUID) (map[uuid.UUID][]entities.EscrowKeyPurpose, error) {
	wanted := make(map[uuid.UUID]bool, len(partyIDs))
	for _, id := range partyIDs {
		wanted[id] = true
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	active := map[uuid.UUID][]entities.EscrowKeyPurpose{}
	for _, v := range r.rows {
		if !wanted[v.PartyID] || v.State != entities.EscrowKeyActive || !scope.CanSee(v.Owner) {
			continue
		}
		if !slices.Contains(active[v.PartyID], v.Purpose) {
			active[v.PartyID] = append(active[v.PartyID], v.Purpose)
		}
	}
	for id := range active {
		slices.Sort(active[id])
	}
	return active, nil
}

// NextVersionNumber implements the repository.
func (r *Versions) NextVersionNumber(ctx context.Context, scope entities.EscrowKeyScope, partyID uuid.UUID, purpose entities.EscrowKeyPurpose) (int, error) {
	vs, _ := r.ListByParty(ctx, scope, partyID, purpose)
	n := 1
	for _, v := range vs {
		if v.Version >= n {
			n = v.Version + 1
		}
	}
	return n, nil
}

// ApplyTransitions implements the repository.
func (r *Versions) ApplyTransitions(_ context.Context, ts []entities.EscrowKeyVersionTransition, audits []*entities.EscrowKeyAuditEvent) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(audits) == 0 {
		return entities.ErrInvalidEscrowKeyAuditEvent
	}
	for _, t := range ts {
		if cur, ok := r.rows[t.Version.ID]; !ok || cur.State != t.ExpectedState {
			return entities.ErrEscrowKeyVersionConflict
		}
	}
	for _, t := range ts {
		r.rows[t.Version.ID] = t.Version.Clone()
	}
	r.Audits = append(r.Audits, audits...)
	return nil
}

// Arrangements is an in-memory EscrowArrangementRepository.
type Arrangements struct {
	mu   sync.Mutex
	rows []*entities.EscrowArrangement
}

// NewArrangements creates an empty repository.
func NewArrangements() *Arrangements { return &Arrangements{} }

func copyArrangement(a *entities.EscrowArrangement) *entities.EscrowArrangement {
	c := *a
	return &c
}

// GetLive implements the repository.
func (r *Arrangements) GetLive(_ context.Context, scope entities.EscrowKeyScope, level entities.EscrowArrangementLevel, tld string, direction entities.EscrowArrangementDirection) (*entities.EscrowArrangement, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, a := range r.rows {
		if a.SupersededAt != nil || a.Level != level || a.Direction != direction {
			continue
		}
		switch level {
		case entities.EscrowArrangementPlatform:
			return copyArrangement(a), nil
		case entities.EscrowArrangementOperator:
			if !scope.IsPlatform() && a.Operator == scope.Operator() {
				return copyArrangement(a), nil
			}
		case entities.EscrowArrangementTLD:
			if !scope.IsPlatform() && a.Operator == scope.Operator() && a.TLD == tld {
				return copyArrangement(a), nil
			}
		}
	}
	return nil, entities.ErrEscrowArrangementNotFound
}

// ListLiveTLDOverrides implements the repository.
func (r *Arrangements) ListLiveTLDOverrides(_ context.Context, scope entities.OperatorID, direction entities.EscrowArrangementDirection) ([]*entities.EscrowArrangement, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []*entities.EscrowArrangement
	for _, a := range r.rows {
		if a.SupersededAt == nil && a.Level == entities.EscrowArrangementTLD && a.Operator == scope && a.Direction == direction {
			out = append(out, copyArrangement(a))
		}
	}
	return out, nil
}

// ListLiveReferencing implements the repository.
func (r *Arrangements) ListLiveReferencing(_ context.Context, scope entities.EscrowKeyScope, partyID uuid.UUID) ([]*entities.EscrowArrangement, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []*entities.EscrowArrangement
	for _, a := range r.rows {
		refers := (a.DepositorPartyID != nil && *a.DepositorPartyID == partyID) || (a.ReceiverPartyID != nil && *a.ReceiverPartyID == partyID)
		visible := a.Level == entities.EscrowArrangementPlatform || (!scope.IsPlatform() && a.Operator == scope.Operator())
		if a.SupersededAt == nil && refers && visible {
			out = append(out, copyArrangement(a))
		}
	}
	return out, nil
}

// Replace implements the repository.
func (r *Arrangements) Replace(_ context.Context, previous, next *entities.EscrowArrangement, _ *entities.EscrowKeyAuditEvent) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, a := range r.rows {
		sameSlot := a.Level == next.Level && a.Operator == next.Operator && a.TLD == next.TLD && a.Direction == next.Direction
		if !sameSlot || a.SupersededAt != nil {
			continue
		}
		if previous == nil || a.ID != previous.ID {
			return entities.ErrEscrowArrangementConflict
		}
		t := next.CreatedAt
		a.SupersededAt = &t
	}
	r.rows = append(r.rows, copyArrangement(next))
	return nil
}

// Remove implements the repository.
func (r *Arrangements) Remove(_ context.Context, current *entities.EscrowArrangement, _ *entities.EscrowKeyAuditEvent) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, a := range r.rows {
		if a.ID == current.ID && a.SupersededAt == nil {
			t := time.Now().UTC()
			if current.SupersededAt != nil {
				t = *current.SupersededAt
			}
			a.SupersededAt = &t
			return nil
		}
	}
	return entities.ErrEscrowArrangementConflict
}

// Set writes a live arrangement for the spec's slot, superseding any live one
// (test setup convenience).
func (r *Arrangements) Set(spec entities.EscrowArrangementSpec) (*entities.EscrowArrangement, error) {
	if spec.Direction == "" {
		spec.Direction = entities.EscrowArrangementInbound
	}
	if spec.At.IsZero() {
		spec.At = time.Now().UTC()
	}
	scope := entities.OperatorEscrowKeyScope(spec.Operator)
	if spec.Level == entities.EscrowArrangementPlatform {
		scope = entities.PlatformEscrowKeyScope(entities.NewPlatformScope())
	}
	prev, err := r.GetLive(context.Background(), scope, spec.Level, spec.TLD, spec.Direction)
	if err == nil {
		spec.Previous = prev
	}
	a, err := entities.NewEscrowArrangement(spec)
	if err != nil {
		return nil, err
	}
	return a, r.Replace(context.Background(), spec.Previous, a, nil)
}
