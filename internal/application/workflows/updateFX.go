package workflows

import (
	"fmt"
	"time"

	"github.com/onasunnymorning/domain-os/internal/application/activities"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

// UpdateFXParams defines the input parameters for the UpdateFX workflow.
// The workflow tolerates being started without arguments (zero values apply),
// which keeps existing schedules and manual triggers compatible.
type UpdateFXParams struct {
	// BaseCurrencies overrides the base currencies to update. When empty, the
	// list is derived from the distinct base currencies configured on phases
	// (falling back to USD) — exactly the set quoting needs.
	BaseCurrencies []string `json:"baseCurrencies,omitempty"`
}

// UpdateFXResult is the structured output of the UpdateFX workflow.
type UpdateFXResult struct {
	StartedAt         time.Time                  `json:"startedAt"`
	CompletedAt       time.Time                  `json:"completedAt"`
	BasesUpdated      []string                   `json:"basesUpdated"`
	RatesStored       int                        `json:"ratesStored"`
	Failed            int                        `json:"failed"`
	DerivedFromPhases bool                       `json:"derivedFromPhases"`
	Notes             []string                   `json:"notes"`
	Failures          []activities.FXBaseFailure `json:"failures,omitempty"`
}

// UpdateFX refreshes exchange rates from the Frankfurter API
// (https://frankfurter.dev/) for every base currency quoting needs, replacing
// each base's rates atomically in the database via a single direct-DB
// activity. A base that fails leaves its previous rates untouched; the run
// fails only when no base could be updated.
func UpdateFX(ctx workflow.Context, params UpdateFXParams) (UpdateFXResult, error) {
	started := workflow.Now(ctx)
	result := UpdateFXResult{
		StartedAt:    started,
		BasesUpdated: []string{},
	}

	// Register a query handler so progress is visible in the UI
	err := workflow.SetQueryHandler(ctx, "progress", func() (UpdateFXResult, error) {
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
		StartToCloseTimeout: 10 * time.Minute,
		HeartbeatTimeout:    2 * time.Minute,
		RetryPolicy:         retrypolicy,
	}
	ctx = workflow.WithActivityOptions(ctx, options)

	var activityResult activities.UpdateFXRatesResult
	updateErr := workflow.ExecuteActivity(ctx, "UpdateFXRates", workflowID, params.BaseCurrencies).Get(ctx, &activityResult)
	if updateErr != nil {
		result.CompletedAt = workflow.Now(ctx)
		result.Notes = append(result.Notes, "FX rate update failed: "+updateErr.Error())
		return result, updateErr
	}

	result.BasesUpdated = activityResult.BasesUpdated
	result.RatesStored = activityResult.RatesStored
	result.Failed = len(activityResult.Failures)
	result.Failures = activityResult.Failures
	result.DerivedFromPhases = activityResult.DerivedFromPhases
	result.CompletedAt = workflow.Now(ctx)

	result.Notes = append(result.Notes, fmt.Sprintf("Stored %d rates across %d base currencies",
		result.RatesStored, len(result.BasesUpdated)))
	if result.Failed > 0 {
		result.Notes = append(result.Notes, fmt.Sprintf("%d base currencies failed — their previous rates remain in place; review the failures list", result.Failed))
	}

	return result, nil
}
