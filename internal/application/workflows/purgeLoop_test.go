package workflows

import (
	"fmt"
	"testing"

	"github.com/onasunnymorning/domain-os/internal/application/activities"
	"github.com/onasunnymorning/domain-os/internal/interface/rest/response"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"go.temporal.io/sdk/testsuite"
)

type PurgeLoopWorkflowTestSuite struct {
	suite.Suite
	testsuite.WorkflowTestSuite
	env *testsuite.TestWorkflowEnvironment
}

func (s *PurgeLoopWorkflowTestSuite) SetupTest() {
	s.env = s.NewTestWorkflowEnvironment()
	s.env.RegisterWorkflow(PurgeLoop)
}

func (s *PurgeLoopWorkflowTestSuite) Test_PurgeLoop_NoDomains() {
	s.env.OnActivity(activities.GetPurgeableDomainCount, mock.Anything, mock.Anything).Return(&response.CountResult{Count: 0}, nil)

	s.env.ExecuteWorkflow(PurgeLoop, PurgeLoopParams{})
	s.Require().True(s.env.IsWorkflowCompleted())
	s.Require().NoError(s.env.GetWorkflowError())

	var result PurgeLoopResult
	s.Require().NoError(s.env.GetWorkflowResult(&result))
	s.Equal(int64(0), result.TotalFound)
	s.Contains(result.Notes, "No purgeable domains found")
}

func (s *PurgeLoopWorkflowTestSuite) Test_PurgeLoop_Success_Mixed() {
	s.env.OnActivity(activities.GetPurgeableDomainCount, mock.Anything, mock.Anything).Return(&response.CountResult{Count: 3}, nil)

	domains := []response.DomainExpiryItem{
		{Name: "purge1.com"},
		{Name: "purge2.com"},
		{Name: "purge3.com"},
	}
	s.env.OnActivity(activities.ListPurgeableDomains, mock.Anything, mock.Anything).Return(domains, nil)

	s.env.OnActivity(activities.PurgeDomain, mock.Anything, "purge1.com").Return(nil)
	s.env.OnActivity(activities.PurgeDomain, mock.Anything, "purge2.com").Return(fmt.Errorf("purge fail"))
	s.env.OnActivity(activities.PurgeDomain, mock.Anything, "purge3.com").Return(nil)

	s.env.ExecuteWorkflow(PurgeLoop, PurgeLoopParams{ConcurrencyLimit: 2})
	s.Require().True(s.env.IsWorkflowCompleted())
	s.Require().NoError(s.env.GetWorkflowError())

	var result PurgeLoopResult
	s.Require().NoError(s.env.GetWorkflowResult(&result))
	s.Equal(int64(3), result.TotalFound)
	s.Equal(3, result.TotalProcessed)
	s.Equal(2, result.Purged)
	s.Equal(1, result.Failed)

	failures := make(map[string]PurgeLoopFailure)
	for _, f := range result.Failures {
		failures[f.DomainName] = f
	}
	s.Contains(failures["purge2.com"].Error, "purge fail")
}

func (s *PurgeLoopWorkflowTestSuite) Test_PurgeLoop_DryRun() {
	s.env.OnActivity(activities.GetPurgeableDomainCount, mock.Anything, mock.Anything).Return(&response.CountResult{Count: 2}, nil)

	domains := []response.DomainExpiryItem{
		{Name: "domain1.com"},
		{Name: "domain2.com"},
	}
	s.env.OnActivity(activities.ListPurgeableDomains, mock.Anything, mock.Anything).Return(domains, nil)

	s.env.ExecuteWorkflow(PurgeLoop, PurgeLoopParams{DryRun: true})
	s.Require().True(s.env.IsWorkflowCompleted())
	s.Require().NoError(s.env.GetWorkflowError())

	var result PurgeLoopResult
	s.Require().NoError(s.env.GetWorkflowResult(&result))
	s.Equal(int64(2), result.TotalFound)
	s.Equal(2, result.TotalProcessed)
	s.Equal(2, result.Purged)
	s.Equal(0, result.Failed)
	s.Contains(result.Notes, "Dry run completed: no state changes made")
}

func TestPurgeLoopWorkflowTestSuite(t *testing.T) {
	suite.Run(t, new(PurgeLoopWorkflowTestSuite))
}
