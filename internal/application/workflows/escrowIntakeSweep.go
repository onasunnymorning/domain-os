package workflows

import (
	"fmt"
	"time"

	"github.com/onasunnymorning/domain-os/internal/application/activities"
	queues "github.com/onasunnymorning/domain-os/internal/infrastructure/temporal"
	enumspb "go.temporal.io/api/enums/v1"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

// EscrowIntakeSweepParams bounds one sweep.
type EscrowIntakeSweepParams struct {
	MaxPairs int `json:"maxPairs,omitempty"` // default 25
}

// EscrowIntakeSweepResult is the sweep's output, visible in the Temporal UI.
type EscrowIntakeSweepResult struct {
	StartedAt   time.Time `json:"startedAt"`
	CompletedAt time.Time `json:"completedAt"`
	Enabled     bool      `json:"enabled"`
	// Started counts intakes this sweep started.
	Started int `json:"started"`
	// AlreadyStarted counts pairs whose intake was already started: by an
	// earlier sweep that has not claimed them yet, or one that is still
	// claiming.
	AlreadyStarted int `json:"alreadyStarted"`
	// FailedToStart counts pairs whose intake could not be started; the next
	// sweep tries again.
	FailedToStart        int           `json:"failedToStart"`
	Deferred             int           `json:"deferred"`
	Unpaired             int           `json:"unpaired"`
	OldestUnpairedWaited time.Duration `json:"oldestUnpairedWaited,omitempty"`
	Ignored              int           `json:"ignored"`
	PlaintextRefused     int           `json:"plaintextRefused"`
	Truncated            bool          `json:"truncated"`
	IntakeWorkflowIDs    []string      `json:"intakeWorkflowIds,omitempty"`
	Notes                []string      `json:"notes,omitempty"`
}

// EscrowIntakeSweepWorkflow turns deposits that arrived over sFTP into
// validation runs. It reads the inbox and starts one EscrowIntakeWorkflow per
// complete deposit — a .ryde/.sig pair, or a lone .xml/.xml.gz when plaintext
// intake is allowed — then finishes; each intake runs on, independently, on
// heavy-batch.
//
// The sweep keeps no state. An intake's workflow ID is derived from the upload
// itself and is never reused, so a pair the previous sweep already started is
// rejected as a duplicate here rather than started twice; once its intake has
// claimed it, the pair is no longer in the inbox at all.
func EscrowIntakeSweepWorkflow(ctx workflow.Context, params EscrowIntakeSweepParams) (EscrowIntakeSweepResult, error) {
	logger := workflow.GetLogger(ctx)
	result := EscrowIntakeSweepResult{StartedAt: workflow.Now(ctx)}
	if err := workflow.SetQueryHandler(ctx, "progress", func() (EscrowIntakeSweepResult, error) { return result, nil }); err != nil {
		return result, fmt.Errorf("failed to set progress query handler: %w", err)
	}

	listCtx := workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout: 2 * time.Minute,
		RetryPolicy:         &temporal.RetryPolicy{InitialInterval: 5 * time.Second, BackoffCoefficient: 2.0, MaximumAttempts: 3},
	})
	var acts *activities.EscrowIntakeActivities
	var listed activities.ListIntakePairsOutput
	if err := workflow.ExecuteActivity(listCtx, acts.ListIntakePairs, activities.ListIntakePairsInput{
		MaxPairs: params.MaxPairs,
	}).Get(listCtx, &listed); err != nil {
		result.CompletedAt = workflow.Now(ctx)
		return result, fmt.Errorf("ListIntakePairs failed: %w", err)
	}
	result.Enabled = listed.Enabled
	result.Deferred, result.Unpaired, result.OldestUnpairedWaited = listed.Deferred, listed.Unpaired, listed.OldestUnpairedWaited
	result.Ignored, result.PlaintextRefused, result.Truncated = listed.Ignored, listed.PlaintextRefused, listed.Truncated
	if !listed.Enabled {
		result.CompletedAt = workflow.Now(ctx)
		result.Notes = append(result.Notes, "sFTP intake is disabled (ESCROW_INTAKE_SFTP_ENABLED)")
		return result, nil
	}

	// Start every intake first, then wait for each to be accepted. The sweep
	// waits only for the start, never for the intake: with ABANDON the intake
	// outlives this run.
	type started struct {
		pair activities.IntakePair
		fut  workflow.ChildWorkflowFuture
	}
	futures := make([]started, 0, len(listed.Pairs))
	for _, pair := range listed.Pairs {
		childCtx := workflow.WithChildOptions(ctx, workflow.ChildWorkflowOptions{
			WorkflowID:            pair.WorkflowID,
			TaskQueue:             queues.QueueHeavyBatch,
			ParentClosePolicy:     enumspb.PARENT_CLOSE_POLICY_ABANDON,
			WorkflowIDReusePolicy: enumspb.WORKFLOW_ID_REUSE_POLICY_REJECT_DUPLICATE,
		})
		futures = append(futures, started{pair: pair, fut: workflow.ExecuteChildWorkflow(childCtx, EscrowIntakeWorkflow, EscrowIntakeParams{
			Scope: pair.Scope, TLD: pair.TLD, Profile: pair.Profile, IntakeID: pair.IntakeID,
			ArtifactKey: pair.ArtifactKey, SignatureKey: pair.SignatureKey, ReceivedAt: pair.ReceivedAt,
		})})
	}
	for _, s := range futures {
		err := s.fut.GetChildWorkflowExecution().Get(ctx, nil)
		switch {
		case err == nil:
			result.Started++
			result.IntakeWorkflowIDs = append(result.IntakeWorkflowIDs, s.pair.WorkflowID)
		case temporal.IsWorkflowExecutionAlreadyStartedError(err):
			result.AlreadyStarted++
		default:
			result.FailedToStart++
			result.Notes = append(result.Notes, fmt.Sprintf("could not start %s: %v", s.pair.WorkflowID, err))
			logger.Error("escrow intake sweep: could not start an intake",
				"correlation_id", workflow.GetInfo(ctx).WorkflowExecution.ID, "intake_workflow_id", s.pair.WorkflowID, "tld", s.pair.TLD, "error", err)
		}
	}
	if result.PlaintextRefused > 0 {
		result.Notes = append(result.Notes, fmt.Sprintf("%d unsigned .xml/.xml.gz deposits left in the inbox: plaintext intake is off (ESCROW_INTAKE_SFTP_ALLOW_PLAINTEXT)", result.PlaintextRefused))
	}
	if result.Deferred > 0 {
		result.Notes = append(result.Notes, fmt.Sprintf("%d more complete pairs wait for the next sweep", result.Deferred))
	}
	result.CompletedAt = workflow.Now(ctx)
	return result, nil
}
