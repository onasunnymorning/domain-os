package workflows

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"testing"
	"time"

	"github.com/onasunnymorning/domain-os/internal/application/activities"
	"github.com/onasunnymorning/domain-os/internal/infrastructure/db/postgres"
	"github.com/onasunnymorning/domain-os/internal/infrastructure/snowflakeidgenerator"
	"github.com/stretchr/testify/suite"
	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/converter"
	"go.temporal.io/sdk/testsuite"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// Define an in-memory StorageAPI mock
type mockStorage struct {
	files map[string][]byte
}

func newMockStorage() *mockStorage {
	return &mockStorage{files: make(map[string][]byte)}
}

func (m *mockStorage) UploadStream(ctx context.Context, key string, reader io.Reader, contentType string) error {
	data, err := io.ReadAll(reader)
	if err != nil {
		return err
	}
	m.files[key] = data
	return nil
}

func (m *mockStorage) DownloadStream(ctx context.Context, key string) (io.ReadCloser, error) {
	data, ok := m.files[key]
	if !ok {
		return nil, fmt.Errorf("file not found: %s", key)
	}
	return io.NopCloser(bytes.NewReader(data)), nil
}

type TLDCleanupWorkflowTestSuite struct {
	suite.Suite
	testsuite.WorkflowTestSuite
	env     *testsuite.TestWorkflowEnvironment
	db      *gorm.DB
	mockS3  *mockStorage
	tldActs *activities.TLDCleanupActivities
	idGen   *snowflakeidgenerator.IDGenerator
}

func (s *TLDCleanupWorkflowTestSuite) SetupTest() {
	s.env = s.NewTestWorkflowEnvironment()
	s.mockS3 = newMockStorage()

	// Use SQLite in-memory for testing the exact database interactions
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	s.Require().NoError(err)
	s.db = db

	err = postgres.AutoMigrate(db)
	s.Require().NoError(err)

	// Clean tables before seeding to avoid unique constraint violations
	db.Exec("DELETE FROM domain_hosts")
	db.Exec("DELETE FROM domains")
	db.Exec("DELETE FROM hosts")
	db.Exec("DELETE FROM contacts")
	db.Exec("DELETE FROM tlds")

	s.tldActs = &activities.TLDCleanupActivities{
		DB:       db,
		S3Client: s.mockS3,
	}

	idG, err := snowflakeidgenerator.NewIDGenerator()
	s.Require().NoError(err)
	s.idGen = idG

	// Register all real activities with the test database & mock storage
	s.env.RegisterActivity(s.tldActs.CheckTLDCanBeDeleted)
	s.env.RegisterActivity(s.tldActs.PlanTLDCleanup)
	s.env.RegisterActivity(s.tldActs.BackupTLDAssets)
	s.env.RegisterActivity(s.tldActs.DeleteTLDAssets)

	// We must register the workflow too, to receive signals
	s.env.RegisterWorkflow(TLDCleanupWorkflow)
}

func (s *TLDCleanupWorkflowTestSuite) seedTestData() {
	// Seed TLDs
	tld1 := postgres.TLD{Name: "example1"}
	tld2 := postgres.TLD{Name: "example2"}
	s.db.Create(&tld1)
	s.db.Create(&tld2)

	// Seed contacts
	sharedContact := postgres.Contact{ID: "C1", RoID: 100}
	s.db.Create(&sharedContact)

	// Create an orphaned contact with explicitly NULL name_int to test the scanning bugfix
	s.db.Exec(`INSERT INTO contacts (id, ro_id, name_int, auth_info) VALUES ('C2', 101, NULL, 'test-auth')`)
	orphanContactID := "C2"

	// Seed hosts
	sharedHost := postgres.Host{RoID: 200, Name: "ns1.example1"}
	orphanHost := postgres.Host{RoID: 201, Name: "ns2.example1"}
	s.db.Create(&sharedHost)
	s.db.Create(&orphanHost)

	// Seed Domains
	d1 := postgres.Domain{
		RoID:         300,
		Name:         "test.example1",
		TLDName:      "example1",
		RegistrantID: &sharedContact.ID, // Shared contact
		AdminID:      &orphanContactID,  // Orphaned on example1 with NULL name_int
	}
	d2 := postgres.Domain{
		RoID:         301,
		Name:         "test.example2",
		TLDName:      "example2",
		RegistrantID: &sharedContact.ID, // Shared contact used in example2
	}
	s.db.Create(&d1)
	s.db.Create(&d2)

	// Link Hosts to Domains
	s.db.Exec("INSERT INTO domain_hosts (domain_ro_id, host_ro_id) VALUES (?, ?)", d1.RoID, sharedHost.RoID)
	s.db.Exec("INSERT INTO domain_hosts (domain_ro_id, host_ro_id) VALUES (?, ?)", d1.RoID, orphanHost.RoID)
	// d2 uses the shared host
	s.db.Exec("INSERT INTO domain_hosts (domain_ro_id, host_ro_id) VALUES (?, ?)", d2.RoID, sharedHost.RoID)
}

func (s *TLDCleanupWorkflowTestSuite) Test_FullCleanup_Success() {
	s.seedTestData()

	// Intercept the workflow to send the permission signal
	s.env.SetOnActivityStartedListener(func(activityInfo *activity.Info, ctx context.Context, args converter.EncodedValues) {
		if activityInfo.ActivityType.Name == "PlanTLDCleanup" {
			// After planning, we send the signal to confirm
			s.env.RegisterDelayedCallback(func() {
				s.env.SignalWorkflow("ConfirmTLDCleanup", true)
			}, time.Millisecond*100)
		}
	})

	params := TLDCleanupParams{
		TLD:              "example1",
		KeepTLDAndPhases: false,
	}

	s.env.ExecuteWorkflow(TLDCleanupWorkflow, params)
	s.Require().True(s.env.IsWorkflowCompleted())
	s.Require().NoError(s.env.GetWorkflowError())

	var result TLDCleanupResponse
	s.Require().NoError(s.env.GetWorkflowResult(&result))
	s.Assert().NotEmpty(result.ManifestKey)
	s.Assert().NotEmpty(result.BackupKey)
	s.Assert().Greater(result.DeletedCount, int64(0))

	// Verify Database State!
	// example1 TLD should NOT exist
	var tCount int64
	s.db.Model(&postgres.TLD{}).Where("name = ?", "example1").Count(&tCount)
	s.Assert().Equal(int64(0), tCount)

	// example1 domain should be deleted
	var dCount int64
	s.db.Model(&postgres.Domain{}).Where("tld_name = ?", "example1").Count(&dCount)
	s.Assert().Equal(int64(0), dCount)

	// orphanContact (C2) should be deleted
	var cOrphanCount int64
	s.db.Model(&postgres.Contact{}).Where("id = ?", "C2").Count(&cOrphanCount)
	s.Assert().Equal(int64(0), cOrphanCount)

	// sharedContact (C1) MUST STILL EXIST because example2 domain uses it
	var cSharedCount int64
	s.db.Model(&postgres.Contact{}).Where("id = ?", "C1").Count(&cSharedCount)
	s.Assert().Equal(int64(1), cSharedCount)

	// orphanHost (201) should be deleted
	var hOrphanCount int64
	s.db.Model(&postgres.Host{}).Where("ro_id = ?", 201).Count(&hOrphanCount)
	s.Assert().Equal(int64(0), hOrphanCount)

	// sharedHost (200) MUST STILL EXIST
	var hSharedCount int64
	s.db.Model(&postgres.Host{}).Where("ro_id = ?", 200).Count(&hSharedCount)
	s.Assert().Equal(int64(1), hSharedCount)
}

func (s *TLDCleanupWorkflowTestSuite) Test_Cleanup_SignalCancelled() {
	s.seedTestData()

	// Send false on the signal to cancel
	s.env.RegisterDelayedCallback(func() {
		s.env.SignalWorkflow("ConfirmTLDCleanup", false)
	}, time.Millisecond*100)

	params := TLDCleanupParams{
		TLD:              "example1",
		KeepTLDAndPhases: false,
	}

	s.env.ExecuteWorkflow(TLDCleanupWorkflow, params)
	s.Require().True(s.env.IsWorkflowCompleted())
	// Should return an error indicating cancellation
	err := s.env.GetWorkflowError()
	s.Require().Error(err)
	s.Assert().Contains(err.Error(), "cleanup aborted by user signal")

	// Domain should NOT have been deleted
	var dCount int64
	s.db.Model(&postgres.Domain{}).Where("tld_name = ?", "example1").Count(&dCount)
	s.Assert().Equal(int64(1), dCount)
}

func TestTLDCleanupWorkflowTestSuite(t *testing.T) {
	suite.Run(t, new(TLDCleanupWorkflowTestSuite))
}
