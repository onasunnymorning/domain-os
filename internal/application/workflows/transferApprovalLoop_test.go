package workflows

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/testsuite"

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
		func(ctx context.Context, correlationID string, dueBy time.Time, limit int) ([]string, error) { return nil, nil },
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

func (s *TransferApprovalLoopTestSuite) Test_NothingDue() {
	s.env.OnActivity("ListDueTransfers", mock.Anything, mock.Anything, mock.Anything, 1000).Return([]string{}, nil)
	s.env.ExecuteWorkflow(TransferApprovalLoop, TransferApprovalLoopParams{})
	s.Require().NoError(s.env.GetWorkflowError())
	r := s.result()
	s.Equal(0, r.TotalFound)
	s.Contains(r.Notes, "No transfers due")
}

func (s *TransferApprovalLoopTestSuite) Test_ApprovesWhatIsDue_AsOfTheRunsStart() {
	var listedBy, approvedBy time.Time
	s.env.OnActivity("ListDueTransfers", mock.Anything, mock.Anything, mock.Anything, 1000).
		Return(func(_ context.Context, _ string, dueBy time.Time, _ int) ([]string, error) {
			listedBy = dueBy
			return []string{"a.test", "b.test", "c.test"}, nil
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

func (s *TransferApprovalLoopTestSuite) Test_FullPage_ContinuesAsNew() {
	page := []string{"a.test", "b.test"}
	s.env.OnActivity("ListDueTransfers", mock.Anything, mock.Anything, mock.Anything, 2).Return(page, nil)
	s.env.OnActivity("BatchApproveDueTransfers", mock.Anything, mock.Anything, mock.Anything, page).
		Return(services.BatchResult{Succeeded: page}, nil)
	s.env.ExecuteWorkflow(TransferApprovalLoop, TransferApprovalLoopParams{BatchSize: 2})
	s.Require().True(s.env.IsWorkflowCompleted())
	err := s.env.GetWorkflowError()
	s.Require().Error(err)
	s.Contains(err.Error(), "continue as new", "a full page drains with continue-as-new")
}

func (s *TransferApprovalLoopTestSuite) Test_FullPage_NoProgress_Stops() {
	page := []string{"a.test", "b.test"}
	s.env.OnActivity("ListDueTransfers", mock.Anything, mock.Anything, mock.Anything, 2).Return(page, nil)
	s.env.OnActivity("BatchApproveDueTransfers", mock.Anything, mock.Anything, mock.Anything, page).
		Return(services.BatchResult{Failed: []services.BatchFailure{{DomainName: "a.test", Error: "x"}, {DomainName: "b.test", Error: "y"}}}, nil)
	s.env.ExecuteWorkflow(TransferApprovalLoop, TransferApprovalLoopParams{BatchSize: 2})
	s.Require().NoError(s.env.GetWorkflowError(), "no hot loop on a poison page")
	s.Equal(2, s.result().Failed)
}

func (s *TransferApprovalLoopTestSuite) Test_ListFailure() {
	s.env.OnActivity("ListDueTransfers", mock.Anything, mock.Anything, mock.Anything, 1000).Return(nil, fmt.Errorf("db down"))
	s.env.ExecuteWorkflow(TransferApprovalLoop, TransferApprovalLoopParams{})
	s.Require().Error(s.env.GetWorkflowError())
}

func TestTransferApprovalLoop(t *testing.T) {
	suite.Run(t, new(TransferApprovalLoopTestSuite))
}
