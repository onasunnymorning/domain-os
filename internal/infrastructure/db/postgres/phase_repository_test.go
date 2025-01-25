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
	ry      *entities.RegistryOperator
}

func TestPhaseSuite(t *testing.T) {
	suite.Run(t, new(PhaseSuite))
}

func (s *PhaseSuite) SetupSuite() {
	s.db = setupTestDB()
	repo := NewGormTLDRepo(s.db)

	// Create a Registry Operator
	ro, _ := entities.NewRegistryOperator("PhaseSuiteRo", "PhaseSuiteRo", "PhaseSuiteRo@my.email")
	roRepo := NewGORMRegistryOperatorRepository(s.db)
	_, err := roRepo.Create(context.Background(), ro)
	s.Require().NoError(err)
	createdRo, err := roRepo.GetByRyID(context.Background(), ro.RyID.String())
	s.Require().NoError(err)
	s.ry = createdRo

	// Create a tld
	tld, _ := entities.NewTLD("phase.test", "PhaseSuiteRo")
	err = repo.Create(context.Background(), tld)
	s.Require().NoError(err)

	readTLD, err := repo.GetByName(context.Background(), tld.Name.String(), false)
	s.Require().NoError(err)
	s.Require().NotNil(readTLD)
	s.Require().Equal(tld, readTLD)
	// Save to the Suite
	s.TLDName = tld.Name.String()
}

func (s *PhaseSuite) TearDownSuite() {
	if s.TLDName != "" {
		repo := NewGormTLDRepo(s.db)
		_ = repo.DeleteByName(context.Background(), s.TLDName)
	}
	if s.ry != nil {
		roRepo := NewGORMRegistryOperatorRepository(s.db)
		_ = roRepo.DeleteByRyID(context.Background(), s.ry.RyID.String())
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
	createdPhase, err := repo.CreatePhase(context.Background(), phase)
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
	_, err = repo.CreatePhase(context.Background(), phase)
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
	_, err = repo.CreatePhase(context.Background(), phase)
	s.Require().NoError(err)

	// Fetch the Phase
	createdPhase, err := repo.GetPhaseByTLDAndName(context.Background(), s.TLDName, phase.Name.String())
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
	_, err = repo.GetPhaseByTLDAndName(context.Background(), s.TLDName, "DoesNotExist")
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
	_, err = repo.CreatePhase(context.Background(), phase)
	s.Require().NoError(err)

	// Fetch the Phase
	createdPhase, err := repo.GetPhaseByTLDAndName(context.Background(), s.TLDName, phase.Name.String())
	s.Require().NoError(err)
	s.Require().NotNil(createdPhase)

	// Delete the Phase
	err = repo.DeletePhaseByTLDAndName(context.Background(), s.TLDName, phase.Name.String())
	s.Require().NoError(err)

	// Fetch the Phase again
	_, err = repo.GetPhaseByTLDAndName(context.Background(), s.TLDName, phase.Name.String())
	s.Require().Error(err)

	// Try and delete a phase again (should not error)
	err = repo.DeletePhaseByTLDAndName(context.Background(), s.TLDName, phase.Name.String())
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
	createdPhase, err := repo.CreatePhase(context.Background(), phase)
	s.Require().NoError(err)

	// Update the phase
	f := false
	endDate := time.Now().UTC().AddDate(0, 0, 1)
	createdPhase.Ends = &endDate
	createdPhase.Policy.AllowAutoRenew = &f
	createdPhase.Policy.BaseCurrency = "PEN"
	createdPhase.Policy.MaxHorizon = 20

	updatedPhase, err := repo.UpdatePhase(context.Background(), createdPhase)
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
	s.Require().Equal(&f, updatedPhase.Policy.AllowAutoRenew)
	s.Require().Equal("PEN", updatedPhase.Policy.BaseCurrency)
	s.Require().Equal(20, updatedPhase.Policy.MaxHorizon)

	// Try and update a phase but remove the TLDName
	createdPhase.TLDName = ""
	_, err = repo.UpdatePhase(context.Background(), createdPhase)
	s.Require().Error(err)
}

func (s *PhaseSuite) TestPhaseRepo_MultiplePrices() {
	tx := s.db.Begin()
	defer tx.Rollback()
	repo := NewGormPhaseRepository(tx)

	// Setup a phase
	phase, err := entities.NewPhase("TestPhase", "GA", time.Now().UTC())
	s.Require().NoError(err)
	s.Require().NotNil(phase)
	phase.TLDName = entities.DomainName(s.TLDName)

	// Create a couple of prices
	prices := [3]*entities.Price{}
	prices[0], _ = entities.NewPrice("USD", 100, 100, 100, 100)
	prices[1], _ = entities.NewPrice("EUR", 100, 100, 100, 100)
	prices[2], _ = entities.NewPrice("GBP", 100, 100, 100, 100)
	// Add the prices
	for _, price := range prices {
		_, err := phase.AddPrice(*price)
		s.Require().NoError(err)
	}
	s.Require().Len(phase.Prices, 3)

	// Create the phase
	createdPhase, err := repo.CreatePhase(context.Background(), phase)
	s.Require().NoError(err)
	// Creating a phase should not create its prices
	s.Require().Len(createdPhase.Prices, 0)
}

func (s *PhaseSuite) TestPhaseRepo_ListPhases() {
	tx := s.db.Begin()
	defer tx.Rollback()
	repo := NewGormPhaseRepository(tx)

	// Setup a phase
	phase, err := entities.NewPhase("TestPhase", "GA", time.Now().UTC())
	s.Require().NoError(err)
	s.Require().NotNil(phase)
	phase.TLDName = entities.DomainName(s.TLDName)

	// Create the phase
	_, err = repo.CreatePhase(context.Background(), phase)
	s.Require().NoError(err)

	// List the phases
	phases, err := repo.ListPhasesByTLD(context.Background(), s.TLDName, 25, "")
	s.Require().NoError(err)
	s.Require().Len(phases, 1)
	s.Require().Equal(phase.Name, phases[0].Name)
	s.Require().Equal(phase.Type, phases[0].Type)
	s.Require().NotNil(phases[0].ID)
	s.Require().NotNil(phases[0].Starts)
	s.Require().NotNil(phases[0].CreatedAt)
	s.Require().NotNil(phases[0].UpdatedAt)
	s.Require().Nil(phases[0].Ends)

	// Create another phase
	phase2, err := entities.NewPhase("TestPhase2", "Launch", time.Now().UTC())
	s.Require().NoError(err)
	phase2.TLDName = entities.DomainName(s.TLDName)
	_, err = repo.CreatePhase(context.Background(), phase2)
	s.Require().NoError(err)

	// List the phases
	phases, err = repo.ListPhasesByTLD(context.Background(), s.TLDName, 25, "")
	s.Require().NoError(err)
	s.Require().Len(phases, 2)

	// Create a third phase
	phase3, err := entities.NewPhase("TestPhase3", "Launch", time.Now().UTC())
	s.Require().NoError(err)
	phase3.TLDName = entities.DomainName(s.TLDName)
	_, err = repo.CreatePhase(context.Background(), phase3)
	s.Require().NoError(err)

	// List the phases
	phases, err = repo.ListPhasesByTLD(context.Background(), s.TLDName, 25, "")
	s.Require().NoError(err)
	s.Require().Len(phases, 3)

	// List the phases for a TLD that doesn't exist
	phases, err = repo.ListPhasesByTLD(context.Background(), "DoesNotExist", 25, "")
	s.Require().NoError(err)
	s.Require().Equal(0, len(phases))

	// Pass in an invalid pageCursor (phase cursor should be an int64)
	_, err = repo.ListPhasesByTLD(context.Background(), s.TLDName, 25, "NotAnInt64")
	s.Require().Error(err)
}
