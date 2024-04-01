package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/onasunnymorning/domain-os/internal/domain/entities"
	"github.com/stretchr/testify/suite"
	"gorm.io/gorm"
)

type PhaseSuite struct {
	suite.Suite
	db      *gorm.DB
	TLDName string
}

func TestPhaseSuite(t *testing.T) {
	suite.Run(t, new(PhaseSuite))
}

func (s *PhaseSuite) SetupSuite() {
	s.db = setupTestDB()
	repo := NewGormTLDRepo(s.db)

	// Create a tld
	tld, _ := entities.NewTLD("phase.test")
	err := repo.Create(tld)
	s.Require().NoError(err)

	readTLD, err := repo.GetByName(tld.Name.String())
	s.Require().NoError(err)
	s.Require().NotNil(readTLD)
	s.Require().Equal(tld, readTLD)
	// Save to the Suite
	s.TLDName = tld.Name.String()
}

func (s *PhaseSuite) TearDownSuite() {
	if s.TLDName != "" {
		repo := NewGormRegistrarRepository(s.db)
		_ = repo.Delete(context.Background(), s.TLDName)
	}
}

func (s *PhaseSuite) TestPhaseRepo_CreatePhase() {
	tx := s.db.Begin()
	defer tx.Rollback()
	repo := NewGormPhaseRepository(tx)

	// Setup a phase
	phase, err := entities.NewPhase("TestPhase", "GA", time.Now().UTC())
	s.Require().NoError(err)
	s.Require().NotNil(phase)
	// Set some attributes
	phase.TLDName = entities.DomainName(s.TLDName)

	// Create the phase
	createdPhase, err := repo.CreatePhase(phase)
	s.Require().NoError(err)
	s.Require().NotNil(createdPhase)
	s.Require().Equal(phase.Name, createdPhase.Name)
	s.Require().Equal(phase.Type, createdPhase.Type)
	s.Require().NotNil(createdPhase.ID)
	s.Require().NotNil(createdPhase.Starts)
	s.Require().NotNil(createdPhase.CreatedAt)
	s.Require().NotNil(createdPhase.UpdatedAt)
	s.Require().Nil(createdPhase.Ends)

	// Try and create the same phase again
	_, err = repo.CreatePhase(phase)
	s.Require().Error(err)
}

func (s *PhaseSuite) TestPhaseRepo_GetPhaseByName() {
	tx := s.db.Begin()
	defer tx.Rollback()
	repo := NewGormPhaseRepository(tx)

	// Setup a phase
	phase, err := entities.NewPhase("TestPhase", "GA", time.Now().UTC())
	s.Require().NoError(err)
	s.Require().NotNil(phase)
	phase.TLDName = entities.DomainName(s.TLDName)

	// Create the phase
	_, err = repo.CreatePhase(phase)
	s.Require().NoError(err)

	// Fetch the Phase
	createdPhase, err := repo.GetPhaseByName(phase.Name.String())
	s.Require().NoError(err)
	s.Require().NotNil(createdPhase)
	s.Require().Equal(phase.Name, createdPhase.Name)
	s.Require().Equal(phase.Type, createdPhase.Type)
	s.Require().NotNil(createdPhase.ID)
	s.Require().NotNil(createdPhase.Starts)
	s.Require().NotNil(createdPhase.CreatedAt)
	s.Require().NotNil(createdPhase.UpdatedAt)
	s.Require().Nil(createdPhase.Ends)

	// Fetch a phase that doesn't exist
	_, err = repo.GetPhaseByName("DoesNotExist")
	s.Require().Error(err)
}

func (s *PhaseSuite) TestPhaseRepo_DeletePhaseByName() {
	tx := s.db.Begin()
	defer tx.Rollback()
	repo := NewGormPhaseRepository(tx)

	// Setup a phase
	phase, err := entities.NewPhase("TestPhase", "GA", time.Now().UTC())
	s.Require().NoError(err)
	s.Require().NotNil(phase)
	phase.TLDName = entities.DomainName(s.TLDName)

	// Create the phase
	_, err = repo.CreatePhase(phase)
	s.Require().NoError(err)

	// Fetch the Phase
	createdPhase, err := repo.GetPhaseByName(phase.Name.String())
	s.Require().NoError(err)
	s.Require().NotNil(createdPhase)

	// Delete the Phase
	err = repo.DeletePhaseByName(phase.Name.String())
	s.Require().NoError(err)

	// Fetch the Phase again
	_, err = repo.GetPhaseByName(phase.Name.String())
	s.Require().Error(err)

	// Try and delete a phase again (should not error)
	err = repo.DeletePhaseByName(phase.Name.String())
	s.Require().NoError(err)

}

func (s *PhaseSuite) TestPhaseRepo_UpdatePhase() {
	tx := s.db.Begin()
	defer tx.Rollback()
	repo := NewGormPhaseRepository(tx)

	// Setup a phase
	phase, err := entities.NewPhase("TestPhase", "GA", time.Now().UTC())
	s.Require().NoError(err)
	s.Require().NotNil(phase)
	phase.TLDName = entities.DomainName(s.TLDName)

	// Create the phase
	createdPhase, err := repo.CreatePhase(phase)
	s.Require().NoError(err)

	// Update the phase
	endDate := time.Now().UTC().AddDate(0, 0, 1)
	createdPhase.Ends = &endDate
	createdPhase.Policy.AllowAutoRenew = false
	createdPhase.Policy.BaseCurrency = "PEN"
	createdPhase.Policy.MaxHorizon = 20

	updatedPhase, err := repo.UpdatePhase(createdPhase)
	s.Require().NoError(err)
	s.Require().NotNil(updatedPhase)
	s.Require().Equal(createdPhase.Name, updatedPhase.Name)
	s.Require().Equal(createdPhase.Type, updatedPhase.Type)
	s.Require().NotNil(updatedPhase.ID)
	s.Require().NotNil(updatedPhase.Starts)
	s.Require().NotNil(updatedPhase.CreatedAt)
	s.Require().NotEqual(createdPhase.UpdatedAt, updatedPhase.UpdatedAt)
	s.Require().NotNil(updatedPhase.Ends)
	s.Require().Equal(endDate, *updatedPhase.Ends)
	s.Require().Equal(false, updatedPhase.Policy.AllowAutoRenew)
	s.Require().Equal("PEN", updatedPhase.Policy.BaseCurrency)
	s.Require().Equal(20, updatedPhase.Policy.MaxHorizon)

	// Try and update a phase but remove the TLDName
	createdPhase.TLDName = ""
	_, err = repo.UpdatePhase(createdPhase)
	s.Require().Error(err)
}
