package entities

import (
	"errors"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Escrow validation (EVE) sentinel errors. These are the domain vocabulary
// for the deposit/validation-run/trusted-key aggregates; outer layers wrap
// them (INV-09) and never mint their own.
var (
	ErrInvalidEscrowDeposit               = errors.New("invalid escrow deposit")
	ErrEscrowDepositNotFound              = errors.New("escrow deposit not found")
	ErrInvalidEscrowDigest                = errors.New("invalid escrow artifact digest: expected 64 hex characters")
	ErrInvalidEscrowValidationRun         = errors.New("invalid escrow validation run")
	ErrEscrowValidationRunNotFound        = errors.New("escrow validation run not found")
	ErrEscrowValidationRunAlreadyFinal    = errors.New("escrow validation run is already final")
	ErrInvalidEscrowTrustedKey            = errors.New("invalid escrow trusted key")
	ErrEscrowTrustedKeyNotFound           = errors.New("escrow trusted key not found")
	ErrEscrowTrustedKeyAlreadyRetired     = errors.New("escrow trusted key is already retired")
	ErrEscrowTrustedKeyInvalidWindow      = errors.New("escrow trusted key validity window is invalid")
	ErrEscrowTrustedKeyInvalidFingerprint = errors.New("escrow trusted key fingerprint must be 40 or 64 hex characters")
	ErrUnknownEscrowProfile               = errors.New("unknown escrow validation profile")
)

var sha256HexRe = regexp.MustCompile(`^[0-9a-f]{64}$`)

// EscrowDeposit is the immutable record of one artifact set received for
// validation, bound to an operator tenant and a TLD at intake time.
//
// The artifact set depends on the profile (issue #415):
//
//   - EscrowProfileRydeSig: a `.ryde` deposit plus its detached `.sig`.
//   - EscrowProfilePlaintextXML: a plaintext `.xml` or `.xml.gz` deposit and
//     no signature at all.
//
// The fields are therefore named for their role, not for one profile's file
// extension: Artifact* is the deposit itself and Signature* is the detached
// signature, which is empty for every unsigned profile.
//
// Immutability is by construction: there is no Update on its repository. A
// replay of the same set (same tenant, TLD, profile and digests) binds to the
// existing row; a different set is a different deposit. The tenant/TLD binding
// comes from the authenticated intake context, never from the artifact itself
// (issue #412, design constraint 2).
type EscrowDeposit struct {
	ID          uuid.UUID
	TenantID    OperatorID // Operator scope the deposit is bound to (ADR-0006)
	TLD         string     // Normalised ASCII TLD, no trailing dot
	Profile     string     // EscrowProfileRydeSig | EscrowProfilePlaintextXML
	ReceivedAt  time.Time
	SubmittedBy string // Authenticated identity that submitted the set
	IntakeRef   string // Opaque reference supplied by the intake path (optional)

	ArtifactObjectKey string // Object-storage key of the immutable deposit artifact
	ArtifactSHA256    string // Lower-case hex digest of the deposit bytes
	ArtifactBytes     int64

	SignatureObjectKey string // Object-storage key of the detached signature; empty when unsigned
	SignatureSHA256    string // Lower-case hex digest of the signature bytes; empty when unsigned
	SignatureBytes     int64

	CreatedAt time.Time
}

// NewEscrowDeposit validates and creates an EscrowDeposit. It returns
// ErrInvalidEscrowDeposit (joined with the specific cause) on any invalid
// input. The signature triple is required for signed profiles and must be
// absent for unsigned ones — a half-populated signature is a bug, not a
// tolerable input.
func NewEscrowDeposit(
	scope OperatorID,
	tld, profile string,
	receivedAt time.Time,
	submittedBy, intakeRef string,
	artifactObjectKey, artifactSHA256 string,
	artifactBytes int64,
	signatureObjectKey, signatureSHA256 string,
	signatureBytes int64,
) (*EscrowDeposit, error) {
	if err := scope.Validate(); err != nil {
		return nil, errors.Join(ErrInvalidEscrowDeposit, err)
	}
	normTLD, err := NormalizeEscrowTLD(tld)
	if err != nil {
		return nil, errors.Join(ErrInvalidEscrowDeposit, err)
	}
	if !IsEscrowProfile(profile) {
		return nil, errors.Join(ErrInvalidEscrowDeposit, ErrUnknownEscrowProfile)
	}
	if receivedAt.IsZero() {
		return nil, errors.Join(ErrInvalidEscrowDeposit, errors.New("receivedAt is required"))
	}
	if strings.TrimSpace(submittedBy) == "" {
		return nil, errors.Join(ErrInvalidEscrowDeposit, errors.New("submittedBy is required"))
	}
	if strings.TrimSpace(artifactObjectKey) == "" {
		return nil, errors.Join(ErrInvalidEscrowDeposit, errors.New("artifactObjectKey is required"))
	}
	if !IsSHA256Hex(artifactSHA256) {
		return nil, errors.Join(ErrInvalidEscrowDeposit, ErrInvalidEscrowDigest)
	}
	if artifactBytes <= 0 {
		return nil, errors.Join(ErrInvalidEscrowDeposit, errors.New("artifact size must be positive"))
	}

	if EscrowProfileIsSigned(profile) {
		if strings.TrimSpace(signatureObjectKey) == "" {
			return nil, errors.Join(ErrInvalidEscrowDeposit, errors.New("signatureObjectKey is required for a signed profile"))
		}
		if !IsSHA256Hex(signatureSHA256) {
			return nil, errors.Join(ErrInvalidEscrowDeposit, ErrInvalidEscrowDigest)
		}
		if signatureBytes <= 0 {
			return nil, errors.Join(ErrInvalidEscrowDeposit, errors.New("signature size must be positive"))
		}
	} else if signatureObjectKey != "" || signatureSHA256 != "" || signatureBytes != 0 {
		return nil, errors.Join(ErrInvalidEscrowDeposit, errors.New("an unsigned profile must not carry signature artifacts"))
	}

	return &EscrowDeposit{
		ID:                 uuid.New(),
		TenantID:           scope,
		TLD:                normTLD,
		Profile:            profile,
		ReceivedAt:         receivedAt.UTC(),
		SubmittedBy:        strings.TrimSpace(submittedBy),
		IntakeRef:          strings.TrimSpace(intakeRef),
		ArtifactObjectKey:  artifactObjectKey,
		ArtifactSHA256:     artifactSHA256,
		ArtifactBytes:      artifactBytes,
		SignatureObjectKey: signatureObjectKey,
		SignatureSHA256:    signatureSHA256,
		SignatureBytes:     signatureBytes,
		CreatedAt:          RoundTime(time.Now().UTC()),
	}, nil
}

// IsSHA256Hex reports whether s is a lower-case 64-character hex string.
func IsSHA256Hex(s string) bool {
	return sha256HexRe.MatchString(s)
}

// NormalizeEscrowTLD lower-cases, trims dots and validates a TLD name for use
// as an escrow binding key. It reuses DomainName validation so the same rules
// apply as elsewhere in the domain.
func NormalizeEscrowTLD(tld string) (string, error) {
	dn, err := NewDomainName(tld)
	if err != nil {
		return "", err
	}
	return dn.String(), nil
}
