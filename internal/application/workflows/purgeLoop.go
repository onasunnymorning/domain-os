package workflows

import (
	"time"

	"github.com/onasunnymorning/domain-os/internal/application/activities"
	"github.com/onasunnymorning/domain-os/internal/interface/rest/response"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
	"go.uber.org/zap"
)

func PurgeLoop(ctx workflow.Context) error {
	// set up our logger
	logger, _ := zap.NewProduction()
	defer logger.Sync()

	// Get the workflow ID
	workflowID := getWorkflowID(ctx)

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

	// Check if there are any domains that are purgeable
	domainCount := &response.CountResult{}
	purgeableDomainCountErr := workflow.ExecuteActivity(ctx, activities.GetPurgeableDomainCount, workflowID).Get(ctx, domainCount)
	if purgeableDomainCountErr != nil {
		logger.Error(
			"Error getting purgeable domain count",
			zap.Error(purgeableDomainCountErr),
		)
		return purgeableDomainCountErr
	}

	// If there are no domains to purge, exit
	if domainCount.Count == 0 {
		return nil
	}

	// Get the list of domains that are purgeable
	domains := []response.DomainExpiryItem{}
	purgeableDomainsError := workflow.ExecuteActivity(ctx, activities.ListPurgeableDomains, workflowID).Get(ctx, &domains)
	if purgeableDomainsError != nil {
		logger.Error(
			"Error getting purgeable domains",
			zap.Error(purgeableDomainsError),
		)
		return purgeableDomainsError
	}

	// Process the list of purgeable domains
	for _, domain := range domains {
		// Purge the domain
		purgeActivityErr := workflow.ExecuteActivity(ctx, activities.PurgeDomain, workflowID, domain.Name).Get(ctx, nil)
		if purgeActivityErr != nil {
			logger.Error(
				"Error purging domain",
				zap.String("domain_name", domain.Name),
				zap.Error(purgeActivityErr),
				zap.Any("domain", domain),
			)
		}
	}

	return nil

}
