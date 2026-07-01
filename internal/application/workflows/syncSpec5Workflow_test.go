package workflows

import (
	"errors"
	"testing"

	"github.com/onasunnymorning/domain-os/internal/application/activities"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"go.temporal.io/sdk/testsuite"
)

type SyncSpec5WorkflowTestSuite struct {
	suite.Suite
	testsuite.WorkflowTestSuite
	env *testsuite.TestWorkflowEnvironment
}

func (s *SyncSpec5WorkflowTestSuite) SetupTest() {
	s.env = s.NewTestWorkflowEnvironment()
	s.env.RegisterWorkflow(SyncSpec5Workflow)
}

func (s *SyncSpec5WorkflowTestSuite) Test_SyncSpec5Workflow_Success() {
	s.env.OnActivity(activities.SyncSpec5, mock.Anything, mock.Anything, mock.Anything).Return(nil)

	s.env.ExecuteWorkflow(SyncSpec5Workflow)
	s.Require().True(s.env.IsWorkflowCompleted())
	s.Require().NoError(s.env.GetWorkflowError())
}

func (s *SyncSpec5WorkflowTestSuite) Test_SyncSpec5Workflow_Failure() {
	s.env.OnActivity(activities.SyncSpec5, mock.Anything, mock.Anything, mock.Anything).Return(errors.New("sync failed"))

	s.env.ExecuteWorkflow(SyncSpec5Workflow)
	s.Require().True(s.env.IsWorkflowCompleted())
	s.Require().Error(s.env.GetWorkflowError())
}

func TestSyncSpec5WorkflowTestSuite(t *testing.T) {
	suite.Run(t, new(SyncSpec5WorkflowTestSuite))
}
