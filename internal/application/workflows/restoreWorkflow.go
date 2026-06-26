package workflows

import (
	"fmt"
	"time"

	"github.com/onasunnymorning/domain-os/internal/application/activities"
	"github.com/onasunnymorning/domain-os/internal/application/commands"
	"github.com/onasunnymorning/domain-os/internal/interface/rest/response"
	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

func RestoreWorkflow(ctx workflow.Context) error {
	// SETUP
	logger := workflow.GetLogger(ctx)

	// Get the workflow ID
	workflowID := getWorkflowID(ctx)
	logger.Debug("Starting restore workflow", "workflow_id", workflowID)

	// RetryPolicy specifies how to automatically handle retries if an Activity fails.
	retrypolicy := &temporal.RetryPolicy{
		InitialInterval:        time.Second,
		BackoffCoefficient:     2.0,
		MaximumInterval:        10 * time.Minute,
		MaximumAttempts:        3, // 0 is unlimited retries
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

	logger.Info(fmt.Sprintf("Found %d PendingRestore domains", len(domainList)), "domain_count", len(domainList), "workflow_id", workflowID)

	// Anything that happens in this loop should log an error, but not break the loop so that individual domains can fail without stopping the workflow
	// Make sure logs are surfaced to be handled and fixed
	for _, domain := range domainList {
		logger.Debug("Processing domain", "domain", domain.Name)
		// Create the renew command
		cmd := commands.RenewDomainCommand{
			Name:  domain.Name,
			ClID:  domain.ClID,
			Years: 1,
		}
		logger.Debug("Renew command created", "domain", cmd.Name, "clid", cmd.ClID)

		// Unset the PendingRestore status
		unsetStatusCommand := commands.ToggleDomainStatusCommand{
			DomainName:    cmd.Name,
			Status:        entities.DomainStatusPendingRestore,
			CorrelationID: workflowID,
		}
		unSetStatusErr := workflow.ExecuteActivity(ctx, activities.UnSetDomainStatus, unsetStatusCommand).Get(ctx, nil)
		if unSetStatusErr != nil {
			logger.Warn("failed to unset PendingRestore status", "domain_name", cmd.Name, "workflow_id", workflowID, "error", unSetStatusErr)
		}

		// Force-Renew the domain
		forceRenewErr := workflow.ExecuteActivity(ctx, activities.RenewDomain, workflowID, cmd, true).Get(ctx, nil)
		if forceRenewErr != nil {
			logger.Error("failed to force renew", "domain_name", cmd.Name, "workflow_id", workflowID, "error", forceRenewErr)

			// if the renew fails, set the domain status to PendingRestore again so we can try again later
			setStatusErr := workflow.ExecuteActivity(ctx, activities.SetDomainStatus, unsetStatusCommand).Get(ctx, nil)
			if setStatusErr != nil {
				logger.Error("failed to re-set PendingRestore status", "domain_name", cmd.Name, "workflow_id", workflowID, "error", setStatusErr)
			}

		}

	}

	return nil
}
