package activities

import (
	"context"
	"io"
	"testing"

	"github.com/onasunnymorning/domain-os/internal/infrastructure/db/postgres"
	"github.com/stretchr/testify/suite"
	"go.temporal.io/sdk/testsuite"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type mockSweepStorage struct {
	uploads map[string][]byte
}

func (m *mockSweepStorage) UploadStream(ctx context.Context, key string, reader io.Reader, contentType string) error {
	data, err := io.ReadAll(reader)
	if err != nil {
		return err
	}
	m.uploads[key] = data
	return nil
}

type Spec5SweepActivitiesTestSuite struct {
	suite.Suite
	db   *gorm.DB
	s3   *mockSweepStorage
	acts *Spec5SweepActivities
}

func (s *Spec5SweepActivitiesTestSuite) SetupTest() {
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	s.Require().NoError(err)
	s.db = db

	err = postgres.AutoMigrate(db)
	s.Require().NoError(err)

	s.s3 = &mockSweepStorage{uploads: make(map[string][]byte)}
	s.acts = &Spec5SweepActivities{
		DB:       s.db,
		S3Client: s.s3,
	}

	// Clean tables
	s.db.Exec("DELETE FROM domains")
	s.db.Exec("DELETE FROM tlds")
	s.db.Exec("DELETE FROM spec5_labels")
}

func (s *Spec5SweepActivitiesTestSuite) TestSweepSpec5Labels_Success() {
	// Seed TLDs
	s.db.Create(&postgres.TLD{Name: "com", Type: "generic"})
	s.db.Create(&postgres.TLD{Name: "net", Type: "generic"})

	// Seed Spec5 Labels
	s.db.Create(&postgres.Spec5Label{Label: "google", Type: "restricted"})
	s.db.Create(&postgres.Spec5Label{Label: "amazon", Type: "restricted"})

	// Seed Domains
	s.db.Create(&postgres.Domain{RoID: 1, Name: "google.com", TLDName: "com", AuthInfo: "auth"})
	s.db.Create(&postgres.Domain{RoID: 2, Name: "other.com", TLDName: "com", AuthInfo: "auth"})
	s.db.Create(&postgres.Domain{RoID: 3, Name: "amazon.net", TLDName: "net", AuthInfo: "auth"})

	args := Spec5SweepArgs{
		TLDs:       []string{"com", "net"},
		WorkflowID: "wf-123",
	}

	wfs := &testsuite.WorkflowTestSuite{}
	env := wfs.NewTestActivityEnvironment()
	env.RegisterActivity(s.acts.SweepSpec5Labels)

	val, err := env.ExecuteActivity(s.acts.SweepSpec5Labels, args)
	s.Require().NoError(err)

	var res Spec5SweepResult
	err = val.Get(&res)
	s.Require().NoError(err)
	s.Require().NotNil(res.TLDResults)

	// com TLD checks
	comResult, exists := res.TLDResults["com"]
	s.Require().True(exists)
	s.Equal(int64(1), comResult.Count)
	s.NotEmpty(comResult.ArtifactKey)

	// net TLD checks
	netResult, exists := res.TLDResults["net"]
	s.Require().True(exists)
	s.Equal(int64(1), netResult.Count)

	// Verify uploaded CSV content
	comCSV := string(s.s3.uploads[comResult.ArtifactKey])
	s.Contains(comCSV, "Domain,Label,Type")
	s.Contains(comCSV, "google.com,google,restricted")

	netCSV := string(s.s3.uploads[netResult.ArtifactKey])
	s.Contains(netCSV, "Domain,Label,Type")
	s.Contains(netCSV, "amazon.net,amazon,restricted")
}

func (s *Spec5SweepActivitiesTestSuite) TestSweepSpec5Labels_NoTLDs() {
	args := Spec5SweepArgs{
		WorkflowID: "wf-123",
	}

	wfs := &testsuite.WorkflowTestSuite{}
	env := wfs.NewTestActivityEnvironment()
	env.RegisterActivity(s.acts.SweepSpec5Labels)

	_, err := env.ExecuteActivity(s.acts.SweepSpec5Labels, args)
	s.Require().Error(err)
	s.Contains(err.Error(), "no TLDs specified for sweep")
}

func (s *Spec5SweepActivitiesTestSuite) TestSweepSpec5Labels_MissingWorkflowID() {
	args := Spec5SweepArgs{
		TLDs: []string{"com"},
	}

	wfs := &testsuite.WorkflowTestSuite{}
	env := wfs.NewTestActivityEnvironment()
	env.RegisterActivity(s.acts.SweepSpec5Labels)

	_, err := env.ExecuteActivity(s.acts.SweepSpec5Labels, args)
	s.Require().Error(err)
	s.Contains(err.Error(), "workflowId is required")
}

func TestSpec5SweepActivitiesTestSuite(t *testing.T) {
	suite.Run(t, new(Spec5SweepActivitiesTestSuite))
}
