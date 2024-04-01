package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/onasunnymorning/domain-os/internal/domain/entities"
	"github.com/stretchr/testify/suite"
	"gorm.io/gorm"
)

type PriceSuite struct {
	suite.Suite
	db        *gorm.DB
	TLDName   string
	PhaseID   int64
	PhaseName string
}

func TestPriceSuite(t *testing.T) {
	suite.Run(t, new(PriceSuite))
}

func (s *PriceSuite) SetupSuite() {
	s.db = setupTestDB()
	repo := NewGormTLDRepo(s.db)
	phaseRepo := NewGormPhaseRepository(s.db)

	// Create a tld
	tld, _ := entities.NewTLD("phaseprice.test")
	err := repo.Create(tld)
	s.Require().NoError(err)

	readTLD, err := repo.GetByName(tld.Name.String())
	s.Require().NoError(err)
	s.Require().NotNil(readTLD)
	s.Require().Equal(tld, readTLD)
	// Save to the Suite
	s.TLDName = tld.Name.String()

	// Create a phase
	phase, _ := entities.NewPhase("TestPhase", "Launch", time.Now().UTC())
	phase.TLDName = entities.DomainName(s.TLDName)
	createdPhase, err := phaseRepo.CreatePhase(context.Background(), phase)

	s.Require().NoError(err)
	s.Require().NotNil(createdPhase)

	s.PhaseID = createdPhase.ID
	s.PhaseName = createdPhase.Name.String()

}

func (s *PriceSuite) TearDownSuite() {
	if s.TLDName != "" {
		repo := NewGormTLDRepo(s.db)
		_ = repo.DeleteByName(s.TLDName)
	}
	if s.PhaseName != "" {
		repo := NewGormPhaseRepository(s.db)
		_ = repo.DeletePhaseByName(context.Background(), s.PhaseName)
	}
}

func (s *PriceSuite) TestPriceRepo_CreatePriceFK() {
	tx := s.db.Begin()
	defer tx.Rollback()
	repo := NewGormPriceRepository(tx)

	// Setup a price
	price, err := entities.NewPrice("USD", 10000, 10000, 10000, 0)
	s.Require().Nil(err)
	s.Require().NotNil(price)

	// Try and create a price with an invalid phase ID
	_, err = repo.CreatePrice(context.Background(), price)
	s.Require().Error(err)
}

func (s *PriceSuite) TestPriceRepo_CreatePrice() {
	tx := s.db.Begin()
	defer tx.Rollback()
	repo := NewGormPriceRepository(tx)

	// Setup a price
	price, err := entities.NewPrice("USD", 10000, 10000, 10000, 0)
	s.Require().Nil(err)
	s.Require().NotNil(price)
	price.PhaseID = s.PhaseID

	// Create the price
	createdPrice, err := repo.CreatePrice(context.Background(), price)
	s.Require().NoError(err)
	s.Require().NotNil(createdPrice)

	s.Require().Equal(price.Currency, createdPrice.Currency)
	s.Require().Equal(price.RegistrationAmount, createdPrice.RegistrationAmount)
	s.Require().Equal(price.RenewalAmount, createdPrice.RenewalAmount)
	s.Require().Equal(price.TransferAmount, createdPrice.TransferAmount)
	s.Require().Equal(price.RestoreAmount, createdPrice.RestoreAmount)

	// Try and create the same price again
	_, err = repo.CreatePrice(context.Background(), price)
	s.Require().Error(err)
}

func (s *PriceSuite) TestPriceRepo_GetPrice() {
	tx := s.db.Begin()
	defer tx.Rollback()
	repo := NewGormPriceRepository(tx)

	// Setup a price
	price, err := entities.NewPrice("USD", 10000, 10000, 10000, 0)
	s.Require().Nil(err)
	s.Require().NotNil(price)
	price.PhaseID = s.PhaseID

	// Create the price
	createdPrice, err := repo.CreatePrice(context.Background(), price)
	s.Require().NoError(err)
	s.Require().NotNil(createdPrice)

	// Retrieve the price
	readPrice, err := repo.GetPrice(context.Background(), s.PhaseID, createdPrice.Currency)
	s.Require().NoError(err)
	s.Require().NotNil(readPrice)

	s.Require().Equal(price.Currency, readPrice.Currency)
	s.Require().Equal(price.RegistrationAmount, readPrice.RegistrationAmount)
	s.Require().Equal(price.RenewalAmount, readPrice.RenewalAmount)
	s.Require().Equal(price.TransferAmount, readPrice.TransferAmount)
	s.Require().Equal(price.RestoreAmount, readPrice.RestoreAmount)

	// Try and retrieve a price that doesn't exist
	readPrice, err = repo.GetPrice(context.Background(), s.PhaseID, "non-existent")
	s.Require().Error(err)
	s.Require().Nil(readPrice)
}

func (s *PriceSuite) TestPriceRepo_DeletePrice() {
	tx := s.db.Begin()
	defer tx.Rollback()
	repo := NewGormPriceRepository(tx)

	// Setup a price
	price, err := entities.NewPrice("USD", 10000, 10000, 10000, 0)
	s.Require().Nil(err)
	s.Require().NotNil(price)
	price.PhaseID = s.PhaseID

	// Create the price
	createdPrice, err := repo.CreatePrice(context.Background(), price)
	s.Require().NoError(err)
	s.Require().NotNil(createdPrice)

	// retrieve the price
	readPrice, err := repo.GetPrice(context.Background(), s.PhaseID, createdPrice.Currency)
	s.Require().NoError(err)
	s.Require().NotNil(readPrice)

	// Delete the price
	err = repo.DeletePrice(context.Background(), s.PhaseID, createdPrice.Currency)
	s.Require().NoError(err)

	// Try and delete the price again
	err = repo.DeletePrice(context.Background(), s.PhaseID, createdPrice.Currency)
	s.Require().NoError(err)

	// Try and retrieve the price
	_, err = repo.GetPrice(context.Background(), s.PhaseID, createdPrice.Currency)
	s.Require().Error(err)

}
