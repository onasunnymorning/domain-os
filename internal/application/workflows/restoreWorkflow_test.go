package workflows

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/converter"
	"go.temporal.io/sdk/testsuite"
	"go.temporal.io/sdk/workflow"

	"github.com/onasunnymorning/domain-os/internal/application/services"
)

type RestoreWorkflowTestSuite struct {
	suite.Suite
	testsuite.WorkflowTestSuite
	env *testsuite.TestWorkflowEnvironment
}

func (s *RestoreWorkflowTestSuite) SetupTest() {
	s.env = s.NewTestWorkflowEnvironment()
	s.env.RegisterWorkflowWithOptions(RestoreWorkflow, workflow.RegisterOptions{Name: RestoreTypeName})
	// The activities are LifecycleActivities methods, registered by name.
	s.env.RegisterActivityWithOptions(
		func(ctx context.Context, correlationID string, dueBy, afterEnd time.Time, afterRoID int64, limit int) ([]services.LapsedRestore, error) {
			return nil, nil
		},
		activity.RegisterOptions{Name: "ListLapsedRestores"},
	)
	s.env.RegisterActivityWithOptions(
		func(ctx context.Context, correlationID string, dueBy time.Time, names []string) (services.BatchResult, error) {
			return services.BatchResult{}, nil
		},
		activity.RegisterOptions{Name: "BatchRevertLapsedRestores"},
	)
}

func (s *RestoreWorkflowTestSuite) result() RestoreLoopResult {
	s.Require().True(s.env.IsWorkflowCompleted())
	var r RestoreLoopResult
	s.Require().NoError(s.env.GetWorkflowResult(&r))
	return r
}

// continuedWith is the input a continue-as-new carries.
func (s *RestoreWorkflowTestSuite) continuedWith() RestoreLoopParams {
	s.Require().True(s.env.IsWorkflowCompleted())
	err := s.env.GetWorkflowError()
	s.Require().Error(err)
	var ce *workflow.ContinueAsNewError
	s.Require().True(errors.As(err, &ce), "continue-as-new, got %v", err)
	s.Equal(RestoreTypeName, ce.WorkflowType.Name)
	var next RestoreLoopParams
	s.Require().NoError(converter.GetDefaultDataConverter().FromPayloads(ce.Input, &next))
	return next
}

func lapsed(names ...string) []services.LapsedRestore {
	base := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	out := make([]services.LapsedRestore, len(names))
	for i, n := range names {
		out[i] = services.LapsedRestore{DomainName: n, RoID: int64(100 + i), RestoreEnd: base.Add(time.Duration(i) * time.Minute)}
	}
	return out
}

func (s *RestoreWorkflowTestSuite) Test_NothingLapsed() {
	s.env.OnActivity("ListLapsedRestores", mock.Anything, mock.Anything, mock.Anything, time.Time{}, int64(0), 1000).Return([]services.LapsedRestore{}, nil)
	s.env.ExecuteWorkflow(RestoreTypeName, RestoreLoopParams{})
	s.Require().NoError(s.env.GetWorkflowError())
	r := s.result()
	s.Equal(0, r.TotalFound)
	s.Contains(r.Notes, "No lapsed restores")
}

func (s *RestoreWorkflowTestSuite) Test_RevertsWhatLapsed_AsOfTheSweepsStart() {
	var listedBy, revertedBy time.Time
	s.env.OnActivity("ListLapsedRestores", mock.Anything, mock.Anything, mock.Anything, time.Time{}, int64(0), 1000).
		Return(func(_ context.Context, _ string, dueBy, _ time.Time, _ int64, _ int) ([]services.LapsedRestore, error) {
			listedBy = dueBy
			return lapsed("a.test", "b.test", "c.test"), nil
		})
	s.env.OnActivity("BatchRevertLapsedRestores", mock.Anything, mock.Anything, mock.Anything, []string{"a.test", "b.test", "c.test"}).
		Return(func(_ context.Context, _ string, dueBy time.Time, _ []string) (services.BatchResult, error) {
			revertedBy = dueBy
			return services.BatchResult{
				Succeeded: []string{"a.test"},
				Skipped:   []string{"b.test"},
				Failed:    []services.BatchFailure{{DomainName: "c.test", Error: "boom"}},
			}, nil
		})
	s.env.ExecuteWorkflow(RestoreTypeName, RestoreLoopParams{})
	s.Require().NoError(s.env.GetWorkflowError())
	r := s.result()
	s.Equal(3, r.TotalFound)
	s.Equal(1, r.Reverted)
	s.Equal(1, r.Skipped)
	s.Equal(1, r.Failed)
	s.Equal(3, r.TotalProcessed)
	s.Equal("c.test", r.Failures[0].DomainName)
	s.False(listedBy.IsZero())
	s.Equal(listedBy, revertedBy, "the batch re-checks against the same time it listed by")
	s.Equal(r.StartedAt, r.DueBy)
}

func (s *RestoreWorkflowTestSuite) Test_FullPage_ContinuesFromItsLastDomain() {
	page := lapsed("a.test", "b.test")
	s.env.OnActivity("ListLapsedRestores", mock.Anything, mock.Anything, mock.Anything, time.Time{}, int64(0), 2).Return(page, nil)
	s.env.OnActivity("BatchRevertLapsedRestores", mock.Anything, mock.Anything, mock.Anything, []string{"a.test", "b.test"}).
		Return(services.BatchResult{Succeeded: []string{"a.test", "b.test"}}, nil)
	s.env.ExecuteWorkflow(RestoreTypeName, RestoreLoopParams{BatchSize: 2})
	next := s.continuedWith()
	s.Equal(1, next.ContinuationCount)
	s.Equal(page[1].RestoreEnd, next.AfterRestoreEnd)
	s.Equal(page[1].RoID, next.AfterRoID)
	s.False(next.DueBy.IsZero(), "the sweep keeps its time")
}

// A page whose domains all fail cannot hold back the ones after it.
func (s *RestoreWorkflowTestSuite) Test_FullPageOfFailures_StillMovesOn() {
	page := lapsed("poison1.test", "poison2.test")
	s.env.OnActivity("ListLapsedRestores", mock.Anything, mock.Anything, mock.Anything, time.Time{}, int64(0), 2).Return(page, nil)
	s.env.OnActivity("BatchRevertLapsedRestores", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(services.BatchResult{Failed: []services.BatchFailure{{DomainName: "poison1.test", Error: "x"}, {DomainName: "poison2.test", Error: "y"}}}, nil)
	s.env.ExecuteWorkflow(RestoreTypeName, RestoreLoopParams{BatchSize: 2})
	next := s.continuedWith()
	s.Equal(page[1].RoID, next.AfterRoID)
}

func (s *RestoreWorkflowTestSuite) Test_ContinuedSweep_KeepsItsTimeAndCursor() {
	dueBy := time.Date(2026, 10, 22, 12, 0, 0, 0, time.UTC)
	after := time.Date(2026, 10, 1, 0, 1, 0, 0, time.UTC)
	s.env.OnActivity("ListLapsedRestores", mock.Anything, mock.Anything, dueBy, after, int64(101), 2).Return(lapsed("c.test"), nil)
	s.env.OnActivity("BatchRevertLapsedRestores", mock.Anything, mock.Anything, dueBy, []string{"c.test"}).
		Return(services.BatchResult{Succeeded: []string{"c.test"}}, nil)
	s.env.ExecuteWorkflow(RestoreTypeName, RestoreLoopParams{BatchSize: 2, ContinuationCount: 1, DueBy: dueBy, AfterRestoreEnd: after, AfterRoID: 101})
	s.Require().NoError(s.env.GetWorkflowError(), "a short page ends the sweep")
	s.Equal(dueBy, s.result().DueBy)
}

func (s *RestoreWorkflowTestSuite) Test_ContinuationCap() {
	page := lapsed("a.test", "b.test")
	s.env.OnActivity("ListLapsedRestores", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, 2).Return(page, nil)
	s.env.OnActivity("BatchRevertLapsedRestores", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(services.BatchResult{Succeeded: []string{"a.test", "b.test"}}, nil)
	s.env.ExecuteWorkflow(RestoreTypeName, RestoreLoopParams{BatchSize: 2, ContinuationCount: maxContinuationRuns})
	s.Require().NoError(s.env.GetWorkflowError(), "the cap ends the sweep; the next hourly run picks up the rest")
}

func (s *RestoreWorkflowTestSuite) Test_ListFailure() {
	s.env.OnActivity("ListLapsedRestores", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, 1000).Return(nil, fmt.Errorf("db down"))
	s.env.ExecuteWorkflow(RestoreTypeName, RestoreLoopParams{})
	s.Require().Error(s.env.GetWorkflowError())
}

func (s *RestoreWorkflowTestSuite) Test_BatchFailure() {
	s.env.OnActivity("ListLapsedRestores", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, 1000).Return(lapsed("a.test"), nil)
	s.env.OnActivity("BatchRevertLapsedRestores", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(services.BatchResult{}, fmt.Errorf("db down"))
	s.env.ExecuteWorkflow(RestoreTypeName, RestoreLoopParams{})
	s.Require().Error(s.env.GetWorkflowError())
}

func TestRestoreWorkflow(t *testing.T) {
	suite.Run(t, new(RestoreWorkflowTestSuite))
}
