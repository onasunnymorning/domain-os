package workflows

import (
	"testing"

	"github.com/onasunnymorning/domain-os/internal/application/activities"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/testsuite"
)

type EscrowKeyProbeWorkflowTestSuite struct {
	suite.Suite
	testsuite.WorkflowTestSuite
	env *testsuite.TestWorkflowEnvironment
}

func TestEscrowKeyProbeWorkflowTestSuite(t *testing.T) {
	suite.Run(t, new(EscrowKeyProbeWorkflowTestSuite))
}

func (s *EscrowKeyProbeWorkflowTestSuite) SetupTest() {
	s.env = s.NewTestWorkflowEnvironment()
	s.env.RegisterWorkflow(EscrowKeyProbeWorkflow)
	s.env.RegisterActivity(&activities.EscrowKeyActivities{})
}

func (s *EscrowKeyProbeWorkflowTestSuite) AfterTest(_, _ string) {
	s.env.AssertExpectations(s.T())
}

var probeParams = EscrowKeyProbeParams{
	Owner: activities.EscrowKeyOwnerRef{Kind: "platform"}, VersionID: "5f0a2c8e-1111-4a52-9d0b-2f7c43f0a429", RequestedBy: "auth0|ops",
}

func (s *EscrowKeyProbeWorkflowTestSuite) TestRecordsVerdict() {
	var acts *activities.EscrowKeyActivities
	s.env.OnActivity(acts.ProbeEscrowKeyVersion, mock.Anything, mock.Anything).
		Return(activities.ProbeEscrowKeyVersionOutput{OK: true, Code: activities.EscrowKeyProbeOK, Fingerprint: "FP"}, nil).Once()
	s.env.OnActivity(acts.RecordEscrowKeyProbe, mock.Anything, mock.MatchedBy(func(in activities.RecordEscrowKeyProbeInput) bool {
		return in.OK && in.Code == activities.EscrowKeyProbeOK && in.VersionID == probeParams.VersionID && in.Actor == "auth0|ops" && in.WorkflowID != "" && !in.At.IsZero()
	})).Return(nil).Once()

	s.env.ExecuteWorkflow(EscrowKeyProbeWorkflow, probeParams)
	s.Require().True(s.env.IsWorkflowCompleted())
	s.Require().NoError(s.env.GetWorkflowError())
	var res EscrowKeyProbeResult
	s.Require().NoError(s.env.GetWorkflowResult(&res))
	s.True(res.OK)
}

func (s *EscrowKeyProbeWorkflowTestSuite) TestStoreOutageIsRecordedAsFailedProbe() {
	var acts *activities.EscrowKeyActivities
	s.env.OnActivity(acts.ProbeEscrowKeyVersion, mock.Anything, mock.Anything).
		Return(activities.ProbeEscrowKeyVersionOutput{}, temporal.NewApplicationError("down", activities.EscrowKeyStoreUnavailableErrorType))
	s.env.OnActivity(acts.RecordEscrowKeyProbe, mock.Anything, mock.MatchedBy(func(in activities.RecordEscrowKeyProbeInput) bool {
		return !in.OK && in.Code == activities.EscrowKeyProbeStoreUnavailable
	})).Return(nil).Once()

	s.env.ExecuteWorkflow(EscrowKeyProbeWorkflow, probeParams)
	s.Require().NoError(s.env.GetWorkflowError())
}

func (s *EscrowKeyProbeWorkflowTestSuite) TestRejectedVersionRecordsNothing() {
	var acts *activities.EscrowKeyActivities
	s.env.OnActivity(acts.ProbeEscrowKeyVersion, mock.Anything, mock.Anything).
		Return(activities.ProbeEscrowKeyVersionOutput{}, temporal.NewNonRetryableApplicationError("revoked", "ESCROW_KEY_PROBE_REJECTED", nil)).Once()

	s.env.ExecuteWorkflow(EscrowKeyProbeWorkflow, probeParams)
	s.Error(s.env.GetWorkflowError())
	s.env.AssertNotCalled(s.T(), "RecordEscrowKeyProbe", mock.Anything, mock.Anything)
}
