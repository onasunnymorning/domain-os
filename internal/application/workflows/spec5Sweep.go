package workflows

import (
	"time"

	"github.com/onasunnymorning/domain-os/internal/application/activities"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

// Spec5SweepParams are the input parameters for Spec5SweepWorkflow.
type Spec5SweepParams struct {
	TLD     string   `json:"tld,omitempty"`     // A single TLD to check, e.g. "com"
	TLDs    []string `json:"tlds,omitempty"`    // A list of TLDs to check, e.g. ["com", "net"]
	AllTLDs bool     `json:"allTlds,omitempty"` // If true, checks the entire system (all TLDs in db)
}

// Spec5SweepWorkflow sweeps the domain inventory and returns matching Spec5 labels.
func Spec5SweepWorkflow(ctx workflow.Context, params Spec5SweepParams) (activities.Spec5SweepResult, error) {
	workflowID := getWorkflowID(ctx)

	// RetryPolicy specifies how to automatically handle retries if the Activity fails.
	retryPolicy := &temporal.RetryPolicy{
		InitialInterval:        time.Second,
		BackoffCoefficient:     2.0,
		MaximumInterval:        10 * time.Minute,
		MaximumAttempts:        3,
		NonRetryableErrorTypes: []string{"none"},
	}

	options := workflow.ActivityOptions{
		// 10 minutes timeout to handle slow DB queries/network
		StartToCloseTimeout: 10 * time.Minute,
		RetryPolicy:         retryPolicy,
	}

	ctx = workflow.WithActivityOptions(ctx, options)

	var acts *activities.Spec5SweepActivities
	args := activities.Spec5SweepArgs{
		TLD:        params.TLD,
		TLDs:       params.TLDs,
		AllTLDs:    params.AllTLDs,
		WorkflowID: workflowID,
	}

	var result activities.Spec5SweepResult
	err := workflow.ExecuteActivity(ctx, acts.SweepSpec5Labels, args).Get(ctx, &result)
	if err != nil {
		workflow.GetLogger(ctx).Error("Failed to sweep Spec5 labels", "error", err)
		return activities.Spec5SweepResult{}, err
	}

	return result, nil
}
