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

type TransferApprovalLoopTestSuite struct {
	suite.Suite
	testsuite.WorkflowTestSuite
	env *testsuite.TestWorkflowEnvironment
}

func (s *TransferApprovalLoopTestSuite) SetupTest() {
	s.env = s.NewTestWorkflowEnvironment()
	s.env.RegisterWorkflow(TransferApprovalLoop)
	// The activities are LifecycleActivities methods, registered by name.
	s.env.RegisterActivityWithOptions(
		func(ctx context.Context, correlationID string, dueBy, afterDate time.Time, afterID int64, limit int) ([]services.DueTransfer, error) {
			return nil, nil
		},
		activity.RegisterOptions{Name: "ListDueTransfers"},
	)
	s.env.RegisterActivityWithOptions(
		func(ctx context.Context, correlationID string, dueBy time.Time, names []string) (services.BatchResult, error) {
			return services.BatchResult{}, nil
		},
		activity.RegisterOptions{Name: "BatchApproveDueTransfers"},
	)
}

func (s *TransferApprovalLoopTestSuite) result() TransferApprovalLoopResult {
	s.Require().True(s.env.IsWorkflowCompleted())
	var r TransferApprovalLoopResult
	s.Require().NoError(s.env.GetWorkflowResult(&r))
	return r
}

// continuedWith is the input a continue-as-new carries.
func (s *TransferApprovalLoopTestSuite) continuedWith() TransferApprovalLoopParams {
	s.Require().True(s.env.IsWorkflowCompleted())
	err := s.env.GetWorkflowError()
	s.Require().Error(err)
	var ce *workflow.ContinueAsNewError
	s.Require().True(errors.As(err, &ce), "continue-as-new, got %v", err)
	var next TransferApprovalLoopParams
	s.Require().NoError(converter.GetDefaultDataConverter().FromPayloads(ce.Input, &next))
	return next
}

func due(names ...string) []services.DueTransfer {
	base := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	out := make([]services.DueTransfer, len(names))
	for i, n := range names {
		out[i] = services.DueTransfer{DomainName: n, ID: int64(100 + i), ActionDate: base.Add(time.Duration(i) * time.Minute)}
	}
	return out
}

func (s *TransferApprovalLoopTestSuite) Test_NothingDue() {
	s.env.OnActivity("ListDueTransfers", mock.Anything, mock.Anything, mock.Anything, time.Time{}, int64(0), 1000).Return([]services.DueTransfer{}, nil)
	s.env.ExecuteWorkflow(TransferApprovalLoop, TransferApprovalLoopParams{})
	s.Require().NoError(s.env.GetWorkflowError())
	r := s.result()
	s.Equal(0, r.TotalFound)
	s.Contains(r.Notes, "No transfers due")
}

func (s *TransferApprovalLoopTestSuite) Test_ApprovesWhatIsDue_AsOfTheSweepsStart() {
	var listedBy, approvedBy time.Time
	s.env.OnActivity("ListDueTransfers", mock.Anything, mock.Anything, mock.Anything, time.Time{}, int64(0), 1000).
		Return(func(_ context.Context, _ string, dueBy, _ time.Time, _ int64, _ int) ([]services.DueTransfer, error) {
			listedBy = dueBy
			return due("a.test", "b.test", "c.test"), nil
		})
	s.env.OnActivity("BatchApproveDueTransfers", mock.Anything, mock.Anything, mock.Anything, []string{"a.test", "b.test", "c.test"}).
		Return(func(_ context.Context, _ string, dueBy time.Time, _ []string) (services.BatchResult, error) {
			approvedBy = dueBy
			return services.BatchResult{
				Succeeded: []string{"a.test"},
				Skipped:   []string{"b.test"},
				Failed:    []services.BatchFailure{{DomainName: "c.test", Error: "boom"}},
			}, nil
		})
	s.env.ExecuteWorkflow(TransferApprovalLoop, TransferApprovalLoopParams{})
	s.Require().NoError(s.env.GetWorkflowError())
	r := s.result()
	s.Equal(3, r.TotalFound)
	s.Equal(1, r.Approved)
	s.Equal(1, r.Skipped)
	s.Equal(1, r.Failed)
	s.Equal(3, r.TotalProcessed)
	s.Equal("c.test", r.Failures[0].DomainName)
	s.False(listedBy.IsZero())
	s.Equal(listedBy, approvedBy, "the batch re-checks against the same due time it listed by")
	s.Equal(r.StartedAt, r.DueBy)
}

func (s *TransferApprovalLoopTestSuite) Test_FullPage_ContinuesFromItsLastTransfer() {
	page := due("a.test", "b.test")
	s.env.OnActivity("ListDueTransfers", mock.Anything, mock.Anything, mock.Anything, time.Time{}, int64(0), 2).Return(page, nil)
	s.env.OnActivity("BatchApproveDueTransfers", mock.Anything, mock.Anything, mock.Anything, []string{"a.test", "b.test"}).
		Return(services.BatchResult{Succeeded: []string{"a.test", "b.test"}}, nil)
	s.env.ExecuteWorkflow(TransferApprovalLoop, TransferApprovalLoopParams{BatchSize: 2})
	next := s.continuedWith()
	s.Equal(1, next.ContinuationCount)
	s.Equal(page[1].ActionDate, next.AfterActionDate)
	s.Equal(page[1].ID, next.AfterID)
	s.False(next.DueBy.IsZero(), "the sweep keeps its due time")
}

// A page whose transfers all fail cannot hold back the ones after it: the
// sweep moves past it, and the next hourly sweep retries them.
func (s *TransferApprovalLoopTestSuite) Test_FullPageOfFailures_StillMovesOn() {
	page := due("poison1.test", "poison2.test")
	s.env.OnActivity("ListDueTransfers", mock.Anything, mock.Anything, mock.Anything, time.Time{}, int64(0), 2).Return(page, nil)
	s.env.OnActivity("BatchApproveDueTransfers", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(services.BatchResult{Failed: []services.BatchFailure{{DomainName: "poison1.test", Error: "x"}, {DomainName: "poison2.test", Error: "y"}}}, nil)
	s.env.ExecuteWorkflow(TransferApprovalLoop, TransferApprovalLoopParams{BatchSize: 2})
	next := s.continuedWith()
	s.Equal(page[1].ID, next.AfterID)
}

func (s *TransferApprovalLoopTestSuite) Test_ContinuedSweep_KeepsItsDueTimeAndCursor() {
	dueBy := time.Date(2026, 10, 22, 12, 0, 0, 0, time.UTC)
	after := time.Date(2026, 10, 1, 0, 1, 0, 0, time.UTC)
	s.env.OnActivity("ListDueTransfers", mock.Anything, mock.Anything, dueBy, after, int64(101), 2).Return(due("c.test"), nil)
	s.env.OnActivity("BatchApproveDueTransfers", mock.Anything, mock.Anything, dueBy, []string{"c.test"}).
		Return(services.BatchResult{Succeeded: []string{"c.test"}}, nil)
	s.env.ExecuteWorkflow(TransferApprovalLoop, TransferApprovalLoopParams{BatchSize: 2, ContinuationCount: 1, DueBy: dueBy, AfterActionDate: after, AfterID: 101})
	s.Require().NoError(s.env.GetWorkflowError(), "a short page ends the sweep")
	s.Equal(dueBy, s.result().DueBy)
}

func (s *TransferApprovalLoopTestSuite) Test_ContinuationCap() {
	page := due("a.test", "b.test")
	s.env.OnActivity("ListDueTransfers", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, 2).Return(page, nil)
	s.env.OnActivity("BatchApproveDueTransfers", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(services.BatchResult{Succeeded: []string{"a.test", "b.test"}}, nil)
	s.env.ExecuteWorkflow(TransferApprovalLoop, TransferApprovalLoopParams{BatchSize: 2, ContinuationCount: maxContinuationRuns})
	s.Require().NoError(s.env.GetWorkflowError(), "the cap ends the sweep; the next hourly run picks up the rest")
}

func (s *TransferApprovalLoopTestSuite) Test_ListFailure() {
	s.env.OnActivity("ListDueTransfers", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, 1000).Return(nil, fmt.Errorf("db down"))
	s.env.ExecuteWorkflow(TransferApprovalLoop, TransferApprovalLoopParams{})
	s.Require().Error(s.env.GetWorkflowError())
}

func TestTransferApprovalLoop(t *testing.T) {
	suite.Run(t, new(TransferApprovalLoopTestSuite))
}
