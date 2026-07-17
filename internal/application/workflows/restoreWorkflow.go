package workflows

import (
	"strconv"
	"time"

	"github.com/onasunnymorning/domain-os/internal/application/activities"
	"github.com/onasunnymorning/domain-os/internal/application/queries"
	"github.com/onasunnymorning/domain-os/internal/application/services"
	"github.com/onasunnymorning/domain-os/internal/interface/rest/response"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

// RestoreLoopParams defines the input parameters for the RestoreWorkflow.
// The workflow tolerates being started without arguments (zero values apply),
// which keeps existing schedules and manual triggers compatible.
type RestoreLoopParams struct {
	// BatchSize caps how many domains are listed and processed per run
	// (default: 1000, the admin API maximum page size).
	BatchSize int `json:"batchSize,omitempty"`
	// ContinuationCount tracks how many times this run has continued-as-new.
	// Managed by the workflow — leave zero when starting a run.
	ContinuationCount int `json:"continuationCount,omitempty"`
}

// RestoreLoopResult is the structured output of the RestoreWorkflow.
type RestoreLoopResult struct {
	StartedAt      time.Time        `json:"startedAt"`
	CompletedAt    time.Time        `json:"completedAt"`
	TotalFound     int              `json:"totalFound"`
	TotalProcessed int              `json:"totalProcessed"`
	Restored       int              `json:"restored"`
	Failed         int              `json:"failed"`
	Skipped        int              `json:"skipped"`
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

// RestoreWorkflow completes restoration for domains in pendingRestore state:
// each domain has PendingRestore unset and is force-renewed for one year in a
// single write. The batch activity is idempotent under retries — domains whose
// restore was already completed are reported as Skipped.
//
// The list is processed one page per run. When a full page was returned there
// may be more pendingRestore domains, so the workflow continues-as-new — but
// only when the run made progress and the continuation cap is not exceeded.
func RestoreWorkflow(ctx workflow.Context, params RestoreLoopParams) (RestoreLoopResult, error) {
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

	// Step 1: List restored domains (one page per run)
	query := &queries.RestoredDomainsQuery{
		PageSize: params.BatchSize,
	}
	domainList := []response.DomainRestoredItem{}
	listErr := workflow.ExecuteActivity(ctx, activities.ListRestoredDomains, workflowID, query).Get(ctx, &domainList)
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
	result.Skipped = len(restoreBatch.Skipped)
	result.TotalProcessed = len(restoreBatch.Succeeded) + len(restoreBatch.Skipped) + len(restoreBatch.Failed)
	for _, f := range restoreBatch.Failed {
		result.addFailure(f.DomainName, f.Error)
	}

	result.CompletedAt = workflow.Now(ctx)

	if result.Failed > 0 {
		result.Notes = append(result.Notes, "Completed with failures — review the failures list for details")
	}

	// A full page means there may be more pendingRestore domains waiting.
	// Drain them with continue-as-new, guarded against hot loops.
	fullPage := len(domainList) >= pageSizeOrDefaultWorkflow(params.BatchSize)
	if fullPage {
		progressed := result.Restored+result.Skipped > 0
		if !progressed {
			result.Notes = append(result.Notes, "Full page processed without progress — "+
				"not continuing to avoid a hot loop. Remaining domains will be retried on the next scheduled run.")
			return result, nil
		}
		if params.ContinuationCount >= maxContinuationRuns {
			result.Notes = append(result.Notes, "Continuation cap reached ("+strconv.Itoa(maxContinuationRuns)+
				" runs) — remaining domains will be picked up by the next scheduled run.")
			return result, nil
		}
		result.Notes = append(result.Notes, "Full page of "+strconv.Itoa(len(domainList))+
			" restored domains processed — continuing in a new run to drain the remainder.")
		nextParams := params
		nextParams.ContinuationCount++
		return result, workflow.NewContinueAsNewError(ctx, RestoreWorkflow, nextParams)
	}

	return result, nil
}

// pageSizeOrDefaultWorkflow mirrors the activity-side page size defaulting so
// the workflow can detect a full page deterministically.
func pageSizeOrDefaultWorkflow(requested int) int {
	if requested > 0 {
		return requested
	}
	return 1000 // activities.BATCHSIZE — kept literal to avoid importing activity state into workflow code
}
