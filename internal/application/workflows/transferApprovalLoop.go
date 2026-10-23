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
	// ContinuationCount is how many times this run has continued-as-new.
	// The workflow manages it; leave it zero.
	ContinuationCount int `json:"continuationCount,omitempty"`
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
// A run lists one page of due transfers, as of the run's start, and
// approves them in one batch activity. The batch reads and checks every
// domain again, so it is idempotent under retries: a transfer answered
// meanwhile, approved by an earlier attempt, or not yet due is Skipped. A
// full page continues-as-new to drain the rest, but only after progress and
// within maxContinuationRuns, so a poison batch cannot loop.
func TransferApprovalLoop(ctx workflow.Context, params TransferApprovalLoopParams) (TransferApprovalLoopResult, error) {
	started := workflow.Now(ctx)
	result := TransferApprovalLoopResult{StartedAt: started, DueBy: started}
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

	var names []string
	if err := workflow.ExecuteActivity(listCtx, "ListDueTransfers", workflowID, started, batchSize).Get(ctx, &names); err != nil {
		result.CompletedAt = workflow.Now(ctx)
		result.Notes = append(result.Notes, "Failed to list due transfers: "+err.Error())
		return result, err
	}
	result.TotalFound = len(names)
	if len(names) == 0 {
		result.CompletedAt = workflow.Now(ctx)
		result.Notes = append(result.Notes, "No transfers due")
		return result, nil
	}

	batchCtx := workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout: 30 * time.Minute,
		HeartbeatTimeout:    2 * time.Minute,
		RetryPolicy:         retry,
	})
	var batch services.BatchResult
	if err := workflow.ExecuteActivity(batchCtx, "BatchApproveDueTransfers", workflowID, started, names).Get(ctx, &batch); err != nil {
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

	if len(names) >= batchSize {
		if result.Approved+result.Skipped == 0 {
			result.Notes = append(result.Notes, "Full page processed without progress — not continuing, to avoid a hot loop. "+
				"The remaining transfers are retried on the next scheduled run.")
			return result, nil
		}
		if params.ContinuationCount >= maxContinuationRuns {
			result.Notes = append(result.Notes, "Continuation cap reached ("+strconv.Itoa(maxContinuationRuns)+
				" runs) — the remaining transfers are picked up by the next scheduled run.")
			return result, nil
		}
		result.Notes = append(result.Notes, "Full page of "+strconv.Itoa(len(names))+
			" due transfers processed — continuing in a new run to drain the remainder.")
		next := params
		next.ContinuationCount++
		return result, workflow.NewContinueAsNewError(ctx, TransferApprovalLoop, next)
	}
	return result, nil
}
