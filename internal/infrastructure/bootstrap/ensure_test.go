package bootstrap

import (
	"testing"
	"time"

	"github.com/onasunnymorning/domain-os/internal/application/workflows"
	"github.com/onasunnymorning/domain-os/internal/infrastructure/temporal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	enumspb "go.temporal.io/api/enums/v1"
	"go.temporal.io/sdk/client"
)

func Test_desiredSchedules_AllDefined(t *testing.T) {
	specs := desiredSchedules()

	// We must have exactly 8 schedules.
	require.Len(t, specs, 8, "expected 8 schedules in desiredSchedules()")

	// Map by ID for easy lookup
	byID := make(map[string]scheduleSpec, len(specs))
	for _, s := range specs {
		byID[s.ID] = s
	}

	t.Run("expiry-loop", func(t *testing.T) {
		s, ok := byID["expiry-loop"]
		require.True(t, ok, "missing schedule expiry-loop")
		assert.Equal(t, temporal.QueueLifecycle, s.Queue)
		assert.Equal(t, time.Hour, s.Interval)
		assert.Equal(t, time.Duration(0), s.Offset)
		assert.Equal(t, time.Hour, s.CatchupWindow)
		require.Len(t, s.Args, 1)
		_, ok = s.Args[0].(workflows.ExpiryLoopParams)
		assert.True(t, ok, "ExpiryLoop args should be ExpiryLoopParams, got %T", s.Args[0])
		assert.NotEmpty(t, s.Note)
	})

	t.Run("purge-loop", func(t *testing.T) {
		s, ok := byID["purge-loop"]
		require.True(t, ok, "missing schedule purge-loop")
		assert.Equal(t, temporal.QueueLifecycle, s.Queue)
		assert.Equal(t, time.Hour, s.Interval)
		assert.Equal(t, 30*time.Minute, s.Offset)
		assert.Equal(t, time.Hour, s.CatchupWindow)
		require.Len(t, s.Args, 1)
		_, ok = s.Args[0].(workflows.PurgeLoopParams)
		assert.True(t, ok, "PurgeLoop args should be PurgeLoopParams, got %T", s.Args[0])
		assert.NotEmpty(t, s.Note)
	})

	t.Run("restore-loop", func(t *testing.T) {
		s, ok := byID["restore-loop"]
		require.True(t, ok, "missing schedule restore-loop")
		assert.Equal(t, temporal.QueueLifecycle, s.Queue)
		assert.Equal(t, 4*time.Hour, s.Interval)
		assert.Equal(t, time.Hour, s.Offset)
		assert.Equal(t, 4*time.Hour, s.CatchupWindow)
		assert.Nil(t, s.Args)
		assert.NotEmpty(t, s.Note)
	})

	t.Run("sync-registrars", func(t *testing.T) {
		s, ok := byID["sync-registrars"]
		require.True(t, ok, "missing schedule sync-registrars")
		assert.Equal(t, temporal.QueueScheduled, s.Queue)
		assert.Equal(t, 24*time.Hour, s.Interval)
		assert.Equal(t, 2*time.Hour, s.Offset)
		assert.Equal(t, 24*time.Hour, s.CatchupWindow)
		require.Len(t, s.Args, 1)
		_, ok = s.Args[0].(workflows.SyncRegistrarsParams)
		require.True(t, ok, "SyncRegistrars args should be SyncRegistrarsParams, got %T", s.Args[0])
		assert.NotEmpty(t, s.Note)
	})

	t.Run("update-fx", func(t *testing.T) {
		s, ok := byID["update-fx"]
		require.True(t, ok, "missing schedule update-fx")
		assert.Equal(t, temporal.QueueFastOps, s.Queue)
		assert.Equal(t, time.Hour, s.Interval)
		assert.Equal(t, 30*time.Minute, s.Offset)
		assert.Equal(t, time.Hour, s.CatchupWindow)
		assert.Nil(t, s.Args)
		assert.NotEmpty(t, s.Note)
	})

	t.Run("sync-spec5", func(t *testing.T) {
		s, ok := byID["sync-spec5"]
		require.True(t, ok, "missing schedule sync-spec5")
		assert.Equal(t, temporal.QueueScheduled, s.Queue)
		assert.Equal(t, 24*time.Hour, s.Interval)
		assert.Equal(t, 4*time.Hour, s.Offset)
		assert.Equal(t, 24*time.Hour, s.CatchupWindow)
		assert.Nil(t, s.Args)
		assert.NotEmpty(t, s.Note)
	})

	t.Run("event-relay", func(t *testing.T) {
		s, ok := byID["event-relay"]
		require.True(t, ok, "missing schedule event-relay")
		assert.Equal(t, temporal.QueueScheduled, s.Queue)
		assert.Equal(t, 5*time.Minute, s.Interval)
		assert.Equal(t, time.Duration(0), s.Offset)
		assert.Equal(t, 5*time.Minute, s.CatchupWindow)
		require.Len(t, s.Args, 1)
		_, ok = s.Args[0].(workflows.EventRelayParams)
		assert.True(t, ok, "EventRelay args should be EventRelayParams, got %T", s.Args[0])
		assert.NotEmpty(t, s.Note)
	})

	t.Run("event-prune", func(t *testing.T) {
		s, ok := byID["event-prune"]
		require.True(t, ok, "missing schedule event-prune")
		assert.Equal(t, temporal.QueueScheduled, s.Queue)
		assert.Equal(t, 24*time.Hour, s.Interval)
		assert.Equal(t, 6*time.Hour, s.Offset)
		assert.Equal(t, 24*time.Hour, s.CatchupWindow)
		require.Len(t, s.Args, 1)
		_, ok = s.Args[0].(workflows.EventPruneParams)
		assert.True(t, ok, "EventPrune args should be EventPruneParams, got %T", s.Args[0])
		assert.NotEmpty(t, s.Note)
	})
}

func Test_desiredSchedules_UniqueIDs(t *testing.T) {
	specs := desiredSchedules()
	ids := make(map[string]bool, len(specs))
	for _, s := range specs {
		assert.False(t, ids[s.ID], "duplicate schedule ID: %s", s.ID)
		ids[s.ID] = true
	}
}

func Test_buildScheduleOptions(t *testing.T) {
	spec := scheduleSpec{
		ID:            "test-schedule",
		Queue:         "test-queue",
		Interval:      2 * time.Hour,
		Offset:        15 * time.Minute,
		Args:          []interface{}{"arg1"},
		CatchupWindow: 3 * time.Hour,
		Note:          "Test note",
	}

	opts := buildScheduleOptions(spec)

	assert.Equal(t, "test-schedule", opts.ID)
	assert.Equal(t, "Test note", opts.Note)
	assert.Equal(t, 3*time.Hour, opts.CatchupWindow)
	assert.Equal(t, enumspb.SCHEDULE_OVERLAP_POLICY_SKIP, opts.Overlap)

	action, ok := opts.Action.(*client.ScheduleWorkflowAction)
	require.True(t, ok)
	assert.Equal(t, "test-schedule-run", action.ID)
	assert.Equal(t, "test-queue", action.TaskQueue)
	assert.Equal(t, []interface{}{"arg1"}, action.Args)

	require.Len(t, opts.Spec.Intervals, 1)
	assert.Equal(t, 2*time.Hour, opts.Spec.Intervals[0].Every)
	assert.Equal(t, 15*time.Minute, opts.Spec.Intervals[0].Offset)
}

func Test_scheduleDrifted(t *testing.T) {
	spec := scheduleSpec{
		ID:            "test",
		Interval:      time.Hour,
		Offset:        30 * time.Minute,
		CatchupWindow: time.Hour,
		Note:          "Test schedule",
	}

	matchingDesc := client.ScheduleDescription{
		Schedule: client.Schedule{
			Spec: &client.ScheduleSpec{
				Intervals: []client.ScheduleIntervalSpec{
					{Every: time.Hour, Offset: 30 * time.Minute},
				},
			},
			Policy: &client.SchedulePolicies{
				Overlap:       enumspb.SCHEDULE_OVERLAP_POLICY_SKIP,
				CatchupWindow: time.Hour,
			},
			State: &client.ScheduleState{
				Note: "Test schedule",
			},
		},
	}

	t.Run("no drift when matching", func(t *testing.T) {
		assert.False(t, scheduleDrifted(matchingDesc, spec))
	})

	t.Run("drift on interval change", func(t *testing.T) {
		desc := matchingDesc
		desc.Schedule.Spec = &client.ScheduleSpec{
			Intervals: []client.ScheduleIntervalSpec{
				{Every: 2 * time.Hour, Offset: 30 * time.Minute},
			},
		}
		assert.True(t, scheduleDrifted(desc, spec))
	})

	t.Run("drift on offset change", func(t *testing.T) {
		desc := matchingDesc
		desc.Schedule.Spec = &client.ScheduleSpec{
			Intervals: []client.ScheduleIntervalSpec{
				{Every: time.Hour, Offset: 15 * time.Minute},
			},
		}
		assert.True(t, scheduleDrifted(desc, spec))
	})

	t.Run("drift on overlap policy change", func(t *testing.T) {
		desc := matchingDesc
		desc.Schedule.Policy = &client.SchedulePolicies{
			Overlap:       enumspb.SCHEDULE_OVERLAP_POLICY_BUFFER_ONE,
			CatchupWindow: time.Hour,
		}
		assert.True(t, scheduleDrifted(desc, spec))
	})

	t.Run("drift on catchup window change", func(t *testing.T) {
		desc := matchingDesc
		desc.Schedule.Policy = &client.SchedulePolicies{
			Overlap:       enumspb.SCHEDULE_OVERLAP_POLICY_SKIP,
			CatchupWindow: 24 * time.Hour,
		}
		assert.True(t, scheduleDrifted(desc, spec))
	})

	t.Run("drift on note change", func(t *testing.T) {
		desc := matchingDesc
		desc.Schedule.State = &client.ScheduleState{
			Note: "Different note",
		}
		assert.True(t, scheduleDrifted(desc, spec))
	})

	t.Run("drift on nil spec", func(t *testing.T) {
		desc := matchingDesc
		desc.Schedule.Spec = nil
		assert.True(t, scheduleDrifted(desc, spec))
	})

	t.Run("drift on nil policy", func(t *testing.T) {
		desc := matchingDesc
		desc.Schedule.Policy = nil
		assert.True(t, scheduleDrifted(desc, spec))
	})

	t.Run("drift on nil state", func(t *testing.T) {
		desc := matchingDesc
		desc.Schedule.State = nil
		assert.True(t, scheduleDrifted(desc, spec))
	})
}
