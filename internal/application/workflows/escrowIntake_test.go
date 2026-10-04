package workflows

import (
	"errors"
	"testing"
	"time"

	"github.com/onasunnymorning/domain-os/internal/application/activities"
	"github.com/onasunnymorning/domain-os/internal/infrastructure/temporal"
	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/converter"
	"go.temporal.io/sdk/testsuite"
	"go.temporal.io/sdk/workflow"
)

// ---- EscrowIntakeSweepWorkflow ----

type EscrowIntakeSweepTestSuite struct {
	suite.Suite
	testsuite.WorkflowTestSuite
	env *testsuite.TestWorkflowEnvironment
}

func TestEscrowIntakeSweepTestSuite(t *testing.T) {
	suite.Run(t, new(EscrowIntakeSweepTestSuite))
}

func (s *EscrowIntakeSweepTestSuite) SetupTest() {
	s.env = s.NewTestWorkflowEnvironment()
	s.env.RegisterWorkflowWithOptions(EscrowIntakeSweepWorkflow, workflow.RegisterOptions{Name: EscrowIntakeSweepTypeName})
	s.env.RegisterWorkflowWithOptions(EscrowIntakeWorkflow, workflow.RegisterOptions{Name: EscrowIntakeTypeName})
}

func (s *EscrowIntakeSweepTestSuite) AfterTest(_, _ string) {
	s.env.AssertExpectations(s.T())
}

func sweepPair(base string) activities.IntakePair {
	id := base + "000000000000000"[:16-len(base)]
	return activities.IntakePair{
		Scope: "ryop1", TLD: "example", IntakeID: id,
		WorkflowID:  "escrow-intake-ryop1-example-" + id,
		ArtifactKey: "sftp/inbox/ryop1/example/" + base + ".ryde", SignatureKey: "sftp/inbox/ryop1/example/" + base + ".sig",
		ReceivedAt: time.Date(2026, 10, 3, 10, 0, 0, 0, time.UTC),
	}
}

func (s *EscrowIntakeSweepTestSuite) Test_Disabled_StartsNothing() {
	var acts *activities.EscrowIntakeActivities
	s.env.OnActivity(acts.ListIntakePairs, mock.Anything, mock.Anything).Return(activities.ListIntakePairsOutput{}, nil).Once()

	s.env.ExecuteWorkflow(EscrowIntakeSweepWorkflow, EscrowIntakeSweepParams{})
	s.Require().True(s.env.IsWorkflowCompleted())
	s.Require().NoError(s.env.GetWorkflowError())
	var res EscrowIntakeSweepResult
	s.Require().NoError(s.env.GetWorkflowResult(&res))
	s.False(res.Enabled)
	s.Zero(res.Started)
	s.NotEmpty(res.Notes)
}

func (s *EscrowIntakeSweepTestSuite) Test_StartsOneIntakePerPairOnHeavyBatch() {
	var acts *activities.EscrowIntakeActivities
	a, b := sweepPair("a"), sweepPair("b")
	s.env.OnActivity(acts.ListIntakePairs, mock.Anything, activities.ListIntakePairsInput{MaxPairs: 7}).Return(activities.ListIntakePairsOutput{
		Enabled: true, Pairs: []activities.IntakePair{a, b}, Deferred: 3, Unpaired: 1, Ignored: 2,
	}, nil).Once()
	s.env.OnWorkflow(EscrowIntakeWorkflow, mock.Anything, mock.MatchedBy(func(p EscrowIntakeParams) bool {
		return p.Scope == "ryop1" && p.TLD == "example" && p.ReceivedAt.Equal(a.ReceivedAt)
	})).Return(EscrowIntakeResult{Disposition: activities.EscrowIntakeArchived}, nil).Times(2)

	started := map[string]string{}
	s.env.SetOnChildWorkflowStartedListener(func(info *workflow.Info, _ workflow.Context, _ converter.EncodedValues) {
		started[info.WorkflowExecution.ID] = info.TaskQueueName
	})

	s.env.ExecuteWorkflow(EscrowIntakeSweepWorkflow, EscrowIntakeSweepParams{MaxPairs: 7})
	s.Require().True(s.env.IsWorkflowCompleted())
	s.Require().NoError(s.env.GetWorkflowError())
	var res EscrowIntakeSweepResult
	s.Require().NoError(s.env.GetWorkflowResult(&res))
	s.True(res.Enabled)
	s.Equal(2, res.Started)
	s.Equal([]string{a.WorkflowID, b.WorkflowID}, res.IntakeWorkflowIDs)
	s.Equal(3, res.Deferred)
	s.Equal(1, res.Unpaired)
	s.Equal(2, res.Ignored)
	s.Equal(map[string]string{a.WorkflowID: temporal.QueueHeavyBatch, b.WorkflowID: temporal.QueueHeavyBatch}, started)
}

func (s *EscrowIntakeSweepTestSuite) Test_PairAlreadyStartedIsNotStartedTwice() {
	var acts *activities.EscrowIntakeActivities
	a := sweepPair("a")
	// The same pair listed twice stands in for a pair whose intake an earlier
	// sweep started and that has not been claimed yet.
	s.env.OnActivity(acts.ListIntakePairs, mock.Anything, mock.Anything).Return(activities.ListIntakePairsOutput{
		Enabled: true, Pairs: []activities.IntakePair{a, a},
	}, nil).Once()
	s.env.OnWorkflow(EscrowIntakeWorkflow, mock.Anything, mock.Anything).Return(
		func(ctx workflow.Context, _ EscrowIntakeParams) (EscrowIntakeResult, error) {
			// Still running when the duplicate start arrives.
			_ = workflow.Sleep(ctx, time.Minute)
			return EscrowIntakeResult{Disposition: activities.EscrowIntakeArchived}, nil
		}).Once()

	s.env.ExecuteWorkflow(EscrowIntakeSweepWorkflow, EscrowIntakeSweepParams{})
	s.Require().True(s.env.IsWorkflowCompleted())
	s.Require().NoError(s.env.GetWorkflowError())
	var res EscrowIntakeSweepResult
	s.Require().NoError(s.env.GetWorkflowResult(&res))
	s.Equal(1, res.Started)
	s.Equal(1, res.AlreadyStarted)
	s.Zero(res.FailedToStart)
}

func (s *EscrowIntakeSweepTestSuite) Test_ListingFailureFailsTheSweep() {
	var acts *activities.EscrowIntakeActivities
	s.env.OnActivity(acts.ListIntakePairs, mock.Anything, mock.Anything).Return(activities.ListIntakePairsOutput{}, errors.New("bucket unreachable"))

	s.env.ExecuteWorkflow(EscrowIntakeSweepWorkflow, EscrowIntakeSweepParams{})
	s.Require().True(s.env.IsWorkflowCompleted())
	s.Require().Error(s.env.GetWorkflowError())
}

// ---- EscrowIntakeWorkflow ----

type EscrowIntakeTestSuite struct {
	suite.Suite
	testsuite.WorkflowTestSuite
	env *testsuite.TestWorkflowEnvironment
}

func TestEscrowIntakeTestSuite(t *testing.T) {
	suite.Run(t, new(EscrowIntakeTestSuite))
}

func (s *EscrowIntakeTestSuite) SetupTest() {
	s.env = s.NewTestWorkflowEnvironment()
	s.env.RegisterWorkflowWithOptions(EscrowIntakeWorkflow, workflow.RegisterOptions{Name: EscrowIntakeTypeName})
	s.env.RegisterWorkflowWithOptions(EscrowValidationWorkflow, workflow.RegisterOptions{Name: EscrowValidationTypeName})
}

func (s *EscrowIntakeTestSuite) AfterTest(_, _ string) {
	s.env.AssertExpectations(s.T())
}

var intakeParams = EscrowIntakeParams{
	Scope: "ryop1", TLD: "example", IntakeID: "0123456789abcdef",
	ArtifactKey: "sftp/inbox/ryop1/example/dep.ryde", SignatureKey: "sftp/inbox/ryop1/example/dep.sig",
	ReceivedAt: time.Date(2026, 10, 3, 10, 0, 0, 0, time.UTC),
}

var intakeClaimed = activities.ClaimIntakeOutput{
	ArtifactKey:  "sftp/claimed/ryop1/example/0123456789abcdef/dep.ryde",
	SignatureKey: "sftp/claimed/ryop1/example/0123456789abcdef/dep.sig",
}

const intakeWorkflowID = "escrow-intake-ryop1-example-0123456789abcdef"

func (s *EscrowIntakeTestSuite) expectClaim() {
	var acts *activities.EscrowIntakeActivities
	s.env.OnActivity(acts.ClaimIntakePair, mock.Anything, activities.ClaimIntakeInput{
		Scope: "ryop1", TLD: "example", Profile: entities.EscrowProfileRydeSig, IntakeID: intakeParams.IntakeID,
		ArtifactKey: intakeParams.ArtifactKey, SignatureKey: intakeParams.SignatureKey,
	}).Return(intakeClaimed, nil).Once()
}

func (s *EscrowIntakeTestSuite) expectSettle(out activities.SettleIntakeOutput) {
	var acts *activities.EscrowIntakeActivities
	s.env.OnActivity(acts.SettleIntakePair, mock.Anything, activities.SettleIntakeInput{
		Scope: "ryop1", TLD: "example", Profile: entities.EscrowProfileRydeSig, IntakeID: intakeParams.IntakeID,
		ValidationWorkflowID: intakeWorkflowID + "-validation",
		ArtifactKey:          intakeClaimed.ArtifactKey, SignatureKey: intakeClaimed.SignatureKey,
	}).Return(out, nil).Once()
}

func (s *EscrowIntakeTestSuite) run() EscrowIntakeResult {
	s.env.SetStartWorkflowOptions(defaultIntakeStartOptions())
	s.env.ExecuteWorkflow(EscrowIntakeWorkflow, intakeParams)
	s.Require().True(s.env.IsWorkflowCompleted())
	s.Require().NoError(s.env.GetWorkflowError())
	var res EscrowIntakeResult
	s.Require().NoError(s.env.GetWorkflowResult(&res))
	return res
}

func (s *EscrowIntakeTestSuite) Test_ValidatesTheClaimedPairAsTheSftpTenant() {
	s.expectClaim()
	s.env.OnWorkflow(EscrowValidationWorkflow, mock.Anything, EscrowValidationParams{
		Scope: "ryop1", TLD: "example", Profile: entities.EscrowProfileRydeSig,
		ArtifactObjectKey: intakeClaimed.ArtifactKey, SignatureObjectKey: intakeClaimed.SignatureKey,
		SubmittedBy: "sftp:ryop1", IntakeRef: "sftp:" + intakeParams.ArtifactKey, ReceivedAt: intakeParams.ReceivedAt,
	}).Return(EscrowValidationResult{Outcome: "PASS", ValidationRunID: "run-1"}, nil).Once()
	s.expectSettle(activities.SettleIntakeOutput{Disposition: activities.EscrowIntakeArchived, ValidationRunID: "run-1"})

	var childID, childQueue string
	s.env.SetOnChildWorkflowStartedListener(func(info *workflow.Info, _ workflow.Context, _ converter.EncodedValues) {
		childID, childQueue = info.WorkflowExecution.ID, info.TaskQueueName
	})

	res := s.run()
	s.Equal("PASS", res.ValidationOutcome)
	s.Empty(res.ValidationError)
	s.Equal(activities.EscrowIntakeArchived, res.Disposition)
	s.Equal("run-1", res.ValidationRunID)
	s.Equal(intakeWorkflowID+"-validation", childID)
	s.Equal(temporal.QueueHeavyBatch, childQueue)
}

func (s *EscrowIntakeTestSuite) Test_FailedValidationIsStillSettled() {
	s.expectClaim()
	s.env.OnWorkflow(EscrowValidationWorkflow, mock.Anything, mock.Anything).
		Return(EscrowValidationResult{}, errors.New("tld is not operated by the caller's scope")).Once()
	s.expectSettle(activities.SettleIntakeOutput{Disposition: activities.EscrowIntakeRejected})

	res := s.run()
	s.Empty(res.ValidationOutcome)
	s.Contains(res.ValidationError, "not operated")
	s.Equal(activities.EscrowIntakeRejected, res.Disposition)
}

func (s *EscrowIntakeTestSuite) Test_ClaimFailureStopsBeforeValidation() {
	var acts *activities.EscrowIntakeActivities
	s.env.OnActivity(acts.ClaimIntakePair, mock.Anything, mock.Anything).
		Return(activities.ClaimIntakeOutput{}, errors.New("an intake object disappeared before it was moved"))

	s.env.SetStartWorkflowOptions(defaultIntakeStartOptions())
	s.env.ExecuteWorkflow(EscrowIntakeWorkflow, intakeParams)
	s.Require().True(s.env.IsWorkflowCompleted())
	s.Require().Error(s.env.GetWorkflowError())
}

// defaultIntakeStartOptions gives the intake the ID the sweep would, so the
// validation child's derived ID is predictable.
func defaultIntakeStartOptions() client.StartWorkflowOptions {
	return client.StartWorkflowOptions{ID: intakeWorkflowID}
}

func (s *EscrowIntakeTestSuite) Test_PlaintextDepositIsValidatedUnsigned() {
	var acts *activities.EscrowIntakeActivities
	params := EscrowIntakeParams{
		Scope: "ryop1", TLD: "example", Profile: entities.EscrowProfilePlaintextXML, IntakeID: intakeParams.IntakeID,
		ArtifactKey: "sftp/inbox/ryop1/example/dep.xml.gz", ReceivedAt: intakeParams.ReceivedAt,
	}
	claimedKey := "sftp/claimed/ryop1/example/0123456789abcdef/dep.xml.gz"
	s.env.OnActivity(acts.ClaimIntakePair, mock.Anything, activities.ClaimIntakeInput{
		Scope: "ryop1", TLD: "example", Profile: entities.EscrowProfilePlaintextXML, IntakeID: params.IntakeID,
		ArtifactKey: params.ArtifactKey,
	}).Return(activities.ClaimIntakeOutput{ArtifactKey: claimedKey}, nil).Once()
	s.env.OnWorkflow(EscrowValidationWorkflow, mock.Anything, EscrowValidationParams{
		Scope: "ryop1", TLD: "example", Profile: entities.EscrowProfilePlaintextXML,
		ArtifactObjectKey: claimedKey,
		SubmittedBy:       "sftp:ryop1", IntakeRef: "sftp:" + params.ArtifactKey, ReceivedAt: params.ReceivedAt,
	}).Return(EscrowValidationResult{Outcome: "PASS", ValidationRunID: "run-x"}, nil).Once()
	s.env.OnActivity(acts.SettleIntakePair, mock.Anything, activities.SettleIntakeInput{
		Scope: "ryop1", TLD: "example", Profile: entities.EscrowProfilePlaintextXML, IntakeID: params.IntakeID,
		ValidationWorkflowID: intakeWorkflowID + "-validation", ArtifactKey: claimedKey,
	}).Return(activities.SettleIntakeOutput{Disposition: activities.EscrowIntakeArchived, ValidationRunID: "run-x"}, nil).Once()

	s.env.SetStartWorkflowOptions(defaultIntakeStartOptions())
	s.env.ExecuteWorkflow(EscrowIntakeWorkflow, params)
	s.Require().True(s.env.IsWorkflowCompleted())
	s.Require().NoError(s.env.GetWorkflowError())
	var res EscrowIntakeResult
	s.Require().NoError(s.env.GetWorkflowResult(&res))
	s.Equal("PASS", res.ValidationOutcome)
	s.Equal(activities.EscrowIntakeArchived, res.Disposition)
}

func (s *EscrowIntakeSweepTestSuite) Test_PassesTheProfileAndReportsRefusedPlaintext() {
	var acts *activities.EscrowIntakeActivities
	x := sweepPair("x")
	x.Profile, x.SignatureKey = entities.EscrowProfilePlaintextXML, ""
	x.ArtifactKey = "sftp/inbox/ryop1/example/x.xml"
	s.env.OnActivity(acts.ListIntakePairs, mock.Anything, mock.Anything).Return(activities.ListIntakePairsOutput{
		Enabled: true, Pairs: []activities.IntakePair{x},
	}, nil).Once()
	s.env.OnWorkflow(EscrowIntakeWorkflow, mock.Anything, EscrowIntakeParams{
		Scope: "ryop1", TLD: "example", Profile: entities.EscrowProfilePlaintextXML, IntakeID: x.IntakeID,
		ArtifactKey: x.ArtifactKey, ReceivedAt: x.ReceivedAt,
	}).Return(EscrowIntakeResult{}, nil).Once()

	s.env.ExecuteWorkflow(EscrowIntakeSweepWorkflow, EscrowIntakeSweepParams{})
	s.Require().NoError(s.env.GetWorkflowError())
	var res EscrowIntakeSweepResult
	s.Require().NoError(s.env.GetWorkflowResult(&res))
	s.Equal(1, res.Started)
}

func (s *EscrowIntakeSweepTestSuite) Test_RefusedPlaintextIsNoted() {
	var acts *activities.EscrowIntakeActivities
	s.env.OnActivity(acts.ListIntakePairs, mock.Anything, mock.Anything).Return(activities.ListIntakePairsOutput{
		Enabled: true, PlaintextRefused: 2,
	}, nil).Once()

	s.env.ExecuteWorkflow(EscrowIntakeSweepWorkflow, EscrowIntakeSweepParams{})
	s.Require().NoError(s.env.GetWorkflowError())
	var res EscrowIntakeSweepResult
	s.Require().NoError(s.env.GetWorkflowResult(&res))
	s.Equal(2, res.PlaintextRefused)
	s.Require().NotEmpty(res.Notes)
	s.Contains(res.Notes[0], "ESCROW_INTAKE_SFTP_ALLOW_PLAINTEXT")
}
