package workflows

import (
	"time"

	"github.com/onasunnymorning/domain-os/internal/application/activities"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

// SyncSpec5Workflow orchestrates the synchronization of Spec5 labels.
func SyncSpec5Workflow(ctx workflow.Context) error {
	// Get the workflow ID
	workflowID := getWorkflowID(ctx)

	// RetryPolicy specifies how to automatically handle retries if an Activity fails.
	retrypolicy := &temporal.RetryPolicy{
		InitialInterval:        time.Second,
		BackoffCoefficient:     2.0,
		MaximumInterval:        10 * time.Minute,
		MaximumAttempts:        3,
		NonRetryableErrorTypes: []string{"none"},
	}

	options := workflow.ActivityOptions{
		// Timeout options specify when to automatically timeout Activity functions.
		StartToCloseTimeout: 10 * time.Minute,
		RetryPolicy:         retrypolicy,
	}

	// Apply the options.
	ctx = workflow.WithActivityOptions(ctx, options)

	// Execute activity to sync Spec5 labels
	err := workflow.ExecuteActivity(ctx, activities.SyncSpec5, workflowID).Get(ctx, nil)
	if err != nil {
		workflow.GetLogger(ctx).Error("Failed to sync Spec5 labels", "error", err)
		return err
	}

	return nil
}
