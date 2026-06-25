package workflows

import (
	"fmt"
	"time"

	"github.com/onasunnymorning/domain-os/internal/application/activities"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

// SeedFromSnapshotParams are the input parameters for SeedFromSnapshotWorkflow.
type SeedFromSnapshotParams struct {
	SnapshotKey string `json:"snapshotKey"` // S3 key prefix, e.g. "snapshot-pre-migration-20260625-080000"
}

// SeedFromSnapshotState tracks workflow progress for the "state" query handler.
type SeedFromSnapshotState struct {
	Phase          string           `json:"phase"` // "validating", "pending_confirmation", "seeding", "completed", "aborted", "failed"
	TableCounts    map[string]int64 `json:"tableCounts,omitempty"`
	TotalRows      int64            `json:"totalRows"`
	Label          string           `json:"label,omitempty"`
	InsertedCounts map[string]int64 `json:"insertedCounts,omitempty"`
	SkippedCounts  map[string]int64 `json:"skippedCounts,omitempty"`
	TotalInserted  int64            `json:"totalInserted"`
	TotalSkipped   int64            `json:"totalSkipped"`
	Error          string           `json:"error,omitempty"`
}

// SeedFromSnapshotResponse is the workflow return value.
type SeedFromSnapshotResponse struct {
	InsertedCounts map[string]int64 `json:"insertedCounts"`
	SkippedCounts  map[string]int64 `json:"skippedCounts"`
	TotalInserted  int64            `json:"totalInserted"`
	TotalSkipped   int64            `json:"totalSkipped"`
}

// SeedFromSnapshotWorkflow populates a database from a previously taken JSONL snapshot.
//
// The workflow has three phases:
//  1. ValidateSnapshot — verifies the snapshot exists and returns table counts
//  2. Await Confirmation — pauses for operator review via ConfirmSeedFromSnapshot signal
//  3. SeedFromSnapshot — streams JSONL from S3 and inserts into Postgres
//
// Uses ON CONFLICT DO NOTHING for idempotent, gap-filling inserts. Existing rows
// are preserved and skipped. This makes it safe to retry and suitable for populating
// an empty or partially-populated database.
//
// The domain_events table is not included in snapshots and will not be seeded.
func SeedFromSnapshotWorkflow(ctx workflow.Context, params SeedFromSnapshotParams) (SeedFromSnapshotResponse, error) {
	state := SeedFromSnapshotState{
		Phase: "validating",
	}

	err := workflow.SetQueryHandler(ctx, "state", func() (SeedFromSnapshotState, error) {
		return state, nil
	})
	if err != nil {
		return SeedFromSnapshotResponse{}, fmt.Errorf("failed to set state query handler: %w", err)
	}

	var snapActivities *activities.SnapshotActivities

	// --- Step 1: Validate Snapshot ---
	validateAO := workflow.ActivityOptions{
		StartToCloseTimeout: 5 * time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts: 3,
		},
	}
	validateCtx := workflow.WithActivityOptions(ctx, validateAO)

	validateArgs := activities.ValidateSnapshotArgs{
		SnapshotKey: params.SnapshotKey,
	}
	var validateResult activities.ValidateSnapshotResult
	if err = workflow.ExecuteActivity(validateCtx, snapActivities.ValidateSnapshot, validateArgs).Get(ctx, &validateResult); err != nil {
		state.Phase = "failed"
		state.Error = err.Error()
		return SeedFromSnapshotResponse{}, fmt.Errorf("ValidateSnapshot failed: %w", err)
	}

	if !validateResult.IsValid {
		state.Phase = "failed"
		state.Error = validateResult.Error
		return SeedFromSnapshotResponse{}, fmt.Errorf("snapshot validation failed: %s", validateResult.Error)
	}

	state.Phase = "pending_confirmation"
	state.TableCounts = validateResult.TableCounts
	state.TotalRows = validateResult.TotalRows
	state.Label = validateResult.Label

	workflow.GetLogger(ctx).Info("Snapshot validated. Awaiting confirmation.",
		"Label", validateResult.Label,
		"TotalRows", validateResult.TotalRows,
	)

	// --- Step 2: Await Confirmation via Signal ---
	confirmationSignalChan := workflow.GetSignalChannel(ctx, "ConfirmSeedFromSnapshot")
	var confirmed bool

	selector := workflow.NewSelector(ctx)
	selector.AddReceive(confirmationSignalChan, func(c workflow.ReceiveChannel, more bool) {
		var signalVal bool
		c.Receive(ctx, &signalVal)
		confirmed = signalVal
	})

	workflow.GetLogger(ctx).Info("Waiting for ConfirmSeedFromSnapshot signal before proceeding with database seeding.")
	selector.Select(ctx)

	if !confirmed {
		state.Phase = "aborted"
		workflow.GetLogger(ctx).Warn("Seeding was cancelled by signal. Aborting workflow.")
		return SeedFromSnapshotResponse{}, fmt.Errorf("seeding aborted by user signal")
	}

	// --- Step 3: Seed from Snapshot ---
	state.Phase = "seeding"

	seedAO := workflow.ActivityOptions{
		StartToCloseTimeout: 12 * time.Hour,
		HeartbeatTimeout:    5 * time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts: 3,
		},
	}
	seedCtx := workflow.WithActivityOptions(ctx, seedAO)

	wfInfo := workflow.GetInfo(ctx)
	seedArgs := activities.SeedFromSnapshotArgs{
		SnapshotKey: params.SnapshotKey,
		WorkflowID:  wfInfo.WorkflowExecution.ID,
	}
	var seedResult activities.SeedFromSnapshotResult
	if err = workflow.ExecuteActivity(seedCtx, snapActivities.SeedFromSnapshot, seedArgs).Get(ctx, &seedResult); err != nil {
		state.Phase = "failed"
		state.Error = err.Error()
		return SeedFromSnapshotResponse{}, fmt.Errorf("SeedFromSnapshot activity failed: %w", err)
	}

	state.Phase = "completed"
	state.InsertedCounts = seedResult.InsertedCounts
	state.SkippedCounts = seedResult.SkippedCounts
	state.TotalInserted = seedResult.TotalInserted
	state.TotalSkipped = seedResult.TotalSkipped

	workflow.GetLogger(ctx).Info("Seeding complete.",
		"TotalInserted", seedResult.TotalInserted,
		"TotalSkipped", seedResult.TotalSkipped,
	)

	return SeedFromSnapshotResponse{
		InsertedCounts: seedResult.InsertedCounts,
		SkippedCounts:  seedResult.SkippedCounts,
		TotalInserted:  seedResult.TotalInserted,
		TotalSkipped:   seedResult.TotalSkipped,
	}, nil
}
