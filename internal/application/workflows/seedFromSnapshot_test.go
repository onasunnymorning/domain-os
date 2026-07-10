package workflows

import (
	"context"
	"testing"
	"time"

	"github.com/onasunnymorning/domain-os/internal/application/activities"
	"github.com/onasunnymorning/domain-os/internal/infrastructure/db/postgres"
	"github.com/stretchr/testify/suite"
	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/converter"
	"go.temporal.io/sdk/testsuite"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type SeedFromSnapshotWorkflowTestSuite struct {
	suite.Suite
	testsuite.WorkflowTestSuite
	env      *testsuite.TestWorkflowEnvironment
	db       *gorm.DB
	mockS3   *mockSnapshotStorage
	snapActs *activities.SnapshotActivities
}

func (s *SeedFromSnapshotWorkflowTestSuite) SetupTest() {
	s.env = s.NewTestWorkflowEnvironment()
	s.mockS3 = newMockSnapshotStorage()

	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	s.Require().NoError(err)
	s.db = db

	err = postgres.AutoMigrate(db)
	s.Require().NoError(err)

	// Clean all tables
	s.db.Exec("DELETE FROM domain_hosts")
	s.db.Exec("DELETE FROM accreditations")
	s.db.Exec("DELETE FROM domains")
	s.db.Exec("DELETE FROM hosts")
	s.db.Exec("DELETE FROM host_addresses")
	s.db.Exec("DELETE FROM contacts")
	s.db.Exec("DELETE FROM nndns")
	s.db.Exec("DELETE FROM phase_fees")
	s.db.Exec("DELETE FROM phase_prices")
	s.db.Exec("DELETE FROM phases")
	s.db.Exec("DELETE FROM tlds")
	s.db.Exec("DELETE FROM premium_labels")
	s.db.Exec("DELETE FROM premium_lists")
	s.db.Exec("DELETE FROM registrars")
	s.db.Exec("DELETE FROM registry_operators")
	s.db.Exec("DELETE FROM iana_registrars")
	s.db.Exec("DELETE FROM spec5_labels")
	s.db.Exec("DELETE FROM fx")
	s.db.Exec("DELETE FROM tld_dns_records")

	s.snapActs = &activities.SnapshotActivities{
		DB:       db,
		S3Client: s.mockS3,
	}

	s.env.RegisterActivity(s.snapActs.TakeSnapshot)
	s.env.RegisterActivity(s.snapActs.ValidateSnapshot)
	s.env.RegisterActivity(s.snapActs.SeedFromSnapshot)
	s.env.RegisterWorkflow(TakeSnapshotWorkflow)
	s.env.RegisterWorkflow(SeedFromSnapshotWorkflow)
}

func (s *SeedFromSnapshotWorkflowTestSuite) seedAndSnapshot() string {
	// Seed data
	ry := postgres.RegistryOperator{RyID: "ry1", Name: "Test Registry", Email: "test@test.com"}
	s.db.Create(&ry)

	tld := postgres.TLD{Name: "test", Type: "generic", RyID: "ry1"}
	s.db.Create(&tld)

	reg := postgres.Registrar{ClID: "reg1", Name: "Test Registrar", NickName: "testreg", Status: "ok"}
	s.db.Create(&reg)

	contact := postgres.Contact{ID: "c1", RoID: 100, ClID: "reg1", AuthInfo: "auth"}
	s.db.Create(&contact)

	host := postgres.Host{RoID: 200, Name: "ns1.test", ClID: "reg1"}
	s.db.Create(&host)

	cID := "c1"
	domain := postgres.Domain{RoID: 300, Name: "example.test", TLDName: "test", ClID: "reg1", RegistrantID: &cID, AuthInfo: "auth"}
	s.db.Create(&domain)

	// Take snapshot
	s.env.ExecuteWorkflow(TakeSnapshotWorkflow, TakeSnapshotParams{Label: "seed-test"})
	s.Require().True(s.env.IsWorkflowCompleted())
	s.Require().NoError(s.env.GetWorkflowError())

	var result TakeSnapshotResponse
	s.Require().NoError(s.env.GetWorkflowResult(&result))

	// Extract the snapshot key prefix (e.g., "wf-id" from "wf-id/snapshot.jsonl")
	snapshotPrefix := result.SnapshotKey[:len(result.SnapshotKey)-len("/snapshot.jsonl")]

	// Wipe database
	s.db.Exec("DELETE FROM domain_hosts")
	s.db.Exec("DELETE FROM accreditations")
	s.db.Exec("DELETE FROM domains")
	s.db.Exec("DELETE FROM hosts")
	s.db.Exec("DELETE FROM contacts")
	s.db.Exec("DELETE FROM registrars")
	s.db.Exec("DELETE FROM phases")
	s.db.Exec("DELETE FROM tlds")
	s.db.Exec("DELETE FROM registry_operators")

	return snapshotPrefix
}

func (s *SeedFromSnapshotWorkflowTestSuite) Test_SeedFromSnapshot_Success() {
	snapshotPrefix := s.seedAndSnapshot()

	// Reset env for seed workflow
	s.env = s.NewTestWorkflowEnvironment()
	s.env.RegisterActivity(s.snapActs.ValidateSnapshot)
	s.env.RegisterActivity(s.snapActs.SeedFromSnapshot)
	s.env.RegisterWorkflow(SeedFromSnapshotWorkflow)

	// Send confirmation after validation completes
	s.env.SetOnActivityStartedListener(func(activityInfo *activity.Info, ctx context.Context, args converter.EncodedValues) {
		if activityInfo.ActivityType.Name == "ValidateSnapshot" {
			s.env.RegisterDelayedCallback(func() {
				s.env.SignalWorkflow("ConfirmSeedFromSnapshot", true)
			}, time.Millisecond*100)
		}
	})

	params := SeedFromSnapshotParams{SnapshotKey: snapshotPrefix}

	s.env.ExecuteWorkflow(SeedFromSnapshotWorkflow, params)
	s.Require().True(s.env.IsWorkflowCompleted())
	s.Require().NoError(s.env.GetWorkflowError())

	var result SeedFromSnapshotResponse
	s.Require().NoError(s.env.GetWorkflowResult(&result))
	s.Assert().Greater(result.TotalInserted, int64(0), "should have inserted some rows")

	// Verify data was restored
	var ryCount int64
	s.db.Model(&postgres.RegistryOperator{}).Count(&ryCount)
	s.Assert().Equal(int64(1), ryCount)

	var tldCount int64
	s.db.Model(&postgres.TLD{}).Count(&tldCount)
	s.Assert().Equal(int64(1), tldCount)

	var regCount int64
	s.db.Model(&postgres.Registrar{}).Count(&regCount)
	s.Assert().Equal(int64(1), regCount)

	var domCount int64
	s.db.Model(&postgres.Domain{}).Count(&domCount)
	s.Assert().Equal(int64(1), domCount)
}

func (s *SeedFromSnapshotWorkflowTestSuite) Test_SeedFromSnapshot_Aborted() {
	snapshotPrefix := s.seedAndSnapshot()

	// Reset env for seed workflow
	s.env = s.NewTestWorkflowEnvironment()
	s.env.RegisterActivity(s.snapActs.ValidateSnapshot)
	s.env.RegisterActivity(s.snapActs.SeedFromSnapshot)
	s.env.RegisterWorkflow(SeedFromSnapshotWorkflow)

	// Send rejection signal
	s.env.RegisterDelayedCallback(func() {
		s.env.SignalWorkflow("ConfirmSeedFromSnapshot", false)
	}, time.Millisecond*100)

	params := SeedFromSnapshotParams{SnapshotKey: snapshotPrefix}

	s.env.ExecuteWorkflow(SeedFromSnapshotWorkflow, params)
	s.Require().True(s.env.IsWorkflowCompleted())

	err := s.env.GetWorkflowError()
	s.Require().Error(err)
	s.Assert().Contains(err.Error(), "seeding aborted by user signal")

	// Database should still be empty
	var domCount int64
	s.db.Model(&postgres.Domain{}).Count(&domCount)
	s.Assert().Equal(int64(0), domCount)
}

func TestSeedFromSnapshotWorkflowTestSuite(t *testing.T) {
	suite.Run(t, new(SeedFromSnapshotWorkflowTestSuite))
}
