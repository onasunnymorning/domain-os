package workflows

import (
	"testing"

	"github.com/onasunnymorning/domain-os/internal/application/activities"
	"github.com/onasunnymorning/domain-os/internal/infrastructure/db/postgres"
	"github.com/stretchr/testify/suite"
	"go.temporal.io/sdk/testsuite"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type TakeSnapshotWorkflowTestSuite struct {
	suite.Suite
	testsuite.WorkflowTestSuite
	env      *testsuite.TestWorkflowEnvironment
	db       *gorm.DB
	mockS3   *mockSnapshotStorage
	snapActs *activities.SnapshotActivities
}

func (s *TakeSnapshotWorkflowTestSuite) SetupTest() {
	s.env = s.NewTestWorkflowEnvironment()
	s.mockS3 = newMockSnapshotStorage()

	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	s.Require().NoError(err)
	s.db = db

	err = postgres.AutoMigrate(db)
	s.Require().NoError(err)

	// Clean tables to avoid unique constraint violations across tests
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
	s.env.RegisterWorkflow(TakeSnapshotWorkflow)
}

func (s *TakeSnapshotWorkflowTestSuite) seedTestData() {
	// Seed minimal data across tables to verify the snapshot captures everything
	ry := postgres.RegistryOperator{RyID: "ry1", Name: "Test Registry", Email: "test@test.com"}
	s.db.Create(&ry)

	tld := postgres.TLD{Name: "test", Type: "generic", RyID: "ry1"}
	s.db.Create(&tld)

	phase := postgres.Phase{ID: 1, Name: "GA", Type: "open", TLDName: "test"}
	s.db.Create(&phase)

	reg := postgres.Registrar{ClID: "reg1", Name: "Test Registrar", NickName: "testreg", Status: "ok"}
	s.db.Create(&reg)

	// Accreditation (join table)
	s.db.Exec("INSERT INTO accreditations (tld_name, registrar_cl_id) VALUES ('test', 'reg1')")

	contact := postgres.Contact{ID: "c1", RoID: 100, ClID: "reg1", AuthInfo: "auth"}
	s.db.Create(&contact)

	host := postgres.Host{RoID: 200, Name: "ns1.test", ClID: "reg1"}
	s.db.Create(&host)

	cID := "c1"
	domain := postgres.Domain{RoID: 300, Name: "example.test", TLDName: "test", ClID: "reg1", RegistrantID: &cID, AuthInfo: "auth"}
	s.db.Create(&domain)

	// Domain-Host join table
	s.db.Exec("INSERT INTO domain_hosts (domain_ro_id, host_ro_id) VALUES (300, 200)")
}

func (s *TakeSnapshotWorkflowTestSuite) Test_TakeSnapshot_Success() {
	s.seedTestData()

	params := TakeSnapshotParams{Label: "test-snapshot"}

	s.env.ExecuteWorkflow(TakeSnapshotWorkflow, params)
	s.Require().True(s.env.IsWorkflowCompleted())
	s.Require().NoError(s.env.GetWorkflowError())

	var result TakeSnapshotResponse
	s.Require().NoError(s.env.GetWorkflowResult(&result))

	// Verify the snapshot was created
	s.Assert().Contains(result.SnapshotKey, "snapshot.jsonl")
	s.Assert().Contains(result.ManifestKey, "manifest.json")
	s.Assert().Greater(result.TotalRows, int64(0), "should have exported some rows")

	// Verify specific tables were exported
	s.Assert().Equal(int64(1), result.TableCounts["registry_operators"])
	s.Assert().Equal(int64(1), result.TableCounts["tlds"])
	s.Assert().Equal(int64(1), result.TableCounts["registrars"])
	s.Assert().Equal(int64(1), result.TableCounts["contacts"])
	s.Assert().Equal(int64(1), result.TableCounts["hosts"])
	s.Assert().Equal(int64(1), result.TableCounts["domains"])
	s.Assert().Equal(int64(1), result.TableCounts["accreditations"])
	s.Assert().Equal(int64(1), result.TableCounts["domain_hosts"])

	// Verify the JSONL and manifest exist in mock S3
	s.Assert().Contains(s.mockS3.files, result.SnapshotKey)
	s.Assert().Contains(s.mockS3.files, result.ManifestKey)
}

func (s *TakeSnapshotWorkflowTestSuite) Test_TakeSnapshot_EmptyDB() {
	// No seeding — empty database
	params := TakeSnapshotParams{Label: "empty"}

	s.env.ExecuteWorkflow(TakeSnapshotWorkflow, params)
	s.Require().True(s.env.IsWorkflowCompleted())
	s.Require().NoError(s.env.GetWorkflowError())

	var result TakeSnapshotResponse
	s.Require().NoError(s.env.GetWorkflowResult(&result))

	// Should succeed with 0 rows
	s.Assert().Equal(int64(0), result.TotalRows)
}

func TestTakeSnapshotWorkflowTestSuite(t *testing.T) {
	suite.Run(t, new(TakeSnapshotWorkflowTestSuite))
}
