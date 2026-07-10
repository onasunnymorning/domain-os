package workflows

import (
	"context"
	"testing"

	"github.com/onasunnymorning/domain-os/internal/application/activities"
	"github.com/onasunnymorning/domain-os/internal/application/services"
	"github.com/onasunnymorning/domain-os/internal/interface/rest/response"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/testsuite"
)

type ExpiryLoopWorkflowTestSuite struct {
	suite.Suite
	testsuite.WorkflowTestSuite
	env *testsuite.TestWorkflowEnvironment
}

func (s *ExpiryLoopWorkflowTestSuite) SetupTest() {
	s.env = s.NewTestWorkflowEnvironment()
	s.env.RegisterWorkflow(ExpiryLoop)

	// Register stub functions for string-based batch activities so the test env can resolve them
	s.env.RegisterActivityWithOptions(
		func(ctx context.Context, correlationID string, domainNames []string, years int) (services.BatchResult, error) {
			return services.BatchResult{}, nil
		},
		activity.RegisterOptions{Name: "BatchAutoRenewDomains"},
	)
	s.env.RegisterActivityWithOptions(
		func(ctx context.Context, correlationID string, domainNames []string) (services.BatchResult, error) {
			return services.BatchResult{}, nil
		},
		activity.RegisterOptions{Name: "BatchExpireDomains"},
	)
}

func (s *ExpiryLoopWorkflowTestSuite) Test_ExpiryLoop_NoDomains() {
	s.env.OnActivity(activities.GetExpiredDomainCount, mock.Anything, mock.Anything, mock.Anything).Return(&response.CountResult{Count: 0}, nil)

	s.env.ExecuteWorkflow(ExpiryLoop, ExpiryLoopParams{})
	s.Require().True(s.env.IsWorkflowCompleted())
	s.Require().NoError(s.env.GetWorkflowError())

	var result ExpiryLoopResult
	s.Require().NoError(s.env.GetWorkflowResult(&result))
	s.Equal(int64(0), result.TotalFound)
	s.Contains(result.Notes, "No expired domains found")
}

func (s *ExpiryLoopWorkflowTestSuite) Test_ExpiryLoop_Success_Mixed() {
	// 4 domains found
	s.env.OnActivity(activities.GetExpiredDomainCount, mock.Anything, mock.Anything, mock.Anything).Return(&response.CountResult{Count: 4}, nil)

	domains := []response.DomainExpiryItem{
		{Name: "renew1.com"},
		{Name: "renew2.com"},
		{Name: "expire1.com"},
		{Name: "expire2.com"},
	}
	s.env.OnActivity(activities.ListExpiringDomains, mock.Anything, mock.Anything, mock.Anything).Return(domains, nil)

	batchCheckRes := activities.CheckDomainsCanAutoRenewResult{
		EligibleForAutoRenew: []string{"renew1.com", "renew2.com"},
		EligibleForExpiry:    []string{"expire1.com", "expire2.com"},
		CheckFailures: []activities.CheckFailure{
			{DomainName: "failcheck.com", Error: "some check error"},
		},
	}
	s.env.OnActivity(activities.CheckDomainsCanAutoRenew, mock.Anything, mock.Anything, []string{"renew1.com", "renew2.com", "expire1.com", "expire2.com"}).Return(batchCheckRes, nil)

	// Batch auto-renew: renew2.com fails
	s.env.OnActivity("BatchAutoRenewDomains", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(services.BatchResult{
		Succeeded: []string{"renew1.com"},
		Failed: []services.BatchFailure{
			{DomainName: "renew2.com", Error: "renew fail"},
		},
	}, nil)

	// Batch expire: expire2.com fails
	s.env.OnActivity("BatchExpireDomains", mock.Anything, mock.Anything, mock.Anything).Return(services.BatchResult{
		Succeeded: []string{"expire1.com"},
		Failed: []services.BatchFailure{
			{DomainName: "expire2.com", Error: "expire fail"},
		},
	}, nil)

	s.env.ExecuteWorkflow(ExpiryLoop, ExpiryLoopParams{ConcurrencyLimit: 2})
	s.Require().True(s.env.IsWorkflowCompleted())
	s.Require().NoError(s.env.GetWorkflowError())

	var result ExpiryLoopResult
	s.Require().NoError(s.env.GetWorkflowResult(&result))
	s.Equal(int64(4), result.TotalFound)
	s.Equal(5, result.TotalProcessed) // 2 auto-renew batch (1 ok + 1 fail) + 2 expire batch (1 ok + 1 fail) + 1 check fail = 5
	s.Equal(1, result.AutoRenewed)
	s.Equal(1, result.Expired)
	s.Equal(3, result.Failed) // 1 check fail + 1 renew fail + 1 expire fail = 3

	// Validate failure details
	failures := make(map[string]ExpiryLoopFailure)
	for _, f := range result.Failures {
		failures[f.DomainName] = f
	}
	s.Equal("some check error", failures["failcheck.com"].Error)
	s.Equal("auto-renew-check", failures["failcheck.com"].Operation)
	s.Contains(failures["renew2.com"].Error, "renew fail")
	s.Equal("auto-renew", failures["renew2.com"].Operation)
	s.Contains(failures["expire2.com"].Error, "expire fail")
	s.Equal("expire", failures["expire2.com"].Operation)
}

func (s *ExpiryLoopWorkflowTestSuite) Test_ExpiryLoop_DryRun() {
	s.env.OnActivity(activities.GetExpiredDomainCount, mock.Anything, mock.Anything, mock.Anything).Return(&response.CountResult{Count: 2}, nil)

	domains := []response.DomainExpiryItem{
		{Name: "domain1.com"},
		{Name: "domain2.com"},
	}
	s.env.OnActivity(activities.ListExpiringDomains, mock.Anything, mock.Anything, mock.Anything).Return(domains, nil)

	batchCheckRes := activities.CheckDomainsCanAutoRenewResult{
		EligibleForAutoRenew: []string{"domain1.com"},
		EligibleForExpiry:    []string{"domain2.com"},
	}
	s.env.OnActivity(activities.CheckDomainsCanAutoRenew, mock.Anything, mock.Anything, []string{"domain1.com", "domain2.com"}).Return(batchCheckRes, nil)

	// In dry run, write activities must NOT be called. If they are, it will panic because they are not mocked.

	s.env.ExecuteWorkflow(ExpiryLoop, ExpiryLoopParams{DryRun: true})
	s.Require().True(s.env.IsWorkflowCompleted())
	s.Require().NoError(s.env.GetWorkflowError())

	var result ExpiryLoopResult
	s.Require().NoError(s.env.GetWorkflowResult(&result))
	s.Equal(int64(2), result.TotalFound)
	s.Equal(2, result.TotalProcessed)
	s.Equal(1, result.AutoRenewed)
	s.Equal(1, result.Expired)
	s.Equal(0, result.Failed)
	s.Contains(result.Notes, "Dry run completed: no state changes made")
}

func (s *ExpiryLoopWorkflowTestSuite) Test_ExpiryLoop_ContinueAsNew() {
	// Count reports 100 domains but list only returns 2 (batch cap hit)
	s.env.OnActivity(activities.GetExpiredDomainCount, mock.Anything, mock.Anything, mock.Anything).Return(&response.CountResult{Count: 100}, nil)

	domains := []response.DomainExpiryItem{
		{Name: "domain1.com"},
		{Name: "domain2.com"},
	}
	s.env.OnActivity(activities.ListExpiringDomains, mock.Anything, mock.Anything, mock.Anything).Return(domains, nil)

	batchCheckRes := activities.CheckDomainsCanAutoRenewResult{
		EligibleForAutoRenew: []string{},
		EligibleForExpiry:    []string{"domain1.com", "domain2.com"},
	}
	s.env.OnActivity(activities.CheckDomainsCanAutoRenew, mock.Anything, mock.Anything, []string{"domain1.com", "domain2.com"}).Return(batchCheckRes, nil)

	s.env.OnActivity("BatchExpireDomains", mock.Anything, mock.Anything, mock.Anything).Return(services.BatchResult{
		Succeeded: []string{"domain1.com", "domain2.com"},
		Failed:    []services.BatchFailure{},
	}, nil)

	s.env.ExecuteWorkflow(ExpiryLoop, ExpiryLoopParams{})
	s.Require().True(s.env.IsWorkflowCompleted())

	// ContinueAsNew surfaces as a ContinueAsNewError — Temporal test framework treats it as a workflow error
	err := s.env.GetWorkflowError()
	s.Require().Error(err)
	s.Contains(err.Error(), "continue as new")
}

func TestExpiryLoopWorkflowTestSuite(t *testing.T) {
	suite.Run(t, new(ExpiryLoopWorkflowTestSuite))
}
