package workflows

import (
	"fmt"
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"

	"github.com/onasunnymorning/domain-os/pkg/domain/queries"
)

// RefreshTMCHParams is the input of the RefreshTMCH workflow. It has no
// settings today; the schedule starts it with zero values.
type RefreshTMCHParams struct{}

// RefreshTMCHResult is the structured output of the RefreshTMCH workflow.
type RefreshTMCHResult struct {
	StartedAt   time.Time                `json:"startedAt"`
	CompletedAt time.Time                `json:"completedAt"`
	Refreshed   int                      `json:"refreshed"`
	Failed      int                      `json:"failed"`
	Sets        []queries.TMCHSetRefresh `json:"sets"`
	Notes       []string                 `json:"notes"`
}

// RefreshTMCH fetches the Trademark Clearinghouse's three data sets (the CA
// certificate bundle, the SMD revocation list and the DNL list; ADR 0024
// §4, roadmap 9a) and stores each one the Clearinghouse has regenerated,
// through one activity. A set that fails keeps what was stored; the run
// fails, and is retried, when any set failed.
func RefreshTMCH(ctx workflow.Context, params RefreshTMCHParams) (RefreshTMCHResult, error) {
	logger := workflow.GetLogger(ctx)
	result := RefreshTMCHResult{StartedAt: workflow.Now(ctx), Sets: []queries.TMCHSetRefresh{}}

	if err := workflow.SetQueryHandler(ctx, "progress", func() (RefreshTMCHResult, error) { return result, nil }); err != nil {
		result.CompletedAt = workflow.Now(ctx)
		result.Notes = append(result.Notes, "Failed to register query handler: "+err.Error())
		return result, err
	}

	ctx = workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		// Three downloads of up to five minutes each, then the DNL replace;
		// the activity heartbeats every 30 seconds while it runs.
		StartToCloseTimeout: 30 * time.Minute,
		HeartbeatTimeout:    5 * time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:    time.Minute,
			BackoffCoefficient: 2.0,
			MaximumInterval:    30 * time.Minute,
			MaximumAttempts:    4,
		},
	})

	var refreshed queries.TMCHRefreshResult
	if err := workflow.ExecuteActivity(ctx, "RefreshTMCHData", getWorkflowID(ctx)).Get(ctx, &refreshed); err != nil {
		logger.Error("TMCH refresh failed", "error", err)
		result.CompletedAt = workflow.Now(ctx)
		result.Notes = append(result.Notes, "TMCH refresh failed: "+err.Error())
		return result, err
	}

	result.Sets = refreshed.Sets
	result.Refreshed = refreshed.Refreshed()
	result.Failed = refreshed.Failed()
	result.CompletedAt = workflow.Now(ctx)
	if len(refreshed.Sets) == 0 {
		result.Notes = append(result.Notes, "No TMCH feed is configured for this installation; nothing to refresh")
		return result, nil
	}
	result.Notes = append(result.Notes, fmt.Sprintf("%d of %d sets refreshed", result.Refreshed, len(result.Sets)))
	return result, nil
}
