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
	db   *gorm.DB
	ryID string
}

func TestPLSuite(t *testing.T) {
	suite.Run(t, new(PLSuite))
}

func (s *PLSuite) SetupSuite() {
	s.db = setupTestDB()

	//  Create a RegistryOperator
	repo := NewGORMRegistryOperatorRepository(s.db)

	ro, _ := entities.NewRegistryOperator("myOperator", "http://example.com", "e@mail.com")
	_, err := repo.Create(context.Background(), ro)
	s.Require().NoError(err)

	s.ryID = ro.RyID.String()
}

func (s *PLSuite) TearDownSuite() {
	if s.ryID != "" {
		repo := NewGORMRegistryOperatorRepository(s.db)
		_ = repo.DeleteByRyID(context.Background(), s.ryID)
	}
}

func (s *PLSuite) TestCreateList() {
	tx := s.db.Begin()
	defer tx.Rollback()
	repo := NewGORMPremiumListRepository(tx)

	pl, _ := entities.NewPremiumList("myPremiums", s.ryID)
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

	pl, _ := entities.NewPremiumList("myPremiums", s.ryID)
	createdPL, err := repo.Create(context.Background(), pl)
	s.Require().NoError(err)
	s.Require().NotNil(createdPL)

	fetchedPL, err := repo.GetByName(context.Background(), "myPremiums")
	s.Require().NoError(err)
	s.Require().NotNil(fetchedPL)
	// Round the times for comparison
	// Round the time to milliseconds before comparing
	createdPL.CreatedAt = entities.RoundTime(createdPL.CreatedAt)
	createdPL.UpdatedAt = entities.RoundTime(createdPL.UpdatedAt)
	fetchedPL.CreatedAt = entities.RoundTime(fetchedPL.CreatedAt)
	fetchedPL.UpdatedAt = entities.RoundTime(fetchedPL.UpdatedAt)
	s.Require().Equal(createdPL, fetchedPL)
}

func (s *PLSuite) TestDeleteByName() {
	tx := s.db.Begin()
	defer tx.Rollback()
	repo := NewGORMPremiumListRepository(tx)

	pl, _ := entities.NewPremiumList("myPremiums", s.ryID)
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

	pl1, _ := entities.NewPremiumList("myPremiums", s.ryID)
	createdPL1, err := repo.Create(context.Background(), pl1)
	s.Require().NoError(err)
	s.Require().NotNil(createdPL1)

	pl2, _ := entities.NewPremiumList("myPremiums2", s.ryID)
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
