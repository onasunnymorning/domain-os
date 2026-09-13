package commands

import (
	"time"

	"github.com/google/uuid"
)

// Escrow key registry write inputs (issue #429, ADR-0009). Business rules live
// in the entities; these carry what the caller supplied. Material fields are
// held only for the duration of the request and are never logged, returned or
// persisted outside the key store.

// CreateEscrowPartyCommand registers a party. The owner is the request's key
// scope: an operator, or the platform.
type CreateEscrowPartyCommand struct {
	Name string
	Kind string // RSP | DEA
	Side string // self | external
}

// ImportEscrowPrivateKeyCommand imports an OpenPGP private key as a new version
// of a party's key for Purpose.
type ImportEscrowPrivateKeyCommand struct {
	Purpose           string
	ArmoredPrivateKey string
	Passphrase        string //nolint:gosec // G117: the request field that carries the passphrase to the key store
	NotBefore         *time.Time
	NotAfter          *time.Time
}

// AddEscrowPublicKeyCommand adds a counterparty's OpenPGP public key as a new
// version of a party's key for Purpose.
type AddEscrowPublicKeyCommand struct {
	Purpose          string
	ArmoredPublicKey string
	NotBefore        *time.Time
	NotAfter         *time.Time
}

// GenerateEscrowSymmetricKeyCommand has the service create a symmetric key
// version (pseudonymisation) that nobody ever sees.
type GenerateEscrowSymmetricKeyCommand struct {
	Purpose string
}

// ActivateEscrowKeyVersionCommand activates a version. ConfirmReplace must be
// set when the purpose allows one active version and another one is active.
type ActivateEscrowKeyVersionCommand struct {
	ConfirmReplace bool
}

// RevokeEscrowKeyVersionCommand withdraws a version from every use.
type RevokeEscrowKeyVersionCommand struct {
	Reason      string
	Compromised bool
}

// DestroyEscrowKeyVersionCommand destroys a version's material. The caller
// repeats the fingerprint, so destroying the wrong key takes two mistakes.
type DestroyEscrowKeyVersionCommand struct {
	ConfirmFingerprint string
}

// SetEscrowArrangementCommand sets the depositor and/or receiver at one level.
// A nil side inherits from the next wider level.
type SetEscrowArrangementCommand struct {
	DepositorPartyID *uuid.UUID
	ReceiverPartyID  *uuid.UUID
}
