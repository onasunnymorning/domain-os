package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/onasunnymorning/domain-os/internal/domain/entities"
	"github.com/stretchr/testify/suite"
	"gorm.io/gorm"
)

type FeeSuite struct {
	suite.Suite
	db        *gorm.DB
	TLDName   string
	PhaseID   int64
	PhaseName string
}

func TestPhaseFeeSuite(t *testing.T) {
	suite.Run(t, new(FeeSuite))
}

func (s *FeeSuite) SetupSuite() {
	s.db = setupTestDB()
	repo := NewGormTLDRepo(s.db)
	phaseRepo := NewGormPhaseRepository(s.db)

	// Create a tld
	tld, _ := entities.NewTLD("phasefee.test")
	err := repo.Create(tld)
	s.Require().NoError(err)

	readTLD, err := repo.GetByName(tld.Name.String())
	s.Require().NoError(err)
	s.Require().NotNil(readTLD)
	s.Require().Equal(tld, readTLD)
	// Save to the Suite
	s.TLDName = tld.Name.String()

	// Create a phase
	phase, _ := entities.NewPhase("TestPhase", "GA", time.Now().UTC())
	phase.TLDName = entities.DomainName(s.TLDName)
	createdPhase, err := phaseRepo.CreatePhase(context.Background(), phase)

	s.Require().NoError(err)
	s.Require().NotNil(createdPhase)

	s.PhaseID = createdPhase.ID
	s.PhaseName = createdPhase.Name.String()

}

func (s *FeeSuite) TearDownSuite() {
	if s.TLDName != "" {
		repo := NewGormTLDRepo(s.db)
		_ = repo.DeleteByName(s.TLDName)
	}
	if s.PhaseName != "" {
		repo := NewGormPhaseRepository(s.db)
		_ = repo.DeletePhaseByName(context.Background(), s.PhaseName)
	}
}

func (s *FeeSuite) TestPhaseRepo_CreateFee() {
	tx := s.db.Begin()
	defer tx.Rollback()
	repo := NewFeeRepository(tx)

	// Setup a fee
	b := true
	fee, _ := entities.NewFee("USD", "verfication fee", 1000, &b)
	fee.PhaseID = s.PhaseID

	// Create the fee
	createdFee, err := repo.CreateFee(context.Background(), fee)
	s.Require().NoError(err)
	s.Require().NotNil(createdFee)

	s.Require().Equal(fee.Name, createdFee.Name)
	s.Require().Equal(fee.Currency, createdFee.Currency)
	s.Require().Equal(fee.Amount, createdFee.Amount)
	s.Require().True(*createdFee.Refundable)

	// Try and create the same fee again
	_, err = repo.CreateFee(context.Background(), fee)
	s.Require().Error(err)

}
