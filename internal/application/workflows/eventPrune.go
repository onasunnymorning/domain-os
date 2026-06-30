package workflows

import (
	"strconv"
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

// EventPruneParams defines the input parameters for the EventPrune workflow.
type EventPruneParams struct {
	RetentionDays int `json:"retentionDays,omitempty"` // default 30
	BatchSize     int `json:"batchSize,omitempty"`     // default 10000
	MaxBatches    int `json:"maxBatches,omitempty"`    // default 50 (safety cap)
}

// EventPruneResult is the structured output of the EventPrune workflow.
type EventPruneResult struct {
	StartedAt      time.Time `json:"startedAt"`
	CompletedAt    time.Time `json:"completedAt"`
	TotalPruned    int64     `json:"totalPruned"`
	TotalBatches   int       `json:"totalBatches"`
	RemainingCount int64     `json:"remainingCount"`
	Notes          []string  `json:"notes"`
}

// EventPrune orchestrates the pruning of old domain events that have exceeded
// the retention window. It deletes events in batches and uses continue-as-new
// when the batch cap is reached to avoid unbounded execution history.
func EventPrune(ctx workflow.Context, params EventPruneParams) (EventPruneResult, error) {
	started := workflow.Now(ctx)
	result := EventPruneResult{
		StartedAt: started,
	}

	// Apply defaults
	if params.RetentionDays <= 0 {
		params.RetentionDays = 30
	}
	if params.BatchSize <= 0 {
		params.BatchSize = 10000
	}
	if params.MaxBatches <= 0 {
		params.MaxBatches = 50
	}

	// Register a query handler so progress is visible in the UI
	err := workflow.SetQueryHandler(ctx, "progress", func() (EventPruneResult, error) {
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

	// Step 1: Count prunable events
	var prunableCount int64
	countErr := workflow.ExecuteActivity(ctx, "CountPrunableEvents", params.RetentionDays).Get(ctx, &prunableCount)
	if countErr != nil {
		result.CompletedAt = workflow.Now(ctx)
		result.Notes = append(result.Notes, "Failed to count prunable events: "+countErr.Error())
		return result, countErr
	}

	// If nothing to prune, return early
	if prunableCount == 0 {
		result.CompletedAt = workflow.Now(ctx)
		result.Notes = append(result.Notes, "No prunable events found")
		return result, nil
	}

	// Step 2: Batch delete loop
	for batch := 0; batch < params.MaxBatches; batch++ {
		var deletedCount int64
		pruneErr := workflow.ExecuteActivity(ctx, "PruneEvents", params.RetentionDays, params.BatchSize).Get(ctx, &deletedCount)
		if pruneErr != nil {
			result.CompletedAt = workflow.Now(ctx)
			result.Notes = append(result.Notes, "Failed to prune events batch "+strconv.Itoa(batch+1)+": "+pruneErr.Error())
			return result, pruneErr
		}

		result.TotalPruned += deletedCount
		result.TotalBatches++

		// If we deleted fewer than batchSize, there are no more to prune
		if deletedCount < int64(params.BatchSize) {
			break
		}
	}

	// Count remaining prunable events
	var remaining int64
	remainErr := workflow.ExecuteActivity(ctx, "CountPrunableEvents", params.RetentionDays).Get(ctx, &remaining)
	if remainErr != nil {
		result.CompletedAt = workflow.Now(ctx)
		result.Notes = append(result.Notes, "Failed to count remaining prunable events: "+remainErr.Error())
		return result, remainErr
	}
	result.RemainingCount = remaining

	result.CompletedAt = workflow.Now(ctx)

	// If we hit the batch cap and there's still work, continue-as-new
	if remaining > 0 && result.TotalBatches >= params.MaxBatches {
		result.Notes = append(result.Notes, "Batch cap reached: pruned "+
			strconv.FormatInt(result.TotalPruned, 10)+" events but "+
			strconv.FormatInt(remaining, 10)+
			" remain. Continuing processing in a new run.")
		return result, workflow.NewContinueAsNewError(ctx, EventPrune, params)
	}

	return result, nil
}
