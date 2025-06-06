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
	Ry        *entities.RegistryOperator
}

func TestPhaseFeeSuite(t *testing.T) {
	suite.Run(t, new(FeeSuite))
}

func (s *FeeSuite) SetupSuite() {
	s.db = setupTestDB()
	repo := NewGormTLDRepo(s.db)
	phaseRepo := NewGormPhaseRepository(s.db)

	// Create a Registry Operator
	ro, _ := entities.NewRegistryOperator("FeeSuiteRo", "FeeSuiteRo", "FeeSuiteRo@my.email")
	roRepo := NewGORMRegistryOperatorRepository(s.db)
	_, err := roRepo.Create(context.Background(), ro)
	s.Require().NoError(err)
	createdRo, err := roRepo.GetByRyID(context.Background(), ro.RyID.String())
	s.Require().NoError(err)
	s.Ry = createdRo

	// Create a tld
	tld, _ := entities.NewTLD("phasefee.test", "FeeSuiteRo")
	err = repo.Create(context.Background(), tld)
	s.Require().NoError(err)

	readTLD, err := repo.GetByName(context.Background(), tld.Name.String(), false)
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
		_ = repo.DeleteByName(context.Background(), s.TLDName)
	}
	if s.PhaseName != "" {
		repo := NewGormPhaseRepository(s.db)
		_ = repo.DeletePhaseByTLDAndName(context.Background(), s.TLDName, s.PhaseName)
	}
	if s.Ry != nil {
		repo := NewGORMRegistryOperatorRepository(s.db)
		_ = repo.DeleteByRyID(context.Background(), s.Ry.RyID.String())
	}
}

func (s *FeeSuite) TestPhaseRepo_CreateFeeFK() {
	tx := s.db.Begin()
	defer tx.Rollback()
	repo := NewFeeRepository(tx)

	// Setup a fee
	b := true
	fee, _ := entities.NewFee("USD", "verfication fee", 1000, &b)

	// Create the fee without a phase ID
	_, err := repo.CreateFee(context.Background(), fee)
	s.Require().Error(err)

}

func (s *FeeSuite) TestPhaseRepo_CreateFee() {
	tx := s.db.Begin()
	defer tx.Rollback()
	repo := NewFeeRepository(tx)

	// Setup a fee
	b := true
	fee, err := entities.NewFee("USD", "verfication fee", 1000, &b)
	s.Require().NoError(err)
	s.Require().NotNil(fee)
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

func (s *FeeSuite) TestPhaseRepo_GetFee() {
	tx := s.db.Begin()
	defer tx.Rollback()
	repo := NewFeeRepository(tx)

	// Setup a fee
	b := true
	fee, _ := entities.NewFee("USD", "verfication fee", 1000, &b)
	fee.PhaseID = s.PhaseID

	// Create the fee
	createdFee, _ := repo.CreateFee(context.Background(), fee)
	s.Require().NotNil(createdFee)

	// Read the Fee
	readFee, err := repo.GetFee(context.Background(), s.PhaseID, fee.Name.String(), fee.Currency)
	s.Require().NoError(err)
	s.Require().NotNil(readFee)

	s.Require().Equal(fee.Name, readFee.Name)
	s.Require().Equal(fee.Currency, readFee.Currency)
	s.Require().Equal(fee.Amount, readFee.Amount)
	s.Require().True(*readFee.Refundable)

	// Read a fee that doesn't exist
	readFee, err = repo.GetFee(context.Background(), s.PhaseID, "non-existent", "USD")
	s.Require().Error(err)
	s.Require().Nil(readFee)

}

func (s *FeeSuite) TestPhaseRepo_DeleteFee() {
	tx := s.db.Begin()
	defer tx.Rollback()
	repo := NewFeeRepository(tx)

	// Setup a fee
	b := true
	fee, _ := entities.NewFee("USD", "verfication fee", 1000, &b)
	fee.PhaseID = s.PhaseID

	// Create the fee
	createdFee, _ := repo.CreateFee(context.Background(), fee)
	s.Require().NotNil(createdFee)

	// Read the Fee
	readFee, err := repo.GetFee(context.Background(), s.PhaseID, fee.Name.String(), fee.Currency)
	s.Require().NoError(err)
	s.Require().NotNil(readFee)

	// Delete the fee
	err = repo.DeleteFee(context.Background(), s.PhaseID, fee.Name.String(), fee.Currency)
	s.Require().NoError(err)

	// Read the Fee now that it is gone
	_, err = repo.GetFee(context.Background(), s.PhaseID, fee.Name.String(), fee.Currency)
	s.Require().Error(err)

	// Delete the fee again
	err = repo.DeleteFee(context.Background(), s.PhaseID, fee.Name.String(), fee.Currency)
	s.Require().NoError(err)
}
