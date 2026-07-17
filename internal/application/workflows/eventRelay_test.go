package workflows

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/onasunnymorning/domain-os/internal/application/activities"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"go.temporal.io/sdk/testsuite"
)

type EventRelayWorkflowTestSuite struct {
	suite.Suite
	testsuite.WorkflowTestSuite
	env *testsuite.TestWorkflowEnvironment
}

func (s *EventRelayWorkflowTestSuite) SetupTest() {
	s.env = s.NewTestWorkflowEnvironment()
	s.env.RegisterWorkflow(EventRelay)

	// Register the REAL activity struct (uninitialized — every call is mocked
	// below). This pins the workflow's string-based activity references to
	// methods that actually exist: if the workflow asks for an activity name
	// the struct doesn't provide, activity resolution fails and the test
	// breaks. A missing RelayEventBatch implementation shipped to production
	// precisely because no test held this contract.
	s.env.RegisterActivity(&activities.EventRelayActivities{})
}

func (s *EventRelayWorkflowTestSuite) Test_EventRelay_NoEvents() {
	s.env.OnActivity("RelayEventBatch", mock.Anything, 200).Return(activities.RelayEventBatchResult{}, nil)
	s.env.OnActivity("CountUnpublishedEvents", mock.Anything).Return(int64(0), nil)

	s.env.ExecuteWorkflow(EventRelay, EventRelayParams{})
	s.Require().True(s.env.IsWorkflowCompleted())
	s.Require().NoError(s.env.GetWorkflowError())

	var result EventRelayResult
	s.Require().NoError(s.env.GetWorkflowResult(&result))
	s.Equal(0, result.TotalArchived)
	s.Equal(0, result.TotalBatches)
	s.Equal(int64(0), result.RemainingCount)
}

func (s *EventRelayWorkflowTestSuite) Test_EventRelay_ArchivesUntilDrained() {
	// Two productive batches, then an empty one signals the drain is complete
	callCount := 0
	s.env.OnActivity("RelayEventBatch", mock.Anything, 200).Return(
		func(ctx context.Context, batchSize int) (activities.RelayEventBatchResult, error) {
			callCount++
			if callCount <= 2 {
				return activities.RelayEventBatchResult{Archived: 200, S3Key: "events/archive/key-" + string(rune('0'+callCount))}, nil
			}
			return activities.RelayEventBatchResult{}, nil
		},
	)
	s.env.OnActivity("CountUnpublishedEvents", mock.Anything).Return(int64(0), nil)

	s.env.ExecuteWorkflow(EventRelay, EventRelayParams{})
	s.Require().True(s.env.IsWorkflowCompleted())
	s.Require().NoError(s.env.GetWorkflowError())

	var result EventRelayResult
	s.Require().NoError(s.env.GetWorkflowResult(&result))
	s.Equal(400, result.TotalArchived)
	s.Equal(2, result.TotalBatches)
	s.Len(result.S3Keys, 2)
	s.Equal(int64(0), result.RemainingCount)
}

func (s *EventRelayWorkflowTestSuite) Test_EventRelay_BatchCap_NotesRemaining() {
	// Every batch is full — the MaxBatches cap stops the loop and the
	// remaining count is reported with a note.
	s.env.OnActivity("RelayEventBatch", mock.Anything, 100).Return(
		activities.RelayEventBatchResult{Archived: 100, S3Key: "events/archive/full"}, nil)
	s.env.OnActivity("CountUnpublishedEvents", mock.Anything).Return(int64(500), nil)

	s.env.ExecuteWorkflow(EventRelay, EventRelayParams{BatchSize: 100, MaxBatches: 3})
	s.Require().True(s.env.IsWorkflowCompleted())
	s.Require().NoError(s.env.GetWorkflowError())

	var result EventRelayResult
	s.Require().NoError(s.env.GetWorkflowResult(&result))
	s.Equal(300, result.TotalArchived)
	s.Equal(3, result.TotalBatches)
	s.Equal(int64(500), result.RemainingCount)

	foundNote := false
	for _, n := range result.Notes {
		if strings.HasPrefix(n, "Batch cap reached") {
			foundNote = true
		}
	}
	s.True(foundNote, "expected the batch-cap note, got: %v", result.Notes)
}

func (s *EventRelayWorkflowTestSuite) Test_EventRelay_BatchError_FailsWorkflow() {
	s.env.OnActivity("RelayEventBatch", mock.Anything, 200).Return(
		activities.RelayEventBatchResult{}, errors.New("s3 unavailable"))

	s.env.ExecuteWorkflow(EventRelay, EventRelayParams{})
	s.Require().True(s.env.IsWorkflowCompleted())
	s.Require().Error(s.env.GetWorkflowError())
}

func TestEventRelayWorkflowTestSuite(t *testing.T) {
	suite.Run(t, new(EventRelayWorkflowTestSuite))
}
