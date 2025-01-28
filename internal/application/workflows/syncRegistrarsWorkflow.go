package workflows

// This workflow implements the processes that are required to keep registrars in-sync with IANA and ICANN.
// Drawing: https://miro.com/app/board/uXjVMwEdn4Y=/?moveToWidget=3458764614806207912&cot=14
// Docs: https://www.notion.so/apex-domains/Registrar-management-1886c0599d5380249221e9d5e7a12b7f?pvs=4

import (
	"fmt"
	"time"

	"github.com/onasunnymorning/domain-os/internal/application/activities"
	"github.com/onasunnymorning/domain-os/internal/domain/entities"
	"github.com/onasunnymorning/domain-os/internal/infrastructure/web/icannregistrars"
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

	// Sync registrars with IANA to ensure that we have up to date information
	syncErr := workflow.ExecuteActivity(ctx, activities.SyncIanaRegistrars, workflowID).Get(ctx, nil)
	if syncErr != nil {
		logger.Error(fmt.Sprintf("failed to sync registrars with IANA: %v", syncErr))
		return syncErr
	}

	// Check if this is our first time syncing registrars (zero registrars in the system)
	var rarCount response.CountResult
	countErr := workflow.ExecuteActivity(ctx, activities.CountRegistrars, workflowID).Get(ctx, &rarCount)
	if countErr != nil {
		logger.Error(fmt.Sprintf("failed to count registrars: %v", countErr))
		return countErr
	}

	// If it is our first time syncing, launch an first import of registrars
	if rarCount.Count == 0 {
		// Get the ICANN registrars
		csvRars, err := icannregistrars.GetICANNCSVRegistrarsFromFile("./initdata/icann_registrars.csv")
		if err != nil {
			logger.Error(fmt.Sprintf("failed to get ICANN registrars from file: %v", err))
		}
		// Get the IANA registrars
		var ianaRars []entities.IANARegistrar
		ianaRarErr := workflow.ExecuteActivity(ctx, activities.GetIANARegistrars, workflowID).Get(ctx, ianaRars)
		if ianaRarErr != nil {
			logger.Error(fmt.Sprintf("failed to get IANA registrars: %v", ianaRarErr))
		}
		// Merge both into a create command
		cmds, createCmdErr := icannregistrars.GetCreateCommands(csvRars, ianaRars)
		if createCmdErr != nil {
			logger.Error(fmt.Sprintf("failed to get create commands: %v", createCmdErr))
		}
		// Create the registrars
		createdRarCounter := 0
		for _, cmd := range cmds {
			createErr := workflow.ExecuteActivity(ctx, activities.CreateRegistrar, workflowID, cmd).Get(ctx, nil)
			if createErr != nil {
				logger.Error(fmt.Sprintf("failed to create registrar: %v", createErr))
			}
			createdRarCounter++
		}

		logger.Info(fmt.Sprintf("created %d registrars", createdRarCounter))

		// nothing further to do
		return nil
	}

	// Update the registrars that have changed

	// First get the IANA registrars
	var ianaRars []entities.IANARegistrar
	ianaRarErr := workflow.ExecuteActivity(ctx, activities.GetIANARegistrars, workflowID).Get(ctx, ianaRars)
	if ianaRarErr != nil {
		logger.Error(fmt.Sprintf("failed to get IANA registrars: %v", ianaRarErr))
		return ianaRarErr
	}

	// For each registrar set the status using the API
	for _, irar := range ianaRars {
		// Use activity to set the status (this is idempotent and will log the change if there is one)
		clid, err := irar.CreateClID()
		if err != nil {
			logger.Error(fmt.Sprintf("failed to create ClID for registrar %d - %s: %v", irar.GurID, irar.Name, err))
		}
		setStatusErr := workflow.ExecuteActivity(ctx, activities.SetRegistrarStatus, workflowID, clid, irar.Status).Get(ctx, nil)
		if setStatusErr != nil {
			logger.Error(fmt.Sprintf("failed to set registrar status: %v", setStatusErr))
			return setStatusErr
		}
	}

	return nil
}
