package workflows

import (
	"strconv"
	"time"

	"github.com/onasunnymorning/domain-os/internal/application/activities"
	"github.com/onasunnymorning/domain-os/internal/application/queries"
	"github.com/onasunnymorning/domain-os/internal/interface/rest/response"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

const (
	// maxFailureSamples caps the number of individual failure records stored in the result,
	// aligned with the QA report sampledItems cap of 50.
	maxFailureSamples = 50
)

// ExpiryLoopParams defines the input parameters for the ExpiryLoop workflow.
type ExpiryLoopParams struct {
	BatchSize             int        `json:"batchSize,omitempty"`
	ConcurrencyLimit      int        `json:"concurrencyLimit,omitempty"`
	DryRun                bool       `json:"dryRun,omitempty"`
	ReferenceTimeOverride *time.Time `json:"referenceTimeOverride,omitempty"`
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

// ExpiryLoop ref: https://www.notion.so/apex-domains/Domain-lifecycle-18200bd9d73849e6abfe2e616f1a3443?pvs=4#2e597291f85a43699422a7ac5f122bc8
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

	// Set defaults
	concurrencyLimit := params.ConcurrencyLimit
	if concurrencyLimit <= 0 {
		concurrencyLimit = 20
	}

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
		Before: referenceTime,
	}

	// RetryPolicy specifies how to automatically handle retries if an Activity fails.
	retrypolicy := &temporal.RetryPolicy{
		InitialInterval:        time.Second,
		BackoffCoefficient:     2.0,
		MaximumInterval:        10 * time.Minute,
		MaximumAttempts:        3, // 0 is unlimited retries
	}

	options := workflow.ActivityOptions{
		// Timeout options specify when to automatically timeout Activity functions.
		StartToCloseTimeout: time.Minute,
		RetryPolicy:         retrypolicy,
	}

	// Apply the options.
	ctx = workflow.WithActivityOptions(ctx, options)

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

	// Step 3: Batch check auto-renew eligibility
	domainNames := make([]string, len(domains))
	for i, d := range domains {
		domainNames[i] = d.Name
	}

	var batchCheckResult activities.CheckDomainsCanAutoRenewResult
	batchCheckErr := workflow.ExecuteActivity(ctx, activities.CheckDomainsCanAutoRenew, workflowID, domainNames).Get(ctx, &batchCheckResult)
	if batchCheckErr != nil {
		result.CompletedAt = workflow.Now(ctx)
		result.Notes = append(result.Notes, "Failed to batch check auto-renew eligibility: "+batchCheckErr.Error())
		return result, batchCheckErr
	}

	// Record any check failures from the batch activity
	for _, fail := range batchCheckResult.CheckFailures {
		result.addFailure(fail.DomainName, "auto-renew-check", fail.Error)
		result.TotalProcessed++
	}

	// Dry run short circuit
	if params.DryRun {
		result.TotalProcessed += len(batchCheckResult.EligibleForAutoRenew) + len(batchCheckResult.EligibleForExpiry)
		result.AutoRenewed = len(batchCheckResult.EligibleForAutoRenew)
		result.Expired = len(batchCheckResult.EligibleForExpiry)
		result.CompletedAt = workflow.Now(ctx)
		result.Notes = append(result.Notes, "Dry run completed: no state changes made")
		return result, nil
	}

	// Step 4: Process write activities in parallel with bounded concurrency
	semCh := workflow.NewBufferedChannel(ctx, concurrencyLimit)

	type futInfo struct {
		domainName string
		operation  string
		future     workflow.Future
	}
	var futures []futInfo

	// Trigger Auto-Renews
	for _, name := range batchCheckResult.EligibleForAutoRenew {
		semCh.Send(ctx, struct{}{})

		f := workflow.ExecuteActivity(ctx, activities.AutoRenewDomain, workflowID, name)
		futures = append(futures, futInfo{
			domainName: name,
			operation:  "auto-renew",
			future:     f,
		})

		workflow.Go(ctx, func(ctx workflow.Context) {
			_ = f.Get(ctx, nil)
			var token struct{}
			semCh.Receive(ctx, &token)
		})
	}

	// Trigger Expiries
	for _, name := range batchCheckResult.EligibleForExpiry {
		semCh.Send(ctx, struct{}{})

		f := workflow.ExecuteActivity(ctx, activities.ExpireDomain, workflowID, name)
		futures = append(futures, futInfo{
			domainName: name,
			operation:  "expire",
			future:     f,
		})

		workflow.Go(ctx, func(ctx workflow.Context) {
			_ = f.Get(ctx, nil)
			var token struct{}
			semCh.Receive(ctx, &token)
		})
	}

	// Gather results
	for _, fut := range futures {
		err := fut.future.Get(ctx, nil)
		result.TotalProcessed++
		if err != nil {
			result.addFailure(fut.domainName, fut.operation, err.Error())
		} else {
			if fut.operation == "auto-renew" {
				result.AutoRenewed++
			} else {
				result.Expired++
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

	// Check if we hit the batch cap and should continue-as-new to drain the remainder
	if int64(len(domains)) < domainCount.Count {
		result.Notes = append(result.Notes, "Batch cap reached: listed "+
			itoa(len(domains))+" of "+itoa64(domainCount.Count)+
			" expired domains. Continuing processing in a new run.")
		return result, workflow.NewContinueAsNewError(ctx, ExpiryLoop, params)
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
