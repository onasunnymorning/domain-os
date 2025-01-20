package workflows

import (
	"time"

	"github.com/onasunnymorning/domain-os/internal/application/activities"
	"github.com/onasunnymorning/domain-os/internal/application/commands"
	"github.com/onasunnymorning/domain-os/internal/domain/entities"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
	"go.uber.org/zap"
)

func RestoreWorkflow(ctx workflow.Context) error {
	// SETUP
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

	// WORKFLOW

	// Get the list of domains that are expiring
	domainList := []entities.Domain{}
	listErr := workflow.ExecuteActivity(ctx, activities.ListRestoredDomains, workflowID).Get(ctx, &domainList)
	if listErr != nil {
		return listErr
	}

	logger.Info(
		"Found restored domains",
		zap.Int("domain_count", len(domainList)),
		zap.String("workflow_id", workflowID),
	)

	for _, domain := range domainList {
		// Create the renew command
		cmd := commands.RenewDomainCommand{
			Name:  domain.Name.String(),
			ClID:  domain.ClID.String(),
			Years: 1,
		}
		// Renew the domain one year
		restoreErr := workflow.ExecuteActivity(ctx, activities.RenewDomain, workflowID, cmd).Get(ctx, nil)
		if restoreErr != nil {
			return restoreErr
		}

		// Unset PendingRestore

		// Get the domain
		domain := entities.Domain{}
		getDomainErr := workflow.ExecuteActivity(ctx, activities.GetDomain, workflowID, domain.Name.String()).Get(ctx, &domain)
		if getDomainErr != nil {
			return getDomainErr
		}

		// Unset the PendingRestore status
		domain.UnSetStatus(entities.DomainStatusPendingRestore)

		// Update the domain
		updateDomainErr := workflow.ExecuteActivity(ctx, activities.UpdateDomain, workflowID, domain).Get(ctx, nil)
		if updateDomainErr != nil {
			return updateDomainErr
		}

	}

	return nil
}
