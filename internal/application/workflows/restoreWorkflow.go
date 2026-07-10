package workflows

import (
	"time"

	"github.com/onasunnymorning/domain-os/internal/application/activities"
	"github.com/onasunnymorning/domain-os/internal/application/services"
	"github.com/onasunnymorning/domain-os/internal/interface/rest/response"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

// RestoreLoopResult is the structured output of the RestoreWorkflow.
type RestoreLoopResult struct {
	StartedAt      time.Time        `json:"startedAt"`
	CompletedAt    time.Time        `json:"completedAt"`
	TotalFound     int              `json:"totalFound"`
	TotalProcessed int              `json:"totalProcessed"`
	Restored       int              `json:"restored"`
	Failed         int              `json:"failed"`
	Notes          []string         `json:"notes"`
	Failures       []RestoreFailure `json:"failures,omitempty"`
}

// RestoreFailure records a single restore failure.
type RestoreFailure struct {
	DomainName string `json:"domainName"`
	Error      string `json:"error"`
}

func (r *RestoreLoopResult) addFailure(domainName, errMsg string) {
	r.Failed++
	if len(r.Failures) < maxFailureSamples {
		r.Failures = append(r.Failures, RestoreFailure{
			DomainName: domainName,
			Error:      errMsg,
		})
	}
}

func RestoreWorkflow(ctx workflow.Context) (RestoreLoopResult, error) {
	started := workflow.Now(ctx)
	result := RestoreLoopResult{
		StartedAt: started,
	}

	// Register a query handler so progress is visible
	err := workflow.SetQueryHandler(ctx, "progress", func() (RestoreLoopResult, error) {
		return result, nil
	})
	if err != nil {
		result.CompletedAt = workflow.Now(ctx)
		result.Notes = append(result.Notes, "Failed to register query handler: "+err.Error())
		return result, err
	}

	workflowID := getWorkflowID(ctx)

	retrypolicy := &temporal.RetryPolicy{
		InitialInterval:    time.Second,
		BackoffCoefficient: 2.0,
		MaximumInterval:    10 * time.Minute,
		MaximumAttempts:    3,
	}

	options := workflow.ActivityOptions{
		StartToCloseTimeout: time.Minute,
		RetryPolicy:         retrypolicy,
	}
	ctx = workflow.WithActivityOptions(ctx, options)

	// Step 1: List restored domains
	domainList := []response.DomainRestoredItem{}
	listErr := workflow.ExecuteActivity(ctx, activities.ListRestoredDomains, workflowID).Get(ctx, &domainList)
	if listErr != nil {
		result.CompletedAt = workflow.Now(ctx)
		result.Notes = append(result.Notes, "Failed to list restored domains: "+listErr.Error())
		return result, listErr
	}

	result.TotalFound = len(domainList)

	if len(domainList) == 0 {
		result.CompletedAt = workflow.Now(ctx)
		result.Notes = append(result.Notes, "No restored domains found")
		return result, nil
	}

	// Step 2: Batch restore
	domainNames := make([]string, len(domainList))
	for i, d := range domainList {
		domainNames[i] = d.Name
	}

	var restoreBatch services.BatchResult
	batchCtx := workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout: 30 * time.Minute,
		HeartbeatTimeout:    2 * time.Minute,
		RetryPolicy:         retrypolicy,
	})
	restoreErr := workflow.ExecuteActivity(batchCtx, "BatchRestoreDomains", workflowID, domainNames).Get(ctx, &restoreBatch)
	if restoreErr != nil {
		result.CompletedAt = workflow.Now(ctx)
		result.Notes = append(result.Notes, "Batch restore failed: "+restoreErr.Error())
		return result, restoreErr
	}

	result.Restored = len(restoreBatch.Succeeded)
	result.TotalProcessed = len(restoreBatch.Succeeded) + len(restoreBatch.Failed)
	for _, f := range restoreBatch.Failed {
		result.addFailure(f.DomainName, f.Error)
	}

	result.CompletedAt = workflow.Now(ctx)

	if result.Failed > 0 {
		result.Notes = append(result.Notes, "Completed with failures — review the failures list for details")
	}

	return result, nil
}
