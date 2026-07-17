package workflows

import (
	"context"
	"strings"
	"testing"

	"github.com/onasunnymorning/domain-os/internal/application/activities"
	"github.com/onasunnymorning/domain-os/internal/application/services"
	"github.com/onasunnymorning/domain-os/internal/interface/rest/response"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"go.temporal.io/sdk/activity"
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

	// Register stub function for string-based batch activity so the test env can resolve it
	s.env.RegisterActivityWithOptions(
		func(ctx context.Context, correlationID string, domainNames []string) (services.BatchResult, error) {
			return services.BatchResult{}, nil
		},
		activity.RegisterOptions{Name: "BatchPurgeDomains"},
	)
}

func (s *PurgeLoopWorkflowTestSuite) Test_PurgeLoop_NoDomains() {
	s.env.OnActivity(activities.GetPurgeableDomainCount, mock.Anything, mock.Anything, mock.Anything).Return(&response.CountResult{Count: 0}, nil)

	s.env.ExecuteWorkflow(PurgeLoop, PurgeLoopParams{})
	s.Require().True(s.env.IsWorkflowCompleted())
	s.Require().NoError(s.env.GetWorkflowError())

	var result PurgeLoopResult
	s.Require().NoError(s.env.GetWorkflowResult(&result))
	s.Equal(int64(0), result.TotalFound)
	s.Contains(result.Notes, "No purgeable domains found")
}

func (s *PurgeLoopWorkflowTestSuite) Test_PurgeLoop_Success_Mixed() {
	s.env.OnActivity(activities.GetPurgeableDomainCount, mock.Anything, mock.Anything, mock.Anything).Return(&response.CountResult{Count: 3}, nil)

	domains := []response.DomainExpiryItem{
		{Name: "purge1.com"},
		{Name: "purge2.com"},
		{Name: "purge3.com"},
	}
	s.env.OnActivity(activities.ListPurgeableDomains, mock.Anything, mock.Anything, mock.Anything).Return(domains, nil)

	// Batch purge: purge2.com fails
	s.env.OnActivity("BatchPurgeDomains", mock.Anything, mock.Anything, mock.Anything).Return(services.BatchResult{
		Succeeded: []string{"purge1.com", "purge3.com"},
		Failed: []services.BatchFailure{
			{DomainName: "purge2.com", Error: "purge fail"},
		},
	}, nil)

	s.env.ExecuteWorkflow(PurgeLoop, PurgeLoopParams{})
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
	s.env.OnActivity(activities.GetPurgeableDomainCount, mock.Anything, mock.Anything, mock.Anything).Return(&response.CountResult{Count: 2}, nil)

	domains := []response.DomainExpiryItem{
		{Name: "domain1.com"},
		{Name: "domain2.com"},
	}
	s.env.OnActivity(activities.ListPurgeableDomains, mock.Anything, mock.Anything, mock.Anything).Return(domains, nil)

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

func (s *PurgeLoopWorkflowTestSuite) Test_PurgeLoop_SkippedAreCounted() {
	s.env.OnActivity(activities.GetPurgeableDomainCount, mock.Anything, mock.Anything, mock.Anything).Return(&response.CountResult{Count: 2}, nil)

	domains := []response.DomainExpiryItem{
		{Name: "purge1.com"},
		{Name: "alreadygone.com"},
	}
	s.env.OnActivity(activities.ListPurgeableDomains, mock.Anything, mock.Anything, mock.Anything).Return(domains, nil)

	// alreadygone.com was purged by a previous (retried) attempt
	s.env.OnActivity("BatchPurgeDomains", mock.Anything, mock.Anything, mock.Anything).Return(services.BatchResult{
		Succeeded: []string{"purge1.com"},
		Skipped:   []string{"alreadygone.com"},
	}, nil)

	s.env.ExecuteWorkflow(PurgeLoop, PurgeLoopParams{})
	s.Require().True(s.env.IsWorkflowCompleted())
	s.Require().NoError(s.env.GetWorkflowError())

	var result PurgeLoopResult
	s.Require().NoError(s.env.GetWorkflowResult(&result))
	s.Equal(1, result.Purged)
	s.Equal(1, result.Skipped)
	s.Equal(0, result.Failed)
	s.Equal(2, result.TotalProcessed)
}

func (s *PurgeLoopWorkflowTestSuite) Test_PurgeLoop_NoProgress_DoesNotContinueAsNew() {
	// Batch cap hit but every purge fails — the workflow must complete
	// instead of continuing-as-new into a hot loop.
	s.env.OnActivity(activities.GetPurgeableDomainCount, mock.Anything, mock.Anything, mock.Anything).Return(&response.CountResult{Count: 100}, nil)

	domains := []response.DomainExpiryItem{
		{Name: "poison1.com"},
	}
	s.env.OnActivity(activities.ListPurgeableDomains, mock.Anything, mock.Anything, mock.Anything).Return(domains, nil)

	s.env.OnActivity("BatchPurgeDomains", mock.Anything, mock.Anything, mock.Anything).Return(services.BatchResult{
		Succeeded: []string{},
		Failed: []services.BatchFailure{
			{DomainName: "poison1.com", Error: "boom"},
		},
	}, nil)

	s.env.ExecuteWorkflow(PurgeLoop, PurgeLoopParams{})
	s.Require().True(s.env.IsWorkflowCompleted())
	s.Require().NoError(s.env.GetWorkflowError())

	var result PurgeLoopResult
	s.Require().NoError(s.env.GetWorkflowResult(&result))
	s.Equal(1, result.Failed)
	foundNote := false
	for _, n := range result.Notes {
		if strings.HasPrefix(n, "Batch cap reached but no progress was made") {
			foundNote = true
		}
	}
	s.True(foundNote, "expected the no-progress note, got: %v", result.Notes)
}

func (s *PurgeLoopWorkflowTestSuite) Test_PurgeLoop_ContinuationCap_StopsChain() {
	s.env.OnActivity(activities.GetPurgeableDomainCount, mock.Anything, mock.Anything, mock.Anything).Return(&response.CountResult{Count: 100}, nil)

	domains := []response.DomainExpiryItem{
		{Name: "domain1.com"},
	}
	s.env.OnActivity(activities.ListPurgeableDomains, mock.Anything, mock.Anything, mock.Anything).Return(domains, nil)

	s.env.OnActivity("BatchPurgeDomains", mock.Anything, mock.Anything, mock.Anything).Return(services.BatchResult{
		Succeeded: []string{"domain1.com"},
	}, nil)

	s.env.ExecuteWorkflow(PurgeLoop, PurgeLoopParams{ContinuationCount: maxContinuationRuns})
	s.Require().True(s.env.IsWorkflowCompleted())
	s.Require().NoError(s.env.GetWorkflowError())

	var result PurgeLoopResult
	s.Require().NoError(s.env.GetWorkflowResult(&result))
	s.Equal(1, result.Purged)
	foundNote := false
	for _, n := range result.Notes {
		if strings.HasPrefix(n, "Continuation cap reached") {
			foundNote = true
		}
	}
	s.True(foundNote, "expected the continuation-cap note, got: %v", result.Notes)
}

func (s *PurgeLoopWorkflowTestSuite) Test_PurgeLoop_ContinueAsNew() {
	// Count reports 100 domains but list only returns 2 (batch cap hit)
	s.env.OnActivity(activities.GetPurgeableDomainCount, mock.Anything, mock.Anything, mock.Anything).Return(&response.CountResult{Count: 100}, nil)

	domains := []response.DomainExpiryItem{
		{Name: "domain1.com"},
		{Name: "domain2.com"},
	}
	s.env.OnActivity(activities.ListPurgeableDomains, mock.Anything, mock.Anything, mock.Anything).Return(domains, nil)

	s.env.OnActivity("BatchPurgeDomains", mock.Anything, mock.Anything, mock.Anything).Return(services.BatchResult{
		Succeeded: []string{"domain1.com", "domain2.com"},
		Failed:    []services.BatchFailure{},
	}, nil)

	s.env.ExecuteWorkflow(PurgeLoop, PurgeLoopParams{})
	s.Require().True(s.env.IsWorkflowCompleted())

	// ContinueAsNew surfaces as a ContinueAsNewError — Temporal test framework treats it as a workflow error
	err := s.env.GetWorkflowError()
	s.Require().Error(err)
	s.Contains(err.Error(), "continue as new")
}

func TestPurgeLoopWorkflowTestSuite(t *testing.T) {
	suite.Run(t, new(PurgeLoopWorkflowTestSuite))
}
