package workflows

import (
	"fmt"
	"time"

	"github.com/onasunnymorning/domain-os/internal/application/activities"
	"github.com/onasunnymorning/domain-os/internal/application/rdevalidate"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

// EscrowValidationParams launches one EVE Stage 1 validation of a .ryde/.sig
// pair (issue #412). Scope and TLD come from the authenticated launch
// context — never from the artifact name or a caller-chosen value.
type EscrowValidationParams struct {
	Scope string `json:"scope"` // entities.OperatorID, validated at launch and again in BindDeposit
	TLD   string `json:"tld"`
	// Profile is entities.EscrowProfileRydeSig or EscrowProfilePlaintextXML.
	// Empty means the signed profile.
	Profile           string `json:"profile,omitempty"`
	ArtifactObjectKey string `json:"artifactObjectKey"`
	// SignatureObjectKey is set only for a signed profile.
	SignatureObjectKey string    `json:"signatureObjectKey,omitempty"`
	SubmittedBy        string    `json:"submittedBy"`
	IntakeRef          string    `json:"intakeRef,omitempty"`
	ReceivedAt         time.Time `json:"receivedAt"`
	// ValidationTimeout bounds the validate step; it should mirror the worker's
	// ESCROW_VALIDATION_TIMEOUT. Zero means the default (2h).
	ValidationTimeout time.Duration `json:"validationTimeout,omitempty"`
}

// EscrowValidationState is exposed through the "state" query for the Launchpad.
type EscrowValidationState struct {
	Phase              string   `json:"phase"` // binding | validating | reporting | finalizing | completed | failed | error
	TLD                string   `json:"tld"`
	DepositID          string   `json:"depositId,omitempty"`
	ValidationRunID    string   `json:"validationRunId,omitempty"`
	Replay             bool     `json:"replay"`
	Outcome            string   `json:"outcome,omitempty"`
	NotificationStatus string   `json:"notificationStatus,omitempty"`
	Codes              []string `json:"codes,omitempty"`
	Error              string   `json:"error,omitempty"`
}

// EscrowValidationResult is the workflow's output.
type EscrowValidationResult struct {
	DepositID          string   `json:"depositId"`
	ValidationRunID    string   `json:"validationRunId"`
	Replay             bool     `json:"replay"`
	Outcome            string   `json:"outcome"`
	Verified           bool     `json:"verified"` // cryptographically verified RDE pass (constraint 5)
	NotificationStatus string   `json:"notificationStatus,omitempty"`
	SummaryKey         string   `json:"summaryKey,omitempty"`
	ReportKey          string   `json:"reportKey,omitempty"`
	NotificationKey    string   `json:"notificationKey,omitempty"`
	Codes              []string `json:"codes,omitempty"`
}

const defaultEscrowValidationTimeout = 2 * time.Hour

// EscrowValidationWorkflow binds, validates, reports and finalises one
// deposit pair. It never calls the import pipeline. Every timestamp comes from
// workflow.Now and every identifier from an activity (INV-06).
//
// Fail closed: an infrastructure failure after the run has been opened
// finalises the run as ERROR and fails the workflow; no notification is
// emitted for a run the service could not decide.
func EscrowValidationWorkflow(ctx workflow.Context, params EscrowValidationParams) (EscrowValidationResult, error) {
	logger := workflow.GetLogger(ctx)
	state := EscrowValidationState{Phase: "binding", TLD: params.TLD}
	if err := workflow.SetQueryHandler(ctx, "state", func() (EscrowValidationState, error) { return state, nil }); err != nil {
		return EscrowValidationResult{}, fmt.Errorf("failed to set state query handler: %w", err)
	}
	info := workflow.GetInfo(ctx)
	wfID, runID := info.WorkflowExecution.ID, info.WorkflowExecution.RunID
	timeout := params.ValidationTimeout
	if timeout <= 0 {
		timeout = defaultEscrowValidationTimeout
	}

	shortRetry := &temporal.RetryPolicy{InitialInterval: 5 * time.Second, BackoffCoefficient: 2.0, MaximumAttempts: 3}
	ctxBind := workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout: 30 * time.Minute, HeartbeatTimeout: 5 * time.Minute, RetryPolicy: shortRetry,
	})
	ctxValidate := workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout: timeout + 10*time.Minute, HeartbeatTimeout: 5 * time.Minute,
		// The activity is idempotent (nothing is written until finalisation),
		// so one retry covers a transient storage or key-store hiccup.
		RetryPolicy: &temporal.RetryPolicy{InitialInterval: 30 * time.Second, BackoffCoefficient: 2.0, MaximumAttempts: 2},
	})
	ctxShort := workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout: 5 * time.Minute, RetryPolicy: shortRetry,
	})

	var acts *activities.EscrowValidationActivities

	// 1. Bind the pair to an immutable deposit record and open a run.
	var bound activities.BindDepositOutput
	if err := workflow.ExecuteActivity(ctxBind, acts.BindDeposit, activities.BindDepositInput{
		Scope: params.Scope, TLD: params.TLD, Profile: params.Profile,
		ArtifactObjectKey: params.ArtifactObjectKey, SignatureObjectKey: params.SignatureObjectKey,
		SubmittedBy: params.SubmittedBy, IntakeRef: params.IntakeRef, ReceivedAt: params.ReceivedAt,
		WorkflowID: wfID, RunID: runID,
	}).Get(ctxBind, &bound); err != nil {
		state.Phase, state.Error = "failed", err.Error()
		return EscrowValidationResult{}, fmt.Errorf("BindDeposit(tld=%s) failed: %w", params.TLD, err)
	}
	state.DepositID, state.ValidationRunID, state.Replay = bound.DepositID.String(), bound.ValidationRunID.String(), bound.Replay
	result := EscrowValidationResult{DepositID: state.DepositID, ValidationRunID: state.ValidationRunID, Replay: bound.Replay}

	// From here on the run exists and must always reach a terminal state.
	finalizeError := func(stage string, cause error) {
		state.Phase, state.Error = "error", cause.Error()
		dctx, _ := workflow.NewDisconnectedContext(ctx)
		dctx = workflow.WithActivityOptions(dctx, workflow.ActivityOptions{StartToCloseTimeout: 5 * time.Minute, RetryPolicy: shortRetry})
		if err := workflow.ExecuteActivity(dctx, acts.FinalizeValidationRun, activities.FinalizeRunInput{
			Scope: params.Scope, ValidationRunID: bound.ValidationRunID, WorkflowID: wfID,
			Result:      rdevalidate.Result{Profile: bound.Profile, Outcome: rdevalidate.OutcomeError, StageReached: rdevalidate.Stage(stage)},
			CompletedAt: workflow.Now(ctx), Failure: stage + ": " + cause.Error(),
		}).Get(dctx, nil); err != nil {
			logger.Error("escrow validation: could not finalise run as ERROR", "correlation_id", wfID, "run_id", state.ValidationRunID, "error", err)
		}
	}

	// 2. Verify, decrypt, unpack, validate — one streaming pass after signature check.
	state.Phase = "validating"
	var res rdevalidate.Result
	if err := workflow.ExecuteActivity(ctxValidate, acts.ValidateArtifacts, activities.ValidateArtifactsInput{
		Scope: params.Scope, TLD: params.TLD, Profile: bound.Profile,
		DepositID: bound.DepositID, ValidationRunID: bound.ValidationRunID, WorkflowID: wfID,
		ArtifactKey: bound.ArtifactKey, SignatureKey: bound.SignatureKey,
		ArtifactSHA256: bound.ArtifactSHA256, SignatureSHA256: bound.SignatureSHA256,
	}).Get(ctxValidate, &res); err != nil {
		finalizeError("validate", err)
		return result, fmt.Errorf("ValidateArtifacts(run=%s) failed: %w", state.ValidationRunID, err)
	}
	validatedAt := workflow.Now(ctx)
	state.Outcome = string(res.Outcome)
	for _, c := range res.Codes() {
		state.Codes = append(state.Codes, string(c))
	}
	result.Outcome, result.Verified, result.Codes = state.Outcome, res.Verified(), state.Codes

	// 3. Emit the run's artifacts. The activity decides which of the three it
	// writes: the summary always, the rdeReport for a decided run of any
	// profile, and the DVPN/DVFN only for a decided run of a signed profile.
	// An ERROR run still gets its summary — that is the run whose findings an
	// operator most needs — but it claims nothing to ICANN.
	state.Phase = "reporting"
	var emitted activities.EmitReportOutput
	if err := workflow.ExecuteActivity(ctxShort, acts.EmitReportAndNotification, activities.EmitReportInput{
		Scope: params.Scope, TLD: params.TLD, Profile: bound.Profile,
		DepositID: bound.DepositID, ValidationRunID: bound.ValidationRunID,
		WorkflowID: wfID, TemporalRunID: runID,
		Result: res, ReceivedAt: params.ReceivedAt, ValidatedAt: validatedAt, Hints: bound.Hints,
	}).Get(ctxShort, &emitted); err != nil {
		// Losing the summary must not lose the run: an ERROR run is finalised
		// as ERROR either way, and a decided run finalises through the same
		// fail-closed path as any other post-bind failure.
		finalizeError("report", err)
		return result, fmt.Errorf("EmitReportAndNotification(run=%s) failed: %w", state.ValidationRunID, err)
	}
	state.NotificationStatus = emitted.NotificationStatus
	result.NotificationStatus = emitted.NotificationStatus
	result.SummaryKey, result.ReportKey, result.NotificationKey = emitted.SummaryKey, emitted.ReportKey, emitted.NotificationKey

	// ERROR: the service could not decide. Record it, claim nothing, fail.
	if res.Outcome == rdevalidate.OutcomeError {
		state.Phase = "finalizing"
		if err := workflow.ExecuteActivity(ctxShort, acts.FinalizeValidationRun, activities.FinalizeRunInput{
			Scope: params.Scope, ValidationRunID: bound.ValidationRunID, WorkflowID: wfID, Result: res,
			SummaryKey: emitted.SummaryKey, CompletedAt: workflow.Now(ctx),
		}).Get(ctxShort, nil); err != nil {
			state.Phase, state.Error = "error", err.Error()
			return result, fmt.Errorf("FinalizeValidationRun(run=%s) failed: %w", state.ValidationRunID, err)
		}
		state.Phase = "error"
		return result, fmt.Errorf("validation of run %s could not be decided (%v); no notification emitted", state.ValidationRunID, state.Codes)
	}

	// 4. Finalise the immutable run record.
	state.Phase = "finalizing"
	if err := workflow.ExecuteActivity(ctxShort, acts.FinalizeValidationRun, activities.FinalizeRunInput{
		Scope: params.Scope, ValidationRunID: bound.ValidationRunID, WorkflowID: wfID, Result: res,
		SummaryKey: emitted.SummaryKey, ReportKey: emitted.ReportKey,
		NotificationKey: emitted.NotificationKey, NotificationStatus: emitted.NotificationStatus,
		CompletedAt: workflow.Now(ctx),
	}).Get(ctxShort, nil); err != nil {
		state.Phase, state.Error = "error", err.Error()
		return result, fmt.Errorf("FinalizeValidationRun(run=%s) failed: %w", state.ValidationRunID, err)
	}
	state.Phase = "completed"
	logger.Info("escrow validation: completed", "correlation_id", wfID, "deposit_id", state.DepositID, "run_id", state.ValidationRunID,
		"tld", params.TLD, "outcome", state.Outcome, "notification", state.NotificationStatus, "codes", state.Codes)
	return result, nil
}
