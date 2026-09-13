package repositories

import (
	"context"

	"github.com/google/uuid"
	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
	"github.com/onasunnymorning/domain-os/pkg/domain/queries"
)

// Escrow key registry ports (issue #429, ADR-0009).
//
// Scope follows ADR-0006: reads take an entities.EscrowKeyScope immediately
// after ctx and enforce it in SQL (an operator sees its own rows and the
// platform's; the platform sees only its own). Writes carry the owner inside
// the entity.
//
// Every mutation is atomic with its audit entry and its outbox event: the
// state change, the escrow_key_audit_events row and the domain_events row are
// written in one transaction, so a committed change always has its audit trail
// and its event, and a rolled-back one has neither.

// EscrowPartyRepository persists parties. Parties are never deleted: runs and
// arrangements keep pointing at them.
type EscrowPartyRepository interface {
	Create(ctx context.Context, p *entities.EscrowParty, audit *entities.EscrowKeyAuditEvent) error
	GetByID(ctx context.Context, scope entities.EscrowKeyScope, id uuid.UUID) (*entities.EscrowParty, error)
	List(ctx context.Context, scope entities.EscrowKeyScope, q queries.ListItemsQuery) ([]*entities.EscrowParty, string, error)
}

// EscrowKeyVersionRepository persists key versions. There is no delete: a
// destroyed version keeps its metadata so the runs that used it stay
// explainable.
type EscrowKeyVersionRepository interface {
	// Create inserts a STAGED version. A fingerprint the party already holds for the
	// same purpose fails with entities.ErrEscrowKeyDuplicateFingerprint;
	// a version number taken concurrently fails with
	// entities.ErrEscrowKeyVersionConflict.
	Create(ctx context.Context, v *entities.EscrowKeyVersion, audit *entities.EscrowKeyAuditEvent) error
	GetByID(ctx context.Context, scope entities.EscrowKeyScope, id uuid.UUID) (*entities.EscrowKeyVersion, error)
	// GetByIDs returns the visible versions among ids, in no particular order.
	// Missing or invisible ids are simply absent.
	GetByIDs(ctx context.Context, scope entities.EscrowKeyScope, ids []uuid.UUID) ([]*entities.EscrowKeyVersion, error)
	// ListByParty returns a party's versions, newest version first. An empty
	// purpose lists every purpose.
	ListByParty(ctx context.Context, scope entities.EscrowKeyScope, partyID uuid.UUID, purpose entities.EscrowKeyPurpose) ([]*entities.EscrowKeyVersion, error)
	// NextVersionNumber returns one more than the highest version of the party
	// and purpose.
	NextVersionNumber(ctx context.Context, scope entities.EscrowKeyScope, partyID uuid.UUID, purpose entities.EscrowKeyPurpose) (int, error)
	// ApplyTransitions performs every transition and inserts every audit event
	// in one transaction. If any row is no longer in its expected state the
	// whole change is rolled back with entities.ErrEscrowKeyVersionConflict.
	ApplyTransitions(ctx context.Context, transitions []entities.EscrowKeyVersionTransition, audits []*entities.EscrowKeyAuditEvent) error
}

// EscrowArrangementRepository persists the immutable revisions of who is on
// each side of a deposit flow.
type EscrowArrangementRepository interface {
	// GetLive returns the live arrangement at one level, or
	// entities.ErrEscrowArrangementNotFound. An operator scope may read its own
	// operator and TLD levels and the platform level; the platform scope only
	// the platform level.
	GetLive(ctx context.Context, scope entities.EscrowKeyScope, level entities.EscrowArrangementLevel, tld string, direction entities.EscrowArrangementDirection) (*entities.EscrowArrangement, error)
	// ListLiveTLDOverrides returns an operator's live TLD-level rows.
	ListLiveTLDOverrides(ctx context.Context, scope entities.OperatorID, direction entities.EscrowArrangementDirection) ([]*entities.EscrowArrangement, error)
	// ListLiveReferencing returns the visible live rows that name the party on
	// either side — where a party is used.
	ListLiveReferencing(ctx context.Context, scope entities.EscrowKeyScope, partyID uuid.UUID) ([]*entities.EscrowArrangement, error)
	// Replace supersedes previous (nil when there is no live row) and inserts
	// next, atomically with the audit event. A previous that is no longer live,
	// or a concurrent first insert, fails with entities.ErrEscrowArrangementConflict.
	Replace(ctx context.Context, previous, next *entities.EscrowArrangement, audit *entities.EscrowKeyAuditEvent) error
	// Remove supersedes a live row without a replacement, so the level inherits again.
	Remove(ctx context.Context, current *entities.EscrowArrangement, audit *entities.EscrowKeyAuditEvent) error
}

// EscrowKeyAuditRepository reads the audit trail. Entries are written only by
// the other repositories, inside the transaction of the change they record.
type EscrowKeyAuditRepository interface {
	// ListByParty returns a party's history — the party, its versions — newest
	// first, cursor-paginated (INV-11).
	ListByParty(ctx context.Context, scope entities.EscrowKeyScope, partyID uuid.UUID, q queries.ListItemsQuery) ([]*entities.EscrowKeyAuditEvent, string, error)
}
