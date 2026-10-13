package entities

import (
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// EPP credentials: the logins a registrar's systems use, their passwords, the
// client-certificate pins and the source-IP allowlist. ADR 0014 records the
// decisions; docs/epp/ARCHITECTURE.md §2 describes the model.

var (
	ErrInvalidEPPCredential     = errors.New("invalid EPP credential")
	ErrEPPLoginNotFound         = errors.New("EPP login not found")
	ErrEPPLoginExists           = errors.New("an EPP login with this ID already exists")
	ErrEPPCredentialNotFound    = errors.New("EPP credential not found")
	ErrEPPCredentialRetired     = errors.New("EPP credential is already retired")
	ErrEPPCredentialLimit       = errors.New("too many active EPP credentials of this kind")
	ErrEPPCertificatePinExists  = errors.New("this certificate key is already pinned")
	ErrEPPAllowedNetworkExists  = errors.New("this network is already allowed")
	ErrInvalidEPPCertificate    = errors.New("invalid client certificate")
	ErrInvalidEPPAllowedNetwork = errors.New("invalid allowed network")
	ErrInvalidEPPPassword       = errors.New("invalid EPP password")
)

// EPPCredentialState is where a credential is in its lifecycle. It is derived
// from the credential's retire time, so a scheduled retirement takes effect
// without anything having to run at that moment.
type EPPCredentialState string

const (
	// EPPCredentialActive credentials are accepted at login.
	EPPCredentialActive EPPCredentialState = "active"
	// EPPCredentialRetiring credentials are still accepted at login, until
	// their retire time.
	EPPCredentialRetiring EPPCredentialState = "retiring"
	// EPPCredentialRetired credentials are refused at login.
	EPPCredentialRetired EPPCredentialState = "retired"
)

// EPPRetireKind is why a credential was retired.
type EPPRetireKind string

const (
	// EPPRetireNormal refuses new logins from the retire time on. Sessions that
	// already authenticated with the credential run until they end.
	EPPRetireNormal EPPRetireKind = "normal"
	// EPPRetireCompromise takes effect at once and also ends every live
	// session that authenticated with the credential.
	EPPRetireCompromise EPPRetireKind = "compromise"
	// EPPRetireReplaced is a password replaced through <login><newPW>, which
	// RFC 5730 says stops working at once.
	EPPRetireReplaced EPPRetireKind = "replaced"
)

// IsValid reports whether k is a known retire kind.
func (k EPPRetireKind) IsValid() bool {
	switch k {
	case EPPRetireNormal, EPPRetireCompromise, EPPRetireReplaced:
		return true
	}
	return false
}

// EPPCredentialLifecycle is the part every credential shares: when it was
// created, when it retires, and when and from where it was last used, so an
// operator can see when an old credential is safe to retire.
type EPPCredentialLifecycle struct {
	ID           uuid.UUID     `json:"id"`
	CreatedAt    time.Time     `json:"created_at"`
	CreatedBy    string        `json:"created_by,omitempty"`
	RetireAt     *time.Time    `json:"retire_at,omitempty"`
	RetireKind   EPPRetireKind `json:"retire_kind,omitempty"`
	RetiredBy    string        `json:"retired_by,omitempty"`
	LastUsedAt   *time.Time    `json:"last_used_at,omitempty"`
	LastUsedFrom string        `json:"last_used_from,omitempty"`
}

func newEPPCredentialLifecycle(by string, now time.Time) EPPCredentialLifecycle {
	return EPPCredentialLifecycle{ID: uuid.New(), CreatedAt: now.UTC(), CreatedBy: by}
}

// State is the credential's state at now.
func (l *EPPCredentialLifecycle) State(now time.Time) EPPCredentialState {
	switch {
	case l.RetireAt == nil:
		return EPPCredentialActive
	case now.Before(*l.RetireAt):
		return EPPCredentialRetiring
	default:
		return EPPCredentialRetired
	}
}

// Usable reports whether the credential is accepted at login at now.
func (l *EPPCredentialLifecycle) Usable(now time.Time) bool {
	return l.State(now) != EPPCredentialRetired
}

// Compromised reports whether the credential was retired as compromised.
func (l *EPPCredentialLifecycle) Compromised() bool {
	return l.RetireKind == EPPRetireCompromise
}

// DefaultEPPRetireWindow is how long a normal retirement leaves a credential
// usable when no retire time is given: long enough for a registrar to notice
// the announcement and switch.
const DefaultEPPRetireWindow = 7 * 24 * time.Hour

// retire schedules the credential's retirement. A normal retirement without
// at takes effect after DefaultEPPRetireWindow; an at that is not in the
// future retires it now. A compromise or a replacement always takes effect
// now. A scheduled retirement can be brought forward or pushed back until it
// has taken effect; a retired credential stays retired.
func (l *EPPCredentialLifecycle) retire(kind EPPRetireKind, at *time.Time, by string, now time.Time) error {
	if !kind.IsValid() {
		return fmt.Errorf("%w: unknown retire kind %q", ErrInvalidEPPCredential, kind)
	}
	if l.State(now) == EPPCredentialRetired {
		return ErrEPPCredentialRetired
	}
	effective := now.UTC()
	if kind == EPPRetireNormal {
		switch {
		case at == nil:
			effective = now.Add(DefaultEPPRetireWindow).UTC()
		case at.After(now):
			effective = at.UTC()
		}
	}
	l.RetireAt = &effective
	l.RetireKind = kind
	l.RetiredBy = by
	return nil
}

// EPPCredentialUse records one successful login's use of a credential.
type EPPCredentialUse struct {
	ID   uuid.UUID
	At   time.Time
	From string
}

// recordUse moves the last-used time forward; it never moves it back.
func (l *EPPCredentialLifecycle) recordUse(at time.Time, from string) {
	if l.LastUsedAt != nil && !at.After(*l.LastUsedAt) {
		return
	}
	t := at.UTC()
	l.LastUsedAt = &t
	l.LastUsedFrom = from
}

// countUsable counts the credentials accepted at login at now.
func countUsable[T interface {
	lifecycle() *EPPCredentialLifecycle
}](items []T, now time.Time) int {
	n := 0
	for _, it := range items {
		if it.lifecycle().Usable(now) {
			n++
		}
	}
	return n
}
