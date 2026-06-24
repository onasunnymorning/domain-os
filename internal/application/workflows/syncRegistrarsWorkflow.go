package workflows

import (
	"strconv"
	"time"

	"github.com/onasunnymorning/domain-os/internal/application/activities"
	"github.com/onasunnymorning/domain-os/internal/application/commands"
	"github.com/onasunnymorning/domain-os/internal/infrastructure/web/icannregistrars"
	"github.com/onasunnymorning/domain-os/internal/interface/rest/response"
	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

const (
	maxSyncFailureSamples = 50
)

// SyncRegistrarsParams defines the input parameters for the SyncRegistrarsWorkflow.
type SyncRegistrarsParams struct {
	BatchSize        int  `json:"batchSize,omitempty"`
	ConcurrencyLimit int  `json:"concurrencyLimit,omitempty"`
	DryRun           bool `json:"dryRun,omitempty"`
}

// SyncRegistrarsResult is the structured output of the SyncRegistrarsWorkflow.
type SyncRegistrarsResult struct {
	StartedAt      time.Time               `json:"startedAt"`
	CompletedAt    time.Time               `json:"completedAt"`
	TotalProcessed int                     `json:"totalProcessed"`
	Created        int                     `json:"created"`
	Updated        int                     `json:"updated"`
	Failed         int                     `json:"failed"`
	Notes          []string                `json:"notes"`
	Failures       []SyncRegistrarsFailure `json:"failures,omitempty"`
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

	concurrencyLimit := params.ConcurrencyLimit
	if concurrencyLimit <= 0 {
		concurrencyLimit = 20
	}

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

	// Sync registrars with IANA
	syncErr := workflow.ExecuteActivity(ctx, activities.SyncIanaRegistrars, workflowID).Get(ctx, nil)
	if syncErr != nil {
		result.CompletedAt = workflow.Now(ctx)
		result.Notes = append(result.Notes, "failed to sync registrars with IANA: "+syncErr.Error())
		logger.Error("failed to sync registrars with IANA", "error", syncErr)
		return result, syncErr
	}

	// Check if this is our first time syncing registrars (zero registrars in the system)
	var rarCount response.CountResult
	countErr := workflow.ExecuteActivity(ctx, activities.CountRegistrars, workflowID).Get(ctx, &rarCount)
	if countErr != nil {
		result.CompletedAt = workflow.Now(ctx)
		result.Notes = append(result.Notes, "failed to count registrars: "+countErr.Error())
		logger.Error("failed to count registrars", "error", countErr)
		return result, countErr
	}

	// If it is our first time syncing, launch the first import of registrars
	if rarCount.Count == 0 {
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
			result.Created += len(chunk)
			result.TotalProcessed += len(chunk)
		}

		result.CompletedAt = workflow.Now(ctx)
		return result, nil
	}

	// Update the registrars that have changed
	var ianaRars []entities.IANARegistrar
	ianaRarErr := workflow.ExecuteActivity(ctx, activities.GetIANARegistrars, workflowID, batchSize).Get(ctx, &ianaRars)
	if ianaRarErr != nil {
		result.CompletedAt = workflow.Now(ctx)
		result.Notes = append(result.Notes, "failed to get IANA registrars: "+ianaRarErr.Error())
		logger.Error("failed to get IANA registrars", "error", ianaRarErr)
		return result, ianaRarErr
	}

	var rars []entities.RegistrarListItem
	rarsErr := workflow.ExecuteActivity(ctx, activities.GetRegistrarListItems, workflowID, batchSize).Get(ctx, &rars)
	if rarsErr != nil {
		result.CompletedAt = workflow.Now(ctx)
		result.Notes = append(result.Notes, "failed to get registrar list items: "+rarsErr.Error())
		logger.Error("failed to get registrar list items", "error", rarsErr)
		return result, rarsErr
	}

	var plan activities.DiffPlanResult
	planErr := workflow.ExecuteActivity(ctx, activities.DiffAndPlanRegistrars, workflowID, ianaRars, rars).Get(ctx, &plan)
	if planErr != nil {
		result.CompletedAt = workflow.Now(ctx)
		result.Notes = append(result.Notes, "failed to diff and plan registrars: "+planErr.Error())
		logger.Error("failed to diff and plan registrars", "error", planErr)
		return result, planErr
	}

	if params.DryRun {
		result.TotalProcessed = len(plan.Creates) + len(plan.Updates)
		result.Created = len(plan.Creates)
		result.Updated = len(plan.Updates)
		result.CompletedAt = workflow.Now(ctx)
		result.Notes = append(result.Notes, "Dry run completed: no state changes made")
		return result, nil
	}

	// Apply creates in chunks via ExecuteActivity
	for chunk := range commands.ChunkCreateRegistrarCommands(plan.Creates, 100) {
		bulkCreateErr := workflow.ExecuteActivity(ctx, activities.BulkCreateRegistrars, workflowID, chunk).Get(ctx, nil)
		if bulkCreateErr != nil {
			result.CompletedAt = workflow.Now(ctx)
			result.Notes = append(result.Notes, "failed to apply registrar creates: "+bulkCreateErr.Error())
			logger.Error("failed to apply registrar creates", "error", bulkCreateErr)
			return result, bulkCreateErr
		}
		result.Created += len(chunk)
		result.TotalProcessed += len(chunk)
	}

	// Apply updates in parallel
	semCh := workflow.NewBufferedChannel(ctx, concurrencyLimit)

	type futInfo struct {
		clID      string
		operation string
		future    workflow.Future
	}
	var futures []futInfo

	for _, upd := range plan.Updates {
		semCh.Send(ctx, struct{}{})
		f := workflow.ExecuteActivity(ctx, activities.SetRegistrarStatus, workflowID, upd.ClID, upd.NewStatus)
		futures = append(futures, futInfo{
			clID:      upd.ClID,
			operation: "update-status",
			future:    f,
		})
		workflow.Go(ctx, func(ctx workflow.Context) {
			_ = f.Get(ctx, nil)
			var token struct{}
			semCh.Receive(ctx, &token)
		})
	}

	// Ensure IANA status is up-to-date
	existingClIDs := make(map[string]struct{}, len(rars))
	for _, r := range rars {
		existingClIDs[r.ClID.String()] = struct{}{}
	}
	for _, ir := range ianaRars {
		clid, _ := ir.CreateClID()
		if _, ok := existingClIDs[clid.String()]; ok {
			semCh.Send(ctx, struct{}{})
			f := workflow.ExecuteActivity(ctx, activities.SetRegistrarIANAStatus, workflowID, clid.String(), ir.Status.String())
			futures = append(futures, futInfo{
				clID:      clid.String(),
				operation: "update-iana-status",
				future:    f,
			})
			workflow.Go(ctx, func(ctx workflow.Context) {
				_ = f.Get(ctx, nil)
				var token struct{}
				semCh.Receive(ctx, &token)
			})
		}
	}

	// Gather results
	for _, fut := range futures {
		err := fut.future.Get(ctx, nil)
		result.TotalProcessed++
		if err != nil {
			result.addFailure(fut.clID, fut.operation, err.Error())
			logger.Error("failed to apply registrar update", "clID", fut.clID, "operation", fut.operation, "error", err)
		} else {
			result.Updated++
		}
	}

	result.CompletedAt = workflow.Now(ctx)

	// Add summary note
	if result.Failed > 0 {
		result.Notes = append(result.Notes, "Completed with failures — review the failures list for details")
		if result.Failed > maxSyncFailureSamples {
			result.Notes = append(result.Notes, "Failure details capped at "+strconv.Itoa(maxSyncFailureSamples)+
				" samples; total failures: "+strconv.Itoa(result.Failed))
		}
	}

	return result, nil
}
