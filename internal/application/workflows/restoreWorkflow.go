package workflows

import (
	"time"

	"github.com/onasunnymorning/domain-os/internal/application/activities"
	"github.com/onasunnymorning/domain-os/internal/application/commands"
	"github.com/onasunnymorning/domain-os/internal/domain/entities"
	"github.com/onasunnymorning/domain-os/internal/interface/rest/response"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
	"go.uber.org/zap"
)

func RestoreWorkflow(ctx workflow.Context) error {
	// SETUP
	// Set up our logger
	logger, _ := zap.NewDevelopment()
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
	domainList := []response.DomainRestoredItem{}
	listErr := workflow.ExecuteActivity(ctx, activities.ListRestoredDomains, workflowID).Get(ctx, &domainList)
	if listErr != nil {
		return listErr
	}

	logger.Info(
		"Found restored domains",
		zap.Int("domain_count", len(domainList)),
		zap.String("workflow_id", workflowID),
	)

	logger.Debug(
		"domainList",
		zap.Any("DomainRestoredItems", domainList),
	)

	// Anything that happens in this loop should log an error, but not break the loop so that individual domains can fail without stopping the workflow
	// Make sure logs are surfaced to be handled and fixed
	for _, domain := range domainList {
		logger.Debug(
			"within loop, working on:",
			zap.Any("DomainRestoredItem", domain),
		)
		// Create the renew command
		cmd := commands.RenewDomainCommand{
			Name:  domain.Name,
			ClID:  domain.ClID,
			Years: 1,
		}
		logger.Debug(
			"renew command created",
			zap.Any("RenewDomainCommand", cmd),
		)

		// Get the domain
		domain := &entities.Domain{}
		getDomainErr := workflow.ExecuteActivity(ctx, activities.GetDomain, workflowID, cmd.Name).Get(ctx, domain)
		if getDomainErr != nil {
			logger.Error(
				"failed to get domain, continuing with next domain",
				zap.String("domain_name", domain.Name.String()),
				zap.String("workflow_id", workflowID),
				zap.Error(getDomainErr),
			)
		}

		// Unset the PendingRestore status
		setStatusErr := domain.UnSetStatus(entities.DomainStatusPendingRestore)
		if setStatusErr != nil {
			logger.Error(
				"failed to unset PendingRestore status",
				zap.String("domain_name", domain.Name.String()),
				zap.String("workflow_id", workflowID),
				zap.Error(setStatusErr),
				zap.Any("domain", domain),
			)
		}

		// Update the domain
		updateDomainErr := workflow.ExecuteActivity(ctx, activities.UpdateDomain, workflowID, domain).Get(ctx, nil)
		if updateDomainErr != nil {
			logger.Error(
				"failed to update domain",
				zap.String("domain_name", domain.Name.String()),
				zap.String("workflow_id", workflowID),
				zap.Error(updateDomainErr),
				zap.Any("domain", domain),
			)
		}

		//TODO: we might need to make some status adjustments as the domains can be clientRenewProhibited or serverRenewProhibited

		// Renew the domain one year
		renewErr := workflow.ExecuteActivity(ctx, activities.RenewDomain, workflowID, cmd).Get(ctx, nil)
		if renewErr != nil {
			logger.Error(
				"failed to renew domain, will attempt to re-set pendingRestore status",
				zap.String("domain_name", domain.Name.String()),
				zap.String("workflow_id", workflowID),
				zap.Error(renewErr),
				zap.Any("renew_command", cmd),
			)
			// if the renew fails, we re-set the PendingRestore status
			setErr := domain.SetStatus(entities.DomainStatusPendingRestore)
			if setErr != nil {
				logger.Error(
					"failed to set PendingRestore status after failed renew as part of restore",
					zap.String("domain_name", domain.Name.String()),
					zap.String("workflow_id", workflowID),
					zap.Error(setErr),
					zap.Any("domain", domain),
				)
			}
			updateErr := workflow.ExecuteActivity(ctx, activities.UpdateDomain, workflowID, domain).Get(ctx, nil)
			if updateErr != nil {
				logger.Error(
					"failed to update domain after failed renew as part of restore",
					zap.String("domain_name", domain.Name.String()),
					zap.String("workflow_id", workflowID),
					zap.Error(updateErr),
					zap.Any("domain", domain),
				)
			}
			logger.Info(
				"re-set PendingRestore status after failed renew as part of restore",
				zap.String("domain_name", domain.Name.String()),
				zap.Any("domain", domain),
			)
		}

	}

	return nil
}
