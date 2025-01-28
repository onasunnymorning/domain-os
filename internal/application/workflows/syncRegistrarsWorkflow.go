package workflows

// This workflow implements the processes that are required to keep registrars in-sync with IANA and ICANN.
// Drawing: https://miro.com/app/board/uXjVMwEdn4Y=/?moveToWidget=3458764614806207912&cot=14
// Docs: https://www.notion.so/apex-domains/Registrar-management-1886c0599d5380249221e9d5e7a12b7f?pvs=4

import (
	"fmt"
	"time"

	"github.com/onasunnymorning/domain-os/internal/application/activities"
	"github.com/onasunnymorning/domain-os/internal/interface/rest/response"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
	"go.uber.org/zap"
)

func SyncRegistrarsWorkflow(ctx workflow.Context) error {
	// SETUP
	// Set up our logger
	logger, _ := zap.NewDevelopment()
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

	// WORKFLOW

	// Sync registrars with IANA
	syncErr := workflow.ExecuteActivity(ctx, activities.SyncIanaRegistrars, workflowID).Get(ctx, nil)
	if syncErr != nil {
		logger.Error(fmt.Sprintf("failed to sync registrars with IANA: %v", syncErr))
		return syncErr
	}

	// Check if this is our first time syncing registrars
	var rarCount *response.CountResult
	countErr := workflow.ExecuteActivity(ctx, activities.CountRegistrars, workflowID).Get(ctx, rarCount)
	if countErr != nil {
		logger.Error(fmt.Sprintf("failed to count registrars: %v", countErr))
		return countErr
	}

	// If it is our first time syncing, launch an import of all registrars
	if rarCount.Count == 0 {
		// Launch an Import Registrars workflow
		//exit
	}

	// Update the registrars that have changed

	return nil
}
