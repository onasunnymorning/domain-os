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

const (
	// maxFailureSamples caps the number of individual failure records stored in the result,
	// aligned with the QA report sampledItems cap of 50.
	maxFailureSamples = 50

	// autoRenewYears is the renewal term applied when the expiry loop
	// auto-renews a domain. Registry policy: auto-renewals are always 1 year.
	autoRenewYears = 1

	// maxContinuationRuns caps the continue-as-new chain length for a single
	// scheduled run. Combined with the progress guard this bounds the work a
	// run can do; anything left over is picked up by the next scheduled run.
	maxContinuationRuns = 50
)

// ExpiryLoopParams defines the input parameters for the ExpiryLoop workflow.
type ExpiryLoopParams struct {
	// BatchSize caps how many domains are listed and processed per run
	// (default: 1000, the admin API maximum page size).
	BatchSize             int        `json:"batchSize,omitempty"`
	DryRun                bool       `json:"dryRun,omitempty"`
	ReferenceTimeOverride *time.Time `json:"referenceTimeOverride,omitempty"`
	// ContinuationCount tracks how many times this run has continued-as-new.
	// Managed by the workflow — leave zero when starting a run.
	ContinuationCount int `json:"continuationCount,omitempty"`
}

// ExpiryLoopResult is the structured output of the ExpiryLoop workflow.
// It reports what happened during the run so operators can observe outcomes
// without digging through logs.
type ExpiryLoopResult struct {
	StartedAt      time.Time           `json:"startedAt"`
	CompletedAt    time.Time           `json:"completedAt"`
	ReferenceTime  time.Time           `json:"referenceTime"`
	TotalFound     int64               `json:"totalFound"`
	TotalProcessed int                 `json:"totalProcessed"`
	AutoRenewed    int                 `json:"autoRenewed"`
	Expired        int                 `json:"expired"`
	Failed         int                 `json:"failed"`
	Skipped        int                 `json:"skipped"`
	Notes          []string            `json:"notes"`
	Failures       []ExpiryLoopFailure `json:"failures,omitempty"`
}

// ExpiryLoopFailure records a single domain processing failure.
type ExpiryLoopFailure struct {
	DomainName string `json:"domainName"`
	Operation  string `json:"operation"` // "auto-renew-check", "auto-renew", "expire"
	Error      string `json:"error"`
}

// addFailure appends a failure record, respecting the maxFailureSamples cap.
// The Failed counter is always incremented regardless of cap.
func (r *ExpiryLoopResult) addFailure(domainName, operation, errMsg string) {
	r.Failed++
	if len(r.Failures) < maxFailureSamples {
		r.Failures = append(r.Failures, ExpiryLoopFailure{
			DomainName: domainName,
			Operation:  operation,
			Error:      errMsg,
		})
	}
}

// ExpiryLoop processes domains that have passed their expiry date: eligible
// domains are auto-renewed, the rest are expired (moved to pendingDelete).
//
// The loop is idempotent end-to-end: eligibility is partitioned via batched DB
// lookups (BatchCheckAutoRenewEligibility) and the batch write activities skip
// domains already transitioned by an earlier attempt, so activity retries can
// never double-renew or double-bill a domain.
//
// If more domains match than fit in one batch, the workflow continues-as-new —
// but only when the current run actually made progress, and never more than
// maxContinuationRuns times. This prevents a batch of persistently failing
// domains from spinning the workflow in a hot loop.
//
// ref: https://www.notion.so/apex-domains/Domain-lifecycle-18200bd9d73849e6abfe2e616f1a3443?pvs=4#2e597291f85a43699422a7ac5f122bc8
func ExpiryLoop(ctx workflow.Context, params ExpiryLoopParams) (ExpiryLoopResult, error) {
	started := workflow.Now(ctx)
	result := ExpiryLoopResult{
		StartedAt: started,
	}

	// Register a query handler so the Temporal UI can observe progress in real-time.
	err := workflow.SetQueryHandler(ctx, "progress", func() (ExpiryLoopResult, error) {
		return result, nil
	})
	if err != nil {
		result.CompletedAt = workflow.Now(ctx)
		result.Notes = append(result.Notes, "Failed to register query handler: "+err.Error())
		return result, err
	}

	// Get the workflow ID for correlation
	workflowID := getWorkflowID(ctx)

	// Lock a single reference time for the entire run to eliminate TOCTOU races
	// between count and list queries.
	var referenceTime time.Time
	if params.ReferenceTimeOverride != nil {
		referenceTime = *params.ReferenceTimeOverride
	} else {
		referenceTime = workflow.Now(ctx).UTC()
	}
	result.ReferenceTime = referenceTime

	// Build the query with a locked reference time — both count and list will use the same cutoff.
	query := queries.ExpiringDomainsQuery{
		Before:   referenceTime,
		PageSize: params.BatchSize,
	}

	// RetryPolicy specifies how to automatically handle retries if an Activity fails.
	retrypolicy := &temporal.RetryPolicy{
		InitialInterval:    time.Second,
		BackoffCoefficient: 2.0,
		MaximumInterval:    10 * time.Minute,
		MaximumAttempts:    3, // 0 is unlimited retries
	}

	options := workflow.ActivityOptions{
		// Timeout options specify when to automatically timeout Activity functions.
		StartToCloseTimeout: time.Minute,
		RetryPolicy:         retrypolicy,
	}

	// Apply the options.
	ctx = workflow.WithActivityOptions(ctx, options)

	// Options for long-running batch activities that heartbeat between chunks.
	batchCtx := workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout: 30 * time.Minute,
		HeartbeatTimeout:    2 * time.Minute,
		RetryPolicy:         retrypolicy,
	})

	// Step 1: Count expired domains
	domainCount := &response.CountResult{}
	getExpiredDomainCountError := workflow.ExecuteActivity(ctx, activities.GetExpiredDomainCount, workflowID, query).Get(ctx, domainCount)
	if getExpiredDomainCountError != nil {
		result.CompletedAt = workflow.Now(ctx)
		result.Notes = append(result.Notes, "Failed to count expired domains: "+getExpiredDomainCountError.Error())
		return result, getExpiredDomainCountError
	}
	result.TotalFound = domainCount.Count

	// If there are no domains to expire, return early with a note
	if domainCount.Count == 0 {
		result.CompletedAt = workflow.Now(ctx)
		result.Notes = append(result.Notes, "No expired domains found")
		return result, nil
	}

	// Step 2: List the expired domains
	domains := []response.DomainExpiryItem{}
	getExpiredDomainsError := workflow.ExecuteActivity(ctx, activities.ListExpiringDomains, workflowID, query).Get(ctx, &domains)
	if getExpiredDomainsError != nil {
		result.CompletedAt = workflow.Now(ctx)
		result.Notes = append(result.Notes, "Failed to list expired domains: "+getExpiredDomainsError.Error())
		return result, getExpiredDomainsError
	}

	if len(domains) == 0 {
		result.CompletedAt = workflow.Now(ctx)
		result.Notes = append(result.Notes, "No expired domains found in list")
		return result, nil
	}

	// Step 3: Partition the batch into auto-renew and expiry candidates using
	// batched DB lookups (one activity, chunked internally with heartbeats).
	domainNames := make([]string, len(domains))
	for i, d := range domains {
		domainNames[i] = d.Name
	}

	var partition services.EligibilityPartition
	partitionErr := workflow.ExecuteActivity(batchCtx, "BatchCheckAutoRenewEligibility", workflowID, domainNames).Get(ctx, &partition)
	if partitionErr != nil {
		result.CompletedAt = workflow.Now(ctx)
		result.Notes = append(result.Notes, "Failed to partition domains for auto-renew eligibility: "+partitionErr.Error())
		return result, partitionErr
	}

	// Record eligibility failures and skips
	for _, fail := range partition.Failures {
		result.addFailure(fail.DomainName, "auto-renew-check", fail.Error)
		result.TotalProcessed++
	}
	result.Skipped += len(partition.Skipped)
	result.TotalProcessed += len(partition.Skipped)

	// Dry run short circuit
	if params.DryRun {
		result.TotalProcessed += len(partition.EligibleForAutoRenew) + len(partition.EligibleForExpiry)
		result.AutoRenewed = len(partition.EligibleForAutoRenew)
		result.Expired = len(partition.EligibleForExpiry)
		result.CompletedAt = workflow.Now(ctx)
		result.Notes = append(result.Notes, "Dry run completed: no state changes made")
		return result, nil
	}

	// Step 4a: Batch auto-renew
	if len(partition.EligibleForAutoRenew) > 0 {
		var autoRenewBatch services.BatchResult
		autoRenewErr := workflow.ExecuteActivity(batchCtx, "BatchAutoRenewDomains", workflowID, partition.EligibleForAutoRenew, autoRenewYears).Get(ctx, &autoRenewBatch)
		if autoRenewErr != nil {
			result.addFailure("batch-auto-renew", "auto-renew", autoRenewErr.Error())
		} else {
			result.AutoRenewed = len(autoRenewBatch.Succeeded)
			result.Skipped += len(autoRenewBatch.Skipped)
			result.TotalProcessed += len(autoRenewBatch.Succeeded) + len(autoRenewBatch.Skipped) + len(autoRenewBatch.Failed)
			for _, f := range autoRenewBatch.Failed {
				result.addFailure(f.DomainName, "auto-renew", f.Error)
			}
		}
	}

	// Step 4b: Batch expire
	if len(partition.EligibleForExpiry) > 0 {
		var expireBatch services.BatchResult
		expireErr := workflow.ExecuteActivity(batchCtx, "BatchExpireDomains", workflowID, partition.EligibleForExpiry).Get(ctx, &expireBatch)
		if expireErr != nil {
			result.addFailure("batch-expire", "expire", expireErr.Error())
		} else {
			result.Expired = len(expireBatch.Succeeded)
			result.Skipped += len(expireBatch.Skipped)
			result.TotalProcessed += len(expireBatch.Succeeded) + len(expireBatch.Skipped) + len(expireBatch.Failed)
			for _, f := range expireBatch.Failed {
				result.addFailure(f.DomainName, "expire", f.Error)
			}
		}
	}

	result.CompletedAt = workflow.Now(ctx)

	// Add summary note
	if result.Failed > 0 {
		result.Notes = append(result.Notes, "Completed with failures — review the failures list for details")
		if result.Failed > maxFailureSamples {
			result.Notes = append(result.Notes, "Failure details capped at "+itoa(maxFailureSamples)+
				" samples; total failures: "+itoa(result.Failed))
		}
	}

	// Check if we hit the batch cap and should continue-as-new to drain the remainder.
	if int64(len(domains)) < domainCount.Count {
		// Only continue when this run actually made progress. Failed domains
		// stay in the query result set, so continuing without progress would
		// spin the workflow in a hot loop over the same poison batch.
		progressed := result.AutoRenewed+result.Expired+result.Skipped > 0
		if !progressed {
			result.Notes = append(result.Notes, "Batch cap reached but no progress was made this run — "+
				"not continuing to avoid a hot loop. Remaining domains will be retried on the next scheduled run.")
			return result, nil
		}
		if params.ContinuationCount >= maxContinuationRuns {
			result.Notes = append(result.Notes, "Continuation cap reached ("+itoa(maxContinuationRuns)+
				" runs) — remaining domains will be picked up by the next scheduled run.")
			return result, nil
		}
		result.Notes = append(result.Notes, "Batch cap reached: listed "+
			itoa(len(domains))+" of "+itoa64(domainCount.Count)+
			" expired domains. Continuing processing in a new run.")
		nextParams := params
		nextParams.ContinuationCount++
		return result, workflow.NewContinueAsNewError(ctx, ExpiryLoop, nextParams)
	}

	return result, nil
}

// itoa converts an int to string.
func itoa(n int) string {
	return strconv.Itoa(n)
}

// itoa64 converts an int64 to string.
func itoa64(n int64) string {
	return strconv.FormatInt(n, 10)
}
