package workflows

import (
	"errors"
	"testing"

	"github.com/onasunnymorning/domain-os/internal/application/activities"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"go.temporal.io/sdk/testsuite"
)

type UpdateFXWorkflowTestSuite struct {
	suite.Suite
	testsuite.WorkflowTestSuite
	env *testsuite.TestWorkflowEnvironment
}

func (s *UpdateFXWorkflowTestSuite) SetupTest() {
	s.env = s.NewTestWorkflowEnvironment()
	s.env.RegisterWorkflow(UpdateFX)

	// Register the REAL activity struct (with nil deps — every call is mocked
	// below) so the workflow's string-based "UpdateFXRates" reference is
	// verified against a method that actually exists.
	s.env.RegisterActivity(activities.NewFXActivitiesWithDeps(nil, nil, nil))
}

func (s *UpdateFXWorkflowTestSuite) Test_UpdateFX_Success() {
	s.env.OnActivity("UpdateFXRates", mock.Anything, mock.Anything, []string(nil)).Return(activities.UpdateFXRatesResult{
		BasesUpdated:      []string{"USD", "PEN"},
		RatesStored:       330,
		DerivedFromPhases: true,
	}, nil)

	s.env.ExecuteWorkflow(UpdateFX, UpdateFXParams{})
	s.Require().True(s.env.IsWorkflowCompleted())
	s.Require().NoError(s.env.GetWorkflowError())

	var result UpdateFXResult
	s.Require().NoError(s.env.GetWorkflowResult(&result))
	s.Equal([]string{"USD", "PEN"}, result.BasesUpdated)
	s.Equal(330, result.RatesStored)
	s.Equal(0, result.Failed)
	s.True(result.DerivedFromPhases)
	s.Contains(result.Notes, "Stored 330 rates across 2 base currencies")
}

func (s *UpdateFXWorkflowTestSuite) Test_UpdateFX_ExplicitBases_PassedThrough() {
	s.env.OnActivity("UpdateFXRates", mock.Anything, mock.Anything, []string{"EUR"}).Return(activities.UpdateFXRatesResult{
		BasesUpdated: []string{"EUR"},
		RatesStored:  165,
	}, nil)

	s.env.ExecuteWorkflow(UpdateFX, UpdateFXParams{BaseCurrencies: []string{"EUR"}})
	s.Require().True(s.env.IsWorkflowCompleted())
	s.Require().NoError(s.env.GetWorkflowError())

	var result UpdateFXResult
	s.Require().NoError(s.env.GetWorkflowResult(&result))
	s.Equal([]string{"EUR"}, result.BasesUpdated)
	s.False(result.DerivedFromPhases)
}

func (s *UpdateFXWorkflowTestSuite) Test_UpdateFX_PartialFailure_CompletesWithNotes() {
	s.env.OnActivity("UpdateFXRates", mock.Anything, mock.Anything, []string(nil)).Return(activities.UpdateFXRatesResult{
		BasesUpdated: []string{"USD"},
		RatesStored:  165,
		Failures: []activities.FXBaseFailure{
			{BaseCurrency: "XXX", Error: "404 not found"},
		},
	}, nil)

	s.env.ExecuteWorkflow(UpdateFX, UpdateFXParams{})
	s.Require().True(s.env.IsWorkflowCompleted())
	s.Require().NoError(s.env.GetWorkflowError())

	var result UpdateFXResult
	s.Require().NoError(s.env.GetWorkflowResult(&result))
	s.Equal(1, result.Failed)
	s.Require().Len(result.Failures, 1)
	s.Equal("XXX", result.Failures[0].BaseCurrency)
}

func (s *UpdateFXWorkflowTestSuite) Test_UpdateFX_ActivityError_FailsWorkflow() {
	s.env.OnActivity("UpdateFXRates", mock.Anything, mock.Anything, []string(nil)).Return(
		activities.UpdateFXRatesResult{}, errors.New("all base currencies failed"))

	s.env.ExecuteWorkflow(UpdateFX, UpdateFXParams{})
	s.Require().True(s.env.IsWorkflowCompleted())
	s.Require().Error(s.env.GetWorkflowError())
}

func TestUpdateFXWorkflowTestSuite(t *testing.T) {
	suite.Run(t, new(UpdateFXWorkflowTestSuite))
}
