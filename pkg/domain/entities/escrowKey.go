package entities

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

// ---------------------------------------------------------------------------
// Escrow key registry — issue #429, ADR-0009.
//
// Keys belong to parties, not to TLDs: a Data Escrow Agent has one encryption
// key whatever TLD a deposit is for, and a registry service provider signs
// every deposit with one signing key. A TLD only decides which parties are on
// each side of its deposits (EscrowArrangement). What a key may be used for is
// derived from the party it belongs to, never chosen freely, and everything
// that differs between kinds of key is one row of EscrowKeyPurposePolicy.
//
// Private and symmetric material never enters this package: a version carries
// a reference to the secret backend (EscrowSecretRef) and public data only.
// ---------------------------------------------------------------------------

// Escrow key registry sentinel errors (INV-09).
var (
	ErrInvalidEscrowKeyOwner          = errors.New("invalid escrow key owner")
	ErrInvalidEscrowParty             = errors.New("invalid escrow party")
	ErrEscrowPartyNotFound            = errors.New("escrow party not found")
	ErrEscrowPartyRoleNotSupported    = errors.New("escrow party role is not supported yet")
	ErrInvalidEscrowKeyVersion        = errors.New("invalid escrow key version")
	ErrEscrowKeyVersionNotFound       = errors.New("escrow key version not found")
	ErrEscrowKeyPurposeNotAllowed     = errors.New("escrow key purpose is not allowed for this party")
	ErrEscrowKeyPurposeNotSupported   = errors.New("escrow key purpose is not supported yet")
	ErrEscrowKeyInvalidTransition     = errors.New("escrow key version cannot make this state transition")
	ErrEscrowKeyNotProbed             = errors.New("escrow key version has no successful probe")
	ErrEscrowKeyDuplicateFingerprint  = errors.New("an escrow key version with this fingerprint already exists")
	ErrEscrowKeyVersionConflict       = errors.New("escrow key version changed concurrently")
	ErrEscrowKeyReplaceNotConfirmed   = errors.New("activating this version replaces the active one and must be confirmed")
	ErrInvalidEscrowArrangement       = errors.New("invalid escrow arrangement")
	ErrEscrowArrangementNotFound      = errors.New("escrow arrangement not found")
	ErrEscrowArrangementConflict      = errors.New("escrow arrangement changed concurrently")
	ErrInvalidEscrowKeyAuditEvent     = errors.New("invalid escrow key audit event")
	ErrEscrowKeyOwnerMismatch         = errors.New("escrow party is not visible to this owner")
	ErrEscrowKeyMaterialNotApplicable = errors.New("escrow key material does not match the purpose")
	ErrEscrowKeyInvalidWindow         = errors.New("escrow key validity window is invalid")
	ErrEscrowKeyInvalidFingerprint    = errors.New("escrow key fingerprint must be 40 or 64 hex characters")

	// Custody errors. Their text is fixed and never echoes material, a secret
	// name or a backend message, because they reach logs and workflow history.
	ErrEscrowKeyStoreNotConfigured  = errors.New("no escrow key store is configured")
	ErrEscrowSecretNotFound         = errors.New("escrow key secret not found in the key store")
	ErrEscrowSecretAlreadyExists    = errors.New("an escrow key secret with this name already exists")
	ErrEscrowKeyStoreUnavailable    = errors.New("escrow key store is unavailable")
	ErrEscrowKeyMaterialUnreadable  = errors.New("escrow key material could not be read or unlocked")
	ErrEscrowKeyFingerprintMismatch = errors.New("escrow key material does not match the recorded fingerprint")
)

// ---------------------------------------------------------------------------
// Owner
// ---------------------------------------------------------------------------

// EscrowKeyOwnerKind says whether a party is installation-wide or belongs to
// one registry operator.
type EscrowKeyOwnerKind string

const (
	// EscrowKeyOwnerPlatform marks an installation-wide party: our own DEA
	// identity, or a catalogue entry for a registry service provider that many
	// operators use. Managed under PlatformScope; usable by every operator.
	EscrowKeyOwnerPlatform EscrowKeyOwnerKind = "platform"
	// EscrowKeyOwnerOperator marks a party that belongs to one operator and is
	// invisible to every other operator.
	EscrowKeyOwnerOperator EscrowKeyOwnerKind = "operator"
)

// EscrowKeyOwner is who owns a party and, through it, its keys and audit trail.
type EscrowKeyOwner struct {
	Kind     EscrowKeyOwnerKind
	Operator OperatorID // empty for the platform
}

// PlatformKeyOwner returns the installation-wide owner.
func PlatformKeyOwner() EscrowKeyOwner {
	return EscrowKeyOwner{Kind: EscrowKeyOwnerPlatform}
}

// OperatorKeyOwner returns the owner for one registry operator.
func OperatorKeyOwner(op OperatorID) EscrowKeyOwner {
	return EscrowKeyOwner{Kind: EscrowKeyOwnerOperator, Operator: op}
}

// Validate checks that the owner is one of the two kinds and is well formed.
func (o EscrowKeyOwner) Validate() error {
	switch o.Kind {
	case EscrowKeyOwnerPlatform:
		if o.Operator != "" {
			return errors.Join(ErrInvalidEscrowKeyOwner, errors.New("a platform owner has no operator"))
		}
		return nil
	case EscrowKeyOwnerOperator:
		if err := o.Operator.Validate(); err != nil {
			return errors.Join(ErrInvalidEscrowKeyOwner, err)
		}
		return nil
	default:
		return errors.Join(ErrInvalidEscrowKeyOwner, errors.New("owner kind must be platform or operator"))
	}
}

// IsPlatform reports whether the owner is the platform.
func (o EscrowKeyOwner) IsPlatform() bool { return o.Kind == EscrowKeyOwnerPlatform }

// VisibleTo reports whether an operator may see and reference what this owner
// owns: its own things and the platform's, never another operator's.
func (o EscrowKeyOwner) VisibleTo(op OperatorID) bool {
	return o.IsPlatform() || (o.Kind == EscrowKeyOwnerOperator && o.Operator == op)
}

// String renders "platform" or "operator:<RyID>".
func (o EscrowKeyOwner) String() string {
	if o.IsPlatform() {
		return string(EscrowKeyOwnerPlatform)
	}
	return fmt.Sprintf("%s:%s", EscrowKeyOwnerOperator, o.Operator)
}

// ---------------------------------------------------------------------------
// Purposes and their policies
// ---------------------------------------------------------------------------

// EscrowKeyPurpose is what a key may be used for.
type EscrowKeyPurpose string

const (
	// EscrowKeyPurposeDecryptInbound decrypts .ryde deposits sent to our DEA identity.
	EscrowKeyPurposeDecryptInbound EscrowKeyPurpose = "decrypt-inbound"
	// EscrowKeyPurposeVerifyInbound verifies the detached signature of a registry service provider.
	EscrowKeyPurposeVerifyInbound EscrowKeyPurpose = "verify-inbound"
	// EscrowKeyPurposePseudonymise is the master HMAC key sanitized derivatives are tokenised with.
	EscrowKeyPurposePseudonymise EscrowKeyPurpose = "pseudonymise"
	// EscrowKeyPurposeSignOutbound is reserved for escrow targets: sign our own deposits.
	EscrowKeyPurposeSignOutbound EscrowKeyPurpose = "sign-outbound"
	// EscrowKeyPurposeEncryptOutbound is reserved for escrow targets: encrypt to a DEA.
	EscrowKeyPurposeEncryptOutbound EscrowKeyPurpose = "encrypt-outbound"
)

// EscrowKeyMaterial is the kind of material a purpose uses.
type EscrowKeyMaterial string

const (
	// EscrowKeyMaterialOpenPGPPrivate lives in the secret backend; the version keeps its public half.
	EscrowKeyMaterialOpenPGPPrivate EscrowKeyMaterial = "openpgp-private"
	// EscrowKeyMaterialOpenPGPPublic is application data held on the version itself.
	EscrowKeyMaterialOpenPGPPublic EscrowKeyMaterial = "openpgp-public"
	// EscrowKeyMaterialSymmetric lives in the secret backend and has no public half.
	EscrowKeyMaterialSymmetric EscrowKeyMaterial = "symmetric"
)

// EscrowKeyPurposePolicy is everything that differs between kinds of key.
// Parties, versions, states, audit and run evidence are shared by all of them.
type EscrowKeyPurposePolicy struct {
	Purpose  EscrowKeyPurpose
	Material EscrowKeyMaterial
	// Supported is false for purposes reserved for escrow targets.
	Supported bool
	// RequiresProbe means a version must have been proven usable on a worker
	// before it can be activated. Public keys need no worker.
	RequiresProbe bool
	// Generatable means the service may create the material itself.
	Generatable bool
	// MaxActive bounds how many versions may be ACTIVE at once; 0 is unbounded.
	// Rollover purposes need overlap; a pseudonymisation key must not have it,
	// because two live keys would make one derivative's tokens unjoinable with
	// the next without anyone having decided that.
	MaxActive int
	// HistoricalByReceipt lets a HISTORICAL version still open deposits that
	// were received before it was deactivated. When false, a HISTORICAL version
	// is only ever used again by a retry of a run that already recorded it.
	HistoricalByReceipt bool
}

var escrowKeyPurposePolicies = map[EscrowKeyPurpose]EscrowKeyPurposePolicy{
	EscrowKeyPurposeDecryptInbound: {
		Purpose: EscrowKeyPurposeDecryptInbound, Material: EscrowKeyMaterialOpenPGPPrivate,
		Supported: true, RequiresProbe: true, HistoricalByReceipt: true,
	},
	EscrowKeyPurposeVerifyInbound: {
		Purpose: EscrowKeyPurposeVerifyInbound, Material: EscrowKeyMaterialOpenPGPPublic,
		Supported: true, HistoricalByReceipt: true,
	},
	EscrowKeyPurposePseudonymise: {
		Purpose: EscrowKeyPurposePseudonymise, Material: EscrowKeyMaterialSymmetric,
		Supported: true, RequiresProbe: true, Generatable: true, MaxActive: 1,
	},
	EscrowKeyPurposeSignOutbound: {
		Purpose: EscrowKeyPurposeSignOutbound, Material: EscrowKeyMaterialOpenPGPPrivate, RequiresProbe: true,
	},
	EscrowKeyPurposeEncryptOutbound: {
		Purpose: EscrowKeyPurposeEncryptOutbound, Material: EscrowKeyMaterialOpenPGPPublic,
	},
}

// EscrowKeyPurposePolicyFor returns the policy for a purpose.
func EscrowKeyPurposePolicyFor(p EscrowKeyPurpose) (EscrowKeyPurposePolicy, error) {
	policy, ok := escrowKeyPurposePolicies[p]
	if !ok {
		return EscrowKeyPurposePolicy{}, errors.Join(ErrInvalidEscrowKeyVersion, fmt.Errorf("unknown key purpose %q", p))
	}
	return policy, nil
}

var (
	pgpFingerprintRe       = regexp.MustCompile(`^([0-9A-F]{40}|[0-9A-F]{64})$`)
	symmetricFingerprintRe = regexp.MustCompile(`^[A-Z2-7]{16}$`)
	escrowNameMaxLen       = 128
)

// NormalizeEscrowKeyFingerprint normalises and validates a fingerprint for the
// material kind: 40/64 upper-case hex for OpenPGP, the 16-character base32
// keyed digest rdesanitize computes for a symmetric key.
func NormalizeEscrowKeyFingerprint(m EscrowKeyMaterial, fingerprint string) (string, error) {
	fp := strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(fingerprint), " ", ""))
	switch m {
	case EscrowKeyMaterialOpenPGPPrivate, EscrowKeyMaterialOpenPGPPublic:
		if !pgpFingerprintRe.MatchString(fp) {
			return "", errors.Join(ErrInvalidEscrowKeyVersion, ErrEscrowKeyInvalidFingerprint)
		}
	case EscrowKeyMaterialSymmetric:
		if !symmetricFingerprintRe.MatchString(fp) {
			return "", errors.Join(ErrInvalidEscrowKeyVersion, errors.New("symmetric key fingerprint must be 16 base32 characters"))
		}
	default:
		return "", errors.Join(ErrInvalidEscrowKeyVersion, fmt.Errorf("unknown key material %q", m))
	}
	return fp, nil
}

// ---------------------------------------------------------------------------
// Caller scope for the key registry
// ---------------------------------------------------------------------------

// EscrowKeyScope is the typed caller scope for key-registry reads and
// management. It is exactly one of the two ADR-0006 kinds that can own keys:
//
//   - an operator, which sees its own parties and the platform's (it can use a
//     platform party in its arrangements) and manages only its own;
//   - the platform, which sees and manages only platform parties.
//
// Like every scope it is a parameter after ctx, never ambient, and its zero
// value is invalid.
type EscrowKeyScope struct {
	platform PlatformScope
	operator OperatorID
}

// OperatorEscrowKeyScope scopes the registry to one operator.
func OperatorEscrowKeyScope(op OperatorID) EscrowKeyScope {
	return EscrowKeyScope{operator: op}
}

// PlatformEscrowKeyScope scopes the registry to the platform.
func PlatformEscrowKeyScope(p PlatformScope) EscrowKeyScope {
	return EscrowKeyScope{platform: p}
}

// Validate checks that exactly one kind was set and that it is valid.
func (s EscrowKeyScope) Validate() error {
	isPlatform := s.platform.Validate() == nil
	switch {
	case isPlatform && s.operator == "":
		return nil
	case !isPlatform && s.operator != "":
		return s.operator.Validate()
	default:
		return errors.Join(ErrInvalidEscrowKeyOwner, errors.New("a key-registry scope is either an operator or the platform"))
	}
}

// IsPlatform reports whether the scope is the platform.
func (s EscrowKeyScope) IsPlatform() bool { return s.platform.Validate() == nil && s.operator == "" }

// Operator returns the operator of an operator scope, or "" for the platform.
func (s EscrowKeyScope) Operator() OperatorID { return s.operator }

// CanSee reports whether things owned by owner are visible in this scope.
func (s EscrowKeyScope) CanSee(owner EscrowKeyOwner) bool {
	if s.IsPlatform() {
		return owner.IsPlatform()
	}
	return owner.VisibleTo(s.operator)
}

// CanManage reports whether things owned by owner may be changed in this scope.
func (s EscrowKeyScope) CanManage(owner EscrowKeyOwner) bool {
	if s.IsPlatform() {
		return owner.IsPlatform()
	}
	return owner.Kind == EscrowKeyOwnerOperator && owner.Operator == s.operator
}

// Owner returns the owner that things created in this scope get.
func (s EscrowKeyScope) Owner() EscrowKeyOwner {
	if s.IsPlatform() {
		return PlatformKeyOwner()
	}
	return OperatorKeyOwner(s.operator)
}

// EscrowKeyMaterialRejection is ErrEscrowKeyMaterialUnreadable with a reason
// the person pasting the key can act on ("this is a private key", "OpenPGP
// could not read it: ..."). The reason describes the *shape* of what was
// submitted and never quotes the material itself, so it is safe to return.
// Without one, callers fall back to the sentinel's fixed text.
type EscrowKeyMaterialRejection struct {
	Reason string
}

func (e *EscrowKeyMaterialRejection) Error() string { return e.Reason }

// Unwrap makes errors.Is(err, ErrEscrowKeyMaterialUnreadable) true, so existing
// handling keeps working.
func (e *EscrowKeyMaterialRejection) Unwrap() error { return ErrEscrowKeyMaterialUnreadable }

// RejectEscrowKeyMaterial builds a rejection carrying reason.
func RejectEscrowKeyMaterial(reason string) error {
	return &EscrowKeyMaterialRejection{Reason: reason}
}
