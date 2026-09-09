package workflows

import (
	"errors"
	"testing"
	"time"

	"github.com/onasunnymorning/domain-os/internal/application/activities"
	"github.com/onasunnymorning/domain-os/internal/application/rdevalidate"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/testsuite"
)

type EscrowValidationWorkflowTestSuite struct {
	suite.Suite
	testsuite.WorkflowTestSuite
	env *testsuite.TestWorkflowEnvironment
}

func TestEscrowValidationWorkflowTestSuite(t *testing.T) {
	suite.Run(t, new(EscrowValidationWorkflowTestSuite))
}

func (s *EscrowValidationWorkflowTestSuite) SetupTest() {
	s.env = s.NewTestWorkflowEnvironment()
	s.env.RegisterWorkflow(EscrowValidationWorkflow)
}

func (s *EscrowValidationWorkflowTestSuite) AfterTest(_, _ string) {
	s.env.AssertExpectations(s.T())
}

var evParams = EscrowValidationParams{
	Scope: "ryop1", TLD: "example", RydeObjectKey: "uploads/a.ryde", SigObjectKey: "uploads/a.sig",
	SubmittedBy: "tester", ReceivedAt: time.Date(2026, 9, 8, 3, 15, 0, 0, time.UTC),
}

func evBound() activities.BindDepositOutput {
	// Fixed ids: the workflows package may not import uuid (INV-06), even in tests.
	return activities.BindDepositOutput{DepositID: [16]byte{1, 2, 3}, ValidationRunID: [16]byte{4, 5, 6}, RydeKey: "archived.ryde", SigKey: "archived.sig", RydeSHA256: "aa", SigSHA256: "bb"}
}

func evResult(outcome rdevalidate.Outcome, codes ...rdevalidate.Code) rdevalidate.Result {
	r := rdevalidate.Result{Profile: rdevalidate.ProfileRydeSig, Outcome: outcome, StageReached: rdevalidate.StageRDE}
	for _, c := range codes {
		r.Findings = append(r.Findings, rdevalidate.Finding{Code: c, Severity: rdevalidate.SeverityError})
	}
	return r
}

func (s *EscrowValidationWorkflowTestSuite) Test_Pass_EmitsDVPN() {
	var acts *activities.EscrowValidationActivities
	bound := evBound()
	s.env.OnActivity(acts.BindDeposit, mock.Anything, mock.MatchedBy(func(in activities.BindDepositInput) bool {
		return in.Scope == "ryop1" && in.TLD == "example" && in.WorkflowID != ""
	})).Return(bound, nil).Once()
	s.env.OnActivity(acts.ValidateArtifacts, mock.Anything, mock.MatchedBy(func(in activities.ValidateArtifactsInput) bool {
		return in.RydeKey == "archived.ryde" && in.RydeSHA256 == "aa" && in.ValidationRunID == bound.ValidationRunID
	})).Return(evResult(rdevalidate.OutcomePass), nil).Once()
	s.env.OnActivity(acts.EmitReportAndNotification, mock.Anything, mock.MatchedBy(func(in activities.EmitReportInput) bool {
		return in.Result.Outcome == rdevalidate.OutcomePass && !in.ValidatedAt.IsZero() && in.ReceivedAt.Equal(evParams.ReceivedAt)
	})).Return(activities.EmitReportOutput{ReportKey: "r.xml", NotificationKey: "n.xml", NotificationStatus: "DVPN"}, nil).Once()
	s.env.OnActivity(acts.FinalizeValidationRun, mock.Anything, mock.MatchedBy(func(in activities.FinalizeRunInput) bool {
		return in.NotificationStatus == "DVPN" && in.ReportKey == "r.xml" && in.Failure == "" && !in.CompletedAt.IsZero()
	})).Return(nil).Once()

	s.env.ExecuteWorkflow(EscrowValidationWorkflow, evParams)
	s.Require().True(s.env.IsWorkflowCompleted())
	s.Require().NoError(s.env.GetWorkflowError())
	var result EscrowValidationResult
	s.Require().NoError(s.env.GetWorkflowResult(&result))
	s.Equal("PASS", result.Outcome)
	s.True(result.Verified)
	s.Equal("DVPN", result.NotificationStatus)
	s.Equal(bound.DepositID.String(), result.DepositID)

	var state EscrowValidationState
	v, err := s.env.QueryWorkflow("state")
	s.Require().NoError(err)
	s.Require().NoError(v.Get(&state))
	s.Equal("completed", state.Phase)
}

func (s *EscrowValidationWorkflowTestSuite) Test_Fail_EmitsDVFN() {
	var acts *activities.EscrowValidationActivities
	s.env.OnActivity(acts.BindDeposit, mock.Anything, mock.Anything).Return(evBound(), nil).Once()
	s.env.OnActivity(acts.ValidateArtifacts, mock.Anything, mock.Anything).Return(evResult(rdevalidate.OutcomeFail, rdevalidate.CodeSigInvalid), nil).Once()
	s.env.OnActivity(acts.EmitReportAndNotification, mock.Anything, mock.MatchedBy(func(in activities.EmitReportInput) bool {
		return in.Result.Outcome == rdevalidate.OutcomeFail
	})).Return(activities.EmitReportOutput{ReportKey: "r.xml", NotificationKey: "n.xml", NotificationStatus: "DVFN"}, nil).Once()
	s.env.OnActivity(acts.FinalizeValidationRun, mock.Anything, mock.MatchedBy(func(in activities.FinalizeRunInput) bool {
		return in.NotificationStatus == "DVFN"
	})).Return(nil).Once()

	s.env.ExecuteWorkflow(EscrowValidationWorkflow, evParams)
	s.Require().NoError(s.env.GetWorkflowError())
	var result EscrowValidationResult
	s.Require().NoError(s.env.GetWorkflowResult(&result))
	s.Equal("FAIL", result.Outcome)
	s.False(result.Verified)
	s.Equal("DVFN", result.NotificationStatus)
	s.Equal([]string{"SIG_INVALID"}, result.Codes)
}

func (s *EscrowValidationWorkflowTestSuite) Test_ErrorOutcome_FinalisesWithoutNotification() {
	var acts *activities.EscrowValidationActivities
	s.env.OnActivity(acts.BindDeposit, mock.Anything, mock.Anything).Return(evBound(), nil).Once()
	s.env.OnActivity(acts.ValidateArtifacts, mock.Anything, mock.Anything).Return(evResult(rdevalidate.OutcomeError, rdevalidate.CodeDecryptKeyUnavailable), nil).Once()
	s.env.OnActivity(acts.FinalizeValidationRun, mock.Anything, mock.MatchedBy(func(in activities.FinalizeRunInput) bool {
		return in.Result.Outcome == rdevalidate.OutcomeError && in.NotificationStatus == "" && in.Failure == ""
	})).Return(nil).Once()
	// EmitReportAndNotification must never be called.

	s.env.ExecuteWorkflow(EscrowValidationWorkflow, evParams)
	s.Require().True(s.env.IsWorkflowCompleted())
	s.Require().Error(s.env.GetWorkflowError(), "an undecided run fails the workflow")
	var state EscrowValidationState
	v, _ := s.env.QueryWorkflow("state")
	s.Require().NoError(v.Get(&state))
	s.Equal("error", state.Phase)
	s.Equal("ERROR", state.Outcome)
}

func (s *EscrowValidationWorkflowTestSuite) Test_ValidateActivityFailure_FinalisesAsError() {
	var acts *activities.EscrowValidationActivities
	s.env.OnActivity(acts.BindDeposit, mock.Anything, mock.Anything).Return(evBound(), nil).Once()
	s.env.OnActivity(acts.ValidateArtifacts, mock.Anything, mock.Anything).
		Return(rdevalidate.Result{}, temporal.NewNonRetryableApplicationError("storage exploded", "TEST", nil)).Once()
	s.env.OnActivity(acts.FinalizeValidationRun, mock.Anything, mock.MatchedBy(func(in activities.FinalizeRunInput) bool {
		return in.Failure != "" && in.Result.Outcome == rdevalidate.OutcomeError
	})).Return(nil).Once()

	s.env.ExecuteWorkflow(EscrowValidationWorkflow, evParams)
	s.Require().True(s.env.IsWorkflowCompleted())
	s.Require().Error(s.env.GetWorkflowError())
}

func (s *EscrowValidationWorkflowTestSuite) Test_BindFailure_NoRunToFinalise() {
	var acts *activities.EscrowValidationActivities
	s.env.OnActivity(acts.BindDeposit, mock.Anything, mock.Anything).
		Return(activities.BindDepositOutput{}, temporal.NewNonRetryableApplicationError("tld is not operated by the caller's scope", "ESCROW_VALIDATION", nil)).Once()

	s.env.ExecuteWorkflow(EscrowValidationWorkflow, evParams)
	s.Require().True(s.env.IsWorkflowCompleted())
	err := s.env.GetWorkflowError()
	s.Require().Error(err)
	var appErr *temporal.ApplicationError
	s.True(errors.As(err, &appErr))
}

func (s *EscrowValidationWorkflowTestSuite) Test_ReplayFlagPropagates() {
	var acts *activities.EscrowValidationActivities
	b := evBound()
	b.Replay = true
	s.env.OnActivity(acts.BindDeposit, mock.Anything, mock.Anything).Return(b, nil).Once()
	s.env.OnActivity(acts.ValidateArtifacts, mock.Anything, mock.Anything).Return(evResult(rdevalidate.OutcomePass), nil).Once()
	s.env.OnActivity(acts.EmitReportAndNotification, mock.Anything, mock.Anything).Return(activities.EmitReportOutput{NotificationStatus: "DVPN"}, nil).Once()
	s.env.OnActivity(acts.FinalizeValidationRun, mock.Anything, mock.Anything).Return(nil).Once()
	s.env.ExecuteWorkflow(EscrowValidationWorkflow, evParams)
	var result EscrowValidationResult
	s.Require().NoError(s.env.GetWorkflowResult(&result))
	s.True(result.Replay)
}
