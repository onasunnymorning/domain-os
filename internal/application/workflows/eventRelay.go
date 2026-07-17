package workflows

import (
	"fmt"
	"strconv"
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

// EventRelayParams defines the input parameters for the EventRelay workflow.
type EventRelayParams struct {
	BatchSize  int `json:"batchSize,omitempty"`  // default 200
	MaxBatches int `json:"maxBatches,omitempty"` // default 10 (safety cap per run)
}

// EventRelayResult is the structured output of the EventRelay workflow.
type EventRelayResult struct {
	StartedAt      time.Time `json:"startedAt"`
	CompletedAt    time.Time `json:"completedAt"`
	TotalArchived  int       `json:"totalArchived"`
	TotalBatches   int       `json:"totalBatches"`
	S3Keys         []string  `json:"s3Keys,omitempty"`
	RemainingCount int64     `json:"remainingCount"`
	Notes          []string  `json:"notes"`
}

// RelayBatchResult is the lightweight result of a single relay batch activity.
// Only metadata crosses the Temporal boundary — events stay inside the activity.
type RelayBatchResult struct {
	Archived int    `json:"archived"`
	S3Key    string `json:"s3Key"`
}

// EventRelay orchestrates the relay of unpublished domain events to S3.
// It runs a consolidated activity per batch that fetches events, archives
// them to S3, and marks them as published — all within a single activity
// execution to avoid serializing large event payloads through Temporal.
func EventRelay(ctx workflow.Context, params EventRelayParams) (EventRelayResult, error) {
	started := workflow.Now(ctx)
	result := EventRelayResult{
		StartedAt: started,
	}

	// Apply defaults
	if params.BatchSize <= 0 {
		params.BatchSize = 200
	}
	if params.MaxBatches <= 0 {
		params.MaxBatches = 10
	}

	// Register a query handler so progress is visible in the UI
	err := workflow.SetQueryHandler(ctx, "progress", func() (EventRelayResult, error) {
		return result, nil
	})
	if err != nil {
		result.CompletedAt = workflow.Now(ctx)
		result.Notes = append(result.Notes, "Failed to register query handler: "+err.Error())
		return result, err
	}

	retrypolicy := &temporal.RetryPolicy{
		InitialInterval:    time.Second,
		BackoffCoefficient: 2.0,
		MaximumInterval:    10 * time.Minute,
		MaximumAttempts:    3,
	}

	options := workflow.ActivityOptions{
		StartToCloseTimeout: 10 * time.Minute,
		HeartbeatTimeout:    2 * time.Minute,
		RetryPolicy:         retrypolicy,
	}

	ctx = workflow.WithActivityOptions(ctx, options)

	// Batch loop: single consolidated activity per batch
	for batch := 0; batch < params.MaxBatches; batch++ {
		var batchResult RelayBatchResult
		batchErr := workflow.ExecuteActivity(ctx, "RelayEventBatch", params.BatchSize).Get(ctx, &batchResult)
		if batchErr != nil {
			result.CompletedAt = workflow.Now(ctx)
			result.Notes = append(result.Notes, fmt.Sprintf("Failed batch %d: %s", batch+1, batchErr.Error()))
			return result, batchErr
		}

		// If nothing was archived, we're done
		if batchResult.Archived == 0 {
			break
		}

		result.TotalArchived += batchResult.Archived
		result.TotalBatches++
		if batchResult.S3Key != "" {
			result.S3Keys = append(result.S3Keys, batchResult.S3Key)
		}
	}

	// Final step: count remaining unpublished events
	var remaining int64
	countErr := workflow.ExecuteActivity(ctx, "CountUnpublishedEvents").Get(ctx, &remaining)
	if countErr != nil {
		result.CompletedAt = workflow.Now(ctx)
		result.Notes = append(result.Notes, "Failed to count remaining events: "+countErr.Error())
		return result, countErr
	}
	result.RemainingCount = remaining

	result.CompletedAt = workflow.Now(ctx)

	if remaining > 0 {
		result.Notes = append(result.Notes, "Batch cap reached: relayed "+
			strconv.Itoa(result.TotalArchived)+" events but "+
			strconv.FormatInt(remaining, 10)+" remain. Schedule another run to continue.")
	}

	return result, nil
}
