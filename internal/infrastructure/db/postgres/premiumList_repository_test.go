package postgres

import (
	"context"
	"testing"

	"github.com/onasunnymorning/domain-os/internal/domain/entities"
	"github.com/stretchr/testify/suite"
	"gorm.io/gorm"
)

type PLSuite struct {
	suite.Suite
	db *gorm.DB
}

func TestPLSuite(t *testing.T) {
	suite.Run(t, new(PLSuite))
}

func (s *PLSuite) SetupSuite() {
	s.db = setupTestDB()
	NewGORMPremiumListRepository(s.db)
}

func (s *PLSuite) TestCreateList() {
	tx := s.db.Begin()
	defer tx.Rollback()
	repo := NewGORMPremiumListRepository(tx)

	pl, _ := entities.NewPremiumList("myPremiums")
	createdPL, err := repo.Create(context.Background(), pl)
	s.Require().NoError(err)
	s.Require().NotNil(createdPL)

	// Try and create a duplicate
	createdPL, err = repo.Create(context.Background(), pl)
	s.Require().Error(err)
	s.Require().Nil(createdPL)
}

func (s *PLSuite) TestGetByName() {
	tx := s.db.Begin()
	defer tx.Rollback()
	repo := NewGORMPremiumListRepository(tx)

	pl, _ := entities.NewPremiumList("myPremiums")
	createdPL, err := repo.Create(context.Background(), pl)
	s.Require().NoError(err)
	s.Require().NotNil(createdPL)

	fetchedPL, err := repo.GetByName(context.Background(), "myPremiums")
	s.Require().NoError(err)
	s.Require().NotNil(fetchedPL)
	s.Require().Equal(createdPL, fetchedPL)
}

func (s *PLSuite) TestDeleteByName() {
	tx := s.db.Begin()
	defer tx.Rollback()
	repo := NewGORMPremiumListRepository(tx)

	pl, _ := entities.NewPremiumList("myPremiums")
	createdPL, err := repo.Create(context.Background(), pl)
	s.Require().NoError(err)
	s.Require().NotNil(createdPL)

	err = repo.DeleteByName(context.Background(), "myPremiums")
	s.Require().NoError(err)

	fetchedPL, err := repo.GetByName(context.Background(), "myPremiums")
	s.Require().Error(err)
	s.Require().Nil(fetchedPL)

	// Try and delete a non-existent premium list
	err = repo.DeleteByName(context.Background(), "non-existent")
	s.Require().NoError(err)
}

func (s *PLSuite) TestList() {
	tx := s.db.Begin()
	defer tx.Rollback()
	repo := NewGORMPremiumListRepository(tx)

	pl1, _ := entities.NewPremiumList("myPremiums")
	createdPL1, err := repo.Create(context.Background(), pl1)
	s.Require().NoError(err)
	s.Require().NotNil(createdPL1)

	pl2, _ := entities.NewPremiumList("myPremiums2")
	createdPL2, err := repo.Create(context.Background(), pl2)
	s.Require().NoError(err)
	s.Require().NotNil(createdPL2)

	pls, err := repo.List(context.Background(), 10, "")
	s.Require().NoError(err)
	s.Require().Len(pls, 2)

	pls, err = repo.List(context.Background(), 1, "")
	s.Require().NoError(err)
	s.Require().Len(pls, 1)

	err = repo.DeleteByName(context.Background(), "myPremiums")
	s.Require().NoError(err)

	pls, err = repo.List(context.Background(), 10, "")
	s.Require().NoError(err)
	s.Require().Len(pls, 1)

	err = repo.DeleteByName(context.Background(), "myPremiums2")
	s.Require().NoError(err)

	pls, err = repo.List(context.Background(), 10, "")
	s.Require().NoError(err)
	s.Require().Len(pls, 0)
}
