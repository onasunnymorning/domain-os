package entities

import (
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
)

// EscrowKeyAuditAction names a change to the key registry. Each action is also
// the suffix of the outbox event type, so the audit trail and the event stream
// share one vocabulary.
type EscrowKeyAuditAction string

const (
	EscrowAuditPartyCreated          EscrowKeyAuditAction = "party.created"
	EscrowAuditVersionImported       EscrowKeyAuditAction = "key_version.imported"
	EscrowAuditVersionGenerated      EscrowKeyAuditAction = "key_version.generated"
	EscrowAuditVersionPublicKeyAdded EscrowKeyAuditAction = "key_version.public_key_added"
	EscrowAuditVersionProbePassed    EscrowKeyAuditAction = "key_version.probe_passed"
	EscrowAuditVersionProbeFailed    EscrowKeyAuditAction = "key_version.probe_failed"
	EscrowAuditVersionActivated      EscrowKeyAuditAction = "key_version.activated"
	EscrowAuditVersionDeactivated    EscrowKeyAuditAction = "key_version.deactivated"
	EscrowAuditVersionRevoked        EscrowKeyAuditAction = "key_version.revoked"
	EscrowAuditVersionDestroyed      EscrowKeyAuditAction = "key_version.destroyed"
	EscrowAuditArrangementChanged    EscrowKeyAuditAction = "arrangement.changed"
	EscrowAuditArrangementRemoved    EscrowKeyAuditAction = "arrangement.removed"
	escrowKeyEventTypePrefix                              = "escrow."
	escrowKeyEventSource                                  = "domain-os/escrow-keys"
)

// EscrowKeyAuditSubject is the kind of record an audit event is about.
type EscrowKeyAuditSubject string

const (
	EscrowAuditSubjectParty       EscrowKeyAuditSubject = "party"
	EscrowAuditSubjectKeyVersion  EscrowKeyAuditSubject = "key_version"
	EscrowAuditSubjectArrangement EscrowKeyAuditSubject = "arrangement"
)

// EscrowKeyAuditEvent is one immutable entry in a party's history. It holds
// identifiers, public data and constant vocabulary only — never key material,
// a passphrase or a secret value. It is written in the same transaction as the
// change it records, together with its outbox event (ADR-0009).
type EscrowKeyAuditEvent struct {
	ID            uuid.UUID
	Owner         EscrowKeyOwner
	Action        EscrowKeyAuditAction
	Subject       EscrowKeyAuditSubject
	SubjectID     uuid.UUID
	PartyID       *uuid.UUID
	Purpose       EscrowKeyPurpose
	Version       int
	Fingerprint   string
	StateBefore   string
	StateAfter    string
	TLD           string
	Reason        string
	Compromised   bool
	Actor         string
	TraceID       string
	CorrelationID string
	At            time.Time
}

// EscrowKeyAuditContext carries who and what request caused a change.
type EscrowKeyAuditContext struct {
	Actor         string
	TraceID       string
	CorrelationID string
	At            time.Time
}

func newEscrowKeyAuditEvent(c EscrowKeyAuditContext, owner EscrowKeyOwner, action EscrowKeyAuditAction, subject EscrowKeyAuditSubject, subjectID uuid.UUID) (*EscrowKeyAuditEvent, error) {
	if err := owner.Validate(); err != nil {
		return nil, errors.Join(ErrInvalidEscrowKeyAuditEvent, err)
	}
	if subjectID == uuid.Nil {
		return nil, errors.Join(ErrInvalidEscrowKeyAuditEvent, errors.New("subject id is required"))
	}
	if c.At.IsZero() {
		return nil, errors.Join(ErrInvalidEscrowKeyAuditEvent, errors.New("event time is required"))
	}
	return &EscrowKeyAuditEvent{
		ID: uuid.New(), Owner: owner, Action: action, Subject: subject, SubjectID: subjectID,
		Actor: strings.TrimSpace(c.Actor), TraceID: c.TraceID, CorrelationID: c.CorrelationID, At: c.At.UTC(),
	}, nil
}

// NewEscrowPartyAuditEvent records a change to a party.
func NewEscrowPartyAuditEvent(c EscrowKeyAuditContext, p *EscrowParty, action EscrowKeyAuditAction) (*EscrowKeyAuditEvent, error) {
	e, err := newEscrowKeyAuditEvent(c, p.Owner, action, EscrowAuditSubjectParty, p.ID)
	if err != nil {
		return nil, err
	}
	id := p.ID
	e.PartyID = &id
	return e, nil
}

// NewEscrowKeyVersionAuditEvent records a change to a version. before may be
// nil for a creation.
func NewEscrowKeyVersionAuditEvent(c EscrowKeyAuditContext, before, after *EscrowKeyVersion, action EscrowKeyAuditAction) (*EscrowKeyAuditEvent, error) {
	if after == nil {
		return nil, errors.Join(ErrInvalidEscrowKeyAuditEvent, errors.New("the version is required"))
	}
	e, err := newEscrowKeyAuditEvent(c, after.Owner, action, EscrowAuditSubjectKeyVersion, after.ID)
	if err != nil {
		return nil, err
	}
	party := after.PartyID
	e.PartyID = &party
	e.Purpose, e.Version, e.Fingerprint = after.Purpose, after.Version, after.Fingerprint
	e.StateAfter = string(after.State)
	if before != nil {
		e.StateBefore = string(before.State)
	}
	e.Reason, e.Compromised = after.RevocationReason, after.Compromised
	return e, nil
}

// NewEscrowArrangementAuditEvent records a change to an arrangement. owner is
// the platform for a platform arrangement and the operator otherwise.
func NewEscrowArrangementAuditEvent(c EscrowKeyAuditContext, a *EscrowArrangement, action EscrowKeyAuditAction) (*EscrowKeyAuditEvent, error) {
	owner := PlatformKeyOwner()
	if a.Level != EscrowArrangementPlatform {
		owner = OperatorKeyOwner(a.Operator)
	}
	e, err := newEscrowKeyAuditEvent(c, owner, action, EscrowAuditSubjectArrangement, a.ID)
	if err != nil {
		return nil, err
	}
	e.TLD, e.Version = a.TLD, a.Revision
	return e, nil
}

// EventType is the outbox event type, e.g. "escrow.key_version.revoked".
func (e *EscrowKeyAuditEvent) EventType() string {
	return escrowKeyEventTypePrefix + string(e.Action)
}

// escrowKeyEventData is the outbox payload. It mirrors the audit row and is
// built only from it, so the two cannot disagree and neither can carry material.
type escrowKeyEventData struct {
	AuditEventID string `json:"auditEventId"`
	Owner        string `json:"owner"`
	Subject      string `json:"subject"`
	SubjectID    string `json:"subjectId"`
	PartyID      string `json:"partyId,omitempty"`
	Purpose      string `json:"purpose,omitempty"`
	Version      int    `json:"version,omitempty"`
	Fingerprint  string `json:"fingerprint,omitempty"`
	StateBefore  string `json:"stateBefore,omitempty"`
	StateAfter   string `json:"stateAfter,omitempty"`
	TLD          string `json:"tld,omitempty"`
	Reason       string `json:"reason,omitempty"`
	Compromised  bool   `json:"compromised,omitempty"`
}

// DomainEvent builds the outbox event for this audit entry.
func (e *EscrowKeyAuditEvent) DomainEvent() DomainEvent {
	data := escrowKeyEventData{
		AuditEventID: e.ID.String(), Owner: e.Owner.String(), Subject: string(e.Subject), SubjectID: e.SubjectID.String(),
		Purpose: string(e.Purpose), Version: e.Version, Fingerprint: e.Fingerprint,
		StateBefore: e.StateBefore, StateAfter: e.StateAfter, TLD: e.TLD, Reason: e.Reason, Compromised: e.Compromised,
	}
	if e.PartyID != nil {
		data.PartyID = e.PartyID.String()
	}
	ev := NewDomainEvent(escrowKeyEventSource, e.EventType(), e.SubjectID.String(), string(e.Action), data)
	ev.Time = e.At
	ev.Actor, ev.TraceID, ev.CorrelationID = e.Actor, e.TraceID, e.CorrelationID
	return ev
}
