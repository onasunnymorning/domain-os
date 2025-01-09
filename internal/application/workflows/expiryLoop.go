package workflows

import (
	"time"

	"github.com/onasunnymorning/domain-os/internal/application/activities"
	"github.com/onasunnymorning/domain-os/internal/interface/rest/response"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"

	"go.uber.org/zap"
)

// ExpiryLoop ref: https://www.notion.so/apex-domains/Domain-lifecycle-18200bd9d73849e6abfe2e616f1a3443?pvs=4#2e597291f85a43699422a7ac5f122bc8
func ExpiryLoop(ctx workflow.Context) error {
	// Set up our logger
	logger, _ := zap.NewProduction()
	defer logger.Sync()

	// Get the workflow ID
	workflowID := getWorkflowID(ctx)
	logger.Debug("Starting expiry loop", zap.String("workflow_id", workflowID))

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

	// See if there are any domains that are expiring
	domainCount := &response.CountResult{}
	GetExpiredDomainCountError := workflow.ExecuteActivity(ctx, activities.GetExpiredDomainCount, workflowID).Get(ctx, domainCount)
	if GetExpiredDomainCountError != nil {
		return GetExpiredDomainCountError
	}
	logger.Info(
		"Found expired domains",
		zap.Int64("domain_count", domainCount.Count),
		zap.String("workflow_id", workflowID),
	)
	// If there are no domains to expire, sleep for 5 mins and check again
	if domainCount.Count == 0 {
		return nil
	}

	// Get the list of domains that are expiring
	domains := []response.DomainExpiryItem{}
	GetExpiredDomainsError := workflow.ExecuteActivity(ctx, activities.ListExpiringDomains, workflowID).Get(ctx, &domains)
	if GetExpiredDomainsError != nil {
		return GetExpiredDomainsError
	}

	// For each domain that is expiring, either renew or delete
	for _, domain := range domains {
		// Check if the domain is eligible for auto-renew
		var canautorenew bool
		canAutoRenewErr := workflow.ExecuteActivity(ctx, activities.CheckDomainCanAutoRenew, workflowID, domain.Name).Get(ctx, &canautorenew)
		if canAutoRenewErr != nil {
			logger.Error(
				"Failed to check if domain is eligible for auto-renew",
				zap.String("domain_name", domain.Name),
				zap.Error(canAutoRenewErr),
				zap.Any("domain", domain),
				zap.String("workflow_id", workflowID),
			)
			continue
		}
		if canautorenew {
			// Try and auto-renew the domain
			autoRenewErr := workflow.ExecuteActivity(ctx, activities.AutoRenewDomain, workflowID, domain.Name).Get(ctx, nil)
			if autoRenewErr != nil {
				logger.Error(
					"Failed to auto-renew domain",
					zap.String("domain_name", domain.Name),
					zap.Error(autoRenewErr),
					zap.Any("domain", domain),
					zap.String("workflow_id", workflowID),
				)
				continue
			}
		} else {
			// If the domain is not eligible for auto-renew, it should expire
			expireErr := workflow.ExecuteActivity(ctx, activities.ExpireDomain, workflowID, domain.Name).Get(ctx, nil)
			if expireErr != nil {
				logger.Error(
					"Failed to expire domain",
					zap.String("domain_name", domain.Name),
					zap.Error(expireErr),
					zap.Any("domain", domain),
					zap.String("workflow_id", workflowID),
				)
				continue
			}
			continue
		}
	}

	return nil
}
