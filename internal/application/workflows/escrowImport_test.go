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
	s.env.OnActivity(acts.ApplyRegistrarMappings, mock.Anything, mock.Anything).Return(activities.ApplyRegistrarMappingsResult{StagedDBKey: "escrow/com/20260624/test-wf/staged.db"}, nil)
	s.env.OnActivity(acts.QAStagedDatabase, mock.Anything, mock.Anything).Return(activities.QAStagedDatabaseResult{Passed: true, QAReportKey: "escrow/com/20260624/test-wf/qa-report.json"}, nil)

	// Ingestion mocks
	s.env.OnActivity(acts.IngestContacts, mock.Anything, mock.Anything).Return(activities.IngestContactsResult{Total: 10, Inserted: 8, Updated: 2}, nil)
	s.env.OnActivity(acts.IngestHosts, mock.Anything, mock.Anything).Return(activities.IngestHostsResult{Total: 5, Inserted: 5, Updated: 0}, nil)
	s.env.OnActivity(acts.IngestDomains, mock.Anything, mock.Anything).Return(activities.IngestDomainsResult{Total: 100, Inserted: 90, Updated: 10}, nil)
	s.env.OnActivity(acts.IngestNNDNs, mock.Anything, mock.Anything).Return(activities.IngestNNDNsResult{Total: 1, Inserted: 1, Updated: 0}, nil)
	s.env.OnActivity(acts.LinkDomainHosts, mock.Anything, mock.Anything).Return(activities.LinkDomainHostsResult{Total: 50}, nil)
	s.env.OnActivity(acts.AccreditRegistrars, mock.Anything, mock.Anything).Return(activities.AccreditRegistrarsResult{Total: 2}, nil)
	s.env.OnActivity(acts.PersistImportSummary, mock.Anything, mock.Anything).Return(activities.PersistImportSummaryResult{SummaryKey: "escrow/com/20260624/test-wf/summary.json"}, nil)
	s.env.OnActivity(acts.VerifyIngestion, mock.Anything, mock.Anything).Return(activities.VerifyIngestionResult{Passed: true, ReportKey: "escrow/com/20260624/test-wf/verification-report.json"}, nil)

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
	s.env.OnActivity(acts.ApplyRegistrarMappings, mock.Anything, mock.Anything).Return(activities.ApplyRegistrarMappingsResult{StagedDBKey: "escrow/com/20260624/test-wf/staged.db"}, nil)
	// QA FAILS
	s.env.OnActivity(acts.QAStagedDatabase, mock.Anything, mock.Anything).Return(activities.QAStagedDatabaseResult{Passed: false, QAReportKey: "escrow/com/20260624/test-wf/qa-report.json"}, nil)

	params := EscrowImportParams{
		TLD:       "com",
		ObjectKey: "uploads/com-escrow.xml",
	}

	s.env.ExecuteWorkflow(EscrowImportWorkflow, params)
	s.Require().True(s.env.IsWorkflowCompleted())

	// QA failure is an application-level outcome, not an infrastructure error.
	// The workflow completes successfully with QAPassed=false.
	s.Require().NoError(s.env.GetWorkflowError())

	var result EscrowImportResult
	s.Require().NoError(s.env.GetWorkflowResult(&result))
	s.False(result.QAPassed)
	s.False(result.Confirmed)
	s.Equal("escrow/com/20260624/test-wf/qa-report.json", result.QAReportKey)
	s.Equal("escrow/com/20260624/test-wf/staged.db", result.StagedDBKey)

	val, errQ := s.env.QueryWorkflow("state")
	s.Require().NoError(errQ)
	var state EscrowImportState
	s.Require().NoError(val.Get(&state))
	s.False(state.QAPassed)
	s.Equal("escrow/com/20260624/test-wf/qa-report.json", state.QAReportKey)
	s.Equal("qa_failed", state.Phase)
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
	s.env.OnActivity(acts.ApplyRegistrarMappings, mock.Anything, mock.Anything).Return(activities.ApplyRegistrarMappingsResult{StagedDBKey: "escrow/com/20260624/test-wf/staged.db"}, nil)
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

	// Rejection is a clean completion (not an error) — same pattern as QA failure
	err := s.env.GetWorkflowError()
	s.Require().NoError(err)

	var result EscrowImportResult
	s.Require().NoError(s.env.GetWorkflowResult(&result))
	s.False(result.Confirmed, "confirmed should be false for rejected import")
	s.True(result.QAPassed, "QA should have passed before rejection")
	s.NotEmpty(result.StagedDBKey)

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

// Test_EscrowImport_MappingGaps_PropagatedToQAResult verifies that when
// ResolveRegistrars reports unmapped registrars, the gaps and counts are
// propagated through to the QA-failed result so the UI can display them.
func (s *EscrowImportWorkflowTestSuite) Test_EscrowImport_MappingGaps_PropagatedToQAResult() {
	var acts *activities.EscrowImportActivities

	unmapped := []activities.UnmappedRegistrar{
		{EscrowID: "JISC5800", Name: "Jisc Services", GurID: 0, DomainCount: 0, HostCount: 299, ContactCount: 0},
		{EscrowID: "JISC5002", Name: "34SP.com", GurID: 0, DomainCount: 0, HostCount: 15, ContactCount: 0},
	}

	s.env.OnActivity(acts.ValidateEscrowSource, mock.Anything, mock.Anything).Return(activities.ValidateEscrowSourceResult{Exists: true}, nil)
	s.env.OnActivity(acts.ParseAndExtractAssets, mock.Anything, mock.Anything).Return(activities.ParseAndExtractAssetsResult{
		RunPrefix: "escrow/best/20260625/test-wf",
		AssetKeys: map[string]string{"domains": "escrow/best/20260625/test-wf/domains.csv"},
	}, nil)
	s.env.OnActivity(acts.BuildStagingDatabase, mock.Anything, mock.Anything).Return(activities.BuildStagingDatabaseResult{DBKey: "escrow/best/20260625/test-wf/ryde.db"}, nil)
	s.env.OnActivity(acts.ResolveRegistrars, mock.Anything, mock.Anything).Return(activities.ResolveRegistrarsResult{
		DBKey:              "escrow/best/20260625/test-wf/ryde.db",
		HasIssues:          true,
		TotalRegistrars:    529,
		MappedCount:        434,
		UnmappedRegistrars: unmapped,
	}, nil)
	s.env.OnActivity(acts.ApplyRegistrarMappings, mock.Anything, mock.Anything).Return(activities.ApplyRegistrarMappingsResult{StagedDBKey: "escrow/best/20260625/test-wf/staged.db"}, nil)
	// QA fails due to NULL clIDs from unmapped registrars
	s.env.OnActivity(acts.QAStagedDatabase, mock.Anything, mock.Anything).Return(activities.QAStagedDatabaseResult{Passed: false, QAReportKey: "escrow/best/20260625/test-wf/qa-report.json"}, nil)

	s.env.ExecuteWorkflow(EscrowImportWorkflow, EscrowImportParams{
		TLD:       "best",
		ObjectKey: "uploads/best-escrow.xml",
	})
	s.Require().True(s.env.IsWorkflowCompleted())
	s.Require().NoError(s.env.GetWorkflowError())

	// Result should carry mapping gaps
	var result EscrowImportResult
	s.Require().NoError(s.env.GetWorkflowResult(&result))
	s.False(result.QAPassed)
	s.Equal(529, result.TotalRegistrars)
	s.Equal(434, result.MappedRegistrars)
	s.Require().Len(result.UnmappedRegistrars, 2)
	s.Equal("JISC5800", result.UnmappedRegistrars[0].EscrowID)
	s.Equal(299, result.UnmappedRegistrars[0].HostCount)

	// State should also carry mapping gaps
	val, errQ := s.env.QueryWorkflow("state")
	s.Require().NoError(errQ)
	var state EscrowImportState
	s.Require().NoError(val.Get(&state))
	s.Equal("qa_failed", state.Phase)
	s.Equal(529, state.TotalRegistrars)
	s.Equal(434, state.MappedRegistrars)
	s.Require().Len(state.UnmappedRegistrars, 2)
}

// Test_EscrowImport_AllMapped_CountsPropagated verifies that when all
// registrars are mapped, the total/mapped counts are still propagated
// (so the UI can show "434 of 434 mapped") and UnmappedRegistrars is empty.
func (s *EscrowImportWorkflowTestSuite) Test_EscrowImport_AllMapped_CountsPropagated() {
	var acts *activities.EscrowImportActivities

	s.env.OnActivity(acts.ValidateEscrowSource, mock.Anything, mock.Anything).Return(activities.ValidateEscrowSourceResult{Exists: true}, nil)
	s.env.OnActivity(acts.ParseAndExtractAssets, mock.Anything, mock.Anything).Return(activities.ParseAndExtractAssetsResult{
		RunPrefix: "escrow/com/20260625/test-wf",
		AssetKeys: map[string]string{"domains": "escrow/com/20260625/test-wf/domains.csv"},
	}, nil)
	s.env.OnActivity(acts.BuildStagingDatabase, mock.Anything, mock.Anything).Return(activities.BuildStagingDatabaseResult{DBKey: "escrow/com/20260625/test-wf/ryde.db"}, nil)
	s.env.OnActivity(acts.ResolveRegistrars, mock.Anything, mock.Anything).Return(activities.ResolveRegistrarsResult{
		DBKey:           "escrow/com/20260625/test-wf/ryde.db",
		HasIssues:       false,
		TotalRegistrars: 50,
		MappedCount:     50,
	}, nil)
	s.env.OnActivity(acts.ApplyRegistrarMappings, mock.Anything, mock.Anything).Return(activities.ApplyRegistrarMappingsResult{StagedDBKey: "escrow/com/20260625/test-wf/staged.db"}, nil)
	s.env.OnActivity(acts.QAStagedDatabase, mock.Anything, mock.Anything).Return(activities.QAStagedDatabaseResult{Passed: true, QAReportKey: "escrow/com/20260625/test-wf/qa-report.json"}, nil)
	s.env.OnActivity(acts.IngestContacts, mock.Anything, mock.Anything).Return(activities.IngestContactsResult{}, nil)
	s.env.OnActivity(acts.IngestHosts, mock.Anything, mock.Anything).Return(activities.IngestHostsResult{}, nil)
	s.env.OnActivity(acts.IngestDomains, mock.Anything, mock.Anything).Return(activities.IngestDomainsResult{}, nil)
	s.env.OnActivity(acts.IngestNNDNs, mock.Anything, mock.Anything).Return(activities.IngestNNDNsResult{}, nil)
	s.env.OnActivity(acts.LinkDomainHosts, mock.Anything, mock.Anything).Return(activities.LinkDomainHostsResult{}, nil)
	s.env.OnActivity(acts.AccreditRegistrars, mock.Anything, mock.Anything).Return(activities.AccreditRegistrarsResult{}, nil)
	s.env.OnActivity(acts.PersistImportSummary, mock.Anything, mock.Anything).Return(activities.PersistImportSummaryResult{}, nil)
	s.env.OnActivity(acts.VerifyIngestion, mock.Anything, mock.Anything).Return(activities.VerifyIngestionResult{Passed: true}, nil)

	s.env.RegisterDelayedCallback(func() {
		s.env.SignalWorkflow("ConfirmEscrowImport", true)
	}, time.Millisecond*50)

	s.env.ExecuteWorkflow(EscrowImportWorkflow, EscrowImportParams{
		TLD:       "com",
		ObjectKey: "uploads/com-escrow.xml",
	})
	s.Require().True(s.env.IsWorkflowCompleted())
	s.Require().NoError(s.env.GetWorkflowError())

	// State should have counts even when all mapped
	val, errQ := s.env.QueryWorkflow("state")
	s.Require().NoError(errQ)
	var state EscrowImportState
	s.Require().NoError(val.Get(&state))
	s.Equal(50, state.TotalRegistrars)
	s.Equal(50, state.MappedRegistrars)
	s.Empty(state.UnmappedRegistrars)
}
