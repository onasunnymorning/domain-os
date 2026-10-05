package workflows

import (
	"fmt"
	"time"

	"github.com/onasunnymorning/domain-os/internal/application/activities"
	queues "github.com/onasunnymorning/domain-os/internal/infrastructure/temporal"
	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
	enumspb "go.temporal.io/api/enums/v1"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

// EscrowIntakeParams is one complete deposit found in the sFTP inbox: a
// .ryde/.sig pair, or a lone .xml/.xml.gz under the plaintext profile. Scope
// and TLD come from the key path, which the sFTP tenant cannot choose: it is
// the authenticated intake context of issue #412.
type EscrowIntakeParams struct {
	Scope string `json:"scope"`
	TLD   string `json:"tld"`
	// Profile is entities.EscrowProfileRydeSig or EscrowProfilePlaintextXML.
	// Empty means the signed profile.
	Profile      string    `json:"profile,omitempty"`
	IntakeID     string    `json:"intakeId"`
	ArtifactKey  string    `json:"artifactKey"`            // inbox key
	SignatureKey string    `json:"signatureKey,omitempty"` // inbox key; signed profile only
	ReceivedAt   time.Time `json:"receivedAt"`
}

// EscrowIntakeState is exposed through the "state" query.
type EscrowIntakeState struct {
	Phase                string `json:"phase"` // claiming | validating | settling | completed
	TLD                  string `json:"tld"`
	ValidationWorkflowID string `json:"validationWorkflowId,omitempty"`
	Disposition          string `json:"disposition,omitempty"`
}

// EscrowIntakeResult is the workflow's output.
type EscrowIntakeResult struct {
	ValidationWorkflowID string `json:"validationWorkflowId"`
	// ValidationOutcome is PASS or FAIL for a decided run. It is empty when
	// validation failed instead; ValidationError then says why.
	ValidationOutcome string `json:"validationOutcome,omitempty"`
	ValidationError   string `json:"validationError,omitempty"`
	ValidationRunID   string `json:"validationRunId,omitempty"`
	// Disposition is what became of the claimed pair: archived or rejected.
	Disposition string `json:"disposition"`
}

// EscrowIntakeWorkflow owns one sFTP deposit from the inbox to the end: it
// claims the pair, validates it with an unchanged EscrowValidationWorkflow,
// and settles the claimed copy. While it runs, the pair in claimed/ is its
// own and no one else's.
//
// The intake succeeds once the pair is settled, whatever validation decided:
// the run, its findings and its notification belong to the validation
// workflow and the validation run record.
func EscrowIntakeWorkflow(ctx workflow.Context, params EscrowIntakeParams) (EscrowIntakeResult, error) {
	logger := workflow.GetLogger(ctx)
	wfID := workflow.GetInfo(ctx).WorkflowExecution.ID
	state := EscrowIntakeState{Phase: "claiming", TLD: params.TLD}
	if err := workflow.SetQueryHandler(ctx, "state", func() (EscrowIntakeState, error) { return state, nil }); err != nil {
		return EscrowIntakeResult{}, fmt.Errorf("failed to set state query handler: %w", err)
	}

	profile := params.Profile
	if profile == "" {
		profile = entities.EscrowProfileRydeSig
	}

	var acts *activities.EscrowIntakeActivities
	// A copy of a deposit up to 10 GiB is server-side but not instant.
	claimCtx := workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout: 30 * time.Minute,
		RetryPolicy:         &temporal.RetryPolicy{InitialInterval: 5 * time.Second, BackoffCoefficient: 2.0, MaximumInterval: 5 * time.Minute, MaximumAttempts: 5},
	})
	// Settling must eventually happen, or the pair sits in claimed/ until the
	// bucket's lifecycle rule expires it, so it retries for longer.
	settleCtx := workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout: 30 * time.Minute,
		RetryPolicy:         &temporal.RetryPolicy{InitialInterval: 10 * time.Second, BackoffCoefficient: 2.0, MaximumInterval: 10 * time.Minute, MaximumAttempts: 20},
	})

	// 1. Claim: move the pair out of the operator's reach.
	var claimed activities.ClaimIntakeOutput
	if err := workflow.ExecuteActivity(claimCtx, acts.ClaimIntakePair, activities.ClaimIntakeInput{
		Scope: params.Scope, TLD: params.TLD, Profile: profile, IntakeID: params.IntakeID,
		ArtifactKey: params.ArtifactKey, SignatureKey: params.SignatureKey,
	}).Get(claimCtx, &claimed); err != nil {
		return EscrowIntakeResult{}, fmt.Errorf("ClaimIntakePair(tld=%s) failed: %w", params.TLD, err)
	}

	// 2. Validate. The child's failure is not this workflow's failure: either
	// way the pair is settled next.
	state.Phase = "validating"
	result := EscrowIntakeResult{ValidationWorkflowID: wfID + "-validation"}
	state.ValidationWorkflowID = result.ValidationWorkflowID
	childCtx := workflow.WithChildOptions(ctx, workflow.ChildWorkflowOptions{
		WorkflowID:            result.ValidationWorkflowID,
		TaskQueue:             queues.QueueHeavyBatch,
		WorkflowIDReusePolicy: enumspb.WORKFLOW_ID_REUSE_POLICY_REJECT_DUPLICATE,
	})
	var validated EscrowValidationResult
	if err := workflow.ExecuteChildWorkflow(childCtx, EscrowValidationTypeName, EscrowValidationParams{
		Scope: params.Scope, TLD: params.TLD, Profile: profile,
		ArtifactObjectKey: claimed.ArtifactKey, SignatureObjectKey: claimed.SignatureKey,
		SubmittedBy: "sftp:" + params.Scope,
		IntakeRef:   "sftp:" + params.ArtifactKey,
		ReceivedAt:  params.ReceivedAt,
	}).Get(ctx, &validated); err != nil {
		result.ValidationError = err.Error()
		logger.Warn("escrow intake: validation did not decide",
			"correlation_id", wfID, "tld", params.TLD, "stage", "validate", "error", err)
	} else {
		result.ValidationOutcome = validated.Outcome
	}

	// 3. Settle: archived when the deposit was bound, rejected when it was not.
	state.Phase = "settling"
	var settled activities.SettleIntakeOutput
	if err := workflow.ExecuteActivity(settleCtx, acts.SettleIntakePair, activities.SettleIntakeInput{
		Scope: params.Scope, TLD: params.TLD, Profile: profile, IntakeID: params.IntakeID,
		ValidationWorkflowID: result.ValidationWorkflowID,
		ArtifactKey:          claimed.ArtifactKey, SignatureKey: claimed.SignatureKey,
	}).Get(settleCtx, &settled); err != nil {
		return result, fmt.Errorf("SettleIntakePair(tld=%s) failed: %w", params.TLD, err)
	}
	result.Disposition, result.ValidationRunID = settled.Disposition, settled.ValidationRunID
	state.Phase, state.Disposition = "completed", settled.Disposition
	return result, nil
}
