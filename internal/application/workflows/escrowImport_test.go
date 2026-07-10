package workflows

import (
	"context"
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
	s.env.OnActivity(acts.ValidateRegistrantRefs, mock.Anything, mock.Anything).Return(activities.ValidateRegistrantRefsResult{}, nil)
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
	s.env.OnActivity(acts.ValidateRegistrantRefs, mock.Anything, mock.Anything).Return(activities.ValidateRegistrantRefsResult{}, nil)
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

// Test_EscrowImport_OverrideSignal_ResolvesAllGaps verifies that when
// ProvideRegistrarOverrides signal is received, the workflow re-runs
// ResolveRegistrars with overrides and proceeds to QA.
func (s *EscrowImportWorkflowTestSuite) Test_EscrowImport_OverrideSignal_ResolvesAllGaps() {
	var acts *activities.EscrowImportActivities

	unmapped := []activities.UnmappedRegistrar{
		{EscrowID: "30619730", Name: "Mi.com.co", GurID: 21607, DomainCount: 42417, HostCount: 399, ContactCount: 28850},
		{EscrowID: "1-140WH7", Name: ".CO Internet S.A.S.", GurID: 111111, DomainCount: 26786, HostCount: 837, ContactCount: 31636},
	}

	s.env.OnActivity(acts.ValidateEscrowSource, mock.Anything, mock.Anything).Return(activities.ValidateEscrowSourceResult{Exists: true}, nil)
	s.env.OnActivity(acts.ParseAndExtractAssets, mock.Anything, mock.Anything).Return(activities.ParseAndExtractAssetsResult{
		RunPrefix: "escrow/co/20260625/test-wf",
		AssetKeys: map[string]string{"domains": "escrow/co/20260625/test-wf/domains.csv"},
	}, nil)
	s.env.OnActivity(acts.BuildStagingDatabase, mock.Anything, mock.Anything).Return(activities.BuildStagingDatabaseResult{DBKey: "escrow/co/20260625/test-wf/ryde.db"}, nil)

	// First call: returns unmapped registrars with domains
	// Second call (after overrides): all mapped
	resolveCallCount := 0
	s.env.OnActivity(acts.ResolveRegistrars, mock.Anything, mock.Anything).Return(func(_ context.Context, args activities.ResolveRegistrarsArgs) (activities.ResolveRegistrarsResult, error) {
		resolveCallCount++
		if resolveCallCount == 1 {
			return activities.ResolveRegistrarsResult{
				DBKey:              "escrow/co/20260625/test-wf/ryde.db",
				HasIssues:          true,
				TotalRegistrars:    390,
				MappedCount:        358,
				UnmappedRegistrars: unmapped,
			}, nil
		}
		// Second call: overrides resolved everything
		s.Require().NotEmpty(args.Overrides, "overrides should be populated on re-resolve")
		return activities.ResolveRegistrarsResult{
			DBKey:           "escrow/co/20260625/test-wf/ryde.db",
			HasIssues:       false,
			TotalRegistrars: 390,
			MappedCount:     390,
		}, nil
	})

	s.env.OnActivity(acts.ApplyRegistrarMappings, mock.Anything, mock.Anything).Return(activities.ApplyRegistrarMappingsResult{StagedDBKey: "escrow/co/20260625/test-wf/staged.db"}, nil)
	s.env.OnActivity(acts.QAStagedDatabase, mock.Anything, mock.Anything).Return(activities.QAStagedDatabaseResult{Passed: true, QAReportKey: "escrow/co/20260625/test-wf/qa-report.json"}, nil)
	s.env.OnActivity(acts.IngestContacts, mock.Anything, mock.Anything).Return(activities.IngestContactsResult{}, nil)
	s.env.OnActivity(acts.ValidateRegistrantRefs, mock.Anything, mock.Anything).Return(activities.ValidateRegistrantRefsResult{}, nil)
	s.env.OnActivity(acts.IngestHosts, mock.Anything, mock.Anything).Return(activities.IngestHostsResult{}, nil)
	s.env.OnActivity(acts.IngestDomains, mock.Anything, mock.Anything).Return(activities.IngestDomainsResult{}, nil)
	s.env.OnActivity(acts.IngestNNDNs, mock.Anything, mock.Anything).Return(activities.IngestNNDNsResult{}, nil)
	s.env.OnActivity(acts.LinkDomainHosts, mock.Anything, mock.Anything).Return(activities.LinkDomainHostsResult{}, nil)
	s.env.OnActivity(acts.AccreditRegistrars, mock.Anything, mock.Anything).Return(activities.AccreditRegistrarsResult{}, nil)
	s.env.OnActivity(acts.PersistImportSummary, mock.Anything, mock.Anything).Return(activities.PersistImportSummaryResult{}, nil)
	s.env.OnActivity(acts.VerifyIngestion, mock.Anything, mock.Anything).Return(activities.VerifyIngestionResult{Passed: true}, nil)

	// Signal 1: Provide overrides when workflow pauses at pending_registrar_overrides
	s.env.RegisterDelayedCallback(func() {
		s.env.SignalWorkflow("ProvideRegistrarOverrides", map[string]interface{}{
			"Mi.com.co":           "21607-micomco",
			".CO Internet S.A.S.": "111111-co-inter",
		})
	}, time.Millisecond*50)

	// Signal 2: Confirm ingestion after QA passes
	s.env.RegisterDelayedCallback(func() {
		s.env.SignalWorkflow("ConfirmEscrowImport", true)
	}, time.Millisecond*100)

	s.env.ExecuteWorkflow(EscrowImportWorkflow, EscrowImportParams{
		TLD:       "co",
		ObjectKey: "uploads/co-escrow.xml",
	})
	s.Require().True(s.env.IsWorkflowCompleted())
	s.Require().NoError(s.env.GetWorkflowError())

	var result EscrowImportResult
	s.Require().NoError(s.env.GetWorkflowResult(&result))
	s.True(result.Confirmed)
	s.True(result.QAPassed)
	s.Equal(390, result.TotalRegistrars)
	s.Equal(390, result.MappedRegistrars)
	s.Empty(result.UnmappedRegistrars)
	s.NotEmpty(result.OverridesProvided, "overrides should be recorded in result")
	s.Equal(2, resolveCallCount, "ResolveRegistrars should be called twice")
}

// Test_EscrowImport_SkipOverrides_ProceedsToQA verifies that when
// SkipRegistrarOverrides signal is received, the workflow skips re-resolution
// and proceeds directly to ApplyRegistrarMappings → QA.
func (s *EscrowImportWorkflowTestSuite) Test_EscrowImport_SkipOverrides_ProceedsToQA() {
	var acts *activities.EscrowImportActivities

	unmapped := []activities.UnmappedRegistrar{
		{EscrowID: "30619730", Name: "Mi.com.co", GurID: 21607, DomainCount: 42417, HostCount: 399, ContactCount: 28850},
	}

	s.env.OnActivity(acts.ValidateEscrowSource, mock.Anything, mock.Anything).Return(activities.ValidateEscrowSourceResult{Exists: true}, nil)
	s.env.OnActivity(acts.ParseAndExtractAssets, mock.Anything, mock.Anything).Return(activities.ParseAndExtractAssetsResult{
		RunPrefix: "escrow/co/20260625/test-wf",
		AssetKeys: map[string]string{"domains": "escrow/co/20260625/test-wf/domains.csv"},
	}, nil)
	s.env.OnActivity(acts.BuildStagingDatabase, mock.Anything, mock.Anything).Return(activities.BuildStagingDatabaseResult{DBKey: "escrow/co/20260625/test-wf/ryde.db"}, nil)
	s.env.OnActivity(acts.ResolveRegistrars, mock.Anything, mock.Anything).Return(activities.ResolveRegistrarsResult{
		DBKey:              "escrow/co/20260625/test-wf/ryde.db",
		HasIssues:          true,
		TotalRegistrars:    390,
		MappedCount:        358,
		UnmappedRegistrars: unmapped,
	}, nil)
	s.env.OnActivity(acts.ApplyRegistrarMappings, mock.Anything, mock.Anything).Return(activities.ApplyRegistrarMappingsResult{StagedDBKey: "escrow/co/20260625/test-wf/staged.db"}, nil)
	// QA fails because domains are unmapped
	s.env.OnActivity(acts.QAStagedDatabase, mock.Anything, mock.Anything).Return(activities.QAStagedDatabaseResult{Passed: false, QAReportKey: "escrow/co/20260625/test-wf/qa-report.json"}, nil)

	// Signal: Skip overrides
	s.env.RegisterDelayedCallback(func() {
		s.env.SignalWorkflow("SkipRegistrarOverrides", true)
	}, time.Millisecond*50)

	s.env.ExecuteWorkflow(EscrowImportWorkflow, EscrowImportParams{
		TLD:       "co",
		ObjectKey: "uploads/co-escrow.xml",
	})
	s.Require().True(s.env.IsWorkflowCompleted())
	s.Require().NoError(s.env.GetWorkflowError())

	var result EscrowImportResult
	s.Require().NoError(s.env.GetWorkflowResult(&result))
	s.False(result.QAPassed, "QA should fail because domains remain unmapped")
	s.False(result.Confirmed)
	s.Equal(358, result.MappedRegistrars)
	s.Require().Len(result.UnmappedRegistrars, 1)
}

// Test_EscrowImport_NoDomainsUnmapped_AutoSkipsGate verifies that when
// unmapped registrars exist but none have domains, the override gate is
// skipped automatically (no signal needed).
func (s *EscrowImportWorkflowTestSuite) Test_EscrowImport_NoDomainsUnmapped_AutoSkipsGate() {
	var acts *activities.EscrowImportActivities

	// Unmapped registrars with only hosts/contacts (no domains)
	unmapped := []activities.UnmappedRegistrar{
		{EscrowID: "JISC5800", Name: "Jisc Services", GurID: 0, DomainCount: 0, HostCount: 299, ContactCount: 0},
		{EscrowID: "dotUSRegServ", Name: ".us Registry Services LLC", GurID: 1111112, DomainCount: 0, HostCount: 0, ContactCount: 1},
	}

	s.env.OnActivity(acts.ValidateEscrowSource, mock.Anything, mock.Anything).Return(activities.ValidateEscrowSourceResult{Exists: true}, nil)
	s.env.OnActivity(acts.ParseAndExtractAssets, mock.Anything, mock.Anything).Return(activities.ParseAndExtractAssetsResult{
		RunPrefix: "escrow/co/20260625/test-wf",
		AssetKeys: map[string]string{"domains": "escrow/co/20260625/test-wf/domains.csv"},
	}, nil)
	s.env.OnActivity(acts.BuildStagingDatabase, mock.Anything, mock.Anything).Return(activities.BuildStagingDatabaseResult{DBKey: "escrow/co/20260625/test-wf/ryde.db"}, nil)
	s.env.OnActivity(acts.ResolveRegistrars, mock.Anything, mock.Anything).Return(activities.ResolveRegistrarsResult{
		DBKey:              "escrow/co/20260625/test-wf/ryde.db",
		HasIssues:          true,
		TotalRegistrars:    390,
		MappedCount:        388,
		UnmappedRegistrars: unmapped,
	}, nil)
	s.env.OnActivity(acts.ApplyRegistrarMappings, mock.Anything, mock.Anything).Return(activities.ApplyRegistrarMappingsResult{StagedDBKey: "escrow/co/20260625/test-wf/staged.db"}, nil)
	s.env.OnActivity(acts.QAStagedDatabase, mock.Anything, mock.Anything).Return(activities.QAStagedDatabaseResult{Passed: true, QAReportKey: "escrow/co/20260625/test-wf/qa-report.json"}, nil)
	s.env.OnActivity(acts.IngestContacts, mock.Anything, mock.Anything).Return(activities.IngestContactsResult{}, nil)
	s.env.OnActivity(acts.ValidateRegistrantRefs, mock.Anything, mock.Anything).Return(activities.ValidateRegistrantRefsResult{}, nil)
	s.env.OnActivity(acts.IngestHosts, mock.Anything, mock.Anything).Return(activities.IngestHostsResult{}, nil)
	s.env.OnActivity(acts.IngestDomains, mock.Anything, mock.Anything).Return(activities.IngestDomainsResult{}, nil)
	s.env.OnActivity(acts.IngestNNDNs, mock.Anything, mock.Anything).Return(activities.IngestNNDNsResult{}, nil)
	s.env.OnActivity(acts.LinkDomainHosts, mock.Anything, mock.Anything).Return(activities.LinkDomainHostsResult{}, nil)
	s.env.OnActivity(acts.AccreditRegistrars, mock.Anything, mock.Anything).Return(activities.AccreditRegistrarsResult{}, nil)
	s.env.OnActivity(acts.PersistImportSummary, mock.Anything, mock.Anything).Return(activities.PersistImportSummaryResult{}, nil)
	s.env.OnActivity(acts.VerifyIngestion, mock.Anything, mock.Anything).Return(activities.VerifyIngestionResult{Passed: true}, nil)

	// Only one signal needed — no override gate expected (auto-skipped)
	s.env.RegisterDelayedCallback(func() {
		s.env.SignalWorkflow("ConfirmEscrowImport", true)
	}, time.Millisecond*50)

	s.env.ExecuteWorkflow(EscrowImportWorkflow, EscrowImportParams{
		TLD:       "co",
		ObjectKey: "uploads/co-escrow.xml",
	})
	s.Require().True(s.env.IsWorkflowCompleted())
	s.Require().NoError(s.env.GetWorkflowError())

	var result EscrowImportResult
	s.Require().NoError(s.env.GetWorkflowResult(&result))
	s.True(result.Confirmed, "should proceed to ingestion without override signal")
	s.True(result.QAPassed)
	s.Empty(result.OverridesProvided, "no overrides should be set when gate is auto-skipped")
}

// Test_EscrowImport_PartialOverrides_ReducesUnmapped verifies that when
// only some overrides are provided, the remaining unmapped registrars are
// propagated through to the result.
func (s *EscrowImportWorkflowTestSuite) Test_EscrowImport_PartialOverrides_ReducesUnmapped() {
	var acts *activities.EscrowImportActivities

	unmapped := []activities.UnmappedRegistrar{
		{EscrowID: "30619730", Name: "Mi.com.co", GurID: 21607, DomainCount: 42417, HostCount: 399, ContactCount: 28850},
		{EscrowID: "1-140WH7", Name: ".CO Internet S.A.S.", GurID: 111111, DomainCount: 26786, HostCount: 837, ContactCount: 31636},
		{EscrowID: "1-14674W", Name: "Central Comercializadora", GurID: 88888, DomainCount: 1472, HostCount: 3, ContactCount: 6225},
	}

	s.env.OnActivity(acts.ValidateEscrowSource, mock.Anything, mock.Anything).Return(activities.ValidateEscrowSourceResult{Exists: true}, nil)
	s.env.OnActivity(acts.ParseAndExtractAssets, mock.Anything, mock.Anything).Return(activities.ParseAndExtractAssetsResult{
		RunPrefix: "escrow/co/20260625/test-wf",
		AssetKeys: map[string]string{"domains": "escrow/co/20260625/test-wf/domains.csv"},
	}, nil)
	s.env.OnActivity(acts.BuildStagingDatabase, mock.Anything, mock.Anything).Return(activities.BuildStagingDatabaseResult{DBKey: "escrow/co/20260625/test-wf/ryde.db"}, nil)

	// First call: 3 unmapped; Second call: 1 still unmapped (partial resolution)
	resolveCallCount := 0
	s.env.OnActivity(acts.ResolveRegistrars, mock.Anything, mock.Anything).Return(func(_ context.Context, _ activities.ResolveRegistrarsArgs) (activities.ResolveRegistrarsResult, error) {
		resolveCallCount++
		if resolveCallCount == 1 {
			return activities.ResolveRegistrarsResult{
				DBKey:              "escrow/co/20260625/test-wf/ryde.db",
				HasIssues:          true,
				TotalRegistrars:    390,
				MappedCount:        358,
				UnmappedRegistrars: unmapped,
			}, nil
		}
		// Second call: 2 of 3 resolved via overrides, 1 remains
		return activities.ResolveRegistrarsResult{
			DBKey:           "escrow/co/20260625/test-wf/ryde.db",
			HasIssues:       true,
			TotalRegistrars: 390,
			MappedCount:     360,
			UnmappedRegistrars: []activities.UnmappedRegistrar{
				{EscrowID: "1-14674W", Name: "Central Comercializadora", GurID: 88888, DomainCount: 1472, HostCount: 3, ContactCount: 6225},
			},
		}, nil
	})

	s.env.OnActivity(acts.ApplyRegistrarMappings, mock.Anything, mock.Anything).Return(activities.ApplyRegistrarMappingsResult{StagedDBKey: "escrow/co/20260625/test-wf/staged.db"}, nil)
	s.env.OnActivity(acts.QAStagedDatabase, mock.Anything, mock.Anything).Return(activities.QAStagedDatabaseResult{Passed: false, QAReportKey: "escrow/co/20260625/test-wf/qa-report.json"}, nil)

	// Provide partial overrides (only 2 of 3)
	s.env.RegisterDelayedCallback(func() {
		s.env.SignalWorkflow("ProvideRegistrarOverrides", map[string]interface{}{
			"Mi.com.co":           "21607-micomco",
			".CO Internet S.A.S.": "111111-co-inter",
		})
	}, time.Millisecond*50)

	s.env.ExecuteWorkflow(EscrowImportWorkflow, EscrowImportParams{
		TLD:       "co",
		ObjectKey: "uploads/co-escrow.xml",
	})
	s.Require().True(s.env.IsWorkflowCompleted())
	s.Require().NoError(s.env.GetWorkflowError())

	var result EscrowImportResult
	s.Require().NoError(s.env.GetWorkflowResult(&result))
	s.False(result.QAPassed, "QA should fail with remaining unmapped registrar")
	s.Equal(360, result.MappedRegistrars)
	s.Require().Len(result.UnmappedRegistrars, 1, "should have 1 remaining unmapped registrar")
	s.Equal("1-14674W", result.UnmappedRegistrars[0].EscrowID)
	s.Require().Len(result.OverridesProvided, 2, "should record the 2 overrides provided")
}
