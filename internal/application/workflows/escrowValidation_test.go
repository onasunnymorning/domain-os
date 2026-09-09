package workflows

import (
	"errors"
	"testing"
	"time"

	"github.com/onasunnymorning/domain-os/internal/application/activities"
	"github.com/onasunnymorning/domain-os/internal/application/rdevalidate"
	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
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
	Scope: "ryop1", TLD: "example", ArtifactObjectKey: "uploads/a.ryde", SignatureObjectKey: "uploads/a.sig",
	SubmittedBy: "tester", ReceivedAt: time.Date(2026, 9, 8, 3, 15, 0, 0, time.UTC),
}

func evBound() activities.BindDepositOutput {
	// Fixed ids: the workflows package may not import uuid (INV-06), even in tests.
	return activities.BindDepositOutput{DepositID: [16]byte{1, 2, 3}, ValidationRunID: [16]byte{4, 5, 6}, Profile: entities.EscrowProfileRydeSig, ArtifactKey: "archived.ryde", SignatureKey: "archived.sig", ArtifactSHA256: "aa", SignatureSHA256: "bb"}
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
		return in.ArtifactKey == "archived.ryde" && in.ArtifactSHA256 == "aa" && in.ValidationRunID == bound.ValidationRunID
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
	// The emit activity still runs for an undecided run: it writes the summary,
	// which is the artifact whose findings say why the run could not be
	// decided. It writes no rdeReport and no notification, because there is
	// nothing to claim.
	s.env.OnActivity(acts.EmitReportAndNotification, mock.Anything, mock.Anything).
		Return(activities.EmitReportOutput{SummaryKey: "escrow-validation/ryop1/example/dep/run/summary.json"}, nil).Once()
	s.env.OnActivity(acts.FinalizeValidationRun, mock.Anything, mock.MatchedBy(func(in activities.FinalizeRunInput) bool {
		return in.Result.Outcome == rdevalidate.OutcomeError && in.NotificationStatus == "" && in.Failure == "" &&
			in.SummaryKey != "" && in.ReportKey == "" && in.NotificationKey == ""
	})).Return(nil).Once()

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

func (s *EscrowValidationWorkflowTestSuite) Test_UnsignedProfile_EmitsNoNotification() {
	var acts *activities.EscrowValidationActivities
	bound := evBound()
	bound.Profile = entities.EscrowProfilePlaintextXML
	bound.SignatureKey, bound.SignatureSHA256 = "", ""
	s.env.OnActivity(acts.BindDeposit, mock.Anything, mock.MatchedBy(func(in activities.BindDepositInput) bool {
		return in.Profile == entities.EscrowProfilePlaintextXML && in.SignatureObjectKey == ""
	})).Return(bound, nil).Once()
	s.env.OnActivity(acts.ValidateArtifacts, mock.Anything, mock.MatchedBy(func(in activities.ValidateArtifactsInput) bool {
		return in.Profile == entities.EscrowProfilePlaintextXML
	})).Return(rdevalidate.Result{Profile: rdevalidate.ProfilePlaintextXML, Outcome: rdevalidate.OutcomePass, StageReached: rdevalidate.StageRDE}, nil).Once()
	// The emit activity still runs — it writes the summary and the rdeReport
	// for any decided run — but it returns no notification, because an
	// unsigned deposit establishes nothing to claim to ICANN. The workflow
	// must carry that emptiness through to finalisation, where the entity
	// rejects an unsigned run that arrives with a notification.
	s.env.OnActivity(acts.EmitReportAndNotification, mock.Anything, mock.MatchedBy(func(in activities.EmitReportInput) bool {
		return in.Profile == entities.EscrowProfilePlaintextXML
	})).Return(activities.EmitReportOutput{
		SummaryKey: "escrow-validation/ryop1/example/dep/run/summary.json",
		ReportKey:  "escrow-validation/ryop1/example/dep/run/report.xml",
	}, nil).Once()
	s.env.OnActivity(acts.FinalizeValidationRun, mock.Anything, mock.MatchedBy(func(in activities.FinalizeRunInput) bool {
		return in.NotificationStatus == "" && in.NotificationKey == "" &&
			in.SummaryKey != "" && in.ReportKey != ""
	})).Return(nil).Once()

	params := evParams
	params.Profile = entities.EscrowProfilePlaintextXML
	params.ArtifactObjectKey, params.SignatureObjectKey = "uploads/a.xml.gz", ""
	s.env.ExecuteWorkflow(EscrowValidationWorkflow, params)
	s.Require().True(s.env.IsWorkflowCompleted())
	s.Require().NoError(s.env.GetWorkflowError())

	var result EscrowValidationResult
	s.Require().NoError(s.env.GetWorkflowResult(&result))
	s.Equal("PASS", result.Outcome)
	s.False(result.Verified, "an unsigned deposit is never a verified pass")
	s.Empty(result.NotificationStatus)
	s.Empty(result.NotificationKey)
	s.NotEmpty(result.SummaryKey, "every run that produced a result gets a summary")
	s.NotEmpty(result.ReportKey, "a decided run gets an rdeReport whatever its profile")
}
