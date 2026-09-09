package entities

import (
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Escrow sanitisation (issue #415) sentinel errors.
var (
	ErrInvalidEscrowSanitizationRun      = errors.New("invalid escrow sanitization run")
	ErrEscrowSanitizationRunNotFound     = errors.New("escrow sanitization run not found")
	ErrEscrowSanitizationRunAlreadyFinal = errors.New("escrow sanitization run is already final")
	ErrEscrowSourceNotAccepted           = errors.New("escrow validation run is not an accepted source for sanitization")
)

// EscrowDerivativeLabel is the required label for the output of a sanitisation
// run, and it is deliberately not the word "anonymous". Deterministic tokens
// stay linkable within a tenant, the HMAC key is sensitive material, and
// retained analytical metadata (country, city, postal code, timestamps, status
// combinations) can re-identify in combination — see NIST SP 800-188.
const EscrowDerivativeLabel = "sanitized-pseudonymized"

// EscrowSanitizationOutcome is the terminal state of a sanitisation run.
//
// QUARANTINED rather than FAIL: a source the profile cannot fully classify is
// not a broken deposit, it is one this agent refuses to transform until the
// profile is extended. Nothing is published in that state.
type EscrowSanitizationOutcome string

const (
	EscrowSanitizationRunning     EscrowSanitizationOutcome = "RUNNING"
	EscrowSanitizationPass        EscrowSanitizationOutcome = "PASS"
	EscrowSanitizationQuarantined EscrowSanitizationOutcome = "QUARANTINED"
	EscrowSanitizationError       EscrowSanitizationOutcome = "ERROR"
)

// EscrowSanitizationCounts is the aggregate statistic set carried by a run and
// its manifest. Counts only: never a value, an excerpt or a name.
type EscrowSanitizationCounts struct {
	// ObjectsByType counts the RDE objects in the derivative, keyed by object
	// namespace URI.
	ObjectsByType map[string]int64 `json:"objectsByType,omitempty"`
	// Elements counts what the classifier did, by action.
	Kept        int64 `json:"kept"`
	Dropped     int64 `json:"dropped"`
	Synthesized int64 `json:"synthesized"`
	Tokenized   int64 `json:"tokenized"`
	Rewritten   int64 `json:"rewritten"`
}

// EscrowSanitizationRun is the immutable record of one attempt to derive a
// sanitized-pseudonymized copy of an already-validated deposit.
//
// The source is never modified, never re-checksummed and never re-labelled:
// the raw deposit stays the authoritative custody artifact and this record only
// points at it. Re-running under a different policy version produces a separate
// row and a separate object; the unique index on
// (tenant, source validation run, policy version) is what makes "never
// overwrites an existing derivative" a database guarantee rather than a habit.
type EscrowSanitizationRun struct {
	ID       uuid.UUID
	TenantID OperatorID
	TLD      string

	SourceValidationRunID uuid.UUID
	SourceDepositID       uuid.UUID
	SourceArtifactSHA256  string // the source object version this derivative was made from

	PolicyVersion   string // the in-repo sanitisation profile version
	WorkflowVersion string
	SyntheticSuffix string // the suffix every in-bailiwick FQDN was rewritten to

	WorkflowID string // Temporal Workflow ID == correlation_id (INV-16)
	RunID      string // Temporal Run ID == trace_id (INV-16)

	Outcome      EscrowSanitizationOutcome
	StageReached string
	Findings     []EscrowFinding

	DerivativeObjectKey string
	DerivativeSHA256    string
	DerivativeBytes     int64
	ManifestObjectKey   string
	Counts              EscrowSanitizationCounts

	StartedAt   time.Time
	CompletedAt *time.Time
}

// NewEscrowSanitizationRun creates a RUNNING run bound to an accepted source
// validation run.
func NewEscrowSanitizationRun(
	scope OperatorID,
	tld string,
	sourceValidationRunID, sourceDepositID uuid.UUID,
	sourceArtifactSHA256, policyVersion, workflowVersion, syntheticSuffix, workflowID, runID string,
	startedAt time.Time,
) (*EscrowSanitizationRun, error) {
	if err := scope.Validate(); err != nil {
		return nil, errors.Join(ErrInvalidEscrowSanitizationRun, err)
	}
	normTLD, err := NormalizeEscrowTLD(tld)
	if err != nil {
		return nil, errors.Join(ErrInvalidEscrowSanitizationRun, err)
	}
	if sourceValidationRunID == uuid.Nil || sourceDepositID == uuid.Nil {
		return nil, errors.Join(ErrInvalidEscrowSanitizationRun, errors.New("source validation run and deposit are required"))
	}
	if !IsSHA256Hex(sourceArtifactSHA256) {
		return nil, errors.Join(ErrInvalidEscrowSanitizationRun, ErrInvalidEscrowDigest)
	}
	if strings.TrimSpace(policyVersion) == "" {
		return nil, errors.Join(ErrInvalidEscrowSanitizationRun, errors.New("policyVersion is required"))
	}
	if strings.TrimSpace(workflowVersion) == "" {
		return nil, errors.Join(ErrInvalidEscrowSanitizationRun, errors.New("workflowVersion is required"))
	}
	// The synthetic suffix is part of the derivative's meaning, so it is
	// validated as a name rather than accepted as free text.
	normSuffix, err := NormalizeEscrowTLD(syntheticSuffix)
	if err != nil {
		return nil, errors.Join(ErrInvalidEscrowSanitizationRun, err)
	}
	if normSuffix == normTLD {
		return nil, errors.Join(ErrInvalidEscrowSanitizationRun, errors.New("the synthetic suffix must differ from the source TLD"))
	}
	if strings.TrimSpace(workflowID) == "" {
		return nil, errors.Join(ErrInvalidEscrowSanitizationRun, errors.New("workflowID is required"))
	}
	if startedAt.IsZero() {
		return nil, errors.Join(ErrInvalidEscrowSanitizationRun, errors.New("startedAt is required"))
	}
	return &EscrowSanitizationRun{
		ID:                    uuid.New(),
		TenantID:              scope,
		TLD:                   normTLD,
		SourceValidationRunID: sourceValidationRunID,
		SourceDepositID:       sourceDepositID,
		SourceArtifactSHA256:  sourceArtifactSHA256,
		PolicyVersion:         policyVersion,
		WorkflowVersion:       workflowVersion,
		SyntheticSuffix:       normSuffix,
		WorkflowID:            workflowID,
		RunID:                 runID,
		Outcome:               EscrowSanitizationRunning,
		Findings:              []EscrowFinding{},
		StartedAt:             startedAt.UTC(),
	}, nil
}

// EscrowSanitizationFinalization carries everything a run learns by the time it
// reaches a terminal state.
type EscrowSanitizationFinalization struct {
	Outcome             EscrowSanitizationOutcome
	StageReached        string
	Findings            []EscrowFinding
	DerivativeObjectKey string
	DerivativeSHA256    string
	DerivativeBytes     int64
	ManifestObjectKey   string
	Counts              EscrowSanitizationCounts
	CompletedAt         time.Time
}

// Finalize moves the run from RUNNING to a terminal outcome and rejects a
// second finalisation. It enforces the rule that matters most here: only a PASS
// may point at a derivative. A quarantined or errored run must point at
// nothing, because nothing was published.
func (r *EscrowSanitizationRun) Finalize(f EscrowSanitizationFinalization) error {
	if r.Outcome != EscrowSanitizationRunning {
		return ErrEscrowSanitizationRunAlreadyFinal
	}
	switch f.Outcome {
	case EscrowSanitizationPass:
		if strings.TrimSpace(f.DerivativeObjectKey) == "" || strings.TrimSpace(f.ManifestObjectKey) == "" {
			return errors.Join(ErrInvalidEscrowSanitizationRun, errors.New("PASS requires a derivative and a manifest"))
		}
		if !IsSHA256Hex(f.DerivativeSHA256) {
			return errors.Join(ErrInvalidEscrowSanitizationRun, ErrInvalidEscrowDigest)
		}
		if f.DerivativeBytes <= 0 {
			return errors.Join(ErrInvalidEscrowSanitizationRun, errors.New("PASS requires a non-empty derivative"))
		}
	case EscrowSanitizationQuarantined, EscrowSanitizationError:
		if f.DerivativeObjectKey != "" || f.DerivativeSHA256 != "" || f.ManifestObjectKey != "" || f.DerivativeBytes != 0 {
			return errors.Join(ErrInvalidEscrowSanitizationRun, errors.New("a run that did not pass must not point at a derivative"))
		}
	default:
		return errors.Join(ErrInvalidEscrowSanitizationRun, errors.New("finalisation outcome must be PASS, QUARANTINED or ERROR"))
	}
	if f.CompletedAt.IsZero() {
		return errors.Join(ErrInvalidEscrowSanitizationRun, errors.New("completedAt is required"))
	}
	if f.CompletedAt.Before(r.StartedAt) {
		return errors.Join(ErrInvalidEscrowSanitizationRun, errors.New("completedAt precedes startedAt"))
	}

	completed := f.CompletedAt.UTC()
	r.Outcome = f.Outcome
	r.StageReached = f.StageReached
	if f.Findings != nil {
		r.Findings = f.Findings
	}
	r.DerivativeObjectKey = f.DerivativeObjectKey
	r.DerivativeSHA256 = f.DerivativeSHA256
	r.DerivativeBytes = f.DerivativeBytes
	r.ManifestObjectKey = f.ManifestObjectKey
	r.Counts = f.Counts
	r.CompletedAt = &completed
	return nil
}

// IsFinal reports whether the run has reached a terminal outcome.
func (r *EscrowSanitizationRun) IsFinal() bool {
	return r.Outcome != EscrowSanitizationRunning
}

// FindingCodes returns the distinct finding codes in order of first appearance.
func (r *EscrowSanitizationRun) FindingCodes() []string {
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

// EscrowValidationRunIsSanitizableSource reports whether a validation run may be
// used as the source of a derivative: it must have passed, because transforming
// a deposit this service has not accepted would put unvalidated content behind
// a label that says it was checked.
func EscrowValidationRunIsSanitizableSource(r *EscrowValidationRun) bool {
	return r != nil && r.Outcome == EscrowValidationPass && IsEscrowProfile(r.Profile)
}
