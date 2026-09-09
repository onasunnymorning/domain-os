package workflows

import (
	"fmt"
	"time"

	"github.com/onasunnymorning/domain-os/internal/application/activities"
	"github.com/onasunnymorning/domain-os/internal/application/rdesanitize"
	"go.temporal.io/sdk/log"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

// EscrowSanitizeParams launches one derivative run against one accepted
// validation run (issue #415).
//
// Scope comes from the authenticated launch context and never from a caller
// parameter (ADR-0006). The TLD is not a parameter at all: it is whatever the
// bound source deposit says, so a derivative cannot be attributed to a TLD the
// source did not belong to.
type EscrowSanitizeParams struct {
	Scope                 string `json:"scope"`
	SourceValidationRunID string `json:"sourceValidationRunId"`
	// SyntheticSuffix overrides ESCROW_SANITIZE_SUFFIX for this run. It must
	// differ from the source TLD.
	SyntheticSuffix string `json:"syntheticSuffix,omitempty"`
	RequestedBy     string `json:"requestedBy"`
	// SanitizeTimeout bounds the produce and verify steps; it should mirror the
	// worker's ESCROW_SANITIZE_TIMEOUT. Zero means the default (4h).
	SanitizeTimeout time.Duration `json:"sanitizeTimeout,omitempty"`
}

// EscrowSanitizeState is exposed through the "state" query for the Launchpad.
type EscrowSanitizeState struct {
	Phase                 string   `json:"phase"` // binding|producing|verifying|finalizing|completed|quarantined|error|failed
	TLD                   string   `json:"tld,omitempty"`
	SanitizationRunID     string   `json:"sanitizationRunId,omitempty"`
	SourceValidationRunID string   `json:"sourceValidationRunId,omitempty"`
	PolicyVersion         string   `json:"policyVersion,omitempty"`
	SyntheticSuffix       string   `json:"syntheticSuffix,omitempty"`
	Replay                bool     `json:"replay"`
	Outcome               string   `json:"outcome,omitempty"`
	Codes                 []string `json:"codes,omitempty"`
	Error                 string   `json:"error,omitempty"`
}

// EscrowSanitizeResult is the workflow's return value.
type EscrowSanitizeResult struct {
	SanitizationRunID string   `json:"sanitizationRunId"`
	Replay            bool     `json:"replay"`
	TLD               string   `json:"tld"`
	PolicyVersion     string   `json:"policyVersion"`
	SyntheticSuffix   string   `json:"syntheticSuffix"`
	Outcome           string   `json:"outcome"`
	DerivativeKey     string   `json:"derivativeKey,omitempty"`
	ManifestKey       string   `json:"manifestKey,omitempty"`
	Codes             []string `json:"codes,omitempty"`
}

const defaultEscrowSanitizeTimeout = 4 * time.Hour

// EscrowSanitizeWorkflow derives a sanitized-pseudonymized copy of an accepted
// deposit for internal analytics and non-production testing.
//
// It is an adjunct to escrow custody, not a modification of it: the raw deposit
// remains the authoritative artifact, this workflow never writes to it, and it
// has no path to the escrow import pipeline or to registry data
// (escrow_validation_isolation_test.go asserts both).
func EscrowSanitizeWorkflow(ctx workflow.Context, params EscrowSanitizeParams) (EscrowSanitizeResult, error) {
	logger := workflow.GetLogger(ctx)
	info := workflow.GetInfo(ctx)
	wfID, runID := info.WorkflowExecution.ID, info.WorkflowExecution.RunID

	state := EscrowSanitizeState{Phase: "binding", SourceValidationRunID: params.SourceValidationRunID}
	if err := workflow.SetQueryHandler(ctx, "state", func() (EscrowSanitizeState, error) { return state, nil }); err != nil {
		return EscrowSanitizeResult{}, fmt.Errorf("SetQueryHandler(state): %w", err)
	}

	timeout := params.SanitizeTimeout
	if timeout <= 0 {
		timeout = defaultEscrowSanitizeTimeout
	}
	shortRetry := &temporal.RetryPolicy{InitialInterval: 5 * time.Second, BackoffCoefficient: 2.0, MaximumAttempts: 3}
	ctxBind := workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout: 30 * time.Minute, RetryPolicy: shortRetry,
	})
	ctxLong := workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout: timeout + 10*time.Minute,
		HeartbeatTimeout:    5 * time.Minute,
		RetryPolicy:         &temporal.RetryPolicy{InitialInterval: 30 * time.Second, BackoffCoefficient: 2.0, MaximumAttempts: 2},
	})
	ctxShort := workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout: 5 * time.Minute, RetryPolicy: shortRetry,
	})

	var acts *activities.EscrowSanitizeActivities

	// 1. Bind an accepted source and open (or find) the run for this policy.
	var bound activities.BindSanitizationSourceOutput
	if err := workflow.ExecuteActivity(ctxBind, acts.BindSanitizationSource, activities.BindSanitizationSourceInput{
		Scope: params.Scope, SourceValidationRunID: params.SourceValidationRunID,
		SyntheticSuffix: params.SyntheticSuffix, WorkflowID: wfID, RunID: runID,
	}).Get(ctxBind, &bound); err != nil {
		state.Phase, state.Error = "failed", err.Error()
		return EscrowSanitizeResult{}, fmt.Errorf("BindSanitizationSource(source=%s) failed: %w", params.SourceValidationRunID, err)
	}
	state.TLD, state.SanitizationRunID = bound.TLD, bound.SanitizationRunID.String()
	state.PolicyVersion, state.SyntheticSuffix, state.Replay = bound.PolicyVersion, bound.SyntheticSuffix, bound.Replay
	result := EscrowSanitizeResult{
		SanitizationRunID: state.SanitizationRunID, Replay: bound.Replay, TLD: bound.TLD,
		PolicyVersion: bound.PolicyVersion, SyntheticSuffix: bound.SyntheticSuffix,
	}

	// A derivative of this source under this policy version already exists and
	// is final. Producing another would either duplicate it or overwrite it,
	// and neither is allowed.
	if bound.AlreadyFinal {
		state.Phase, state.Outcome = "completed", bound.ExistingOutcome
		result.Outcome, result.DerivativeKey, result.ManifestKey = bound.ExistingOutcome, bound.DerivativeKey, bound.ManifestKey
		logger.Info("escrow sanitization: derivative already exists for this policy version",
			"correlation_id", wfID, "sanitization_run_id", state.SanitizationRunID, "outcome", bound.ExistingOutcome)
		return result, nil
	}

	// From here the run exists and must always reach a terminal state.
	finalizeError := func(stage string, cause error) {
		state.Phase, state.Error = "error", cause.Error()
		dctx, _ := workflow.NewDisconnectedContext(ctx)
		dctx = workflow.WithActivityOptions(dctx, workflow.ActivityOptions{StartToCloseTimeout: 5 * time.Minute, RetryPolicy: shortRetry})
		if err := workflow.ExecuteActivity(dctx, acts.FinalizeSanitizationRun, activities.FinalizeSanitizationRunInput{
			Scope: params.Scope, SanitizationRunID: bound.SanitizationRunID, WorkflowID: wfID,
			CompletedAt: workflow.Now(ctx), Failure: stage + ": " + cause.Error(),
		}).Get(dctx, nil); err != nil {
			logger.Error("escrow sanitization: could not finalise run as ERROR",
				"correlation_id", wfID, "sanitization_run_id", state.SanitizationRunID, "error", err)
		}
	}

	// 2. Stream the source through the profile into the pending object.
	state.Phase = "producing"
	var produced activities.ProduceDerivativeOutput
	if err := workflow.ExecuteActivity(ctxLong, acts.ProduceDerivative, activities.ProduceDerivativeInput{
		Scope: params.Scope, SanitizationRunID: bound.SanitizationRunID, WorkflowID: wfID,
		TLD: bound.TLD, SourceProfile: bound.SourceProfile,
		ArtifactKey: bound.ArtifactKey, SignatureKey: bound.SignatureKey,
		ArtifactSHA256: bound.ArtifactSHA256, SignatureSHA256: bound.SignatureSHA256,
		SyntheticSuffix: bound.SyntheticSuffix, StagingKey: bound.StagingKey,
	}).Get(ctxLong, &produced); err != nil {
		finalizeError("produce", err)
		return result, fmt.Errorf("ProduceDerivative(run=%s) failed: %w", state.SanitizationRunID, err)
	}

	// Nothing was staged, so there is nothing to verify or publish. Record the
	// outcome and stop.
	if produced.Result.Outcome != rdesanitize.OutcomePass {
		return finishSanitize(ctx, ctxShort, acts, params, bound, produced, activities.VerifyDerivativeOutput{Result: produced.Result}, &state, result, logger)
	}

	// 3. Scan and re-validate what was staged, then publish it.
	state.Phase = "verifying"
	var verified activities.VerifyDerivativeOutput
	if err := workflow.ExecuteActivity(ctxLong, acts.VerifyDerivative, activities.VerifyDerivativeInput{
		Scope: params.Scope, SanitizationRunID: bound.SanitizationRunID, WorkflowID: wfID, RunID: runID,
		TLD: bound.TLD, DepositID: bound.DepositID, SourceValidationRunID: bound.SourceValidationRunID,
		SourceProfile: bound.SourceProfile, SourceArtifactSHA256: bound.ArtifactSHA256,
		SyntheticSuffix: bound.SyntheticSuffix, StagingKey: bound.StagingKey,
		DerivativeKey: bound.DerivativeKey, ManifestKey: bound.ManifestKey,
		DerivativeSHA256: produced.DerivativeSHA256, DerivativeBytes: produced.DerivativeBytes,
		TokenKeyID: produced.TokenKeyID, Produced: produced.Result,
	}).Get(ctxLong, &verified); err != nil {
		finalizeError("verify", err)
		return result, fmt.Errorf("VerifyDerivative(run=%s) failed: %w", state.SanitizationRunID, err)
	}

	return finishSanitize(ctx, ctxShort, acts, params, bound, produced, verified, &state, result, logger)
}

// finishSanitize writes the terminal record and shapes the workflow result. A
// quarantine is a legitimate outcome and completes the workflow; only a
// service-side ERROR fails it.
func finishSanitize(
	ctx, ctxShort workflow.Context,
	acts *activities.EscrowSanitizeActivities,
	params EscrowSanitizeParams,
	bound activities.BindSanitizationSourceOutput,
	produced activities.ProduceDerivativeOutput,
	verified activities.VerifyDerivativeOutput,
	state *EscrowSanitizeState,
	result EscrowSanitizeResult,
	logger log.Logger,
) (EscrowSanitizeResult, error) {
	wfID := workflow.GetInfo(ctx).WorkflowExecution.ID
	res := verified.Result
	state.Outcome = string(res.Outcome)
	for _, c := range res.Codes() {
		state.Codes = append(state.Codes, string(c))
	}
	result.Outcome, result.Codes = state.Outcome, state.Codes
	result.DerivativeKey, result.ManifestKey = verified.DerivativeKey, verified.ManifestKey

	state.Phase = "finalizing"
	if err := workflow.ExecuteActivity(ctxShort, acts.FinalizeSanitizationRun, activities.FinalizeSanitizationRunInput{
		Scope: params.Scope, SanitizationRunID: bound.SanitizationRunID, WorkflowID: wfID, Result: res,
		DerivativeKey: verified.DerivativeKey, ManifestKey: verified.ManifestKey,
		DerivativeSHA256: produced.DerivativeSHA256, DerivativeBytes: produced.DerivativeBytes,
		CompletedAt: workflow.Now(ctx),
	}).Get(ctxShort, nil); err != nil {
		state.Phase, state.Error = "error", err.Error()
		return result, fmt.Errorf("FinalizeSanitizationRun(run=%s) failed: %w", state.SanitizationRunID, err)
	}

	switch res.Outcome {
	case rdesanitize.OutcomePass:
		state.Phase = "completed"
		logger.Info("escrow sanitization: completed",
			"correlation_id", wfID, "sanitization_run_id", state.SanitizationRunID, "tld", state.TLD,
			"outcome", state.Outcome, "policy_version", state.PolicyVersion)
		return result, nil
	case rdesanitize.OutcomeQuarantined:
		// The source is not one this agent will transform under this policy.
		// That is a decision, not a failure: the run completes and records it.
		state.Phase = "quarantined"
		logger.Info("escrow sanitization: source quarantined",
			"correlation_id", wfID, "sanitization_run_id", state.SanitizationRunID, "tld", state.TLD,
			"outcome", state.Outcome, "codes", state.Codes)
		return result, nil
	default:
		state.Phase = "error"
		return result, fmt.Errorf("sanitization of run %s could not be decided (%v); nothing was published", state.SanitizationRunID, state.Codes)
	}
}
