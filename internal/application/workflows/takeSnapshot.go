package workflows

import (
	"fmt"
	"time"

	"github.com/onasunnymorning/domain-os/internal/application/activities"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

// TakeSnapshotParams are the input parameters for TakeSnapshotWorkflow.
type TakeSnapshotParams struct {
	Label string `json:"label"` // Optional user-provided label, e.g. "pre-migration-2026-06-25"
	Note  string `json:"note"`  // Free-text description of the snapshot's purpose
}

// TakeSnapshotState tracks workflow progress for the "state" query handler.
type TakeSnapshotState struct {
	Phase       string           `json:"phase"` // "taking_snapshot", "completed", "failed"
	TableCounts map[string]int64 `json:"tableCounts,omitempty"`
	TotalRows   int64            `json:"totalRows"`
	SnapshotKey string           `json:"snapshotKey,omitempty"`
	ManifestKey string           `json:"manifestKey,omitempty"`
	Error       string           `json:"error,omitempty"`
}

// TakeSnapshotResponse is the workflow return value.
type TakeSnapshotResponse struct {
	SnapshotKey string           `json:"snapshotKey"`
	ManifestKey string           `json:"manifestKey"`
	TableCounts map[string]int64 `json:"tableCounts"`
	TotalRows   int64            `json:"totalRows"`
}

// TakeSnapshotWorkflow exports the entire database as a JSONL snapshot to S3.
//
// This is a single-step workflow: it delegates all work to the TakeSnapshot activity,
// which streams all tables (in FK-safe order) as JSONL to S3 via io.Pipe → UploadStream.
//
// The domain_events table is intentionally excluded from snapshots.
func TakeSnapshotWorkflow(ctx workflow.Context, params TakeSnapshotParams) (TakeSnapshotResponse, error) {
	state := TakeSnapshotState{
		Phase: "taking_snapshot",
	}

	err := workflow.SetQueryHandler(ctx, "state", func() (TakeSnapshotState, error) {
		return state, nil
	})
	if err != nil {
		return TakeSnapshotResponse{}, fmt.Errorf("failed to set state query handler: %w", err)
	}

	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 12 * time.Hour,
		HeartbeatTimeout:    5 * time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts: 3,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	var snapActivities *activities.SnapshotActivities

	wfInfo := workflow.GetInfo(ctx)
	args := activities.TakeSnapshotArgs{
		Label:      params.Label,
		Note:       params.Note,
		WorkflowID: wfInfo.WorkflowExecution.ID,
	}

	var result activities.TakeSnapshotResult
	if err = workflow.ExecuteActivity(ctx, snapActivities.TakeSnapshot, args).Get(ctx, &result); err != nil {
		state.Phase = "failed"
		state.Error = err.Error()
		return TakeSnapshotResponse{}, fmt.Errorf("TakeSnapshot activity failed: %w", err)
	}

	state.Phase = "completed"
	state.TableCounts = result.TableCounts
	state.TotalRows = result.TotalRows
	state.SnapshotKey = result.SnapshotKey
	state.ManifestKey = result.ManifestKey

	return TakeSnapshotResponse{
		SnapshotKey: result.SnapshotKey,
		ManifestKey: result.ManifestKey,
		TableCounts: result.TableCounts,
		TotalRows:   result.TotalRows,
	}, nil
}
