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
	syncRegistrarScheduleIDPrefix         = "sync_registrar_schedule_"
	syncRegistrarScheduleWorkflowIDPrefix = "sync_registrar_schedule_workflow_"
)

func CreateSyncRegistrarScheduleHourly(cfg temporal.TemporalClientconfig) (string, error) {
	ctx := context.Background()

	scheduleID := syncRegistrarScheduleIDPrefix + uuid.NewString()
	workflowID := syncRegistrarScheduleWorkflowIDPrefix + uuid.NewString()

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
					Every:  time.Hour * 24,
					Offset: time.Hour * 2,
				},
			},
		},
		Action: &client.ScheduleWorkflowAction{
			ID:        workflowID,
			Workflow:  workflows.SyncRegistrarsWorkflow,
			TaskQueue: cfg.WorkerQueue,
		},
	})
	if err != nil {
		return "", err
	}
	return scheduleHandle.GetID(), nil
}
