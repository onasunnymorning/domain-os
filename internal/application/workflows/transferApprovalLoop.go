package workflows

import (
	"strconv"
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"

	"github.com/onasunnymorning/domain-os/internal/application/services"
)

// TransferApprovalLoopParams are the input of TransferApprovalLoop. Zero
// values apply when it is started without arguments, as the schedule does.
type TransferApprovalLoopParams struct {
	// BatchSize caps how many due transfers one run lists and approves
	// (default 1000).
	BatchSize int `json:"batchSize,omitempty"`
	// ContinuationCount is how many times this sweep has continued-as-new.
	// The workflow manages it; leave it zero.
	ContinuationCount int `json:"continuationCount,omitempty"`
	// DueBy, AfterActionDate and AfterID carry one sweep across
	// continue-as-new: the due time it started with, and the (acDate, ID)
	// of the last transfer it listed. The workflow manages them; leave them
	// zero.
	DueBy           time.Time `json:"dueBy,omitempty"`
	AfterActionDate time.Time `json:"afterActionDate,omitempty"`
	AfterID         int64     `json:"afterId,omitempty"`
}

// TransferApprovalLoopResult is the outcome of one run.
type TransferApprovalLoopResult struct {
	StartedAt      time.Time         `json:"startedAt"`
	CompletedAt    time.Time         `json:"completedAt"`
	DueBy          time.Time         `json:"dueBy"`
	TotalFound     int               `json:"totalFound"`
	TotalProcessed int               `json:"totalProcessed"`
	Approved       int               `json:"approved"`
	Failed         int               `json:"failed"`
	Skipped        int               `json:"skipped"`
	Notes          []string          `json:"notes"`
	Failures       []TransferFailure `json:"failures,omitempty"`
}

// TransferFailure is one domain whose transfer could not be approved.
type TransferFailure struct {
	DomainName string `json:"domainName"`
	Error      string `json:"error"`
}

func (r *TransferApprovalLoopResult) addFailure(domainName, errMsg string) {
	r.Failed++
	if len(r.Failures) < maxFailureSamples {
		r.Failures = append(r.Failures, TransferFailure{DomainName: domainName, Error: errMsg})
	}
}

// TransferApprovalLoop approves, as the registry, the domain transfers the
// losing registrar has not answered by their acDate: the end of the phase's
// pending transfer period (5 days by default; ICANN Transfer Policy). Each
// becomes serverApproved and completes as an approval by the losing
// registrar does (entities.DomainTransfer), and both registrars get a poll
// message.
//
// A sweep lists the transfers due as of its start, of domains that still
// exist, a page per run in (acDate, ID) order, and approves each page in
// one batch activity. The batch reads and checks every domain again, so it
// is idempotent under retries: a transfer answered meanwhile, approved by
// an earlier attempt, or not yet due is Skipped. A full page continues-as-
// new from the page's last transfer, so a transfer that cannot be approved
// is passed over, not listed again, and cannot hold back the ones after it;
// it is retried by the next hourly sweep. maxContinuationRuns bounds a
// sweep.
func TransferApprovalLoop(ctx workflow.Context, params TransferApprovalLoopParams) (TransferApprovalLoopResult, error) {
	started := workflow.Now(ctx)
	dueBy := params.DueBy
	if dueBy.IsZero() {
		dueBy = started // a new sweep
	}
	result := TransferApprovalLoopResult{StartedAt: started, DueBy: dueBy}
	if err := workflow.SetQueryHandler(ctx, "progress", func() (TransferApprovalLoopResult, error) {
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

	var due []services.DueTransfer
	if err := workflow.ExecuteActivity(listCtx, "ListDueTransfers", workflowID, dueBy, params.AfterActionDate, params.AfterID, batchSize).Get(ctx, &due); err != nil {
		result.CompletedAt = workflow.Now(ctx)
		result.Notes = append(result.Notes, "Failed to list due transfers: "+err.Error())
		return result, err
	}
	result.TotalFound = len(due)
	if len(due) == 0 {
		result.CompletedAt = workflow.Now(ctx)
		result.Notes = append(result.Notes, "No transfers due")
		return result, nil
	}

	batchCtx := workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout: 30 * time.Minute,
		HeartbeatTimeout:    2 * time.Minute,
		RetryPolicy:         retry,
	})
	names := make([]string, len(due))
	for i, d := range due {
		names[i] = d.DomainName
	}
	var batch services.BatchResult
	if err := workflow.ExecuteActivity(batchCtx, "BatchApproveDueTransfers", workflowID, dueBy, names).Get(ctx, &batch); err != nil {
		result.CompletedAt = workflow.Now(ctx)
		result.Notes = append(result.Notes, "Batch approval failed: "+err.Error())
		return result, err
	}
	result.Approved = len(batch.Succeeded)
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
	if len(due) >= batchSize {
		if params.ContinuationCount >= maxContinuationRuns {
			result.Notes = append(result.Notes, "Continuation cap reached ("+strconv.Itoa(maxContinuationRuns)+
				" runs) — the remaining transfers are picked up by the next scheduled run.")
			return result, nil
		}
		result.Notes = append(result.Notes, "Full page of "+strconv.Itoa(len(due))+
			" due transfers processed — continuing in a new run from the last one.")
		last := due[len(due)-1]
		next := params
		next.ContinuationCount++
		next.DueBy, next.AfterActionDate, next.AfterID = dueBy, last.ActionDate, last.ID
		return result, workflow.NewContinueAsNewError(ctx, TransferApprovalLoop, next)
	}
	return result, nil
}
