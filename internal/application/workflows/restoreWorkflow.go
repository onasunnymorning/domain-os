package workflows

import (
	"fmt"
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

	// Get the list of domains that are PendingRestore
	domainList := []response.DomainRestoredItem{}
	listErr := workflow.ExecuteActivity(ctx, activities.ListRestoredDomains, workflowID).Get(ctx, &domainList)
	if listErr != nil {
		return listErr
	}

	logger.Info(
		fmt.Sprintf("Found %d PendingRestore domains", len(domainList)),
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

		// Unset the PendingRestore status
		unsetStatusCommand := commands.ToggleDomainStatusCommand{
			DomainName:    cmd.Name,
			Status:        entities.DomainStatusPendingRestore,
			CorrelationID: workflowID,
		}
		unSetStatusErr := workflow.ExecuteActivity(ctx, activities.UnSetDomainStatus, unsetStatusCommand).Get(ctx, nil)
		if unSetStatusErr != nil {
			logger.Warn(
				"failed to unset PendingRestore status",
				zap.String("domain_name", cmd.Name),
				zap.String("workflow_id", workflowID),
				zap.Error(unSetStatusErr),
			)
		}

		// Force-Renew the domain
		forceRenewErr := workflow.ExecuteActivity(ctx, activities.RenewDomain, workflowID, cmd, true).Get(ctx, nil)
		if forceRenewErr != nil {
			logger.Error(
				"failed to force a renew as part of the restore process",
				zap.String("domain_name", cmd.Name),
				zap.String("workflow_id", workflowID),
				zap.Error(forceRenewErr),
			)

			// if the renew fails, set the domain status to PendingRestore again so we can try again later
			setStatusErr := workflow.ExecuteActivity(ctx, activities.SetDomainStatus, unsetStatusCommand).Get(ctx, nil)
			if setStatusErr != nil {
				logger.Error(
					"failed to re-set PendingRestore status after failed renew as part of the restore process",
					zap.String("domain_name", cmd.Name),
					zap.String("workflow_id", workflowID),
					zap.Error(setStatusErr),
				)
			}

		}

	}

	return nil
}
