package workflows

import (
	"errors"
	"testing"

	"github.com/onasunnymorning/domain-os/internal/application/activities"
	"github.com/onasunnymorning/domain-os/internal/application/rdesanitize"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"go.temporal.io/sdk/testsuite"
)

type EscrowSanitizeWorkflowTestSuite struct {
	suite.Suite
	testsuite.WorkflowTestSuite
	env *testsuite.TestWorkflowEnvironment
}

func TestEscrowSanitizeWorkflowTestSuite(t *testing.T) {
	suite.Run(t, new(EscrowSanitizeWorkflowTestSuite))
}

func (s *EscrowSanitizeWorkflowTestSuite) SetupTest() {
	s.env = s.NewTestWorkflowEnvironment()
	s.env.RegisterWorkflow(EscrowSanitizeWorkflow)
}

func (s *EscrowSanitizeWorkflowTestSuite) AfterTest(_, _ string) {
	s.env.AssertExpectations(s.T())
}

var esParams = EscrowSanitizeParams{
	Scope:                 "ryop1",
	SourceValidationRunID: "9f1d6a1e-7f0e-4a49-9a7f-4a0a1a2b3c4d",
	RequestedBy:           "tester",
}

func esBound() activities.BindSanitizationSourceOutput {
	// Fixed ids: the workflows package may not import uuid (INV-06), even in tests.
	return activities.BindSanitizationSourceOutput{
		SanitizationRunID: [16]byte{7, 8, 9}, TLD: "example", DepositID: [16]byte{1, 2, 3},
		SourceValidationRunID: [16]byte{4, 5, 6}, SourceProfile: "ryde+sig",
		ArtifactKey: "archived.ryde", SignatureKey: "archived.sig",
		ArtifactSHA256: "aa", SignatureSHA256: "bb",
		SyntheticSuffix: "artful-dodger", PolicyVersion: rdesanitize.PolicyVersion,
		StagingKey: "pending/x.xml.gz", DerivativeKey: "sanitized/x.xml.gz", ManifestKey: "sanitized/m.json",
	}
}

func esResult(outcome rdesanitize.Outcome, codes ...rdesanitize.Code) rdesanitize.Result {
	r := rdesanitize.Result{Outcome: outcome, StageReached: rdesanitize.StageRewrite}
	for _, c := range codes {
		r.Findings = append(r.Findings, rdesanitize.Finding{Code: c, Severity: rdesanitize.SeverityError})
	}
	return r
}

func (s *EscrowSanitizeWorkflowTestSuite) Test_Pass_PublishesAndFinalises() {
	var acts *activities.EscrowSanitizeActivities
	bound := esBound()
	s.env.OnActivity(acts.BindSanitizationSource, mock.Anything, mock.MatchedBy(func(in activities.BindSanitizationSourceInput) bool {
		return in.Scope == "ryop1" && in.SourceValidationRunID == esParams.SourceValidationRunID && in.WorkflowID != ""
	})).Return(bound, nil).Once()
	s.env.OnActivity(acts.ProduceDerivative, mock.Anything, mock.MatchedBy(func(in activities.ProduceDerivativeInput) bool {
		return in.StagingKey == "pending/x.xml.gz" && in.SyntheticSuffix == "artful-dodger"
	})).Return(activities.ProduceDerivativeOutput{
		Result: esResult(rdesanitize.OutcomePass), DerivativeSHA256: "cc", DerivativeBytes: 128, TokenKeyID: "FP",
	}, nil).Once()
	s.env.OnActivity(acts.VerifyDerivative, mock.Anything, mock.MatchedBy(func(in activities.VerifyDerivativeInput) bool {
		return in.DerivativeSHA256 == "cc" && in.TokenKeyID == "FP" && in.StagingKey == "pending/x.xml.gz"
	})).Return(activities.VerifyDerivativeOutput{
		Result: esResult(rdesanitize.OutcomePass), DerivativeKey: "sanitized/x.xml.gz", ManifestKey: "sanitized/m.json",
	}, nil).Once()
	s.env.OnActivity(acts.FinalizeSanitizationRun, mock.Anything, mock.MatchedBy(func(in activities.FinalizeSanitizationRunInput) bool {
		return in.DerivativeKey == "sanitized/x.xml.gz" && in.ManifestKey == "sanitized/m.json" && in.Failure == ""
	})).Return(nil).Once()

	s.env.ExecuteWorkflow(EscrowSanitizeWorkflow, esParams)
	s.Require().True(s.env.IsWorkflowCompleted())
	s.Require().NoError(s.env.GetWorkflowError())

	var result EscrowSanitizeResult
	s.Require().NoError(s.env.GetWorkflowResult(&result))
	s.Equal("PASS", result.Outcome)
	s.Equal("sanitized/x.xml.gz", result.DerivativeKey)
	s.Equal(rdesanitize.PolicyVersion, result.PolicyVersion)

	var state EscrowSanitizeState
	v, err := s.env.QueryWorkflow("state")
	s.Require().NoError(err)
	s.Require().NoError(v.Get(&state))
	s.Equal("completed", state.Phase)
}

func (s *EscrowSanitizeWorkflowTestSuite) Test_Quarantined_PublishesNothingAndCompletes() {
	var acts *activities.EscrowSanitizeActivities
	s.env.OnActivity(acts.BindSanitizationSource, mock.Anything, mock.Anything).Return(esBound(), nil).Once()
	s.env.OnActivity(acts.ProduceDerivative, mock.Anything, mock.Anything).Return(activities.ProduceDerivativeOutput{
		Result: esResult(rdesanitize.OutcomeQuarantined, rdesanitize.CodePolicyUnknownElement),
	}, nil).Once()
	// VerifyDerivative is deliberately not mocked: nothing was staged, so
	// verifying or publishing would be a bug.
	s.env.OnActivity(acts.FinalizeSanitizationRun, mock.Anything, mock.MatchedBy(func(in activities.FinalizeSanitizationRunInput) bool {
		return in.DerivativeKey == "" && in.ManifestKey == ""
	})).Return(nil).Once()

	s.env.ExecuteWorkflow(EscrowSanitizeWorkflow, esParams)
	s.Require().True(s.env.IsWorkflowCompleted())
	s.Require().NoError(s.env.GetWorkflowError(), "a quarantine is a decision, not a failure")

	var result EscrowSanitizeResult
	s.Require().NoError(s.env.GetWorkflowResult(&result))
	s.Equal("QUARANTINED", result.Outcome)
	s.Empty(result.DerivativeKey)
	s.Equal([]string{"POLICY_UNKNOWN_ELEMENT"}, result.Codes)
}

func (s *EscrowSanitizeWorkflowTestSuite) Test_ServiceError_FinalisesAndFails() {
	var acts *activities.EscrowSanitizeActivities
	s.env.OnActivity(acts.BindSanitizationSource, mock.Anything, mock.Anything).Return(esBound(), nil).Once()
	s.env.OnActivity(acts.ProduceDerivative, mock.Anything, mock.Anything).Return(activities.ProduceDerivativeOutput{
		Result: esResult(rdesanitize.OutcomeError, rdesanitize.CodeTokenKeyUnavailable),
	}, nil).Once()
	s.env.OnActivity(acts.FinalizeSanitizationRun, mock.Anything, mock.Anything).Return(nil).Once()

	s.env.ExecuteWorkflow(EscrowSanitizeWorkflow, esParams)
	s.Require().True(s.env.IsWorkflowCompleted())
	s.Require().Error(s.env.GetWorkflowError(), "a service-side failure must not look like a completed run")
}

func (s *EscrowSanitizeWorkflowTestSuite) Test_ActivityFailure_FinalisesThroughADisconnectedContext() {
	var acts *activities.EscrowSanitizeActivities
	s.env.OnActivity(acts.BindSanitizationSource, mock.Anything, mock.Anything).Return(esBound(), nil).Once()
	s.env.OnActivity(acts.ProduceDerivative, mock.Anything, mock.Anything).
		Return(activities.ProduceDerivativeOutput{}, errors.New("object store unavailable"))
	s.env.OnActivity(acts.FinalizeSanitizationRun, mock.Anything, mock.MatchedBy(func(in activities.FinalizeSanitizationRunInput) bool {
		return in.Failure != "" && in.DerivativeKey == ""
	})).Return(nil).Once()

	s.env.ExecuteWorkflow(EscrowSanitizeWorkflow, esParams)
	s.Require().True(s.env.IsWorkflowCompleted())
	s.Require().Error(s.env.GetWorkflowError())
}

func (s *EscrowSanitizeWorkflowTestSuite) Test_AlreadyDerived_DoesNothing() {
	var acts *activities.EscrowSanitizeActivities
	bound := esBound()
	bound.Replay, bound.AlreadyFinal, bound.ExistingOutcome = true, true, "PASS"
	bound.DerivativeKey, bound.ManifestKey = "sanitized/old.xml.gz", "sanitized/old.json"
	s.env.OnActivity(acts.BindSanitizationSource, mock.Anything, mock.Anything).Return(bound, nil).Once()
	// Nothing else is mocked: re-deriving would either duplicate the existing
	// derivative or overwrite it, and neither is allowed.

	s.env.ExecuteWorkflow(EscrowSanitizeWorkflow, esParams)
	s.Require().True(s.env.IsWorkflowCompleted())
	s.Require().NoError(s.env.GetWorkflowError())

	var result EscrowSanitizeResult
	s.Require().NoError(s.env.GetWorkflowResult(&result))
	s.True(result.Replay)
	s.Equal("PASS", result.Outcome)
	s.Equal("sanitized/old.xml.gz", result.DerivativeKey)
}
