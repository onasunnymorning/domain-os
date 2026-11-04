package workflows

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"go.temporal.io/sdk/testsuite"

	"github.com/onasunnymorning/domain-os/internal/application/activities"
	"github.com/onasunnymorning/domain-os/pkg/domain/queries"
)

type RefreshTMCHWorkflowTestSuite struct {
	suite.Suite
	testsuite.WorkflowTestSuite
	env *testsuite.TestWorkflowEnvironment
}

func (s *RefreshTMCHWorkflowTestSuite) SetupTest() {
	s.env = s.NewTestWorkflowEnvironment()
	s.env.RegisterWorkflow(RefreshTMCH)
	// The real activity struct, so the string name the workflow uses is
	// checked against a method that exists; every call is mocked.
	s.env.RegisterActivity(activities.NewTMCHActivitiesWithService(nil))
}

func (s *RefreshTMCHWorkflowTestSuite) Test_RefreshTMCH_Success() {
	s.env.OnActivity("RefreshTMCHData", mock.Anything, mock.Anything).Return(queries.TMCHRefreshResult{Sets: []queries.TMCHSetRefresh{
		{Set: "certificates", Outcome: queries.TMCHUnchanged},
		{Set: "smdrl", Outcome: queries.TMCHRefreshed},
		{Set: "dnl", Outcome: queries.TMCHRefreshed},
	}}, nil)

	s.env.ExecuteWorkflow(RefreshTMCH, RefreshTMCHParams{})
	s.Require().True(s.env.IsWorkflowCompleted())
	s.Require().NoError(s.env.GetWorkflowError())

	var result RefreshTMCHResult
	s.Require().NoError(s.env.GetWorkflowResult(&result))
	s.Equal(2, result.Refreshed)
	s.Equal(0, result.Failed)
	s.Len(result.Sets, 3)
	s.Contains(result.Notes, "2 of 3 sets refreshed")
}

func (s *RefreshTMCHWorkflowTestSuite) Test_RefreshTMCH_NotConfigured() {
	s.env.OnActivity("RefreshTMCHData", mock.Anything, mock.Anything).Return(queries.TMCHRefreshResult{}, nil)

	s.env.ExecuteWorkflow(RefreshTMCH, RefreshTMCHParams{})
	s.Require().True(s.env.IsWorkflowCompleted())
	s.Require().NoError(s.env.GetWorkflowError())

	var result RefreshTMCHResult
	s.Require().NoError(s.env.GetWorkflowResult(&result))
	s.Equal(0, result.Refreshed)
	s.Contains(result.Notes, "No TMCH feed is configured for this installation; nothing to refresh")
}

func (s *RefreshTMCHWorkflowTestSuite) Test_RefreshTMCH_ActivityFails() {
	s.env.OnActivity("RefreshTMCHData", mock.Anything, mock.Anything).Return(queries.TMCHRefreshResult{}, errors.New("dnl: 401 Unauthorized"))

	s.env.ExecuteWorkflow(RefreshTMCH, RefreshTMCHParams{})
	s.Require().True(s.env.IsWorkflowCompleted())
	err := s.env.GetWorkflowError()
	s.Require().Error(err)
	s.Contains(err.Error(), "401 Unauthorized")
}

func TestRefreshTMCHWorkflowTestSuite(t *testing.T) {
	suite.Run(t, new(RefreshTMCHWorkflowTestSuite))
}
