package workflows

// This workflow implements the processes that are required to keep registrars in-sync with IANA and ICANN.
// Drawing: https://miro.com/app/board/uXjVMwEdn4Y=/?moveToWidget=3458764614806207912&cot=14
// Docs: https://www.notion.so/apex-domains/Registrar-management-1886c0599d5380249221e9d5e7a12b7f?pvs=4

import (
	"fmt"
	"os"
	"time"

	"github.com/onasunnymorning/domain-os/internal/application/activities"
	"github.com/onasunnymorning/domain-os/internal/application/commands"
	"github.com/onasunnymorning/domain-os/internal/domain/entities"
	"github.com/onasunnymorning/domain-os/internal/infrastructure/web/icannregistrars"
	"github.com/onasunnymorning/domain-os/internal/interface/rest/response"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
	"go.uber.org/zap"
)

func SyncRegistrarsWorkflow(ctx workflow.Context, batchsize int) error {
	// SETUP
	// Set up our logger
	logger, _ := zap.NewProduction()
	defer logger.Sync()
	// Set envars
	apiHost := os.Getenv("API_HOST")
	apiPort := os.Getenv("API_PORT")
	bearerToken := "Bearer " + os.Getenv("ADMIN_TOKEN")
	baseURL := fmt.Sprintf("http://%s:%s", apiHost, apiPort)
	logger.Debug(fmt.Sprintf("baseURL: %s", baseURL))

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

	// If it is our first time syncing, launch the first import of registrars
	if rarCount.Count == 0 {
		// Get the ICANN registrars via activity (to keep workflow deterministic)
		var csvRars []icannregistrars.CSVRegistrar
		getIcannErr := workflow.ExecuteActivity(ctx, activities.GetICANNRegistrars, workflowID, "./initdata/icannRegistrarList.csv").Get(ctx, &csvRars)
		if getIcannErr != nil {
			logger.Error(fmt.Sprintf("failed to get ICANN registrars from file via activity: %v", getIcannErr))
		}
		// Get the IANA registrars
		var ianaRars []entities.IANARegistrar
		ianaRarErr := workflow.ExecuteActivity(ctx, activities.GetIANARegistrars, workflowID, baseURL, bearerToken, batchsize).Get(ctx, &ianaRars)
		if ianaRarErr != nil {
			logger.Error(fmt.Sprintf("failed to get IANA registrars: %v", ianaRarErr))
		}
		// Merge both into a create command
		cmds := []commands.CreateRegistrarCommand{}
		createCmdErr := workflow.ExecuteActivity(ctx, activities.MakeCreateRegistrarCommands, workflowID, csvRars, ianaRars).Get(ctx, &cmds)
		if createCmdErr != nil {
			logger.Error(fmt.Sprintf("failed to get create commands: %v", createCmdErr))
		}
		// Create the registrars
		createdRarCounter := 0
		// Process the commands in chunks
		for chunk := range commands.ChunkCreateRegistrarCommands(cmds, 100) {
			if err := activities.BulkCreateRegistrars(workflowID, chunk); err != nil {
				return err
			}
			createdRarCounter += len(chunk)
		}

		logger.Info(fmt.Sprintf("created %d registrars", createdRarCounter))

		// TODO: launch as new the same workflow so the sync happens after the init
		return nil
	}

	// Update the registrars that have changed

	// First get the IANA registrars
	var ianaRars []entities.IANARegistrar
	ianaRarErr := workflow.ExecuteActivity(ctx, activities.GetIANARegistrars, workflowID, baseURL, bearerToken, batchsize).Get(ctx, &ianaRars)
	if ianaRarErr != nil {
		logger.Error(fmt.Sprintf("failed to get IANA registrars: %v", ianaRarErr))
		return ianaRarErr
	}

	// Get our existing registrars
	var rars []entities.RegistrarListItem
	rarsErr := workflow.ExecuteActivity(ctx, activities.GetRegistrarListItems, workflowID, baseURL, bearerToken, batchsize).Get(ctx, &rars)
	if rarsErr != nil {
		logger.Error(fmt.Sprintf("failed to get registrar list items: %v", rarsErr))
		return rarsErr
	}

	// Compute plan via activity
	var plan activities.DiffPlanResult
	planErr := workflow.ExecuteActivity(ctx, activities.DiffAndPlanRegistrars, workflowID, ianaRars, rars).Get(ctx, &plan)
	if planErr != nil {
		logger.Error(fmt.Sprintf("failed to diff and plan registrars: %v", planErr))
		return planErr
	}

	// Apply creates in chunks
	createdRarCounter := 0
	for chunk := range commands.ChunkCreateRegistrarCommands(plan.Creates, 100) {
		if err := activities.BulkCreateRegistrars(workflowID, chunk); err != nil {
			return err
		}
		createdRarCounter += len(chunk)
	}
	if createdRarCounter > 0 {
		logger.Info(fmt.Sprintf("created %d registrars", createdRarCounter))
	}

	// Apply updates
	for _, upd := range plan.Updates {
		if err := workflow.ExecuteActivity(ctx, activities.SetRegistrarStatus, workflowID, upd.ClID, upd.NewStatus).Get(ctx, nil); err != nil {
			logger.Error(fmt.Sprintf("failed to set registrar status for %s: %v", upd.ClID, err))
		}
	}

	return nil
}
