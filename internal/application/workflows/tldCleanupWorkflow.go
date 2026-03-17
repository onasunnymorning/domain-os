package workflows

import (
	"fmt"
	"time"

	"github.com/onasunnymorning/domain-os/internal/application/activities"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

type TLDCleanupParams struct {
	TLD              string
	KeepTLDAndPhases bool
}

type TLDCleanupResponse struct {
	ManifestKey  string
	BackupKey    string
	DeletedCount int64
}

// TLDCleanupWorkflow safely backs up and removes all assets (Domains, Contacts, etc.)
// associated with a given TLD after receiving explicit user confirmation.
func TLDCleanupWorkflow(ctx workflow.Context, params TLDCleanupParams) (TLDCleanupResponse, error) {
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 12 * time.Hour,       // These can take a long time for 5M domains
		HeartbeatTimeout:    5 * time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts: 3,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	var tldActivities *activities.TLDCleanupActivities // Activity struct pointer for method references

	// 1. Check if TLD can be deleted
	var checkResult activities.CheckTLDCanBeDeletedResult
	checkArgs := activities.CheckTLDCanBeDeletedArgs{TLD: params.TLD, KeepTLDAndPhases: params.KeepTLDAndPhases}
	if err := workflow.ExecuteActivity(ctx, tldActivities.CheckTLDCanBeDeleted, checkArgs).Get(ctx, &checkResult); err != nil {
		return TLDCleanupResponse{}, fmt.Errorf("check TLDCleanup failed: %w", err)
	}

	if !checkResult.CanBeDeleted {
		return TLDCleanupResponse{}, fmt.Errorf("TLD %s cannot be cleaned up: %s", params.TLD, checkResult.Reason)
	}

	workflow.GetLogger(ctx).Info("TLD check passed. Starting planning phase.")

	// 2. Plan Cleanup
	wfInfo := workflow.GetInfo(ctx)
	planArgs := activities.PlanTLDCleanupArgs{
		TLD:              params.TLD,
		WorkflowID:       wfInfo.WorkflowExecution.ID,
		KeepTLDAndPhases: params.KeepTLDAndPhases,
	}
	var planResult activities.PlanTLDCleanupResult
	if err := workflow.ExecuteActivity(ctx, tldActivities.PlanTLDCleanup, planArgs).Get(ctx, &planResult); err != nil {
		return TLDCleanupResponse{}, fmt.Errorf("planning phase failed: %w", err)
	}

	workflow.GetLogger(ctx).Info("Planning complete. Manifest uploaded.", 
		"ManifestKey", planResult.ManifestKey,
		"Domains", planResult.DomainCount,
		"Contacts", planResult.ContactCount,
		"Hosts", planResult.HostCount,
	)

	// 3. Await User Confirmation via Signal
	confirmationSignalChan := workflow.GetSignalChannel(ctx, "ConfirmTLDCleanup")
	var confirmed bool
	
	// Wait on the signal channel
	// We use a selector to safely block
	selector := workflow.NewSelector(ctx)
	selector.AddReceive(confirmationSignalChan, func(c workflow.ReceiveChannel, more bool) {
		var signalVal bool
		c.Receive(ctx, &signalVal)
		confirmed = signalVal
	})

	workflow.GetLogger(ctx).Info("Waiting for ConfirmTLDCleanup signal before proceeding with backup and deletion.")
	selector.Select(ctx)

	if !confirmed {
		workflow.GetLogger(ctx).Warn("Cleanup was cancelled by signal. Aborting workflow.")
		return TLDCleanupResponse{ManifestKey: planResult.ManifestKey}, fmt.Errorf("cleanup aborted by user signal")
	}

	// 4. Backup Assets
	workflow.GetLogger(ctx).Info("Confirmation received. Proceeding with S3 stream Backup.")
	backupArgs := activities.BackupTLDAssetsArgs{
		ManifestKey: planResult.ManifestKey,
		WorkflowID:  wfInfo.WorkflowExecution.ID,
		TLD:         params.TLD,
	}
	var backupResult activities.BackupTLDAssetsResult
	if err := workflow.ExecuteActivity(ctx, tldActivities.BackupTLDAssets, backupArgs).Get(ctx, &backupResult); err != nil {
		return TLDCleanupResponse{ManifestKey: planResult.ManifestKey}, fmt.Errorf("backup failed: %w", err)
	}

	workflow.GetLogger(ctx).Info("Backup complete.", "EntitiesSaved", backupResult.EntitiesSaved, "BackupKey", backupResult.BackupKey)

	// 5. Delete Assets (Cascade order built in)
	workflow.GetLogger(ctx).Info("Starting deletion of assets using S3 stream.")
	deleteArgs := activities.DeleteTLDAssetsArgs{
		ManifestKey: planResult.ManifestKey,
	}
	var deleteResult activities.DeleteTLDAssetsResult
	if err := workflow.ExecuteActivity(ctx, tldActivities.DeleteTLDAssets, deleteArgs).Get(ctx, &deleteResult); err != nil {
		return TLDCleanupResponse{
			ManifestKey: planResult.ManifestKey,
			BackupKey:   backupResult.BackupKey,
		}, fmt.Errorf("deletion failed: %w", err)
	}

	workflow.GetLogger(ctx).Info("TLD Cleanup complete.", "DeletedCount", deleteResult.DeletedCount)

	return TLDCleanupResponse{
		ManifestKey:  planResult.ManifestKey,
		BackupKey:    backupResult.BackupKey,
		DeletedCount: deleteResult.DeletedCount,
	}, nil
}
