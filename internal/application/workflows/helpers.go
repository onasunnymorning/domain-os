package workflows

import (
	"go.temporal.io/sdk/workflow"
)

// getWorkflowID returns the workflow ID from a workflow context.
// Note: This function must be called from within a workflow execution context (i.e., workflow code),
// NOT from an activity, use it to set the workflow ID in the context of a workflow and pass it to activities.
func getWorkflowID(ctx workflow.Context) string {
	info := workflow.GetInfo(ctx)
	return info.WorkflowExecution.ID
}
