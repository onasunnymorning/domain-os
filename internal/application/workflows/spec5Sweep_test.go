package workflows

import (
	"errors"
	"testing"

	"github.com/onasunnymorning/domain-os/internal/application/activities"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"go.temporal.io/sdk/testsuite"
)

type Spec5SweepWorkflowTestSuite struct {
	suite.Suite
	testsuite.WorkflowTestSuite
	env *testsuite.TestWorkflowEnvironment
}

func (s *Spec5SweepWorkflowTestSuite) SetupTest() {
	s.env = s.NewTestWorkflowEnvironment()
	s.env.RegisterWorkflow(Spec5SweepWorkflow)
}

func (s *Spec5SweepWorkflowTestSuite) Test_Spec5SweepWorkflow_Success() {
	var acts *activities.Spec5SweepActivities
	mockResult := activities.Spec5SweepResult{
		TLDResults: map[string]activities.Spec5SweepTLDResult{
			"com": {
				Count:       1,
				ArtifactKey: "spec5-sweep/wf-123/com-matching-spec5.csv",
				DownloadURL: "http://mock-s3-presigned-url/",
			},
		},
	}
	s.env.OnActivity(acts.SweepSpec5Labels, mock.Anything, mock.Anything).Return(mockResult, nil)

	params := Spec5SweepParams{
		TLDs: []string{"com"},
	}

	s.env.ExecuteWorkflow(Spec5SweepWorkflow, params)
	s.Require().True(s.env.IsWorkflowCompleted())
	s.Require().NoError(s.env.GetWorkflowError())

	var result activities.Spec5SweepResult
	err := s.env.GetWorkflowResult(&result)
	s.Require().NoError(err)
	s.Equal(int64(1), result.TLDResults["com"].Count)
}

func (s *Spec5SweepWorkflowTestSuite) Test_Spec5SweepWorkflow_Failure() {
	var acts *activities.Spec5SweepActivities
	s.env.OnActivity(acts.SweepSpec5Labels, mock.Anything, mock.Anything).Return(activities.Spec5SweepResult{}, errors.New("activity failed"))

	params := Spec5SweepParams{
		TLDs: []string{"com"},
	}

	s.env.ExecuteWorkflow(Spec5SweepWorkflow, params)
	s.Require().True(s.env.IsWorkflowCompleted())
	s.Require().Error(s.env.GetWorkflowError())
}

func TestSpec5SweepWorkflowTestSuite(t *testing.T) {
	suite.Run(t, new(Spec5SweepWorkflowTestSuite))
}
