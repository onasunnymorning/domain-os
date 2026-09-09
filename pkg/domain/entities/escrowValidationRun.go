package entities

import (
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
)

// EscrowValidationOutcome is the terminal (or running) state of one
// validation run.
type EscrowValidationOutcome string

const (
	// EscrowValidationRunning is the only non-terminal outcome.
	EscrowValidationRunning EscrowValidationOutcome = "RUNNING"
	// EscrowValidationPass means every required check passed; the run is a
	// cryptographically verified RDE pass only if Profile == EscrowProfileRydeSig.
	EscrowValidationPass EscrowValidationOutcome = "PASS"
	// EscrowValidationFail means the deposit was received but is invalid (DVFN).
	EscrowValidationFail EscrowValidationOutcome = "FAIL"
	// EscrowValidationError means the service could not decide (our key missing,
	// storage failure, timeout). No notification is emitted for ERROR: we
	// cannot honestly claim the deposit is invalid, and we must not claim it is
	// valid (fail closed).
	EscrowValidationError EscrowValidationOutcome = "ERROR"
)

// EscrowNotificationStatus mirrors the rdeNotification status codes this
// service can emit. DRFN (missing deposit) is deliberately absent: it is
// scheduled outside this feature.
type EscrowNotificationStatus string

const (
	EscrowNotificationNone EscrowNotificationStatus = ""
	EscrowNotificationDVPN EscrowNotificationStatus = "DVPN" // Deposit Verification Pass Notification
	EscrowNotificationDVFN EscrowNotificationStatus = "DVFN" // Deposit Verification Fail Notification
)

// EscrowProfileRydeSig is the only profile that can yield a cryptographically
// verified pass (issue #412, design constraint 5).
const EscrowProfileRydeSig = "ryde+sig"

// EscrowFinding is the persisted form of one validation check result. It is
// the domain mirror of the validator's finding type (the domain layer cannot
// import it, INV-14). Message and Locator carry constant text plus numbers
// only — never untrusted names or payload content.
type EscrowFinding struct {
	Code       string    `json:"code"`
	Severity   string    `json:"severity"`
	Stage      string    `json:"stage"`
	ObjectType string    `json:"objectType,omitempty"`
	Locator    string    `json:"locator,omitempty"`
	Message    string    `json:"message"`
	At         time.Time `json:"at"`
}

// EscrowValidationRun is one immutable execution of the validation pipeline
// against one EscrowDeposit. A run is created RUNNING and finalised exactly
// once; a second finalisation is rejected with ErrEscrowValidationRunAlreadyFinal.
type EscrowValidationRun struct {
	ID        uuid.UUID
	DepositID uuid.UUID
	TenantID  OperatorID
	TLD       string

	WorkflowID string // Temporal Workflow ID == correlation_id (INV-16)
	RunID      string // Temporal Run ID == trace_id (INV-16)
	Profile    string

	Outcome      EscrowValidationOutcome
	StageReached string
	Findings     []EscrowFinding

	SigningKeyFingerprint    string // Trusted registry key that signed the .sig
	DecryptionKeyFingerprint string // Service key that decrypted the .ryde
	PlaintextSHA256          string

	RDEDepositID string
	RDEKind      string
	RDEResend    int
	RDEWatermark *time.Time

	ReportObjectKey       string
	NotificationObjectKey string
	NotificationStatus    EscrowNotificationStatus

	StartedAt   time.Time
	CompletedAt *time.Time
}

// NewEscrowValidationRun creates a RUNNING run bound to a deposit.
func NewEscrowValidationRun(depositID uuid.UUID, scope OperatorID, tld, workflowID, runID, profile string, startedAt time.Time) (*EscrowValidationRun, error) {
	if depositID == uuid.Nil {
		return nil, errors.Join(ErrInvalidEscrowValidationRun, errors.New("depositID is required"))
	}
	if err := scope.Validate(); err != nil {
		return nil, errors.Join(ErrInvalidEscrowValidationRun, err)
	}
	normTLD, err := NormalizeEscrowTLD(tld)
	if err != nil {
		return nil, errors.Join(ErrInvalidEscrowValidationRun, err)
	}
	if strings.TrimSpace(workflowID) == "" {
		return nil, errors.Join(ErrInvalidEscrowValidationRun, errors.New("workflowID is required"))
	}
	if strings.TrimSpace(profile) == "" {
		return nil, errors.Join(ErrInvalidEscrowValidationRun, errors.New("profile is required"))
	}
	if startedAt.IsZero() {
		return nil, errors.Join(ErrInvalidEscrowValidationRun, errors.New("startedAt is required"))
	}
	return &EscrowValidationRun{
		ID:         uuid.New(),
		DepositID:  depositID,
		TenantID:   scope,
		TLD:        normTLD,
		WorkflowID: workflowID,
		RunID:      runID,
		Profile:    profile,
		Outcome:    EscrowValidationRunning,
		Findings:   []EscrowFinding{},
		StartedAt:  startedAt.UTC(),
	}, nil
}

// EscrowValidationFinalization carries everything a run learns by the time it
// reaches a terminal state.
type EscrowValidationFinalization struct {
	Outcome                  EscrowValidationOutcome
	StageReached             string
	Findings                 []EscrowFinding
	SigningKeyFingerprint    string
	DecryptionKeyFingerprint string
	PlaintextSHA256          string
	RDEDepositID             string
	RDEKind                  string
	RDEResend                int
	RDEWatermark             *time.Time
	ReportObjectKey          string
	NotificationObjectKey    string
	NotificationStatus       EscrowNotificationStatus
	CompletedAt              time.Time
}

// Finalize moves the run from RUNNING to a terminal outcome. It enforces the
// outcome/notification pairing (PASS↔DVPN, FAIL↔DVFN, ERROR↔none) and rejects
// a second finalisation.
func (r *EscrowValidationRun) Finalize(f EscrowValidationFinalization) error {
	if r.Outcome != EscrowValidationRunning {
		return ErrEscrowValidationRunAlreadyFinal
	}
	switch f.Outcome {
	case EscrowValidationPass:
		if f.NotificationStatus != EscrowNotificationDVPN {
			return errors.Join(ErrInvalidEscrowValidationRun, errors.New("PASS requires a DVPN notification"))
		}
	case EscrowValidationFail:
		if f.NotificationStatus != EscrowNotificationDVFN {
			return errors.Join(ErrInvalidEscrowValidationRun, errors.New("FAIL requires a DVFN notification"))
		}
	case EscrowValidationError:
		if f.NotificationStatus != EscrowNotificationNone {
			return errors.Join(ErrInvalidEscrowValidationRun, errors.New("ERROR must not carry a notification"))
		}
	default:
		return errors.Join(ErrInvalidEscrowValidationRun, errors.New("finalisation outcome must be PASS, FAIL or ERROR"))
	}
	if f.CompletedAt.IsZero() {
		return errors.Join(ErrInvalidEscrowValidationRun, errors.New("completedAt is required"))
	}
	if f.CompletedAt.Before(r.StartedAt) {
		return errors.Join(ErrInvalidEscrowValidationRun, errors.New("completedAt precedes startedAt"))
	}

	completed := f.CompletedAt.UTC()
	r.Outcome = f.Outcome
	r.StageReached = f.StageReached
	if f.Findings != nil {
		r.Findings = f.Findings
	}
	r.SigningKeyFingerprint = f.SigningKeyFingerprint
	r.DecryptionKeyFingerprint = f.DecryptionKeyFingerprint
	r.PlaintextSHA256 = f.PlaintextSHA256
	r.RDEDepositID = f.RDEDepositID
	r.RDEKind = f.RDEKind
	r.RDEResend = f.RDEResend
	r.RDEWatermark = f.RDEWatermark
	r.ReportObjectKey = f.ReportObjectKey
	r.NotificationObjectKey = f.NotificationObjectKey
	r.NotificationStatus = f.NotificationStatus
	r.CompletedAt = &completed
	return nil
}

// IsFinal reports whether the run has reached a terminal outcome.
func (r *EscrowValidationRun) IsFinal() bool {
	return r.Outcome != EscrowValidationRunning
}

// Verified reports whether this run is a cryptographically verified RDE pass:
// only the signed+encrypted profile can make that claim (constraint 5).
func (r *EscrowValidationRun) Verified() bool {
	return r.Outcome == EscrowValidationPass && r.Profile == EscrowProfileRydeSig
}

// FindingCodes returns the distinct finding codes in order of first appearance.
func (r *EscrowValidationRun) FindingCodes() []string {
	seen := map[string]bool{}
	var out []string
	for _, f := range r.Findings {
		if !seen[f.Code] {
			seen[f.Code] = true
			out = append(out, f.Code)
		}
	}
	return out
}
