// Package bootstrap provides self-healing startup routines for Temporal
// infrastructure. It ensures all required schedules exist on every deploy,
// eliminating the need for manual CLI commands or init containers.
package bootstrap

import (
	"context"
	"errors"
	"log"
	"time"

	"github.com/onasunnymorning/domain-os/internal/application/workflows"
	"github.com/onasunnymorning/domain-os/internal/infrastructure/temporal"
	"go.temporal.io/api/serviceerror"
	"go.temporal.io/sdk/client"
)

// scheduleSpec defines a schedule to be ensured on startup.
type scheduleSpec struct {
	ID       string        // Deterministic, fixed ID — makes Create idempotent
	Workflow interface{}   // Workflow function reference
	Queue    string        // Task queue to run on
	Interval time.Duration // How often to run
	Offset   time.Duration // Offset within the interval
	Args     []interface{} // Workflow arguments (nil for zero-param)
}

// desiredSchedules returns the canonical list of schedules that must exist.
// This is the single source of truth — add or remove schedules here.
func desiredSchedules() []scheduleSpec {
	return []scheduleSpec{
		{
			ID:       "dominos-expiry-loop",
			Workflow: workflows.ExpiryLoop,
			Queue:    temporal.QueueLifecycle,
			Interval: time.Hour,
			Offset:   0,
		},
		{
			ID:       "dominos-purge-loop",
			Workflow: workflows.PurgeLoop,
			Queue:    temporal.QueueLifecycle,
			Interval: time.Hour,
			Offset:   30 * time.Minute,
		},
		{
			ID:       "dominos-restore-workflow",
			Workflow: workflows.RestoreWorkflow,
			Queue:    temporal.QueueLifecycle,
			Interval: time.Hour,
			Offset:   15 * time.Minute,
		},
		{
			ID:       "dominos-sync-registrars",
			Workflow: workflows.SyncRegistrarsWorkflow,
			Queue:    temporal.QueueLifecycle,
			Interval: 24 * time.Hour,
			Offset:   2 * time.Hour,
			Args:     []interface{}{100}, // batchSize
		},
		{
			ID:       "dominos-update-fx",
			Workflow: workflows.UpdateFX,
			Queue:    temporal.QueueData,
			Interval: time.Hour,
			Offset:   30 * time.Minute,
		},
	}
}

// EnsureTemporalInfrastructure is a self-healing startup routine that ensures
// all required Temporal schedules exist. It is idempotent — safe to call on
// every deploy. If a schedule already exists, it is silently skipped.
//
// Call this once at worker startup, after the Temporal client is connected.
func EnsureTemporalInfrastructure(c client.Client) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	schedules := desiredSchedules()
	ensured := 0

	for _, spec := range schedules {
		err := ensureSchedule(ctx, c, spec)
		if err != nil {
			log.Printf("[infra] WARNING: failed to ensure schedule %q: %v", spec.ID, err)
			continue
		}
		ensured++
	}

	log.Printf("[infra] Schedule reconciliation complete: %d/%d schedules ensured", ensured, len(schedules))
}

// ensureSchedule creates a schedule if it doesn't exist, or skips if it does.
func ensureSchedule(ctx context.Context, c client.Client, spec scheduleSpec) error {
	_, err := c.ScheduleClient().Create(ctx, client.ScheduleOptions{
		ID: spec.ID,
		Spec: client.ScheduleSpec{
			Intervals: []client.ScheduleIntervalSpec{
				{
					Every:  spec.Interval,
					Offset: spec.Offset,
				},
			},
		},
		Action: &client.ScheduleWorkflowAction{
			ID:        spec.ID + "-run",
			Workflow:  spec.Workflow,
			Args:      spec.Args,
			TaskQueue: spec.Queue,
		},
	})

	if err != nil {
		// "Already exists" is the expected case on subsequent startups — not an error.
		var alreadyExists *serviceerror.AlreadyExists
		if errors.As(err, &alreadyExists) {
			log.Printf("[infra] Schedule %q already exists — skipping", spec.ID)
			return nil
		}
		return err
	}

	log.Printf("[infra] Schedule %q created (every %s, offset %s, queue %s)",
		spec.ID, spec.Interval, spec.Offset, spec.Queue)
	return nil
}
