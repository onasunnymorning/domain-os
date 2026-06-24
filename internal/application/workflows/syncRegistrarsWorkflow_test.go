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
	s.env.OnActivity(activities.SyncIanaRegistrars, mock.Anything).Return(nil)

	// Count is 0 (first import)
	s.env.OnActivity(activities.CountRegistrars, mock.Anything).Return(&response.CountResult{Count: 0}, nil)

	// Get ICANN and IANA
	csvRars := []icannregistrars.CSVRegistrar{{Name: "rar1"}}
	s.env.OnActivity(activities.GetICANNRegistrars, mock.Anything, "./initdata/icannRegistrarList.csv").Return(csvRars, nil)

	ianaRars := []entities.IANARegistrar{{Name: "iana1", GurID: 1}}
	s.env.OnActivity(activities.GetIANARegistrars, mock.Anything, 100).Return(ianaRars, nil)

	// Make creates
	createCmds := []commands.CreateRegistrarCommand{{ClID: "CL1"}}
	s.env.OnActivity(activities.MakeCreateRegistrarCommands, mock.Anything, csvRars, ianaRars).Return(createCmds, nil)

	// Bulk Create
	s.env.OnActivity(activities.BulkCreateRegistrars, mock.Anything, createCmds).Return(nil)

	s.env.ExecuteWorkflow(SyncRegistrarsWorkflow, SyncRegistrarsParams{BatchSize: 100})
	s.Require().True(s.env.IsWorkflowCompleted())
	s.Require().NoError(s.env.GetWorkflowError())

	var result SyncRegistrarsResult
	s.Require().NoError(s.env.GetWorkflowResult(&result))
	s.Equal(1, result.Created)
	s.Equal(1, result.TotalProcessed)
	s.Equal(0, result.Failed)
}

func (s *SyncRegistrarsWorkflowTestSuite) Test_SyncRegistrars_UpdatePath() {
	s.env.OnActivity(activities.SyncIanaRegistrars, mock.Anything).Return(nil)

	// Count is 5 (not first import)
	s.env.OnActivity(activities.CountRegistrars, mock.Anything).Return(&response.CountResult{Count: 5}, nil)

	// Get IANA and existing
	ianaRars := []entities.IANARegistrar{
		{Name: "iana1", GurID: 1, Status: entities.IANARegistrarStatusAccredited},
	}
	s.env.OnActivity(activities.GetIANARegistrars, mock.Anything, 100).Return(ianaRars, nil)

	existingRars := []entities.RegistrarListItem{
		{ClID: entities.ClIDType("1-iana1")},
	}
	s.env.OnActivity(activities.GetRegistrarListItems, mock.Anything, 100).Return(existingRars, nil)

	// Diff plan
	plan := activities.DiffPlanResult{
		Creates: []commands.CreateRegistrarCommand{{ClID: "NEWCL"}},
		Updates: []commands.UpdateRegistrarStatusCommand{{ClID: "1-iana1", NewStatus: "active"}},
	}
	s.env.OnActivity(activities.DiffAndPlanRegistrars, mock.Anything, ianaRars, existingRars).Return(plan, nil)

	// Bulk Create the creates
	s.env.OnActivity(activities.BulkCreateRegistrars, mock.Anything, plan.Creates).Return(nil)

	// Parallel Updates
	s.env.OnActivity(activities.SetRegistrarStatus, mock.Anything, "1-iana1", "active").Return(nil)
	s.env.OnActivity(activities.SetRegistrarIANAStatus, mock.Anything, "1-iana1", "Accredited").Return(fmt.Errorf("iana update fail"))

	s.env.ExecuteWorkflow(SyncRegistrarsWorkflow, SyncRegistrarsParams{BatchSize: 100, ConcurrencyLimit: 2})
	s.Require().True(s.env.IsWorkflowCompleted())
	s.Require().NoError(s.env.GetWorkflowError())

	var result SyncRegistrarsResult
	s.Require().NoError(s.env.GetWorkflowResult(&result))
	s.Equal(1, result.Created)
	s.Equal(1, result.Updated)
	s.Equal(1, result.Failed)
	s.Equal(3, result.TotalProcessed) // 1 create + 2 updates = 3 total

	s.Equal("1-iana1", result.Failures[0].ClID)
	s.Contains(result.Failures[0].Error, "iana update fail")
	s.Equal("update-iana-status", result.Failures[0].Operation)
}

func (s *SyncRegistrarsWorkflowTestSuite) Test_SyncRegistrars_DryRun() {
	s.env.OnActivity(activities.SyncIanaRegistrars, mock.Anything).Return(nil)
	s.env.OnActivity(activities.CountRegistrars, mock.Anything).Return(&response.CountResult{Count: 5}, nil)

	ianaRars := []entities.IANARegistrar{{Name: "iana1", GurID: 1}}
	s.env.OnActivity(activities.GetIANARegistrars, mock.Anything, 100).Return(ianaRars, nil)

	existingRars := []entities.RegistrarListItem{{ClID: entities.ClIDType("1-iana1")}}
	s.env.OnActivity(activities.GetRegistrarListItems, mock.Anything, 100).Return(existingRars, nil)

	plan := activities.DiffPlanResult{
		Creates: []commands.CreateRegistrarCommand{{ClID: "NEWCL"}},
		Updates: []commands.UpdateRegistrarStatusCommand{{ClID: "1-iana1", NewStatus: "active"}},
	}
	s.env.OnActivity(activities.DiffAndPlanRegistrars, mock.Anything, ianaRars, existingRars).Return(plan, nil)

	// No write activities mocked because they must not be executed during DryRun.

	s.env.ExecuteWorkflow(SyncRegistrarsWorkflow, SyncRegistrarsParams{BatchSize: 100, DryRun: true})
	s.Require().True(s.env.IsWorkflowCompleted())
	s.Require().NoError(s.env.GetWorkflowError())

	var result SyncRegistrarsResult
	s.Require().NoError(s.env.GetWorkflowResult(&result))
	s.Equal(1, result.Created)
	s.Equal(1, result.Updated)
	s.Equal(0, result.Failed)
	s.Contains(result.Notes, "Dry run completed: no state changes made")
}

func TestSyncRegistrarsWorkflowTestSuite(t *testing.T) {
	suite.Run(t, new(SyncRegistrarsWorkflowTestSuite))
}
