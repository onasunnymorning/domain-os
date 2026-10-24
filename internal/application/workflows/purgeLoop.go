package workflows

import (
	"strconv"
	"time"

	"github.com/onasunnymorning/domain-os/internal/application/activities"
	"github.com/onasunnymorning/domain-os/internal/application/services"
	"github.com/onasunnymorning/domain-os/internal/interface/rest/response"
	"github.com/onasunnymorning/domain-os/pkg/domain/queries"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

const (
	maxPurgeFailureSamples = 50
	// defaultReleaseWindowMinutes is how long after a scheduled run starts
	// its purge may begin: the hourly schedule leaves 15 minutes for the
	// purge itself before the next run.
	defaultReleaseWindowMinutes = 45
)

// PurgeLoopParams defines the input parameters for the PurgeLoop workflow.
type PurgeLoopParams struct {
	// BatchSize caps how many domains are listed and processed per run
	// (default: 1000, the admin API maximum page size).
	BatchSize             int        `json:"batchSize,omitempty"`
	DryRun                bool       `json:"dryRun,omitempty"`
	ReferenceTimeOverride *time.Time `json:"referenceTimeOverride,omitempty"`
	// ContinuationCount tracks how many times this run has continued-as-new.
	// Managed by the workflow — leave zero when starting a run.
	ContinuationCount int `json:"continuationCount,omitempty"`
	// ReleaseWindowMinutes is the window the purge starts at a random
	// offset within (randomised release, docs/epp/ARCHITECTURE.md §5).
	// Unset is 45 minutes; 0 starts at once, for a manual run.
	ReleaseWindowMinutes *int `json:"releaseWindowMinutes,omitempty"`
}

// releaseWindow is the run's release window.
func (p PurgeLoopParams) releaseWindow() time.Duration {
	if p.ReleaseWindowMinutes == nil {
		return defaultReleaseWindowMinutes * time.Minute
	}
	return time.Duration(max(*p.ReleaseWindowMinutes, 0)) * time.Minute
}

// PurgeLoopResult is the structured output of the PurgeLoop workflow.
type PurgeLoopResult struct {
	StartedAt     time.Time `json:"startedAt"`
	CompletedAt   time.Time `json:"completedAt"`
	ReferenceTime time.Time `json:"referenceTime"`
	// ReleaseOffset is how long the run waited before purging.
	ReleaseOffset  time.Duration      `json:"releaseOffset"`
	TotalFound     int64              `json:"totalFound"`
	TotalProcessed int                `json:"totalProcessed"`
	Purged         int                `json:"purged"`
	Failed         int                `json:"failed"`
	Skipped        int                `json:"skipped"`
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

// PurgeLoop permanently deletes domains that are pendingDelete and whose purge
// date has passed.
//
// The release is randomised (anti-dropcatch): a scheduled run waits a
// random offset within its release window before it purges, and each
// batch is purged in a random order, so the moment a name is released
// cannot be known in advance. Both draws are activities (ReleaseOffset,
// ShuffleForRelease), keeping the workflow deterministic (INV-06).
// Continuations, which drain a large batch, do not wait again.
//
// The run locks a single reference time which both the count and list queries
// evaluate (serialized as the `before` cutoff), eliminating TOCTOU races. The
// batch purge activity is idempotent under retries: already-purged domains are
// reported as Skipped. Continue-as-new only fires when the run made progress
// and the continuation cap has not been reached, so a poison batch cannot spin
// the workflow in a hot loop.
func PurgeLoop(ctx workflow.Context, params PurgeLoopParams) (PurgeLoopResult, error) {
	started := workflow.Now(ctx)
	result := PurgeLoopResult{
		StartedAt: started,
	}

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

	// Lock a single reference time for the entire run.
	var referenceTime time.Time
	if params.ReferenceTimeOverride != nil {
		referenceTime = *params.ReferenceTimeOverride
	} else {
		referenceTime = workflow.Now(ctx).UTC()
	}
	result.ReferenceTime = referenceTime

	// Randomised release: wait a random offset within the window first.
	// What is purged is still what was purgeable at the reference time.
	if !params.DryRun && params.ContinuationCount == 0 {
		if window := params.releaseWindow(); window > 0 {
			var offset time.Duration
			if err := workflow.ExecuteActivity(ctx, activities.ReleaseOffset, window).Get(ctx, &offset); err != nil {
				result.Notes = append(result.Notes, "Release offset not drawn; purging at once: "+err.Error())
			} else {
				if err := workflow.Sleep(ctx, offset); err != nil {
					result.CompletedAt = workflow.Now(ctx)
					return result, err
				}
				// Only now: the progress query must not tell when the
				// purge will start.
				result.ReleaseOffset = offset
			}
		}
	}

	// Both count and list serialize this cutoff to the admin API, so they
	// evaluate the same instant.
	query := queries.PurgeableDomainsQuery{
		Before:   referenceTime,
		PageSize: params.BatchSize,
	}

	// Step 1: Count purgeable domains
	domainCount := &response.CountResult{}
	countErr := workflow.ExecuteActivity(ctx, activities.GetPurgeableDomainCount, workflowID, query).Get(ctx, domainCount)
	if countErr != nil {
		result.CompletedAt = workflow.Now(ctx)
		result.Notes = append(result.Notes, "Failed to count purgeable domains: "+countErr.Error())
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

	// Step 3: Batch purge, in a random order.
	domainNames := make([]string, len(domains))
	for i, d := range domains {
		domainNames[i] = d.Name
	}
	var shuffled []string
	if err := workflow.ExecuteActivity(ctx, activities.ShuffleForRelease, domainNames).Get(ctx, &shuffled); err != nil {
		result.Notes = append(result.Notes, "Release order not shuffled; purging in listed order: "+err.Error())
	} else {
		domainNames = shuffled
	}

	var purgeBatch services.BatchResult
	batchCtx := workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout: 30 * time.Minute,
		HeartbeatTimeout:    2 * time.Minute,
		RetryPolicy:         retrypolicy,
	})
	purgeErr := workflow.ExecuteActivity(batchCtx, "BatchPurgeDomains", workflowID, domainNames).Get(ctx, &purgeBatch)
	if purgeErr != nil {
		result.addFailure("batch-purge", purgeErr.Error())
	} else {
		result.Purged = len(purgeBatch.Succeeded)
		result.Skipped = len(purgeBatch.Skipped)
		result.TotalProcessed = len(purgeBatch.Succeeded) + len(purgeBatch.Skipped) + len(purgeBatch.Failed)
		for _, f := range purgeBatch.Failed {
			result.addFailure(f.DomainName, f.Error)
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

	// Check if we hit the batch cap and should continue-as-new to drain the remainder.
	if int64(len(domains)) < domainCount.Count {
		// Only continue when this run actually made progress — failed domains
		// stay in the query result set, so continuing without progress would
		// spin the workflow in a hot loop over the same poison batch.
		progressed := result.Purged+result.Skipped > 0
		if !progressed {
			result.Notes = append(result.Notes, "Batch cap reached but no progress was made this run — "+
				"not continuing to avoid a hot loop. Remaining domains will be retried on the next scheduled run.")
			return result, nil
		}
		if params.ContinuationCount >= maxContinuationRuns {
			result.Notes = append(result.Notes, "Continuation cap reached ("+strconv.Itoa(maxContinuationRuns)+
				" runs) — remaining domains will be picked up by the next scheduled run.")
			return result, nil
		}
		result.Notes = append(result.Notes, "Batch cap reached: listed "+
			strconv.Itoa(len(domains))+" of "+strconv.FormatInt(domainCount.Count, 10)+
			" purgeable domains. Continuing processing in a new run.")
		nextParams := params
		nextParams.ContinuationCount++
		return result, workflow.NewContinueAsNewError(ctx, PurgeLoop, nextParams)
	}

	return result, nil
}
