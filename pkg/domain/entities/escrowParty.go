package entities

import (
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
)

// EscrowPartyKind is the role an organisation plays in escrow.
type EscrowPartyKind string

const (
	// EscrowPartyRSP is a registry service provider: it produces and signs deposits.
	EscrowPartyRSP EscrowPartyKind = "RSP"
	// EscrowPartyDEA is a data escrow agent: it receives, decrypts and verifies deposits.
	EscrowPartyDEA EscrowPartyKind = "DEA"
)

// EscrowPartySide says whose keys these are.
type EscrowPartySide string

const (
	// EscrowPartySelf is an identity this installation operates. We hold its
	// private and symmetric keys (in the secret backend).
	EscrowPartySelf EscrowPartySide = "self"
	// EscrowPartyExternal is a counterparty. We only ever hold its public keys.
	EscrowPartyExternal EscrowPartySide = "external"
)

// EscrowParty is an organisation on one side of an escrow flow, and the thing
// keys belong to. See ADR-0009.
type EscrowParty struct {
	ID        uuid.UUID
	Owner     EscrowKeyOwner
	Name      string
	Kind      EscrowPartyKind
	Side      EscrowPartySide
	CreatedAt time.Time
	CreatedBy string
}

// EscrowPartyRole is what a party is in the direction deposits travel.
//
// It is derived from Kind and Side and adds no state. Kind and side stay the
// record's classification, because the resolver, the arrangements and the
// persisted audit match on them; the role is the same fact in the words the
// operator-facing surfaces use, so nobody outside this package has to know
// that an escrow source is an RSP on the external side.
type EscrowPartyRole string

const (
	// EscrowRoleSource sends escrow deposits to us and signs them.
	EscrowRoleSource EscrowPartyRole = "source"
	// EscrowRoleReceivingIdentity is one of our own identities: deposits sent
	// to us are encrypted to it, and we open them with its private key.
	EscrowRoleReceivingIdentity EscrowPartyRole = "receiving-identity"
	// EscrowRoleSendingIdentity is one of our own identities, which signs the
	// deposits we send out. Reserved: it arrives with escrow targets.
	EscrowRoleSendingIdentity EscrowPartyRole = "sending-identity"
	// EscrowRoleDestination receives escrow deposits from us. Reserved, as above.
	EscrowRoleDestination EscrowPartyRole = "destination"
)

// escrowPartyPurposes derives what a party's keys may be used for, and
// escrowPartyRole derives what that party is called wherever a person reads
// it. Both are functions of kind x side, so the table below is the whole
// mapping between this record's classification and the vocabulary of the API
// and the interface:
//
//	kind x side      role                 purposes                       the interface calls it
//	DEA x self       receiving-identity   decrypt-inbound, pseudonymise  one of our receiving identities
//	RSP x external   source               verify-inbound                 an escrow source
//	RSP x self       sending-identity     sign-outbound (reserved)       one of our sending identities
//	DEA x external   destination          encrypt-outbound (reserved)    an escrow destination
//
// The purposes are named for a person in the same way: decrypt-inbound is the
// receiving identity's "decryption key", verify-inbound is the source's
// "verification key", and pseudonymise is the "pseudonymisation key".
//
// Reserved roles return purposes whose policy is not Supported, which is what
// NewEscrowParty refuses on.
func escrowPartyPurposes(kind EscrowPartyKind, side EscrowPartySide) []EscrowKeyPurpose {
	switch {
	case kind == EscrowPartyDEA && side == EscrowPartySelf:
		return []EscrowKeyPurpose{EscrowKeyPurposeDecryptInbound, EscrowKeyPurposePseudonymise}
	case kind == EscrowPartyRSP && side == EscrowPartyExternal:
		return []EscrowKeyPurpose{EscrowKeyPurposeVerifyInbound}
	case kind == EscrowPartyRSP && side == EscrowPartySelf:
		return []EscrowKeyPurpose{EscrowKeyPurposeSignOutbound}
	case kind == EscrowPartyDEA && side == EscrowPartyExternal:
		return []EscrowKeyPurpose{EscrowKeyPurposeEncryptOutbound}
	default:
		return nil
	}
}

// escrowPartyRole names the party in the direction deposits travel. See the
// table at escrowPartyPurposes.
func escrowPartyRole(kind EscrowPartyKind, side EscrowPartySide) EscrowPartyRole {
	switch {
	case kind == EscrowPartyDEA && side == EscrowPartySelf:
		return EscrowRoleReceivingIdentity
	case kind == EscrowPartyRSP && side == EscrowPartyExternal:
		return EscrowRoleSource
	case kind == EscrowPartyRSP && side == EscrowPartySelf:
		return EscrowRoleSendingIdentity
	case kind == EscrowPartyDEA && side == EscrowPartyExternal:
		return EscrowRoleDestination
	default:
		return ""
	}
}

// NewEscrowParty validates and creates a party. Only the roles that have a
// supported purpose today can be created: our own DEA identity and external
// registry service providers. The outbound roles arrive with escrow targets.
func NewEscrowParty(owner EscrowKeyOwner, name string, kind EscrowPartyKind, side EscrowPartySide, createdBy string, at time.Time) (*EscrowParty, error) {
	if err := owner.Validate(); err != nil {
		return nil, errors.Join(ErrInvalidEscrowParty, err)
	}
	name = strings.TrimSpace(name)
	if name == "" || len(name) > escrowNameMaxLen {
		return nil, errors.Join(ErrInvalidEscrowParty, errors.New("name is required and at most 128 characters"))
	}
	purposes := escrowPartyPurposes(kind, side)
	if purposes == nil {
		return nil, errors.Join(ErrInvalidEscrowParty, errors.New("kind must be RSP or DEA and side must be self or external"))
	}
	supported := false
	for _, p := range purposes {
		if escrowKeyPurposePolicies[p].Supported {
			supported = true
		}
	}
	if !supported {
		return nil, ErrEscrowPartyRoleNotSupported
	}
	if at.IsZero() {
		return nil, errors.Join(ErrInvalidEscrowParty, errors.New("creation time is required"))
	}
	return &EscrowParty{
		ID:        uuid.New(),
		Owner:     owner,
		Name:      name,
		Kind:      kind,
		Side:      side,
		CreatedAt: RoundTime(at.UTC()),
		CreatedBy: strings.TrimSpace(createdBy),
	}, nil
}

// Purposes returns what this party's keys may be used for.
func (p *EscrowParty) Purposes() []EscrowKeyPurpose {
	return escrowPartyPurposes(p.Kind, p.Side)
}

// Role returns what this party is in the direction deposits travel. It is
// empty only for a kind/side combination that cannot exist.
func (p *EscrowParty) Role() EscrowPartyRole {
	return escrowPartyRole(p.Kind, p.Side)
}

// AllowsPurpose reports whether a key of this purpose may belong to the party,
// and whether that purpose is supported yet.
func (p *EscrowParty) AllowsPurpose(purpose EscrowKeyPurpose) error {
	for _, allowed := range p.Purposes() {
		if allowed == purpose {
			if !escrowKeyPurposePolicies[purpose].Supported {
				return ErrEscrowKeyPurposeNotSupported
			}
			return nil
		}
	}
	return ErrEscrowKeyPurposeNotAllowed
}

// CanDeposit reports whether the party can be the depositor of an inbound flow.
func (p *EscrowParty) CanDeposit() bool {
	return p.Kind == EscrowPartyRSP && p.Side == EscrowPartyExternal
}

// CanReceive reports whether the party can be the receiver of an inbound flow.
func (p *EscrowParty) CanReceive() bool {
	return p.Kind == EscrowPartyDEA && p.Side == EscrowPartySelf
}
