package workflows

import (
	"fmt"
	"testing"

	"github.com/onasunnymorning/domain-os/internal/application/activities"
	"github.com/onasunnymorning/domain-os/internal/application/commands"
	"github.com/onasunnymorning/domain-os/internal/infrastructure/web/icannregistrars"
	"github.com/onasunnymorning/domain-os/internal/interface/rest/response"
	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"go.temporal.io/sdk/testsuite"
)

type SyncRegistrarsWorkflowTestSuite struct {
	suite.Suite
	testsuite.WorkflowTestSuite
	env *testsuite.TestWorkflowEnvironment
}

func (s *SyncRegistrarsWorkflowTestSuite) SetupTest() {
	s.env = s.NewTestWorkflowEnvironment()
	s.env.RegisterWorkflow(SyncRegistrarsWorkflow)
}

func (s *SyncRegistrarsWorkflowTestSuite) Test_SyncRegistrars_FirstImport() {
	// SyncIana succeeds
	s.env.OnActivity(activities.SyncIanaRegistrars, mock.Anything, mock.Anything).Return(nil)

	// Count is 0 (first import)
	s.env.OnActivity(activities.CountRegistrars, mock.Anything, mock.Anything).Return(&response.CountResult{Count: 0}, nil)

	// Get ICANN and IANA
	csvRars := []icannregistrars.CSVRegistrar{{Name: "rar1"}}
	s.env.OnActivity(activities.GetICANNRegistrars, mock.Anything, mock.Anything, "./initdata/icannRegistrarList.csv").Return(csvRars, nil)

	ianaRars := []entities.IANARegistrar{{Name: "iana1", GurID: 1}}
	s.env.OnActivity(activities.GetIANARegistrars, mock.Anything, mock.Anything, mock.Anything).Return(ianaRars, nil)

	// Make creates
	createCmds := []commands.CreateRegistrarCommand{{ClID: "CL1"}}
	s.env.OnActivity(activities.MakeCreateRegistrarCommands, mock.Anything, mock.Anything, csvRars, ianaRars).Return(createCmds, nil)

	// Bulk Create
	s.env.OnActivity(activities.BulkCreateRegistrars, mock.Anything, mock.Anything, createCmds).Return(nil)

	s.env.ExecuteWorkflow(SyncRegistrarsWorkflow, SyncRegistrarsParams{})
	s.Require().True(s.env.IsWorkflowCompleted())
	s.Require().NoError(s.env.GetWorkflowError())

	var result SyncRegistrarsResult
	s.Require().NoError(s.env.GetWorkflowResult(&result))
	s.Equal(1, result.Created)
	s.Equal(1, result.TotalProcessed)
	s.Equal(0, result.Failed)
	s.Equal(1, result.TotalIANA)
	s.Equal(0, result.TotalExisting)
	// Verify created items detail
	s.Require().Len(result.CreatedItems, 1)
	s.Equal("CL1", result.CreatedItems[0].ClID)
}

func (s *SyncRegistrarsWorkflowTestSuite) Test_SyncRegistrars_UpdatePath_Success() {
	s.env.OnActivity(activities.SyncIanaRegistrars, mock.Anything, mock.Anything).Return(nil)

	// Count is 5 (not first import)
	s.env.OnActivity(activities.CountRegistrars, mock.Anything, mock.Anything).Return(&response.CountResult{Count: 5}, nil)

	// Get IANA and existing
	ianaRars := []entities.IANARegistrar{
		{Name: "iana1", GurID: 1, Status: entities.IANARegistrarStatusAccredited},
		{Name: "iana2", GurID: 2, Status: entities.IANARegistrarStatusTerminated},
	}
	s.env.OnActivity(activities.GetIANARegistrars, mock.Anything, mock.Anything, mock.Anything).Return(ianaRars, nil)

	existingRars := []entities.RegistrarListItem{
		{ClID: entities.ClIDType("1-iana1"), Status: entities.RegistrarStatusOK, IANAStatus: entities.IANARegistrarStatusAccredited},
	}
	s.env.OnActivity(activities.GetRegistrarListItems, mock.Anything, mock.Anything, mock.Anything).Return(existingRars, nil)

	// Diff plan: one create (iana2 is new), no updates (iana1 is already OK/Accredited)
	plan := activities.DiffPlanResult{
		Creates: []commands.CreateRegistrarCommand{{ClID: "2-iana2", GurID: 2, Name: "iana2"}},
		Updates: []commands.UpdateRegistrarStatusCommand{},
	}
	s.env.OnActivity(activities.DiffAndPlanRegistrars, mock.Anything, mock.Anything, ianaRars, existingRars).Return(plan, nil)

	// Bulk Create the creates
	s.env.OnActivity(activities.BulkCreateRegistrars, mock.Anything, mock.Anything, plan.Creates).Return(nil)

	// No bulk update because Updates is empty

	s.env.ExecuteWorkflow(SyncRegistrarsWorkflow, SyncRegistrarsParams{})
	s.Require().True(s.env.IsWorkflowCompleted())
	s.Require().NoError(s.env.GetWorkflowError())

	var result SyncRegistrarsResult
	s.Require().NoError(s.env.GetWorkflowResult(&result))
	s.Equal(1, result.Created)
	s.Equal(0, result.Updated)
	s.Equal(0, result.Failed)
	s.Equal(1, result.TotalProcessed)
	s.Equal(2, result.TotalIANA)
	s.Equal(1, result.TotalExisting)
	s.Equal(1, result.Unchanged) // iana1 was already in sync
}

func (s *SyncRegistrarsWorkflowTestSuite) Test_SyncRegistrars_UpdatePath_WithStatusUpdates() {
	s.env.OnActivity(activities.SyncIanaRegistrars, mock.Anything, mock.Anything).Return(nil)
	s.env.OnActivity(activities.CountRegistrars, mock.Anything, mock.Anything).Return(&response.CountResult{Count: 5}, nil)

	ianaRars := []entities.IANARegistrar{
		{Name: "iana1", GurID: 1, Status: entities.IANARegistrarStatusTerminated},
	}
	s.env.OnActivity(activities.GetIANARegistrars, mock.Anything, mock.Anything, mock.Anything).Return(ianaRars, nil)

	existingRars := []entities.RegistrarListItem{
		{ClID: entities.ClIDType("1-iana1"), Status: entities.RegistrarStatusOK, IANAStatus: entities.IANARegistrarStatusAccredited},
	}
	s.env.OnActivity(activities.GetRegistrarListItems, mock.Anything, mock.Anything, mock.Anything).Return(existingRars, nil)

	// Diff plan: platform and IANA status both changed
	plan := activities.DiffPlanResult{
		Creates: []commands.CreateRegistrarCommand{},
		Updates: []commands.UpdateRegistrarStatusCommand{
			{ClID: "1-iana1", NewStatus: "terminated", OldStatus: "ok", NewIANAStatus: "Terminated", OldIANAStatus: "Accredited"},
		},
	}
	s.env.OnActivity(activities.DiffAndPlanRegistrars, mock.Anything, mock.Anything, ianaRars, existingRars).Return(plan, nil)

	// Bulk status update succeeds
	bulkResult := activities.BulkUpdateResult{
		Updated:    1,
		Failed:     0,
		UpdatedIDs: []string{"1-iana1"},
		Errors:     []activities.BulkUpdateError{},
	}
	s.env.OnActivity(activities.BulkUpdateRegistrarStatuses, mock.Anything, mock.Anything, plan.Updates).Return(bulkResult, nil)

	s.env.ExecuteWorkflow(SyncRegistrarsWorkflow, SyncRegistrarsParams{})
	s.Require().True(s.env.IsWorkflowCompleted())
	s.Require().NoError(s.env.GetWorkflowError())

	var result SyncRegistrarsResult
	s.Require().NoError(s.env.GetWorkflowResult(&result))
	s.Equal(0, result.Created)
	s.Equal(1, result.Updated)
	s.Equal(0, result.Failed)
	s.Equal(1, result.TotalProcessed)
	s.Equal(0, result.Unchanged)
	// Verify updated items detail
	s.Require().Len(result.UpdatedItems, 1)
	s.Equal("1-iana1", result.UpdatedItems[0].ClID)
	s.Equal("ok", result.UpdatedItems[0].OldStatus)
	s.Equal("terminated", result.UpdatedItems[0].NewStatus)
	s.Equal("Accredited", result.UpdatedItems[0].OldIANAStatus)
	s.Equal("Terminated", result.UpdatedItems[0].NewIANAStatus)
}

func (s *SyncRegistrarsWorkflowTestSuite) Test_SyncRegistrars_UpdatePath_WithPartialFailures() {
	s.env.OnActivity(activities.SyncIanaRegistrars, mock.Anything, mock.Anything).Return(nil)
	s.env.OnActivity(activities.CountRegistrars, mock.Anything, mock.Anything).Return(&response.CountResult{Count: 5}, nil)

	ianaRars := []entities.IANARegistrar{
		{Name: "iana1", GurID: 1, Status: entities.IANARegistrarStatusTerminated},
	}
	s.env.OnActivity(activities.GetIANARegistrars, mock.Anything, mock.Anything, mock.Anything).Return(ianaRars, nil)

	existingRars := []entities.RegistrarListItem{
		{ClID: entities.ClIDType("1-iana1"), Status: entities.RegistrarStatusOK, IANAStatus: entities.IANARegistrarStatusAccredited},
	}
	s.env.OnActivity(activities.GetRegistrarListItems, mock.Anything, mock.Anything, mock.Anything).Return(existingRars, nil)

	plan := activities.DiffPlanResult{
		Creates: []commands.CreateRegistrarCommand{},
		Updates: []commands.UpdateRegistrarStatusCommand{
			{ClID: "1-iana1", NewStatus: "terminated", OldStatus: "ok", NewIANAStatus: "Terminated", OldIANAStatus: "Accredited"},
		},
	}
	s.env.OnActivity(activities.DiffAndPlanRegistrars, mock.Anything, mock.Anything, ianaRars, existingRars).Return(plan, nil)

	// Bulk update with partial failure: status update OK but IANA status fails
	bulkResult := activities.BulkUpdateResult{
		Updated:    0,
		Failed:     1,
		UpdatedIDs: []string{},
		Errors: []activities.BulkUpdateError{
			{ClID: "1-iana1", Operation: "update-iana-status", Error: "HTTP 500 Internal Server Error"},
		},
	}
	s.env.OnActivity(activities.BulkUpdateRegistrarStatuses, mock.Anything, mock.Anything, plan.Updates).Return(bulkResult, nil)

	s.env.ExecuteWorkflow(SyncRegistrarsWorkflow, SyncRegistrarsParams{})
	s.Require().True(s.env.IsWorkflowCompleted())
	s.Require().NoError(s.env.GetWorkflowError())

	var result SyncRegistrarsResult
	s.Require().NoError(s.env.GetWorkflowResult(&result))
	s.Equal(0, result.Created)
	s.Equal(0, result.Updated)
	s.Equal(1, result.Failed)
	s.Equal(1, result.TotalProcessed)

	s.Require().Len(result.Failures, 1)
	s.Equal("1-iana1", result.Failures[0].ClID)
	s.Equal("update-iana-status", result.Failures[0].Operation)
	s.Contains(result.Failures[0].Error, "HTTP 500")
	s.Contains(result.Notes, "Completed with failures — review the failures list for details")
}

func (s *SyncRegistrarsWorkflowTestSuite) Test_SyncRegistrars_DryRun_Incremental() {
	s.env.OnActivity(activities.SyncIanaRegistrars, mock.Anything, mock.Anything).Return(nil)
	s.env.OnActivity(activities.CountRegistrars, mock.Anything, mock.Anything).Return(&response.CountResult{Count: 5}, nil)

	ianaRars := []entities.IANARegistrar{
		{Name: "iana1", GurID: 1, Status: entities.IANARegistrarStatusAccredited},
		{Name: "iana2", GurID: 2, Status: entities.IANARegistrarStatusTerminated},
	}
	s.env.OnActivity(activities.GetIANARegistrars, mock.Anything, mock.Anything, mock.Anything).Return(ianaRars, nil)

	existingRars := []entities.RegistrarListItem{
		{ClID: entities.ClIDType("1-iana1"), Status: entities.RegistrarStatusOK, IANAStatus: entities.IANARegistrarStatusAccredited},
	}
	s.env.OnActivity(activities.GetRegistrarListItems, mock.Anything, mock.Anything, mock.Anything).Return(existingRars, nil)

	plan := activities.DiffPlanResult{
		Creates:         []commands.CreateRegistrarCommand{{ClID: "NEWCL"}},
		Updates:         []commands.UpdateRegistrarStatusCommand{{ClID: "1-iana1", NewStatus: "terminated"}},
		SkippedReserved: 0,
	}
	s.env.OnActivity(activities.DiffAndPlanRegistrars, mock.Anything, mock.Anything, ianaRars, existingRars).Return(plan, nil)

	// No write activities mocked because they must not be executed during DryRun.

	s.env.ExecuteWorkflow(SyncRegistrarsWorkflow, SyncRegistrarsParams{DryRun: true})
	s.Require().True(s.env.IsWorkflowCompleted())
	s.Require().NoError(s.env.GetWorkflowError())

	var result SyncRegistrarsResult
	s.Require().NoError(s.env.GetWorkflowResult(&result))
	s.Equal(1, result.Created)
	s.Equal(1, result.Updated)
	s.Equal(0, result.Failed)
	s.Equal(2, result.TotalProcessed)
	s.Equal(2, result.TotalIANA)
	s.Equal(1, result.TotalExisting)
	s.Contains(result.Notes, "Dry run completed: no state changes made")
}

func (s *SyncRegistrarsWorkflowTestSuite) Test_SyncRegistrars_DryRun_Bootstrap() {
	s.env.OnActivity(activities.SyncIanaRegistrars, mock.Anything, mock.Anything).Return(nil)
	s.env.OnActivity(activities.CountRegistrars, mock.Anything, mock.Anything).Return(&response.CountResult{Count: 0}, nil)

	csvRars := []icannregistrars.CSVRegistrar{{Name: "rar1"}}
	s.env.OnActivity(activities.GetICANNRegistrars, mock.Anything, mock.Anything, "./initdata/icannRegistrarList.csv").Return(csvRars, nil)

	ianaRars := []entities.IANARegistrar{
		{Name: "iana1", GurID: 1, Status: entities.IANARegistrarStatusAccredited},
		{Name: "iana2", GurID: 2, Status: entities.IANARegistrarStatusReserved},
	}
	s.env.OnActivity(activities.GetIANARegistrars, mock.Anything, mock.Anything, mock.Anything).Return(ianaRars, nil)

	createCmds := []commands.CreateRegistrarCommand{{ClID: "CL1"}}
	s.env.OnActivity(activities.MakeCreateRegistrarCommands, mock.Anything, mock.Anything, csvRars, ianaRars).Return(createCmds, nil)

	s.env.ExecuteWorkflow(SyncRegistrarsWorkflow, SyncRegistrarsParams{DryRun: true})
	s.Require().True(s.env.IsWorkflowCompleted())
	s.Require().NoError(s.env.GetWorkflowError())

	var result SyncRegistrarsResult
	s.Require().NoError(s.env.GetWorkflowResult(&result))
	s.Equal(1, result.Created)
	s.Equal(0, result.Failed)
	s.Equal(2, result.TotalIANA)
	s.Equal(0, result.TotalExisting)
	s.Contains(result.Notes, "Dry run completed: no state changes made")
}

func (s *SyncRegistrarsWorkflowTestSuite) Test_SyncRegistrars_SyncIanaFailure() {
	s.env.OnActivity(activities.SyncIanaRegistrars, mock.Anything, mock.Anything).Return(fmt.Errorf("IANA XML unavailable"))

	s.env.ExecuteWorkflow(SyncRegistrarsWorkflow, SyncRegistrarsParams{})
	s.Require().True(s.env.IsWorkflowCompleted())

	err := s.env.GetWorkflowError()
	s.Require().Error(err)
	s.Contains(err.Error(), "IANA XML unavailable")
}

func (s *SyncRegistrarsWorkflowTestSuite) Test_SyncRegistrars_SkippedReservedSurfaced() {
	s.env.OnActivity(activities.SyncIanaRegistrars, mock.Anything, mock.Anything).Return(nil)
	s.env.OnActivity(activities.CountRegistrars, mock.Anything, mock.Anything).Return(&response.CountResult{Count: 5}, nil)

	ianaRars := []entities.IANARegistrar{
		{Name: "iana1", GurID: 1, Status: entities.IANARegistrarStatusAccredited},
		{Name: "reserved1", GurID: 3000, Status: entities.IANARegistrarStatusReserved},
		{Name: "reserved2", GurID: 3001, Status: entities.IANARegistrarStatusReserved},
	}
	s.env.OnActivity(activities.GetIANARegistrars, mock.Anything, mock.Anything, mock.Anything).Return(ianaRars, nil)

	existingRars := []entities.RegistrarListItem{
		{ClID: entities.ClIDType("1-iana1"), Status: entities.RegistrarStatusOK, IANAStatus: entities.IANARegistrarStatusAccredited},
	}
	s.env.OnActivity(activities.GetRegistrarListItems, mock.Anything, mock.Anything, mock.Anything).Return(existingRars, nil)

	plan := activities.DiffPlanResult{
		Creates:         []commands.CreateRegistrarCommand{},
		Updates:         []commands.UpdateRegistrarStatusCommand{},
		SkippedReserved: 2,
	}
	s.env.OnActivity(activities.DiffAndPlanRegistrars, mock.Anything, mock.Anything, ianaRars, existingRars).Return(plan, nil)

	s.env.ExecuteWorkflow(SyncRegistrarsWorkflow, SyncRegistrarsParams{})
	s.Require().True(s.env.IsWorkflowCompleted())
	s.Require().NoError(s.env.GetWorkflowError())

	var result SyncRegistrarsResult
	s.Require().NoError(s.env.GetWorkflowResult(&result))
	s.Equal(0, result.Created)
	s.Equal(0, result.Updated)
	s.Equal(0, result.Failed)
	s.Equal(2, result.Skipped)
	s.Equal(1, result.Unchanged) // iana1 was already in sync
	s.Equal(3, result.TotalIANA)
}

func TestSyncRegistrarsWorkflowTestSuite(t *testing.T) {
	suite.Run(t, new(SyncRegistrarsWorkflowTestSuite))
}
