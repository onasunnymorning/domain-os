package postgres

import (
	"context"
	"testing"

	"github.com/onasunnymorning/domain-os/internal/domain/entities"
	"github.com/stretchr/testify/suite"
	"gorm.io/gorm"
)

type PLabelSuite struct {
	suite.Suite
	db       *gorm.DB
	ryID     string
	listName string
}

func TestPLabelSuite(t *testing.T) {
	suite.Run(t, new(PLabelSuite))
}

func (s *PLabelSuite) SetupSuite() {
	s.db = setupTestDB()

	//  Create a RegistryOperator
	repo := NewGORMRegistryOperatorRepository(s.db)

	ro, _ := entities.NewRegistryOperator("myOperator", "http://example.com", "e@mail.com")
	_, err := repo.Create(context.Background(), ro)
	s.Require().NoError(err)

	s.ryID = ro.RyID.String()

	// Create a PremiumList
	pl, _ := entities.NewPremiumList("myPremiums", s.ryID)
	repoPL := NewGORMPremiumListRepository(s.db)
	createdPL, err := repoPL.Create(context.Background(), pl)
	s.Require().NoError(err)
	s.Require().NotNil(createdPL)
	s.listName = createdPL.Name
}

func (s *PLabelSuite) TearDownSuite() {
	if s.listName != "" {
		repo := NewGORMPremiumListRepository(s.db)
		_ = repo.DeleteByName(context.Background(), s.listName)
	}
	if s.ryID != "" {
		repo := NewGORMRegistryOperatorRepository(s.db)
		_ = repo.DeleteByRyID(context.Background(), s.ryID)
	}
}

func (s *PLabelSuite) TestPremiumLabelRepo_Create() {
	tx := s.db.Begin()
	defer tx.Rollback()

	repo := NewGORMPremiumLabelRepository(tx)

	pl, _ := entities.NewPremiumLabel("myLabel", 100, 200, 300, 400, "USD", "class", s.listName)
	createdPL, err := repo.Create(context.Background(), pl)
	s.Require().NoError(err)
	s.Require().NotNil(createdPL)

	// Try and create a duplicate
	createdPL, err = repo.Create(context.Background(), pl)
	s.Require().Error(err)
	s.Require().Nil(createdPL)

	// Try and create with a non-existing list
	pl.PremiumListName = "nonExistingList"
	createdPL, err = repo.Create(context.Background(), pl)
	s.Require().Error(err)
	s.Require().Nil(createdPL)

}

func (s *PLabelSuite) TestPremiumLabelRepo_GetByLabelListAndCurrency() {
	tx := s.db.Begin()
	defer tx.Rollback()

	repo := NewGORMPremiumLabelRepository(tx)

	pl, _ := entities.NewPremiumLabel("myLabel", 100, 200, 300, 400, "USD", "class", s.listName)
	createdPL, err := repo.Create(context.Background(), pl)
	s.Require().NoError(err)
	s.Require().NotNil(createdPL)

	fetchedPL, err := repo.GetByLabelListAndCurrency(context.Background(), "myLabel", s.listName, "USD")
	s.Require().NoError(err)
	s.Require().NotNil(fetchedPL)
	s.Require().Equal(createdPL, fetchedPL)

	// Try retrieving a non-existing premium label
	fetchedPL, err = repo.GetByLabelListAndCurrency(context.Background(), "nonExistingLabel", s.listName, "USD")
	s.Require().Error(err)
	s.Require().Nil(fetchedPL)
}

func (s *PLabelSuite) TestPremiumLabelRepo_DeleteByLabelListAndCurrency() {
	tx := s.db.Begin()
	defer tx.Rollback()

	repo := NewGORMPremiumLabelRepository(tx)

	pl, _ := entities.NewPremiumLabel("myLabel", 100, 200, 300, 400, "USD", "class", s.listName)
	createdPL, err := repo.Create(context.Background(), pl)
	s.Require().NoError(err)
	s.Require().NotNil(createdPL)

	err = repo.DeleteByLabelListAndCurrency(context.Background(), "myLabel", s.listName, "USD")
	s.Require().NoError(err)

	// Try and retrieve the deleted premium label
	fetchedPL, err := repo.GetByLabelListAndCurrency(context.Background(), "myLabel", s.listName, "USD")
	s.Require().Error(err)
	s.Require().Nil(fetchedPL)

	// Try and delete a non-existing premium label
	err = repo.DeleteByLabelListAndCurrency(context.Background(), "myLabel", s.listName, "USD")
	s.Require().NoError(err)
}

func (s *PLabelSuite) TestPremiumLabelRepo_List() {
	tx := s.db.Begin()
	defer tx.Rollback()

	repo := NewGORMPremiumLabelRepository(tx)

	pl1, _ := entities.NewPremiumLabel("myLabel", 100, 200, 300, 400, "USD", "class", s.listName)
	_, err := repo.Create(context.Background(), pl1)
	s.Require().NoError(err)

	pl2, _ := entities.NewPremiumLabel("myLabel2", 100, 200, 300, 400, "USD", "class", s.listName)
	_, err = repo.Create(context.Background(), pl2)
	s.Require().NoError(err)

	pl3, _ := entities.NewPremiumLabel("myLabel3", 100, 200, 300, 400, "USD", "class", s.listName)
	_, err = repo.Create(context.Background(), pl3)
	s.Require().NoError(err)

	pls, err := repo.List(context.Background(), 10, "", "", "")
	s.Require().NoError(err)
	s.Require().Len(pls, 3)

	// Limit to 2
	pls, err = repo.List(context.Background(), 2, "", "", "")
	s.Require().NoError(err)
	s.Require().Len(pls, 2)

	// Delete one of the premium labels
	err = repo.DeleteByLabelListAndCurrency(context.Background(), "myLabel", s.listName, "USD")
	s.Require().NoError(err)

	// List again
	pls, err = repo.List(context.Background(), 10, "", "", "")
	s.Require().NoError(err)
	s.Require().Len(pls, 2)

	// Delete the rest
	err = repo.DeleteByLabelListAndCurrency(context.Background(), "myLabel2", s.listName, "USD")
	s.Require().NoError(err)
	err = repo.DeleteByLabelListAndCurrency(context.Background(), "myLabel3", s.listName, "USD")
	s.Require().NoError(err)

	// List again
	pls, err = repo.List(context.Background(), 10, "", "", "")
	s.Require().NoError(err)
	s.Require().Len(pls, 0)
}
