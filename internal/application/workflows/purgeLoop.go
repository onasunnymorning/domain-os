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
	maxPurgeFailureSamples = 50
)

// PurgeLoopParams defines the input parameters for the PurgeLoop workflow.
type PurgeLoopParams struct {
	BatchSize             int        `json:"batchSize,omitempty"`
	ConcurrencyLimit      int        `json:"concurrencyLimit,omitempty"`
	DryRun                bool       `json:"dryRun,omitempty"`
	ReferenceTimeOverride *time.Time `json:"referenceTimeOverride,omitempty"`
}

// PurgeLoopResult is the structured output of the PurgeLoop workflow.
type PurgeLoopResult struct {
	StartedAt      time.Time          `json:"startedAt"`
	CompletedAt    time.Time          `json:"completedAt"`
	TotalFound     int64              `json:"totalFound"`
	TotalProcessed int                `json:"totalProcessed"`
	Purged         int                `json:"purged"`
	Failed         int                `json:"failed"`
	Notes          []string           `json:"notes"`
	Failures       []PurgeLoopFailure `json:"failures,omitempty"`
}

// PurgeLoopFailure records a single domain purge failure.
type PurgeLoopFailure struct {
	DomainName string `json:"domainName"`
	Error      string `json:"error"`
}

// addFailure appends a failure record to the PurgeLoopResult.
func (r *PurgeLoopResult) addFailure(domainName, errMsg string) {
	r.Failed++
	if len(r.Failures) < maxPurgeFailureSamples {
		r.Failures = append(r.Failures, PurgeLoopFailure{
			DomainName: domainName,
			Error:      errMsg,
		})
	}
}

// PurgeLoop orchestrates domain purging operations.
func PurgeLoop(ctx workflow.Context, params PurgeLoopParams) (PurgeLoopResult, error) {
	started := workflow.Now(ctx)
	result := PurgeLoopResult{
		StartedAt: started,
	}

	logger := workflow.GetLogger(ctx)

	// Register a query handler so progress is visible in the UI
	err := workflow.SetQueryHandler(ctx, "progress", func() (PurgeLoopResult, error) {
		return result, nil
	})
	if err != nil {
		result.CompletedAt = workflow.Now(ctx)
		result.Notes = append(result.Notes, "Failed to register query handler: "+err.Error())
		return result, err
	}

	workflowID := getWorkflowID(ctx)

	concurrencyLimit := params.ConcurrencyLimit
	if concurrencyLimit <= 0 {
		concurrencyLimit = 20
	}

	retrypolicy := &temporal.RetryPolicy{
		InitialInterval:        time.Second,
		BackoffCoefficient:     2.0,
		MaximumInterval:        10 * time.Minute,
		MaximumAttempts:        3, // 0 is unlimited retries
		NonRetryableErrorTypes: []string{"none"},
	}

	options := workflow.ActivityOptions{
		// Timeout options specify when to automatically timeout Activity functions.
		StartToCloseTimeout: time.Minute,
		RetryPolicy:         retrypolicy,
	}

	// Apply the options.
	ctx = workflow.WithActivityOptions(ctx, options)

	// Step 1: Count purgeable domains
	var referenceTime time.Time
	if params.ReferenceTimeOverride != nil {
		referenceTime = *params.ReferenceTimeOverride
	} else {
		referenceTime = workflow.Now(ctx).UTC()
	}

	query := queries.PurgeableDomainsQuery{
		After: referenceTime,
	}

	domainCount := &response.CountResult{}
	countErr := workflow.ExecuteActivity(ctx, activities.GetPurgeableDomainCount, workflowID, query).Get(ctx, domainCount)
	if countErr != nil {
		result.CompletedAt = workflow.Now(ctx)
		result.Notes = append(result.Notes, "Failed to count purgeable domains: "+countErr.Error())
		logger.Error("Failed to count purgeable domains", "error", countErr)
		return result, countErr
	}
	result.TotalFound = domainCount.Count

	// If there are no domains to purge, return early
	if domainCount.Count == 0 {
		result.CompletedAt = workflow.Now(ctx)
		result.Notes = append(result.Notes, "No purgeable domains found")
		return result, nil
	}

	// Step 2: List the purgeable domains
	domains := []response.DomainExpiryItem{}
	listErr := workflow.ExecuteActivity(ctx, activities.ListPurgeableDomains, workflowID, query).Get(ctx, &domains)
	if listErr != nil {
		result.CompletedAt = workflow.Now(ctx)
		result.Notes = append(result.Notes, "Failed to list purgeable domains: "+listErr.Error())
		logger.Error("Failed to list purgeable domains", "error", listErr)
		return result, listErr
	}

	if len(domains) == 0 {
		result.CompletedAt = workflow.Now(ctx)
		result.Notes = append(result.Notes, "No purgeable domains found in list")
		return result, nil
	}

	// Dry run short circuit
	if params.DryRun {
		result.TotalProcessed = len(domains)
		result.Purged = len(domains)
		result.CompletedAt = workflow.Now(ctx)
		result.Notes = append(result.Notes, "Dry run completed: no state changes made")
		return result, nil
	}

	// Step 3: Purge domains in parallel with bounded concurrency
	semCh := workflow.NewBufferedChannel(ctx, concurrencyLimit)

	type futInfo struct {
		domainName string
		future     workflow.Future
	}
	var futures []futInfo

	for _, domain := range domains {
		semCh.Send(ctx, struct{}{})

		f := workflow.ExecuteActivity(ctx, activities.PurgeDomain, workflowID, domain.Name)
		futures = append(futures, futInfo{
			domainName: domain.Name,
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
			result.addFailure(fut.domainName, err.Error())
			logger.Error("Error purging domain", "domain", fut.domainName, "error", err)
		} else {
			result.Purged++
		}
	}

	result.CompletedAt = workflow.Now(ctx)

	// Add summary note
	if result.Failed > 0 {
		result.Notes = append(result.Notes, "Completed with failures — review the failures list for details")
		if result.Failed > maxPurgeFailureSamples {
			result.Notes = append(result.Notes, "Failure details capped at "+strconv.Itoa(maxPurgeFailureSamples)+
				" samples; total failures: "+strconv.Itoa(result.Failed))
		}
	}

	// Check if we hit the batch cap and should continue-as-new to drain the remainder
	if int64(len(domains)) < domainCount.Count {
		result.Notes = append(result.Notes, "Batch cap reached: listed "+
			strconv.Itoa(len(domains))+" of "+strconv.FormatInt(domainCount.Count, 10)+
			" purgeable domains. Continuing processing in a new run.")
		return result, workflow.NewContinueAsNewError(ctx, PurgeLoop, params)
	}

	return result, nil
}
