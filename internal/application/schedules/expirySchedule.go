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
	schedlueIDPrefix = "expiry_schedule_"
	workflowIDPrefix = "expiry_schedule_workflow_"
)

func CreateExpiryScheduleHourly(cfg temporal.TemporalClientconfig) (string, error) {
	ctx := context.Background()

	scheduleID := schedlueIDPrefix + uuid.NewString()
	workflowID := workflowIDPrefix + uuid.NewString()

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
					Every: time.Hour,
				},
			},
		},
		Action: &client.ScheduleWorkflowAction{
			ID:        workflowID,
			Workflow:  workflows.ExpiryLoop,
			TaskQueue: cfg.WorkerQueue,
		},
	})
	if err != nil {
		return "", err
	}
	return scheduleHandle.GetID(), nil
}
