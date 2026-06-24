package workflows

import (
	"strconv"
	"time"

	"github.com/onasunnymorning/domain-os/internal/application/activities"
	"github.com/onasunnymorning/domain-os/internal/application/commands"
	"github.com/onasunnymorning/domain-os/internal/infrastructure/web/icannregistrars"
	"github.com/onasunnymorning/domain-os/internal/interface/rest/response"
	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
	"go.temporal.io/sdk/log"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

const (
	maxSyncFailureSamples = 50
	maxSyncDetailSamples  = 200
)

// SyncRegistrarsParams defines the input parameters for the SyncRegistrarsWorkflow.
type SyncRegistrarsParams struct {
	BatchSize        int  `json:"batchSize,omitempty"`
	ConcurrencyLimit int  `json:"concurrencyLimit,omitempty"`
	DryRun           bool `json:"dryRun,omitempty"`
}

// SyncRegistrarsResult is the structured output of the SyncRegistrarsWorkflow.
// It provides full checks-and-balances reporting so operators can verify the
// sync produced the expected outcome without digging through logs.
type SyncRegistrarsResult struct {
	StartedAt      time.Time               `json:"startedAt"`
	CompletedAt    time.Time               `json:"completedAt"`
	TotalIANA      int                     `json:"totalIana"`      // Total IANA registrars from source
	TotalExisting  int                     `json:"totalExisting"`  // Total platform registrars before sync
	TotalProcessed int                     `json:"totalProcessed"` // Items that required action (creates + updates)
	Created        int                     `json:"created"`
	Updated        int                     `json:"updated"`
	Skipped        int                     `json:"skipped"`   // Reserved registrars skipped
	Unchanged      int                     `json:"unchanged"` // Registrars already in sync (no action needed)
	Failed         int                     `json:"failed"`
	Notes          []string                `json:"notes"`
	CreatedItems   []SyncCreatedRegistrar  `json:"createdItems,omitempty"`
	UpdatedItems   []SyncUpdatedRegistrar  `json:"updatedItems,omitempty"`
	Failures       []SyncRegistrarsFailure `json:"failures,omitempty"`
}

// SyncCreatedRegistrar records a registrar that was created during the sync.
type SyncCreatedRegistrar struct {
	ClID       string `json:"clId"`
	Name       string `json:"name"`
	GurID      int    `json:"gurId"`
	Status     string `json:"status"`
	IANAStatus string `json:"ianaStatus"`
}

// SyncUpdatedRegistrar records a registrar whose status was updated during the sync.
type SyncUpdatedRegistrar struct {
	ClID          string `json:"clId"`
	OldStatus     string `json:"oldStatus,omitempty"`
	NewStatus     string `json:"newStatus,omitempty"`
	OldIANAStatus string `json:"oldIanaStatus,omitempty"`
	NewIANAStatus string `json:"newIanaStatus,omitempty"`
}

// SyncRegistrarsFailure records a single registrar sync failure.
type SyncRegistrarsFailure struct {
	ClID      string `json:"clId"`
	Operation string `json:"operation"` // "create", "update-status", "update-iana-status"
	Error     string `json:"error"`
}

// addFailure appends a failure record to the SyncRegistrarsResult.
func (r *SyncRegistrarsResult) addFailure(clID, operation, errMsg string) {
	r.Failed++
	if len(r.Failures) < maxSyncFailureSamples {
		r.Failures = append(r.Failures, SyncRegistrarsFailure{
			ClID:      clID,
			Operation: operation,
			Error:     errMsg,
		})
	}
}

// SyncRegistrarsWorkflow orchestrates synchronization of registrar data.
func SyncRegistrarsWorkflow(ctx workflow.Context, params SyncRegistrarsParams) (SyncRegistrarsResult, error) {
	started := workflow.Now(ctx)
	result := SyncRegistrarsResult{
		StartedAt: started,
	}

	logger := workflow.GetLogger(ctx)

	// Register query handler for progress tracking
	err := workflow.SetQueryHandler(ctx, "progress", func() (SyncRegistrarsResult, error) {
		return result, nil
	})
	if err != nil {
		result.CompletedAt = workflow.Now(ctx)
		result.Notes = append(result.Notes, "failed to register query handler: "+err.Error())
		return result, err
	}

	workflowID := getWorkflowID(ctx)

	batchSize := params.BatchSize
	if batchSize <= 0 {
		batchSize = 100
	}

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
		RetryPolicy:         retrypolicy,
	}

	// Apply the options.
	ctx = workflow.WithActivityOptions(ctx, options)

	// Step 1: Sync registrars with IANA (refresh the iana_registrars table from XML source)
	syncErr := workflow.ExecuteActivity(ctx, activities.SyncIanaRegistrars, workflowID).Get(ctx, nil)
	if syncErr != nil {
		result.CompletedAt = workflow.Now(ctx)
		result.Notes = append(result.Notes, "failed to sync registrars with IANA: "+syncErr.Error())
		logger.Error("failed to sync registrars with IANA", "error", syncErr)
		return result, syncErr
	}

	// Step 2: Check if this is our first time syncing registrars (zero registrars in the system)
	var rarCount response.CountResult
	countErr := workflow.ExecuteActivity(ctx, activities.CountRegistrars, workflowID).Get(ctx, &rarCount)
	if countErr != nil {
		result.CompletedAt = workflow.Now(ctx)
		result.Notes = append(result.Notes, "failed to count registrars: "+countErr.Error())
		logger.Error("failed to count registrars", "error", countErr)
		return result, countErr
	}

	// Bootstrap path: first import of registrars
	if rarCount.Count == 0 {
		return syncRegistrarsBootstrap(ctx, params, batchSize, workflowID, result, logger)
	}

	// Sync path: diff and apply incremental changes
	return syncRegistrarsIncremental(ctx, params, batchSize, workflowID, result, logger)
}

// syncRegistrarsBootstrap handles the first-time import when zero registrars exist.
func syncRegistrarsBootstrap(ctx workflow.Context, params SyncRegistrarsParams, batchSize int, workflowID string, result SyncRegistrarsResult, logger log.Logger) (SyncRegistrarsResult, error) {
	var csvRars []icannregistrars.CSVRegistrar
	getIcannErr := workflow.ExecuteActivity(ctx, activities.GetICANNRegistrars, workflowID, "./initdata/icannRegistrarList.csv").Get(ctx, &csvRars)
	if getIcannErr != nil {
		result.CompletedAt = workflow.Now(ctx)
		result.Notes = append(result.Notes, "failed to get ICANN registrars from file: "+getIcannErr.Error())
		logger.Error("failed to get ICANN registrars from file", "error", getIcannErr)
		return result, getIcannErr
	}

	var ianaRars []entities.IANARegistrar
	ianaRarErr := workflow.ExecuteActivity(ctx, activities.GetIANARegistrars, workflowID, batchSize).Get(ctx, &ianaRars)
	if ianaRarErr != nil {
		result.CompletedAt = workflow.Now(ctx)
		result.Notes = append(result.Notes, "failed to get IANA registrars: "+ianaRarErr.Error())
		logger.Error("failed to get IANA registrars", "error", ianaRarErr)
		return result, ianaRarErr
	}

	result.TotalIANA = len(ianaRars)
	result.TotalExisting = 0

	cmds := []commands.CreateRegistrarCommand{}
	createCmdErr := workflow.ExecuteActivity(ctx, activities.MakeCreateRegistrarCommands, workflowID, csvRars, ianaRars).Get(ctx, &cmds)
	if createCmdErr != nil {
		result.CompletedAt = workflow.Now(ctx)
		result.Notes = append(result.Notes, "failed to build create commands: "+createCmdErr.Error())
		logger.Error("failed to build create commands", "error", createCmdErr)
		return result, createCmdErr
	}

	if params.DryRun {
		result.TotalProcessed = len(cmds)
		result.Created = len(cmds)
		result.Skipped = result.TotalIANA - len(cmds)
		result.CompletedAt = workflow.Now(ctx)
		result.Notes = append(result.Notes, "Dry run completed: no state changes made")
		return result, nil
	}

	// Process the creates in chunks via ExecuteActivity
	for chunk := range commands.ChunkCreateRegistrarCommands(cmds, 100) {
		bulkCreateErr := workflow.ExecuteActivity(ctx, activities.BulkCreateRegistrars, workflowID, chunk).Get(ctx, nil)
		if bulkCreateErr != nil {
			result.CompletedAt = workflow.Now(ctx)
			result.Notes = append(result.Notes, "failed bulk creating registrars: "+bulkCreateErr.Error())
			logger.Error("failed bulk creating registrars", "error", bulkCreateErr)
			return result, bulkCreateErr
		}
		for _, cmd := range chunk {
			result.Created++
			result.TotalProcessed++
			if len(result.CreatedItems) < maxSyncDetailSamples {
				result.CreatedItems = append(result.CreatedItems, SyncCreatedRegistrar{
					ClID:       cmd.ClID,
					Name:       cmd.Name,
					GurID:      cmd.GurID,
					Status:     cmd.Status,
					IANAStatus: string(cmd.IANAStatus),
				})
			}
		}
	}

	result.Skipped = result.TotalIANA - result.Created
	result.CompletedAt = workflow.Now(ctx)
	return result, nil
}

// syncRegistrarsIncremental handles the diff-and-apply sync path for subsequent runs.
func syncRegistrarsIncremental(ctx workflow.Context, params SyncRegistrarsParams, batchSize int, workflowID string, result SyncRegistrarsResult, logger log.Logger) (SyncRegistrarsResult, error) {
	// Step 3: Fetch IANA registrars (source of truth)
	var ianaRars []entities.IANARegistrar
	ianaRarErr := workflow.ExecuteActivity(ctx, activities.GetIANARegistrars, workflowID, batchSize).Get(ctx, &ianaRars)
	if ianaRarErr != nil {
		result.CompletedAt = workflow.Now(ctx)
		result.Notes = append(result.Notes, "failed to get IANA registrars: "+ianaRarErr.Error())
		logger.Error("failed to get IANA registrars", "error", ianaRarErr)
		return result, ianaRarErr
	}

	// Step 4: Fetch existing platform registrars (now includes IANAStatus for proper diffing)
	var rars []entities.RegistrarListItem
	rarsErr := workflow.ExecuteActivity(ctx, activities.GetRegistrarListItems, workflowID, batchSize).Get(ctx, &rars)
	if rarsErr != nil {
		result.CompletedAt = workflow.Now(ctx)
		result.Notes = append(result.Notes, "failed to get registrar list items: "+rarsErr.Error())
		logger.Error("failed to get registrar list items", "error", rarsErr)
		return result, rarsErr
	}

	result.TotalIANA = len(ianaRars)
	result.TotalExisting = len(rars)

	// Step 5: Diff IANA source against existing registrars — produces creates and status updates
	var plan activities.DiffPlanResult
	planErr := workflow.ExecuteActivity(ctx, activities.DiffAndPlanRegistrars, workflowID, ianaRars, rars).Get(ctx, &plan)
	if planErr != nil {
		result.CompletedAt = workflow.Now(ctx)
		result.Notes = append(result.Notes, "failed to diff and plan registrars: "+planErr.Error())
		logger.Error("failed to diff and plan registrars", "error", planErr)
		return result, planErr
	}

	result.Skipped = plan.SkippedReserved
	// Unchanged = IANA registrars that exist and had no status diff, minus skipped
	result.Unchanged = result.TotalIANA - len(plan.Creates) - len(plan.Updates) - plan.SkippedReserved

	if params.DryRun {
		result.TotalProcessed = len(plan.Creates) + len(plan.Updates)
		result.Created = len(plan.Creates)
		result.Updated = len(plan.Updates)
		// Populate details for dry run preview
		for _, cmd := range plan.Creates {
			if len(result.CreatedItems) < maxSyncDetailSamples {
				result.CreatedItems = append(result.CreatedItems, SyncCreatedRegistrar{
					ClID:       cmd.ClID,
					Name:       cmd.Name,
					GurID:      cmd.GurID,
					Status:     cmd.Status,
					IANAStatus: string(cmd.IANAStatus),
				})
			}
		}
		for _, upd := range plan.Updates {
			if len(result.UpdatedItems) < maxSyncDetailSamples {
				result.UpdatedItems = append(result.UpdatedItems, SyncUpdatedRegistrar{
					ClID:          upd.ClID,
					OldStatus:     upd.OldStatus,
					NewStatus:     upd.NewStatus,
					OldIANAStatus: upd.OldIANAStatus,
					NewIANAStatus: upd.NewIANAStatus,
				})
			}
		}
		result.CompletedAt = workflow.Now(ctx)
		result.Notes = append(result.Notes, "Dry run completed: no state changes made")
		return result, nil
	}

	// Step 6: Apply creates in chunks
	for chunk := range commands.ChunkCreateRegistrarCommands(plan.Creates, 100) {
		bulkCreateErr := workflow.ExecuteActivity(ctx, activities.BulkCreateRegistrars, workflowID, chunk).Get(ctx, nil)
		if bulkCreateErr != nil {
			result.CompletedAt = workflow.Now(ctx)
			result.Notes = append(result.Notes, "failed to apply registrar creates: "+bulkCreateErr.Error())
			logger.Error("failed to apply registrar creates", "error", bulkCreateErr)
			return result, bulkCreateErr
		}
		for _, cmd := range chunk {
			result.Created++
			result.TotalProcessed++
			if len(result.CreatedItems) < maxSyncDetailSamples {
				result.CreatedItems = append(result.CreatedItems, SyncCreatedRegistrar{
					ClID:       cmd.ClID,
					Name:       cmd.Name,
					GurID:      cmd.GurID,
					Status:     cmd.Status,
					IANAStatus: string(cmd.IANAStatus),
				})
			}
		}
	}

	// Step 7: Apply status updates in a single batched activity
	// Uses longer timeout since it makes sequential HTTP calls for each update
	if len(plan.Updates) > 0 {
		// Build lookup map for update details (old→new status pairs)
		updateMap := make(map[string]commands.UpdateRegistrarStatusCommand, len(plan.Updates))
		for _, upd := range plan.Updates {
			updateMap[upd.ClID] = upd
		}

		bulkUpdateCtx := workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
			StartToCloseTimeout: 10 * time.Minute,
			RetryPolicy: &temporal.RetryPolicy{
				InitialInterval:        time.Second,
				BackoffCoefficient:     2.0,
				MaximumInterval:        10 * time.Minute,
				MaximumAttempts:        3,
				NonRetryableErrorTypes: []string{"none"},
			},
		})

		var bulkResult activities.BulkUpdateResult
		bulkErr := workflow.ExecuteActivity(bulkUpdateCtx, activities.BulkUpdateRegistrarStatuses, workflowID, plan.Updates).Get(ctx, &bulkResult)
		if bulkErr != nil {
			result.CompletedAt = workflow.Now(ctx)
			result.Notes = append(result.Notes, "failed to apply bulk status updates: "+bulkErr.Error())
			logger.Error("failed to apply bulk status updates", "error", bulkErr)
			return result, bulkErr
		}

		result.Updated = bulkResult.Updated
		result.TotalProcessed += bulkResult.Updated + bulkResult.Failed

		// Populate updated item details from successfully updated ClIDs
		for _, clID := range bulkResult.UpdatedIDs {
			if len(result.UpdatedItems) < maxSyncDetailSamples {
				if upd, ok := updateMap[clID]; ok {
					result.UpdatedItems = append(result.UpdatedItems, SyncUpdatedRegistrar{
						ClID:          upd.ClID,
						OldStatus:     upd.OldStatus,
						NewStatus:     upd.NewStatus,
						OldIANAStatus: upd.OldIANAStatus,
						NewIANAStatus: upd.NewIANAStatus,
					})
				}
			}
		}

		// Surface individual failures from the bulk operation
		for _, e := range bulkResult.Errors {
			result.addFailure(e.ClID, e.Operation, e.Error)
		}
	}

	result.CompletedAt = workflow.Now(ctx)

	// Add summary notes
	if result.Failed > 0 {
		result.Notes = append(result.Notes, "Completed with failures — review the failures list for details")
		if result.Failed > maxSyncFailureSamples {
			result.Notes = append(result.Notes, "Failure details capped at "+strconv.Itoa(maxSyncFailureSamples)+
				" samples; total failures: "+strconv.Itoa(result.Failed))
		}
	}

	return result, nil
}
