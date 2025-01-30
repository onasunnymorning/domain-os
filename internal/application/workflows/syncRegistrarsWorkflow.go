package workflows

// This workflow implements the processes that are required to keep registrars in-sync with IANA and ICANN.
// Drawing: https://miro.com/app/board/uXjVMwEdn4Y=/?moveToWidget=3458764614806207912&cot=14
// Docs: https://www.notion.so/apex-domains/Registrar-management-1886c0599d5380249221e9d5e7a12b7f?pvs=4

import (
	"fmt"
	"log"
	"os"
	"strings"
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
	bearerToken := "Bearer " + os.Getenv("API_TOKEN")
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
		// Get the ICANN registrars
		csvRars, err := icannregistrars.GetICANNCSVRegistrarsFromFile("./initdata/icann_registrars.csv")
		if err != nil {
			logger.Error(fmt.Sprintf("failed to get ICANN registrars from file: %v", err))
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

	// Compare the two lists and update the platform as necessary
	for _, ianaRar := range ianaRars {
		// Create a ClID for the IANA registrar using our naming convention
		clid, _ := ianaRar.CreateClID()
		found := false
		for _, rar := range rars {
			if clid == rar.ClID {
				// Found the registrar
				found = true
				// compare statuses
				cmd := commands.CompareIANARegistrarStatusWithRarStatus(ianaRar, rar)
				if cmd != nil {
					// update the registrar status
					err := workflow.ExecuteActivity(ctx, activities.SetRegistrarStatus, workflowID, cmd.ClID, cmd.NewStatus).Get(ctx, nil)
					if err != nil {
						logger.Error(fmt.Sprintf("failed to set registrar status: %v", err))
					}

				}
				// Only one match is expected
				break
			}
		}

		if !found {

			// Do not create reserved registrars, except the ones Reserved for Pre-Delegation Testing transactions (id's 9995 and 9996) Ref: https://www.iana.org/assignments/registrar-ids/registrar-ids.xhtml
			if (strings.EqualFold(ianaRar.Status.String(), string(entities.IANARegistrarStatusReserved))) && !(ianaRar.GurID == 9995 || ianaRar.GurID == 9996) {
				log.Printf("found new IANARegistrar: %s, but it is reserved, skipping\n", clid)
				continue
			}

			log.Printf("found new IANARegistrar: %s, creating it\n", clid)

			// Create our Create command
			cmd, err := commands.CreateCreateRegistrarCommandFromIANARegistrar(ianaRar)
			if err != nil {
				return err
			}

			// create the registrar
			createdRar := entities.Registrar{}
			createErr := workflow.ExecuteActivity(ctx, activities.CreateRegistrar, workflowID, *cmd).Get(ctx, &createdRar)
			if createErr != nil {
				return createErr
			}

		}

	}

	return nil
}
