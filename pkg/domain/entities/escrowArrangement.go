package entities

import (
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
)

// EscrowArrangementLevel is how widely an arrangement applies.
type EscrowArrangementLevel string

const (
	// EscrowArrangementPlatform is the installation-wide default.
	EscrowArrangementPlatform EscrowArrangementLevel = "platform"
	// EscrowArrangementOperator is one operator's default for all its TLDs.
	EscrowArrangementOperator EscrowArrangementLevel = "operator"
	// EscrowArrangementTLD is an override for one TLD.
	EscrowArrangementTLD EscrowArrangementLevel = "tld"
)

// EscrowArrangementDirection is which way deposits flow. Only inbound exists;
// outbound is the escrow target and reuses this shape with sender/recipient.
type EscrowArrangementDirection string

// EscrowArrangementInbound is a deposit sent to us for validation.
const EscrowArrangementInbound EscrowArrangementDirection = "inbound"

// EscrowArrangement says which parties are on each side of a deposit flow at
// one level. A nil side inherits from the next wider level. Rows are
// immutable: a change supersedes the live row and writes the next revision, so
// a run can record exactly which revisions it resolved.
type EscrowArrangement struct {
	ID               uuid.UUID
	Level            EscrowArrangementLevel
	Operator         OperatorID // empty at platform level
	TLD              string     // set only at TLD level
	Direction        EscrowArrangementDirection
	DepositorPartyID *uuid.UUID // the RSP whose signing keys are trusted
	ReceiverPartyID  *uuid.UUID // our DEA identity whose keys decrypt and pseudonymise
	Revision         int
	CreatedAt        time.Time
	CreatedBy        string
	SupersededAt     *time.Time
}

// EscrowArrangementSpec is the input to NewEscrowArrangement. Parties are
// passed as entities so role and visibility are checked here, not at the edge.
type EscrowArrangementSpec struct {
	Level     EscrowArrangementLevel
	Operator  OperatorID
	TLD       string
	Direction EscrowArrangementDirection
	Depositor *EscrowParty
	Receiver  *EscrowParty
	// Previous is the live row being superseded at the same level, if any; the
	// new revision follows it.
	Previous  *EscrowArrangement
	CreatedBy string
	At        time.Time
}

// NewEscrowArrangement validates and creates the next revision of an arrangement.
func NewEscrowArrangement(spec EscrowArrangementSpec) (*EscrowArrangement, error) {
	if spec.Direction != EscrowArrangementInbound {
		return nil, errors.Join(ErrInvalidEscrowArrangement, errors.New("direction must be inbound"))
	}
	a := &EscrowArrangement{
		ID:        uuid.New(),
		Level:     spec.Level,
		Direction: spec.Direction,
		Revision:  1,
		CreatedBy: strings.TrimSpace(spec.CreatedBy),
	}
	switch spec.Level {
	case EscrowArrangementPlatform:
		if spec.Operator != "" || spec.TLD != "" {
			return nil, errors.Join(ErrInvalidEscrowArrangement, errors.New("a platform arrangement has no operator or TLD"))
		}
	case EscrowArrangementOperator, EscrowArrangementTLD:
		if err := spec.Operator.Validate(); err != nil {
			return nil, errors.Join(ErrInvalidEscrowArrangement, err)
		}
		a.Operator = spec.Operator
		if spec.Level == EscrowArrangementTLD {
			tld, err := NormalizeEscrowTLD(spec.TLD)
			if err != nil {
				return nil, errors.Join(ErrInvalidEscrowArrangement, err)
			}
			a.TLD = tld
		} else if spec.TLD != "" {
			return nil, errors.Join(ErrInvalidEscrowArrangement, errors.New("an operator arrangement has no TLD"))
		}
	default:
		return nil, errors.Join(ErrInvalidEscrowArrangement, errors.New("level must be platform, operator or tld"))
	}
	if spec.Depositor == nil && spec.Receiver == nil {
		return nil, errors.Join(ErrInvalidEscrowArrangement, errors.New("set a depositor, a receiver or both; clear an override by removing it"))
	}
	if spec.Depositor != nil {
		if !spec.Depositor.CanDeposit() {
			return nil, errors.Join(ErrInvalidEscrowArrangement, errors.New("the depositor must be an external RSP"))
		}
		if err := a.checkVisible(spec.Depositor); err != nil {
			return nil, err
		}
		id := spec.Depositor.ID
		a.DepositorPartyID = &id
	}
	if spec.Receiver != nil {
		if !spec.Receiver.CanReceive() {
			return nil, errors.Join(ErrInvalidEscrowArrangement, errors.New("the receiver must be one of our own DEA identities"))
		}
		if err := a.checkVisible(spec.Receiver); err != nil {
			return nil, err
		}
		id := spec.Receiver.ID
		a.ReceiverPartyID = &id
	}
	if spec.Previous != nil {
		p := spec.Previous
		if p.Level != a.Level || p.Operator != a.Operator || p.TLD != a.TLD || p.Direction != a.Direction {
			return nil, errors.Join(ErrInvalidEscrowArrangement, errors.New("the previous revision is for a different level, operator, TLD or direction"))
		}
		if p.SupersededAt != nil {
			return nil, ErrEscrowArrangementConflict
		}
		a.Revision = p.Revision + 1
	}
	if spec.At.IsZero() {
		return nil, errors.Join(ErrInvalidEscrowArrangement, errors.New("creation time is required"))
	}
	a.CreatedAt = RoundTime(spec.At.UTC())
	return a, nil
}

// checkVisible keeps an arrangement from pointing at a party its level cannot
// see: a platform default may only use platform parties, and an operator may
// use its own and the platform's, never another operator's.
func (a *EscrowArrangement) checkVisible(p *EscrowParty) error {
	if a.Level == EscrowArrangementPlatform {
		if !p.Owner.IsPlatform() {
			return errors.Join(ErrInvalidEscrowArrangement, ErrEscrowKeyOwnerMismatch)
		}
		return nil
	}
	if !p.Owner.VisibleTo(a.Operator) {
		return errors.Join(ErrInvalidEscrowArrangement, ErrEscrowKeyOwnerMismatch)
	}
	return nil
}

// Supersede marks the row as replaced or removed.
func (a *EscrowArrangement) Supersede(at time.Time) error {
	if a.SupersededAt != nil {
		return ErrEscrowArrangementConflict
	}
	if at.IsZero() {
		return errors.Join(ErrInvalidEscrowArrangement, errors.New("supersession time is required"))
	}
	t := at.UTC()
	a.SupersededAt = &t
	return nil
}

// EscrowArrangementSide is one resolved side of an effective arrangement.
type EscrowArrangementSide struct {
	PartyID uuid.UUID              `json:"partyId"`
	From    EscrowArrangementLevel `json:"from"` // the level it was inherited from
}

// EffectiveEscrowArrangement is what a TLD's deposits resolve to after
// inheritance, plus the revision of every level consulted (0 = no row).
type EffectiveEscrowArrangement struct {
	Depositor        *EscrowArrangementSide `json:"depositor,omitempty"`
	Receiver         *EscrowArrangementSide `json:"receiver,omitempty"`
	TLDRevision      int                    `json:"tldRevision"`
	OperatorRevision int                    `json:"operatorRevision"`
	PlatformRevision int                    `json:"platformRevision"`
}

// ResolveEscrowArrangement applies "most specific wins", independently for
// each side, over the live rows at TLD, operator and platform level. Any of
// them may be nil.
func ResolveEscrowArrangement(tld, operator, platform *EscrowArrangement) EffectiveEscrowArrangement {
	var eff EffectiveEscrowArrangement
	for _, a := range []*EscrowArrangement{tld, operator, platform} {
		if a == nil {
			continue
		}
		switch a.Level {
		case EscrowArrangementTLD:
			eff.TLDRevision = a.Revision
		case EscrowArrangementOperator:
			eff.OperatorRevision = a.Revision
		case EscrowArrangementPlatform:
			eff.PlatformRevision = a.Revision
		}
		if eff.Depositor == nil && a.DepositorPartyID != nil {
			eff.Depositor = &EscrowArrangementSide{PartyID: *a.DepositorPartyID, From: a.Level}
		}
		if eff.Receiver == nil && a.ReceiverPartyID != nil {
			eff.Receiver = &EscrowArrangementSide{PartyID: *a.ReceiverPartyID, From: a.Level}
		}
	}
	return eff
}
