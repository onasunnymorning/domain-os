package workflows

import (
	"context"
	"fmt"
	"testing"

	"github.com/onasunnymorning/domain-os/internal/application/activities"
	"github.com/onasunnymorning/domain-os/internal/application/services"
	"github.com/onasunnymorning/domain-os/internal/interface/rest/response"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/testsuite"
)

type RestoreWorkflowTestSuite struct {
	suite.Suite
	testsuite.WorkflowTestSuite
	env *testsuite.TestWorkflowEnvironment
}

func (s *RestoreWorkflowTestSuite) SetupTest() {
	s.env = s.NewTestWorkflowEnvironment()
	s.env.RegisterWorkflow(RestoreWorkflow)

	// Register stub function for string-based batch activity so the test env can resolve it
	s.env.RegisterActivityWithOptions(
		func(ctx context.Context, correlationID string, domainNames []string) (services.BatchResult, error) {
			return services.BatchResult{}, nil
		},
		activity.RegisterOptions{Name: "BatchRestoreDomains"},
	)
}

func (s *RestoreWorkflowTestSuite) Test_RestoreWorkflow_NoDomains() {
	s.env.OnActivity(activities.ListRestoredDomains, mock.Anything, mock.Anything, mock.Anything).
		Return([]response.DomainRestoredItem{}, nil)

	s.env.ExecuteWorkflow(RestoreWorkflow)
	s.Require().True(s.env.IsWorkflowCompleted())
	s.Require().NoError(s.env.GetWorkflowError())

	var result RestoreLoopResult
	s.Require().NoError(s.env.GetWorkflowResult(&result))
	s.Equal(0, result.TotalFound)
	s.Contains(result.Notes, "No restored domains found")
}

func (s *RestoreWorkflowTestSuite) Test_RestoreWorkflow_Success() {
	domains := []response.DomainRestoredItem{
		{Name: "restored1.com", RoID: "ro1", ClID: "reg1"},
		{Name: "restored2.com", RoID: "ro2", ClID: "reg2"},
	}
	s.env.OnActivity(activities.ListRestoredDomains, mock.Anything, mock.Anything, mock.Anything).
		Return(domains, nil)

	s.env.OnActivity("BatchRestoreDomains", mock.Anything, mock.Anything, mock.Anything).
		Return(services.BatchResult{
			Succeeded: []string{"restored1.com", "restored2.com"},
			Failed:    []services.BatchFailure{},
		}, nil)

	s.env.ExecuteWorkflow(RestoreWorkflow)
	s.Require().True(s.env.IsWorkflowCompleted())
	s.Require().NoError(s.env.GetWorkflowError())

	var result RestoreLoopResult
	s.Require().NoError(s.env.GetWorkflowResult(&result))
	s.Equal(2, result.TotalFound)
	s.Equal(2, result.Restored)
	s.Equal(0, result.Failed)
	s.Equal(2, result.TotalProcessed)
	s.Empty(result.Failures)
}

func (s *RestoreWorkflowTestSuite) Test_RestoreWorkflow_PartialFailure() {
	domains := []response.DomainRestoredItem{
		{Name: "ok.com", RoID: "ro1", ClID: "reg1"},
		{Name: "fail.com", RoID: "ro2", ClID: "reg2"},
	}
	s.env.OnActivity(activities.ListRestoredDomains, mock.Anything, mock.Anything, mock.Anything).
		Return(domains, nil)

	s.env.OnActivity("BatchRestoreDomains", mock.Anything, mock.Anything, mock.Anything).
		Return(services.BatchResult{
			Succeeded: []string{"ok.com"},
			Failed: []services.BatchFailure{
				{DomainName: "fail.com", Error: "restore failed: grace period expired"},
			},
		}, nil)

	s.env.ExecuteWorkflow(RestoreWorkflow)
	s.Require().True(s.env.IsWorkflowCompleted())
	s.Require().NoError(s.env.GetWorkflowError())

	var result RestoreLoopResult
	s.Require().NoError(s.env.GetWorkflowResult(&result))
	s.Equal(2, result.TotalFound)
	s.Equal(1, result.Restored)
	s.Equal(1, result.Failed)
	s.Equal(2, result.TotalProcessed)
	s.Require().Len(result.Failures, 1)
	s.Equal("fail.com", result.Failures[0].DomainName)
	s.Contains(result.Failures[0].Error, "grace period expired")
	s.Contains(result.Notes, "Completed with failures — review the failures list for details")
}

func (s *RestoreWorkflowTestSuite) Test_RestoreWorkflow_ListError() {
	s.env.OnActivity(activities.ListRestoredDomains, mock.Anything, mock.Anything, mock.Anything).
		Return(nil, fmt.Errorf("API unavailable: connection refused"))

	s.env.ExecuteWorkflow(RestoreWorkflow)
	s.Require().True(s.env.IsWorkflowCompleted())
	s.Require().Error(s.env.GetWorkflowError())
}

func TestRestoreWorkflowTestSuite(t *testing.T) {
	suite.Run(t, new(RestoreWorkflowTestSuite))
}
