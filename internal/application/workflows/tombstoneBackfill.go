package workflows

import (
	"fmt"
	"strconv"
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

// TombstoneBackfillParams configures the backfill run.
type TombstoneBackfillParams struct {
	BatchSize  int `json:"batchSize,omitempty"`  // events per batch (default 200)
	MaxBatches int `json:"maxBatches,omitempty"` // safety cap (default 50)
}

// TombstoneBackfillResult holds the outcome.
type TombstoneBackfillResult struct {
	StartedAt         time.Time `json:"startedAt"`
	CompletedAt       time.Time `json:"completedAt"`
	EventsScanned     int64     `json:"eventsScanned"`
	TombstonesCreated int64     `json:"tombstonesCreated"`
	TombstonesSkipped int64     `json:"tombstonesSkipped"` // already existed
	TotalBatches      int       `json:"totalBatches"`
	RemainingCount    int64     `json:"remainingCount"`
	Errors            int64     `json:"errors"`
	Notes             []string  `json:"notes"`
}

// TombstoneBackfillBatchResult is returned by each batch activity.
type TombstoneBackfillBatchResult struct {
	EventsScanned     int64  `json:"eventsScanned"`
	TombstonesCreated int64  `json:"tombstonesCreated"`
	TombstonesSkipped int64  `json:"tombstonesSkipped"`
	Errors            int64  `json:"errors"`
	LastCursor        string `json:"lastCursor"`
}

// TombstoneBackfill orchestrates the backfill of tombstones for domains that
// were purged before the tombstone system existed. It scans domain.purged
// events and creates tombstone records for any that are missing. Uses
// continue-as-new when the batch cap is reached to avoid unbounded history.
func TombstoneBackfill(ctx workflow.Context, params TombstoneBackfillParams) (TombstoneBackfillResult, error) {
	started := workflow.Now(ctx)
	result := TombstoneBackfillResult{
		StartedAt: started,
	}

	// Apply defaults
	if params.BatchSize <= 0 {
		params.BatchSize = 200
	}
	if params.MaxBatches <= 0 {
		params.MaxBatches = 50
	}

	// Register a query handler so progress is visible in the UI
	err := workflow.SetQueryHandler(ctx, "progress", func() (TombstoneBackfillResult, error) {
		return result, nil
	})
	if err != nil {
		result.CompletedAt = workflow.Now(ctx)
		result.Notes = append(result.Notes, "Failed to register query handler: "+err.Error())
		return result, err
	}

	retrypolicy := &temporal.RetryPolicy{
		InitialInterval:    2 * time.Second,
		BackoffCoefficient: 2.0,
		MaximumInterval:    5 * time.Minute,
		MaximumAttempts:    3,
	}

	options := workflow.ActivityOptions{
		StartToCloseTimeout: 10 * time.Minute,
		HeartbeatTimeout:    2 * time.Minute,
		RetryPolicy:         retrypolicy,
	}

	ctx = workflow.WithActivityOptions(ctx, options)

	// Step 1: Count domain.purged events
	var purgeCount int64
	countErr := workflow.ExecuteActivity(ctx, "CountPurgeEvents").Get(ctx, &purgeCount)
	if countErr != nil {
		result.CompletedAt = workflow.Now(ctx)
		result.Notes = append(result.Notes, "Failed to count purge events: "+countErr.Error())
		return result, countErr
	}

	// If nothing to process, return early
	if purgeCount == 0 {
		result.CompletedAt = workflow.Now(ctx)
		result.Notes = append(result.Notes, "No domain.purged events found")
		return result, nil
	}

	result.Notes = append(result.Notes, fmt.Sprintf("Found %d domain.purged events to scan", purgeCount))

	// Step 2: Batch processing loop
	cursor := ""
	for batch := 0; batch < params.MaxBatches; batch++ {
		var batchResult TombstoneBackfillBatchResult
		batchErr := workflow.ExecuteActivity(ctx, "BackfillTombstonesBatch", params.BatchSize, cursor).Get(ctx, &batchResult)
		if batchErr != nil {
			result.CompletedAt = workflow.Now(ctx)
			result.Notes = append(result.Notes, "Failed batch "+strconv.Itoa(batch+1)+": "+batchErr.Error())
			return result, batchErr
		}

		result.EventsScanned += batchResult.EventsScanned
		result.TombstonesCreated += batchResult.TombstonesCreated
		result.TombstonesSkipped += batchResult.TombstonesSkipped
		result.Errors += batchResult.Errors
		result.TotalBatches++
		cursor = batchResult.LastCursor

		// If we processed fewer than batchSize, there are no more events
		if batchResult.EventsScanned < int64(params.BatchSize) {
			break
		}
	}

	// Step 3: Count remaining purge events without tombstones
	var remaining int64
	remainErr := workflow.ExecuteActivity(ctx, "CountPurgeEventsWithoutTombstones").Get(ctx, &remaining)
	if remainErr != nil {
		result.CompletedAt = workflow.Now(ctx)
		result.Notes = append(result.Notes, "Failed to count remaining events: "+remainErr.Error())
		return result, remainErr
	}
	result.RemainingCount = remaining

	result.CompletedAt = workflow.Now(ctx)

	// If we hit the batch cap and there's still work, continue-as-new
	if remaining > 0 && result.TotalBatches >= params.MaxBatches {
		result.Notes = append(result.Notes, fmt.Sprintf(
			"Batch cap reached: created %d tombstones (%d skipped, %d errors) across %d batches but %d events remain. Continuing in a new run.",
			result.TombstonesCreated, result.TombstonesSkipped, result.Errors,
			result.TotalBatches, remaining,
		))
		return result, workflow.NewContinueAsNewError(ctx, TombstoneBackfill, params)
	}

	result.Notes = append(result.Notes, fmt.Sprintf(
		"Backfill complete: created %d tombstones, skipped %d (already existed), %d errors across %d batches",
		result.TombstonesCreated, result.TombstonesSkipped, result.Errors, result.TotalBatches,
	))

	return result, nil
}
