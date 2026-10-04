package workflows

import "github.com/onasunnymorning/domain-os/internal/application/serialdrift"

// Registered Temporal workflow type names.
//
// The type name is what Temporal persists in history, in schedule actions and
// in visibility (the Web UI "Workflow Type" column, `WorkflowType = "..."`
// queries, the workflow_type metric label). It is NOT the Go function name:
// each workflow below is registered under its function name minus the
// "Workflow" suffix, via RegisterWorkflowWithOptions.
//
// A worker's alias only exists in the process that registered it, so any
// process that does not register workflows (the API server, ry-admin, the
// zone-slaving service, schedule actions) must start a workflow by one of these
// names. Passing the Go function there would send the old name. Use these
// constants everywhere a workflow is started by type: client ExecuteWorkflow,
// ExecuteChildWorkflow, NewContinueAsNewError and ScheduleWorkflowAction.
//
// Workflows whose function name has no "Workflow" suffix (UpdateFX, EventRelay,
// EventPrune, ExpiryLoop, PurgeLoop, TombstoneBackfill) register under the
// function name and need no constant.
//
// TestWorkflowTypeNames pins each constant to its function.
const (
	CheckSerialDriftTypeName  = serialdrift.WorkflowTypeName // CheckSerialDriftWorkflow
	EscrowImportTypeName      = "EscrowImport"               // EscrowImportWorkflow
	EscrowIntakeTypeName      = "EscrowIntake"               // EscrowIntakeWorkflow
	EscrowIntakeSweepTypeName = "EscrowIntakeSweep"          // EscrowIntakeSweepWorkflow
	EscrowKeyProbeTypeName    = "EscrowKeyProbe"             // EscrowKeyProbeWorkflow
	EscrowSanitizeTypeName    = "EscrowSanitize"             // EscrowSanitizeWorkflow
	EscrowValidationTypeName  = "EscrowValidation"           // EscrowValidationWorkflow
	RestoreTypeName           = "Restore"                    // RestoreWorkflow
	Spec5SweepTypeName        = "Spec5Sweep"                 // Spec5SweepWorkflow
	SyncRegistrarsTypeName    = "SyncRegistrars"             // SyncRegistrarsWorkflow
	SyncSpec5TypeName         = "SyncSpec5"                  // SyncSpec5Workflow
	TLDCleanupTypeName        = "TLDCleanup"                 // TLDCleanupWorkflow
)
