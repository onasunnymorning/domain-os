package entities

import (
	"errors"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
)

var pgpFingerprintRe = regexp.MustCompile(`^([0-9A-F]{40}|[0-9A-F]{64})$`)

const pgpPublicKeyArmorHeader = "-----BEGIN PGP PUBLIC KEY BLOCK-----"

// EscrowTrustedKey is a registry operator's OpenPGP signing key that this
// service trusts for one TLD. It is application data, not a secret: the key is
// public. Overlapping validity windows for the same TLD are normal — a
// registry rolls its key on its own schedule, and deposits signed by either
// key must verify during the overlap.
//
// Retirement is a timestamp, never a delete, so a run that verified against a
// since-retired key still points at the row it used.
type EscrowTrustedKey struct {
	ID               uuid.UUID
	TenantID         OperatorID
	TLD              string
	Fingerprint      string // Upper-case hex, 40 (v4) or 64 (v5/v6) characters
	ArmoredPublicKey string
	Label            string
	ValidFrom        time.Time
	ValidTo          *time.Time // nil = open-ended
	RetiredAt        *time.Time // nil = not retired
	CreatedAt        time.Time
}

// NewEscrowTrustedKey validates and creates a trusted key registration. The
// fingerprint is expected to have been computed from the armored key by the
// caller (the domain does not parse OpenPGP); it is normalised to upper case.
func NewEscrowTrustedKey(scope OperatorID, tld, fingerprint, armoredPublicKey, label string, validFrom time.Time, validTo *time.Time) (*EscrowTrustedKey, error) {
	if err := scope.Validate(); err != nil {
		return nil, errors.Join(ErrInvalidEscrowTrustedKey, err)
	}
	normTLD, err := NormalizeEscrowTLD(tld)
	if err != nil {
		return nil, errors.Join(ErrInvalidEscrowTrustedKey, err)
	}
	fp := strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(fingerprint), " ", ""))
	if !pgpFingerprintRe.MatchString(fp) {
		return nil, errors.Join(ErrInvalidEscrowTrustedKey, ErrEscrowTrustedKeyInvalidFingerprint)
	}
	armored := strings.TrimSpace(armoredPublicKey)
	if !strings.HasPrefix(armored, pgpPublicKeyArmorHeader) {
		return nil, errors.Join(ErrInvalidEscrowTrustedKey, errors.New("armoredPublicKey must be an ASCII-armored PGP public key block"))
	}
	if validFrom.IsZero() {
		return nil, errors.Join(ErrInvalidEscrowTrustedKey, errors.New("validFrom is required"))
	}
	if validTo != nil && !validTo.After(validFrom) {
		return nil, errors.Join(ErrInvalidEscrowTrustedKey, ErrEscrowTrustedKeyInvalidWindow)
	}
	var vt *time.Time
	if validTo != nil {
		t := validTo.UTC()
		vt = &t
	}
	return &EscrowTrustedKey{
		ID:               uuid.New(),
		TenantID:         scope,
		TLD:              normTLD,
		Fingerprint:      fp,
		ArmoredPublicKey: armored,
		Label:            strings.TrimSpace(label),
		ValidFrom:        validFrom.UTC(),
		ValidTo:          vt,
		CreatedAt:        RoundTime(time.Now().UTC()),
	}, nil
}

// Active reports whether the key may be used to verify a signature at the
// given instant: inside its validity window and not retired.
func (k *EscrowTrustedKey) Active(at time.Time) bool {
	if k.RetiredAt != nil && !at.Before(*k.RetiredAt) {
		return false
	}
	if at.Before(k.ValidFrom) {
		return false
	}
	if k.ValidTo != nil && !at.Before(*k.ValidTo) {
		return false
	}
	return true
}

// Retire marks the key as no longer trusted from the given instant.
func (k *EscrowTrustedKey) Retire(at time.Time) error {
	if k.RetiredAt != nil {
		return ErrEscrowTrustedKeyAlreadyRetired
	}
	if at.IsZero() {
		return errors.Join(ErrInvalidEscrowTrustedKey, errors.New("retirement time is required"))
	}
	t := at.UTC()
	k.RetiredAt = &t
	return nil
}
