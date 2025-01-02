package workflows

import (
	"time"

	"github.com/onasunnymorning/domain-os/internal/application/activities"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
	"go.uber.org/zap"
)

func UpdateFX(ctx workflow.Context) error {
	// set up our logger
	logger, _ := zap.NewProduction()
	defer logger.Sync()

	// RetryPolicy specifies how to automatically handle retries if an Activity fails.
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
		// Optionally provide a customized RetryPolicy.
		// Temporal retries failed Activities by default.
		RetryPolicy: retrypolicy,
	}

	// Apply the options.
	ctx = workflow.WithActivityOptions(ctx, options)

	// Update USD
	currencies := []string{"USD", "EUR", "PEN", "GBP", "RUB", "CAD", "AUD"}
	for _, currency := range currencies {
		updateErr := workflow.ExecuteActivity(ctx, activities.UpdateFX, currency).Get(ctx, nil)
		if updateErr != nil {
			logger.Error(
				"Error updating FX",
				zap.String("currency", currency),
				zap.Error(updateErr),
			)
		}
	}

	return nil

}
