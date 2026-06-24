package workflows

import (
	"testing"
	"time"

	"github.com/onasunnymorning/domain-os/internal/application/activities"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"go.temporal.io/sdk/testsuite"
)

type EscrowImportWorkflowTestSuite struct {
	suite.Suite
	testsuite.WorkflowTestSuite
	env *testsuite.TestWorkflowEnvironment
}

func (s *EscrowImportWorkflowTestSuite) SetupTest() {
	s.env = s.NewTestWorkflowEnvironment()
	s.env.RegisterWorkflow(EscrowImportWorkflow)
}

func (s *EscrowImportWorkflowTestSuite) Test_EscrowImport_Success() {
	var acts *activities.EscrowImportActivities

	// Staging mocks
	s.env.OnActivity(acts.ValidateEscrowSource, mock.Anything, mock.Anything).Return(activities.ValidateEscrowSourceResult{Exists: true}, nil)
	s.env.OnActivity(acts.ParseAndExtractAssets, mock.Anything, mock.Anything).Return(activities.ParseAndExtractAssetsResult{
		RunPrefix: "escrow/com/20260624/test-wf",
		AssetKeys: map[string]string{"domains": "escrow/com/20260624/test-wf/domains.csv"},
		HasIssues: false,
	}, nil)
	s.env.OnActivity(acts.BuildStagingDatabase, mock.Anything, mock.Anything).Return(activities.BuildStagingDatabaseResult{DBKey: "escrow/com/20260624/test-wf/ryde.db"}, nil)
	s.env.OnActivity(acts.ResolveRegistrars, mock.Anything, mock.Anything).Return(activities.ResolveRegistrarsResult{DBKey: "escrow/com/20260624/test-wf/ryde.db"}, nil)
	s.env.OnActivity(acts.FinalizeStaging, mock.Anything, mock.Anything).Return(activities.FinalizeStagingResult{StagedDBKey: "escrow/com/20260624/test-wf/staged.db"}, nil)
	s.env.OnActivity(acts.QAStagedDatabase, mock.Anything, mock.Anything).Return(activities.QAStagedDatabaseResult{Passed: true, QAReportKey: "escrow/com/20260624/test-wf/qa-report.json"}, nil)

	// Ingestion mocks
	s.env.OnActivity(acts.IngestContacts, mock.Anything, mock.Anything).Return(activities.IngestContactsResult{Total: 10, Inserted: 8, Updated: 2}, nil)
	s.env.OnActivity(acts.IngestHosts, mock.Anything, mock.Anything).Return(activities.IngestHostsResult{Total: 5, Inserted: 5, Updated: 0}, nil)
	s.env.OnActivity(acts.IngestDomains, mock.Anything, mock.Anything).Return(activities.IngestDomainsResult{Total: 100, Inserted: 90, Updated: 10}, nil)
	s.env.OnActivity(acts.IngestNNDNs, mock.Anything, mock.Anything).Return(activities.IngestNNDNsResult{Total: 1, Inserted: 1, Updated: 0}, nil)
	s.env.OnActivity(acts.LinkDomainHosts, mock.Anything, mock.Anything).Return(activities.LinkDomainHostsResult{Total: 50}, nil)
	s.env.OnActivity(acts.AccreditRegistrars, mock.Anything, mock.Anything).Return(activities.AccreditRegistrarsResult{Total: 2}, nil)
	s.env.OnActivity(acts.PersistImportSummary, mock.Anything, mock.Anything).Return(activities.PersistImportSummaryResult{SummaryKey: "escrow/com/20260624/test-wf/summary.json"}, nil)

	// Send signal ConfirmEscrowImport (true) after staging/QA completes
	s.env.RegisterDelayedCallback(func() {
		s.env.SignalWorkflow("ConfirmEscrowImport", true)
	}, time.Millisecond*50)

	params := EscrowImportParams{
		TLD:       "com",
		ObjectKey: "uploads/com-escrow.xml",
	}

	s.env.ExecuteWorkflow(EscrowImportWorkflow, params)
	s.Require().True(s.env.IsWorkflowCompleted())
	s.Require().NoError(s.env.GetWorkflowError())

	var result EscrowImportResult
	s.Require().NoError(s.env.GetWorkflowResult(&result))
	s.Equal("com", result.TLD)
	s.Equal("escrow/com/20260624/test-wf/qa-report.json", result.QAReportKey)
	s.Equal("escrow/com/20260624/test-wf/staged.db", result.StagedDBKey)
	s.True(result.Confirmed)
	s.Equal(int64(90), result.IngestedCounts["domains_inserted"])
}

func (s *EscrowImportWorkflowTestSuite) Test_EscrowImport_QAFails_Blocked() {
	var acts *activities.EscrowImportActivities

	s.env.OnActivity(acts.ValidateEscrowSource, mock.Anything, mock.Anything).Return(activities.ValidateEscrowSourceResult{Exists: true}, nil)
	s.env.OnActivity(acts.ParseAndExtractAssets, mock.Anything, mock.Anything).Return(activities.ParseAndExtractAssetsResult{
		RunPrefix: "escrow/com/20260624/test-wf",
		AssetKeys: map[string]string{"domains": "escrow/com/20260624/test-wf/domains.csv"},
		HasIssues: false,
	}, nil)
	s.env.OnActivity(acts.BuildStagingDatabase, mock.Anything, mock.Anything).Return(activities.BuildStagingDatabaseResult{DBKey: "escrow/com/20260624/test-wf/ryde.db"}, nil)
	s.env.OnActivity(acts.ResolveRegistrars, mock.Anything, mock.Anything).Return(activities.ResolveRegistrarsResult{DBKey: "escrow/com/20260624/test-wf/ryde.db"}, nil)
	s.env.OnActivity(acts.FinalizeStaging, mock.Anything, mock.Anything).Return(activities.FinalizeStagingResult{StagedDBKey: "escrow/com/20260624/test-wf/staged.db"}, nil)
	// QA FAILS
	s.env.OnActivity(acts.QAStagedDatabase, mock.Anything, mock.Anything).Return(activities.QAStagedDatabaseResult{Passed: false, QAReportKey: "escrow/com/20260624/test-wf/qa-report.json"}, nil)

	params := EscrowImportParams{
		TLD:       "com",
		ObjectKey: "uploads/com-escrow.xml",
	}

	s.env.ExecuteWorkflow(EscrowImportWorkflow, params)
	s.Require().True(s.env.IsWorkflowCompleted())

	err := s.env.GetWorkflowError()
	s.Require().Error(err)
	s.Contains(err.Error(), "validation checks")

	val, errQ := s.env.QueryWorkflow("state")
	s.Require().NoError(errQ)
	var state EscrowImportState
	s.Require().NoError(val.Get(&state))
	s.False(state.QAPassed)
	s.Equal("escrow/com/20260624/test-wf/qa-report.json", state.QAReportKey)
	s.Equal("failed", state.Phase)
}

func (s *EscrowImportWorkflowTestSuite) Test_EscrowImport_UserRejected() {
	var acts *activities.EscrowImportActivities

	s.env.OnActivity(acts.ValidateEscrowSource, mock.Anything, mock.Anything).Return(activities.ValidateEscrowSourceResult{Exists: true}, nil)
	s.env.OnActivity(acts.ParseAndExtractAssets, mock.Anything, mock.Anything).Return(activities.ParseAndExtractAssetsResult{
		RunPrefix: "escrow/com/20260624/test-wf",
		AssetKeys: map[string]string{"domains": "escrow/com/20260624/test-wf/domains.csv"},
		HasIssues: false,
	}, nil)
	s.env.OnActivity(acts.BuildStagingDatabase, mock.Anything, mock.Anything).Return(activities.BuildStagingDatabaseResult{DBKey: "escrow/com/20260624/test-wf/ryde.db"}, nil)
	s.env.OnActivity(acts.ResolveRegistrars, mock.Anything, mock.Anything).Return(activities.ResolveRegistrarsResult{DBKey: "escrow/com/20260624/test-wf/ryde.db"}, nil)
	s.env.OnActivity(acts.FinalizeStaging, mock.Anything, mock.Anything).Return(activities.FinalizeStagingResult{StagedDBKey: "escrow/com/20260624/test-wf/staged.db"}, nil)
	s.env.OnActivity(acts.QAStagedDatabase, mock.Anything, mock.Anything).Return(activities.QAStagedDatabaseResult{Passed: true, QAReportKey: "escrow/com/20260624/test-wf/qa-report.json"}, nil)

	// Send signal ConfirmEscrowImport (false) to abort import
	s.env.RegisterDelayedCallback(func() {
		s.env.SignalWorkflow("ConfirmEscrowImport", false)
	}, time.Millisecond*50)

	params := EscrowImportParams{
		TLD:       "com",
		ObjectKey: "uploads/com-escrow.xml",
	}

	s.env.ExecuteWorkflow(EscrowImportWorkflow, params)
	s.Require().True(s.env.IsWorkflowCompleted())

	err := s.env.GetWorkflowError()
	s.Require().Error(err)
	s.Contains(err.Error(), "aborted by user signal")

	val, errQ := s.env.QueryWorkflow("state")
	s.Require().NoError(errQ)
	var state EscrowImportState
	s.Require().NoError(val.Get(&state))
	s.True(state.QAPassed)
	s.Equal("aborted", state.Phase)
}

func TestEscrowImportWorkflowTestSuite(t *testing.T) {
	suite.Run(t, new(EscrowImportWorkflowTestSuite))
}
