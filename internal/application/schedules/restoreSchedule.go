package schedules

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/onasunnymorning/domain-os/internal/application/workflows"
	"github.com/onasunnymorning/domain-os/internal/infrastructure/temporal"
	"go.temporal.io/sdk/client"
)

var (
	restoreScheduleIDPrefix = "restore_schedule_"
	restoreWorkflowIDPrefix = "restore_workflow_"
)

func CreateRestoreScheduleDaily(cfg temporal.TemporalClientconfig) (string, error) {
	ctx := context.Background()

	scheduleID := restoreScheduleIDPrefix + uuid.NewString()
	workflowID := restoreWorkflowIDPrefix + uuid.NewString()

	// Create a Temporal client
	temporalClient, err := temporal.GetTemporalClient(cfg)
	if err != nil {
		return "", err
	}

	// Create the schedule.
	scheduleHandle, err := temporalClient.ScheduleClient().Create(ctx, client.ScheduleOptions{
		ID: scheduleID,
		Spec: client.ScheduleSpec{
			Intervals: []client.ScheduleIntervalSpec{
				{
					Every:  time.Hour,
					Offset: time.Minute * 15,
				},
			},
		},
		Action: &client.ScheduleWorkflowAction{
			ID:        workflowID,
			Workflow:  workflows.RestoreWorkflow,
			TaskQueue: cfg.WorkerQueue,
		},
	})
	if err != nil {
		return "", err
	}
	return scheduleHandle.GetID(), nil
}
