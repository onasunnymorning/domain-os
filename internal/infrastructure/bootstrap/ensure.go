// Package bootstrap provides self-healing startup routines for Temporal
// infrastructure. It ensures all required schedules exist on every deploy,
// eliminating the need for manual CLI commands or init containers.
package bootstrap

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/onasunnymorning/domain-os/internal/application/workflows"
	"github.com/onasunnymorning/domain-os/internal/infrastructure/temporal"
	enumspb "go.temporal.io/api/enums/v1"
	"go.temporal.io/api/serviceerror"
	"go.temporal.io/sdk/client"
)

// scheduleSpec defines a schedule to be ensured on startup.
type scheduleSpec struct {
	ID            string        // Deterministic, fixed ID — makes Create idempotent
	Workflow      interface{}   // Workflow function reference
	Queue         string        // Task queue to run on
	Interval      time.Duration // How often to run
	Offset        time.Duration // Offset within the interval
	Args          []interface{} // Workflow arguments (nil for zero-param)
	CatchupWindow time.Duration // How far back to catch up after an outage
	Note          string        // Human-readable description shown in Temporal UI
}

// desiredSchedules returns the canonical list of schedules that must exist.
// This is the single source of truth — add or remove schedules here.
func desiredSchedules() []scheduleSpec {
	return []scheduleSpec{
		{
			ID:            "expiry-loop",
			Workflow:      workflows.ExpiryLoop,
			Queue:         temporal.QueueLifecycle,
			Interval:      time.Hour,
			Offset:        0,
			Args:          []interface{}{workflows.ExpiryLoopParams{}},
			CatchupWindow: time.Hour,
			Note:          "Expires domains hourly — managed by bootstrap",
		},
		{
			ID:            "purge-loop",
			Workflow:      workflows.PurgeLoop,
			Queue:         temporal.QueueLifecycle,
			Interval:      time.Hour,
			Offset:        30 * time.Minute,
			Args:          []interface{}{workflows.PurgeLoopParams{}},
			CatchupWindow: time.Hour,
			Note:          "Purges redeemed domains hourly — managed by bootstrap",
		},
		{
			ID:            "restore-loop",
			Workflow:      workflows.RestoreWorkflow,
			Queue:         temporal.QueueLifecycle,
			Interval:      4 * time.Hour,
			Offset:        time.Hour,
			Args:          []interface{}{workflows.RestoreLoopParams{}},
			CatchupWindow: 4 * time.Hour,
			Note:          "Restores domains every 4 hours — managed by bootstrap",
		},
		{
			ID:            "sync-registrars",
			Workflow:      workflows.SyncRegistrarsWorkflow,
			Queue:         temporal.QueueScheduled,
			Interval:      24 * time.Hour,
			Offset:        2 * time.Hour,
			Args:          []interface{}{workflows.SyncRegistrarsParams{}},
			CatchupWindow: 24 * time.Hour,
			Note:          "Syncs registrars with IANA daily — managed by bootstrap",
		},
		{
			ID:            "update-fx",
			Workflow:      workflows.UpdateFX,
			Queue:         temporal.QueueFastOps,
			Interval:      time.Hour,
			Offset:        30 * time.Minute,
			CatchupWindow: time.Hour,
			Note:          "Updates FX rates hourly — managed by bootstrap",
		},
		{
			ID:            "sync-spec5",
			Workflow:      workflows.SyncSpec5Workflow,
			Queue:         temporal.QueueScheduled,
			Interval:      24 * time.Hour,
			Offset:        4 * time.Hour,
			CatchupWindow: 24 * time.Hour,
			Note:          "Syncs Spec5 XML daily — managed by bootstrap",
		},
		{
			ID:            "event-relay",
			Workflow:      workflows.EventRelay,
			Queue:         temporal.QueueScheduled,
			Interval:      5 * time.Minute,
			Offset:        0,
			Args:          []interface{}{workflows.EventRelayParams{}},
			CatchupWindow: 5 * time.Minute,
			Note:          "Archives events to S3 every 5 minutes — managed by bootstrap",
		},
		{
			ID:            "event-prune",
			Workflow:      workflows.EventPrune,
			Queue:         temporal.QueueScheduled,
			Interval:      24 * time.Hour,
			Offset:        6 * time.Hour,
			Args:          []interface{}{workflows.EventPruneParams{}},
			CatchupWindow: 24 * time.Hour,
			Note:          "Prunes archived events daily — managed by bootstrap",
		},
	}
}

// EnsureTemporalInfrastructure is a self-healing startup routine that ensures
// all required Temporal schedules exist with the correct configuration. It is
// idempotent — safe to call on every deploy.
//
// Behavior:
//   - If a schedule does not exist, it is created.
//   - If a schedule exists but its configuration has drifted from the desired
//     state, it is updated to match.
//   - If a schedule already matches, it is silently skipped.
//
// Call this once at worker startup, after the Temporal client is connected.
func EnsureTemporalInfrastructure(c client.Client) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	schedules := desiredSchedules()
	created, updated, upToDate := 0, 0, 0

	for _, spec := range schedules {
		action, err := ensureSchedule(ctx, c, spec)
		if err != nil {
			log.Printf("[infra] WARNING: failed to ensure schedule %q: %v", spec.ID, err)
			continue
		}
		switch action {
		case scheduleCreated:
			created++
		case scheduleUpdated:
			updated++
		case scheduleUpToDate:
			upToDate++
		}
	}

	log.Printf("[infra] Schedule reconciliation complete: %d created, %d updated, %d up-to-date (total: %d)",
		created, updated, upToDate, len(schedules))
}

type scheduleAction int

const (
	scheduleCreated  scheduleAction = iota
	scheduleUpdated  scheduleAction = iota
	scheduleUpToDate scheduleAction = iota
)

// buildScheduleOptions returns the ScheduleOptions for a given spec.
func buildScheduleOptions(spec scheduleSpec) client.ScheduleOptions {
	return client.ScheduleOptions{
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
		Overlap:       enumspb.SCHEDULE_OVERLAP_POLICY_SKIP,
		CatchupWindow: spec.CatchupWindow,
		Note:          spec.Note,
	}
}

// ensureSchedule creates a schedule if it doesn't exist, or updates it if the
// configuration has drifted from the desired state.
func ensureSchedule(ctx context.Context, c client.Client, spec scheduleSpec) (scheduleAction, error) {
	opts := buildScheduleOptions(spec)

	_, err := c.ScheduleClient().Create(ctx, opts)
	if err == nil {
		log.Printf("[infra] Schedule %q created (every %s, offset %s, queue %s)",
			spec.ID, spec.Interval, spec.Offset, spec.Queue)
		return scheduleCreated, nil
	}

	// "Already exists" is the expected case on subsequent startups.
	// The SDK may return either serviceerror.AlreadyExists (gRPC-level) or
	// ErrScheduleAlreadyRunning (SDK-level, from go.temporal.io/sdk/internal)
	// depending on the code path. We check for both.
	var alreadyExists *serviceerror.AlreadyExists
	isAlreadyExists := errors.As(err, &alreadyExists) ||
		strings.Contains(err.Error(), "already registered")
	if !isAlreadyExists {
		return 0, fmt.Errorf("create schedule %q: %w", spec.ID, err)
	}

	// Schedule exists — check for drift and update if needed.
	handle := c.ScheduleClient().GetHandle(ctx, spec.ID)
	desc, descErr := handle.Describe(ctx)
	if descErr != nil {
		return 0, fmt.Errorf("describe schedule %q for drift check: %w", spec.ID, descErr)
	}

	if !scheduleDrifted(*desc, spec) {
		log.Printf("[infra] Schedule %q is up to date — skipping", spec.ID)
		return scheduleUpToDate, nil
	}

	// Update the schedule to match the desired state.
	updateErr := handle.Update(ctx, client.ScheduleUpdateOptions{
		DoUpdate: func(input client.ScheduleUpdateInput) (*client.ScheduleUpdate, error) {
			// Replace the schedule spec and policies with our desired state.
			input.Description.Schedule.Spec = &client.ScheduleSpec{
				Intervals: []client.ScheduleIntervalSpec{
					{
						Every:  spec.Interval,
						Offset: spec.Offset,
					},
				},
			}
			input.Description.Schedule.Policy = &client.SchedulePolicies{
				Overlap:       enumspb.SCHEDULE_OVERLAP_POLICY_SKIP,
				CatchupWindow: spec.CatchupWindow,
			}
			input.Description.Schedule.State = &client.ScheduleState{
				Note: spec.Note,
			}
			return &client.ScheduleUpdate{
				Schedule: &input.Description.Schedule,
			}, nil
		},
	})
	if updateErr != nil {
		return 0, fmt.Errorf("update drifted schedule %q: %w", spec.ID, updateErr)
	}

	log.Printf("[infra] Schedule %q updated (drift detected — reconciled to desired state)", spec.ID)
	return scheduleUpdated, nil
}

// scheduleDrifted returns true if the existing schedule's configuration differs
// from the desired spec. Only checks the fields we manage.
func scheduleDrifted(desc client.ScheduleDescription, spec scheduleSpec) bool {
	sched := desc.Schedule

	// Check interval
	if sched.Spec != nil && len(sched.Spec.Intervals) > 0 {
		interval := sched.Spec.Intervals[0]
		if interval.Every != spec.Interval || interval.Offset != spec.Offset {
			return true
		}
	} else {
		// No spec or intervals means drift
		return true
	}

	// Check workflow action (task queue, workflow ID pattern)
	if action, ok := sched.Action.(*client.ScheduleWorkflowAction); ok {
		if action.TaskQueue != spec.Queue {
			log.Printf("[infra] Schedule %q task queue drifted: have %q, want %q", spec.ID, action.TaskQueue, spec.Queue)
			return true
		}
	}

	// Check overlap policy
	if sched.Policy != nil {
		if sched.Policy.Overlap != enumspb.SCHEDULE_OVERLAP_POLICY_SKIP {
			return true
		}
		if sched.Policy.CatchupWindow != spec.CatchupWindow {
			return true
		}
	} else {
		// No policy means drift — we want explicit policies
		return true
	}

	// Check note
	if sched.State != nil {
		if sched.State.Note != spec.Note {
			return true
		}
	} else {
		return true
	}

	return false
}
