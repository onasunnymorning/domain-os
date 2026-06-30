package workflows

import (
	"strconv"
	"time"

	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

// EventRelayParams defines the input parameters for the EventRelay workflow.
type EventRelayParams struct {
	BatchSize  int `json:"batchSize,omitempty"`  // default 500
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

// EventRelay orchestrates the relay of unpublished domain events to S3.
// It fetches events in batches, archives each batch to S3, marks them as
// published, and repeats until all events are relayed or the batch cap is hit.
func EventRelay(ctx workflow.Context, params EventRelayParams) (EventRelayResult, error) {
	started := workflow.Now(ctx)
	result := EventRelayResult{
		StartedAt: started,
	}

	// Apply defaults
	if params.BatchSize <= 0 {
		params.BatchSize = 500
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

	workflowID := getWorkflowID(ctx)
	_ = workflowID // available for activity correlation if needed

	retrypolicy := &temporal.RetryPolicy{
		InitialInterval:    time.Second,
		BackoffCoefficient: 2.0,
		MaximumInterval:    10 * time.Minute,
		MaximumAttempts:    3,
	}

	options := workflow.ActivityOptions{
		StartToCloseTimeout: 5 * time.Minute,
		RetryPolicy:         retrypolicy,
	}

	ctx = workflow.WithActivityOptions(ctx, options)

	// Batch loop: fetch → archive → mark published
	for batch := 0; batch < params.MaxBatches; batch++ {
		// Step A: Fetch unpublished events
		var events []entities.DomainEvent
		fetchErr := workflow.ExecuteActivity(ctx, "FetchUnpublishedEvents", params.BatchSize).Get(ctx, &events)
		if fetchErr != nil {
			result.CompletedAt = workflow.Now(ctx)
			result.Notes = append(result.Notes, "Failed to fetch unpublished events: "+fetchErr.Error())
			return result, fetchErr
		}

		// If no events returned, we're done
		if len(events) == 0 {
			break
		}

		// Step B: Archive the batch to S3
		var s3Key string
		archiveErr := workflow.ExecuteActivity(ctx, "ArchiveEventsToS3", events).Get(ctx, &s3Key)
		if archiveErr != nil {
			result.CompletedAt = workflow.Now(ctx)
			result.Notes = append(result.Notes, "Failed to archive events to S3: "+archiveErr.Error())
			return result, archiveErr
		}
		result.S3Keys = append(result.S3Keys, s3Key)

		// Step C: Collect event IDs and mark them as published
		eventIDs := make([]string, len(events))
		for i, e := range events {
			eventIDs[i] = e.ID
		}

		markErr := workflow.ExecuteActivity(ctx, "MarkEventsPublished", eventIDs).Get(ctx, nil)
		if markErr != nil {
			result.CompletedAt = workflow.Now(ctx)
			result.Notes = append(result.Notes, "Failed to mark events as published: "+markErr.Error())
			return result, markErr
		}

		// Update counters
		result.TotalArchived += len(events)
		result.TotalBatches++
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
