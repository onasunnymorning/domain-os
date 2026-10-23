package workflows

import (
	"strconv"
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"

	"github.com/onasunnymorning/domain-os/internal/application/services"
)

// RestoreLoopParams are the input of RestoreWorkflow. Zero values apply when
// it is started without arguments, as the schedule does.
type RestoreLoopParams struct {
	// BatchSize caps how many lapsed restores one run lists and reverts
	// (default 1000).
	BatchSize int `json:"batchSize,omitempty"`
	// ContinuationCount is how many times this sweep has continued-as-new.
	// The workflow manages it; leave it zero.
	ContinuationCount int `json:"continuationCount,omitempty"`
	// DueBy, AfterRestoreEnd and AfterRoID carry one sweep across
	// continue-as-new: the time it started with, and the (deadline, ROID) of
	// the last domain it listed. The workflow manages them; leave them zero.
	DueBy           time.Time `json:"dueBy,omitempty"`
	AfterRestoreEnd time.Time `json:"afterRestoreEnd,omitempty"`
	AfterRoID       int64     `json:"afterRoid,omitempty"`
}

// RestoreLoopResult is the outcome of one run.
type RestoreLoopResult struct {
	StartedAt      time.Time        `json:"startedAt"`
	CompletedAt    time.Time        `json:"completedAt"`
	DueBy          time.Time        `json:"dueBy"`
	TotalFound     int              `json:"totalFound"`
	TotalProcessed int              `json:"totalProcessed"`
	Reverted       int              `json:"reverted"`
	Failed         int              `json:"failed"`
	Skipped        int              `json:"skipped"`
	Notes          []string         `json:"notes"`
	Failures       []RestoreFailure `json:"failures,omitempty"`
}

// RestoreFailure is one domain whose lapsed restore could not be reverted.
type RestoreFailure struct {
	DomainName string `json:"domainName"`
	Error      string `json:"error"`
}

func (r *RestoreLoopResult) addFailure(domainName, errMsg string) {
	r.Failed++
	if len(r.Failures) < maxFailureSamples {
		r.Failures = append(r.Failures, RestoreFailure{DomainName: domainName, Error: errMsg})
	}
}

// RestoreWorkflow reverts the restores whose report the registrar did not
// send in time (RFC 3915): a domain pendingRestore past its
// PendingRestoreEnd (the phase's pending restore period, 7 days by
// default) returns to pendingDelete as it was before the request, in its
// redemption period if that has not ended, with its original purge date
// (entities.Domain.RevertRestore). A restore completes in the EPP command
// that sends the report, never here.
//
// A sweep lists the restores lapsed as of its start, a page per run in
// (deadline, ROID) order, and reverts each page in one batch activity. The
// batch reads and checks every domain again, so it is idempotent under
// retries: a domain restored meanwhile, already reverted, or whose restore
// has not lapsed is Skipped. A full page continues-as-new from the page's
// last domain, so a domain that cannot be reverted is passed over, not
// listed again, and cannot hold back the ones after it; the next sweep
// retries it. maxContinuationRuns bounds a sweep.
func RestoreWorkflow(ctx workflow.Context, params RestoreLoopParams) (RestoreLoopResult, error) {
	started := workflow.Now(ctx)
	dueBy := params.DueBy
	if dueBy.IsZero() {
		dueBy = started // a new sweep
	}
	result := RestoreLoopResult{StartedAt: started, DueBy: dueBy}
	if err := workflow.SetQueryHandler(ctx, "progress", func() (RestoreLoopResult, error) {
		return result, nil
	}); err != nil {
		result.CompletedAt = workflow.Now(ctx)
		result.Notes = append(result.Notes, "Failed to register query handler: "+err.Error())
		return result, err
	}
	workflowID := getWorkflowID(ctx)
	retry := &temporal.RetryPolicy{
		InitialInterval:    time.Second,
		BackoffCoefficient: 2.0,
		MaximumInterval:    10 * time.Minute,
		MaximumAttempts:    3,
	}
	listCtx := workflow.WithActivityOptions(ctx, workflow.ActivityOptions{StartToCloseTimeout: time.Minute, RetryPolicy: retry})
	batchSize := pageSizeOrDefaultWorkflow(params.BatchSize)

	var lapsed []services.LapsedRestore
	if err := workflow.ExecuteActivity(listCtx, "ListLapsedRestores", workflowID, dueBy, params.AfterRestoreEnd, params.AfterRoID, batchSize).Get(ctx, &lapsed); err != nil {
		result.CompletedAt = workflow.Now(ctx)
		result.Notes = append(result.Notes, "Failed to list lapsed restores: "+err.Error())
		return result, err
	}
	result.TotalFound = len(lapsed)
	if len(lapsed) == 0 {
		result.CompletedAt = workflow.Now(ctx)
		result.Notes = append(result.Notes, "No lapsed restores")
		return result, nil
	}

	batchCtx := workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout: 30 * time.Minute,
		HeartbeatTimeout:    2 * time.Minute,
		RetryPolicy:         retry,
	})
	names := make([]string, len(lapsed))
	for i, l := range lapsed {
		names[i] = l.DomainName
	}
	var batch services.BatchResult
	if err := workflow.ExecuteActivity(batchCtx, "BatchRevertLapsedRestores", workflowID, dueBy, names).Get(ctx, &batch); err != nil {
		result.CompletedAt = workflow.Now(ctx)
		result.Notes = append(result.Notes, "Batch revert failed: "+err.Error())
		return result, err
	}
	result.Reverted = len(batch.Succeeded)
	result.Skipped = len(batch.Skipped)
	result.TotalProcessed = len(batch.Succeeded) + len(batch.Skipped) + len(batch.Failed)
	for _, f := range batch.Failed {
		result.addFailure(f.DomainName, f.Error)
	}
	result.CompletedAt = workflow.Now(ctx)
	if result.Failed > 0 {
		result.Notes = append(result.Notes, "Completed with failures — review the failures list for details")
	}

	// A full page may have more after it. The cursor moves past this page
	// whatever became of it, so the sweep always advances.
	if len(lapsed) >= batchSize {
		if params.ContinuationCount >= maxContinuationRuns {
			result.Notes = append(result.Notes, "Continuation cap reached ("+strconv.Itoa(maxContinuationRuns)+
				" runs) — the remaining restores are picked up by the next scheduled run.")
			return result, nil
		}
		result.Notes = append(result.Notes, "Full page of "+strconv.Itoa(len(lapsed))+
			" lapsed restores processed — continuing in a new run from the last one.")
		last := lapsed[len(lapsed)-1]
		next := params
		next.ContinuationCount++
		next.DueBy, next.AfterRestoreEnd, next.AfterRoID = dueBy, last.RestoreEnd, last.RoID
		return result, workflow.NewContinueAsNewError(ctx, RestoreTypeName, next)
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
