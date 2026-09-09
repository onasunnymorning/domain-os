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
)

var sha256HexRe = regexp.MustCompile(`^[0-9a-f]{64}$`)

// EscrowDeposit is the immutable record of one artifact pair received for
// validation: a `.ryde` deposit and its detached `.sig`, bound to an operator
// tenant and a TLD at intake time.
//
// Immutability is by construction: there is no Update on its repository. A
// replay of the same pair (same tenant, TLD and digests) binds to the existing
// row; a different pair is a different deposit. The tenant/TLD binding comes
// from the authenticated intake context, never from the artifact itself
// (issue #412, design constraint 2).
type EscrowDeposit struct {
	ID          uuid.UUID
	TenantID    OperatorID // Operator scope the deposit is bound to (ADR-0006)
	TLD         string     // Normalised ASCII TLD, no trailing dot
	ReceivedAt  time.Time
	SubmittedBy string // Authenticated identity that submitted the pair
	IntakeRef   string // Opaque reference supplied by the intake path (optional)

	RydeObjectKey string // Object-storage key of the immutable .ryde artifact
	SigObjectKey  string // Object-storage key of the immutable .sig artifact
	RydeSHA256    string // Lower-case hex digest of the .ryde bytes
	SigSHA256     string // Lower-case hex digest of the .sig bytes
	RydeBytes     int64
	SigBytes      int64

	CreatedAt time.Time
}

// NewEscrowDeposit validates and creates an EscrowDeposit. It returns
// ErrInvalidEscrowDeposit (joined with the specific cause) on any invalid
// input.
func NewEscrowDeposit(
	scope OperatorID,
	tld string,
	receivedAt time.Time,
	submittedBy, intakeRef string,
	rydeObjectKey, sigObjectKey string,
	rydeSHA256, sigSHA256 string,
	rydeBytes, sigBytes int64,
) (*EscrowDeposit, error) {
	if err := scope.Validate(); err != nil {
		return nil, errors.Join(ErrInvalidEscrowDeposit, err)
	}
	normTLD, err := NormalizeEscrowTLD(tld)
	if err != nil {
		return nil, errors.Join(ErrInvalidEscrowDeposit, err)
	}
	if receivedAt.IsZero() {
		return nil, errors.Join(ErrInvalidEscrowDeposit, errors.New("receivedAt is required"))
	}
	if strings.TrimSpace(submittedBy) == "" {
		return nil, errors.Join(ErrInvalidEscrowDeposit, errors.New("submittedBy is required"))
	}
	if strings.TrimSpace(rydeObjectKey) == "" || strings.TrimSpace(sigObjectKey) == "" {
		return nil, errors.Join(ErrInvalidEscrowDeposit, errors.New("rydeObjectKey and sigObjectKey are required"))
	}
	if !IsSHA256Hex(rydeSHA256) || !IsSHA256Hex(sigSHA256) {
		return nil, errors.Join(ErrInvalidEscrowDeposit, ErrInvalidEscrowDigest)
	}
	if rydeBytes <= 0 || sigBytes <= 0 {
		return nil, errors.Join(ErrInvalidEscrowDeposit, errors.New("artifact sizes must be positive"))
	}

	return &EscrowDeposit{
		ID:            uuid.New(),
		TenantID:      scope,
		TLD:           normTLD,
		ReceivedAt:    receivedAt.UTC(),
		SubmittedBy:   strings.TrimSpace(submittedBy),
		IntakeRef:     strings.TrimSpace(intakeRef),
		RydeObjectKey: rydeObjectKey,
		SigObjectKey:  sigObjectKey,
		RydeSHA256:    rydeSHA256,
		SigSHA256:     sigSHA256,
		RydeBytes:     rydeBytes,
		SigBytes:      sigBytes,
		CreatedAt:     RoundTime(time.Now().UTC()),
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
