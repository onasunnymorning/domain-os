package schedules

import (
	"context"
	"log"
	"time"

	"github.com/google/uuid"
	"github.com/onasunnymorning/domain-os/internal/application/workflows"
	"github.com/onasunnymorning/domain-os/internal/infrastructure/temporal"
	"go.temporal.io/sdk/client"
)

var (
	expirySchedlueIDPrefix         = "expiry_schedule_"
	expiryScheduleWorkflowIDPrefix = "expiry_schedule_workflow_"
)

func CreateExpiryScheduleHourly(cfg temporal.TemporalClientconfig) (string, error) {
	ctx := context.Background()

	scheduleID := expirySchedlueIDPrefix + uuid.NewString()
	workflowID := expiryScheduleWorkflowIDPrefix + uuid.NewString()

	// Create a Temporal client
	temporalClient, err := temporal.GetTemporalClient(cfg)
	if err != nil {
		return "", err
	}
	defer temporalClient.Close()

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

func DeleteExpirySchedule(scheduleID string, cfg temporal.TemporalClientconfig) error {
	ctx := context.Background()

	// Create a Temporal client
	temporalClient, err := temporal.GetTemporalClient(cfg)
	if err != nil {
		return err
	}
	defer temporalClient.Close()

	// list schedules
	listView, _ := temporalClient.ScheduleClient().List(ctx, client.ScheduleListOptions{
		PageSize: 1,
	})

	// delete the expiry schedule
	for listView.HasNext() {
		log.Println(listView.Next())
	}

	return nil
}
