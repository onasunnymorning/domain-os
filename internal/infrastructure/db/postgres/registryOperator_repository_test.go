package postgres

import (
	"testing"
	"time"

	"github.com/onasunnymorning/domain-os/internal/domain/entities"
	"github.com/stretchr/testify/suite"
	"gorm.io/gorm"
)

type RySuite struct {
	suite.Suite
	db *gorm.DB
}

func TestRySuite(t *testing.T) {
	suite.Run(t, new(RySuite))
}

func (s *RySuite) SetupSuite() {
	s.db = setupTestDB()
	NewGORMRegistryOperatorRepository(s.db)
}

func (s *RySuite) TestCreateRy() {
	tx := s.db.Begin()
	defer tx.Rollback()
	repo := NewGORMRegistryOperatorRepository(tx)

	ry, _ := entities.NewRegistryOperator("ra-dix", "Radix Inc.", "s@radix.com")
	createdRy, err := repo.Create(ry)
	s.Require().NoError(err)
	s.Require().NotNil(createdRy)

	// Try and create a duplicate
	createdRy, err = repo.Create(ry)
	s.Require().Error(err)
	s.Require().Nil(createdRy)
}

func (s *RySuite) TestGetByRyID() {
	tx := s.db.Begin()
	defer tx.Rollback()
	repo := NewGORMRegistryOperatorRepository(tx)

	ry, _ := entities.NewRegistryOperator("ra-dix", "Radix Inc.", "s@radix.com")
	createdRy, err := repo.Create(ry)
	s.Require().NoError(err)
	s.Require().NotNil(createdRy)

	fetchedRy, err := repo.GetByRyID("ra-dix")
	s.Require().NoError(err)
	s.Require().NotNil(fetchedRy)
	// Round the time to milliseconds before comparing
	createdRy.CreatedAt = createdRy.CreatedAt.Truncate(time.Nanosecond)
	createdRy.UpdatedAt = createdRy.UpdatedAt.Truncate(time.Nanosecond)
	s.Require().Equal(createdRy, fetchedRy)

	// Try and fetch a non-existent registry operator
	fetchedRy, err = repo.GetByRyID("non-existent")
	s.Require().Error(err)
	s.Require().Nil(fetchedRy)
}

func (s *RySuite) TestUpdateRy() {
	tx := s.db.Begin()
	defer tx.Rollback()
	repo := NewGORMRegistryOperatorRepository(tx)

	ry, _ := entities.NewRegistryOperator("ra-dix", "Radix Inc.", "s@radix.com")
	createdRy, err := repo.Create(ry)
	s.Require().NoError(err)
	s.Require().NotNil(createdRy)

	createdRy.Name = "Radix Inc. Ltd."
	updatedRy, err := repo.Update(createdRy)
	s.Require().NoError(err)
	s.Require().NotNil(updatedRy)
	s.Require().Equal(updatedRy.Name, "Radix Inc. Ltd.")
}

func (s *RySuite) TestDeleteRy() {
	tx := s.db.Begin()
	defer tx.Rollback()
	repo := NewGORMRegistryOperatorRepository(tx)

	ry, _ := entities.NewRegistryOperator("ra-dix", "Radix Inc.", "s@radix.com")
	createdRy, err := repo.Create(ry)
	s.Require().NoError(err)
	s.Require().NotNil(createdRy)

	err = repo.DeleteByRyID("ra-dix")
	s.Require().NoError(err)

	// Try and fetch the deleted registry operator
	fetchedRy, err := repo.GetByRyID("ra-dix")
	s.Require().ErrorIs(err, entities.ErrRegistryOperatorNotFound)
	s.Require().Nil(fetchedRy)

	// Try and delete a non-existent registry operator
	err = repo.DeleteByRyID("non-existent")
	s.Require().NoError(err)
}
