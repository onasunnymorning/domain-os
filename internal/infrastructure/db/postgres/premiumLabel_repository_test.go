package postgres

import (
	"context"
	"testing"

	"github.com/onasunnymorning/domain-os/internal/application/queries"
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

	pl1, err := entities.NewPremiumLabel("myLabel", 100, 200, 300, 400, "USD", "class1", s.listName)
	s.Require().NoError(err)
	createdPL1, err := repo.Create(context.Background(), pl1)
	s.Require().NoError(err)
	s.Require().NotNil(createdPL1)

	pl2, err := entities.NewPremiumLabel("myLabel2", 100, 200, 300, 400, "USD", "class2", s.listName)
	s.Require().NoError(err)
	createdPL2, err := repo.Create(context.Background(), pl2)
	s.Require().NoError(err)
	s.Require().NotNil(createdPL2)

	pl3, err := entities.NewPremiumLabel("superdomain", 1000, 2000, 300, 4000, "PEN", "class2", s.listName)
	s.Require().NoError(err)
	createdPL3, err := repo.Create(context.Background(), pl3)
	s.Require().NoError(err)
	s.Require().NotNil(createdPL3)

	pls, cursor, err := repo.List(context.Background(), queries.ListItemsQuery{
		PageSize: 10,
	})
	s.Require().NoError(err)
	s.Require().Len(pls, 3)
	s.Require().Equal(cursor, "")

	// Limit to 2
	pls, cursor, err = repo.List(context.Background(), queries.ListItemsQuery{
		PageSize: 2,
	})
	s.Require().NoError(err)
	s.Require().Len(pls, 2)
	s.Require().NotEmpty(cursor)

	// Retrieve the next and last page
	pls, cursor, err = repo.List(context.Background(), queries.ListItemsQuery{
		PageSize:   2,
		PageCursor: cursor,
	})
	s.Require().NoError(err)
	s.Require().Len(pls, 1)
	s.Require().Equal(cursor, "")

	//  Test Filters: ClassEquals
	pls, cursor, err = repo.List(context.Background(), queries.ListItemsQuery{
		PageSize: 10,
		Filter: queries.ListPremiumLabelsFilter{
			ClassEquals: "class1",
		},
	})
	s.Require().NoError(err)
	s.Require().Len(pls, 1)
	s.Require().Equal(cursor, "")

	//  Test Filters: LabelLike
	pls, cursor, err = repo.List(context.Background(), queries.ListItemsQuery{
		PageSize: 10,
		Filter: queries.ListPremiumLabelsFilter{
			LabelLike: "myLabel",
		},
	})
	s.Require().NoError(err)
	s.Require().Len(pls, 2)
	s.Require().Equal(cursor, "")

	//  Test Filters: CurrencyEquals
	pls, cursor, err = repo.List(context.Background(), queries.ListItemsQuery{
		PageSize: 10,
		Filter: queries.ListPremiumLabelsFilter{
			CurrencyEquals: "USD",
		},
	})
	s.Require().NoError(err)
	s.Require().Len(pls, 2)
	s.Require().Equal(cursor, "")

	//  Test Filters: PremiumListNameEquals
	pls, cursor, err = repo.List(context.Background(), queries.ListItemsQuery{
		PageSize: 10,
		Filter: queries.ListPremiumLabelsFilter{
			PremiumListNameEquals: s.listName,
		},
	})
	s.Require().NoError(err)
	s.Require().Len(pls, 3)
	s.Require().Equal(cursor, "")

	//  Test Filters: PremiumListNameEquals + LabelLike
	pls, cursor, err = repo.List(context.Background(), queries.ListItemsQuery{
		PageSize: 10,
		Filter: queries.ListPremiumLabelsFilter{
			PremiumListNameEquals: s.listName,
			LabelLike:             "super",
		},
	})
	s.Require().NoError(err)
	s.Require().Len(pls, 1)
	s.Require().Equal(cursor, "")

	//  Test Filters: ClassEquals with pagination
	pls, cursor, err = repo.List(context.Background(), queries.ListItemsQuery{
		PageSize: 2,
		Filter: queries.ListPremiumLabelsFilter{
			ClassEquals: "class2",
		},
	})
	s.Require().NoError(err)
	s.Require().Len(pls, 2)
	s.Require().Equal(cursor, "")

	//  Test Filters: ClassEquals with pagination
	pls, cursor, err = repo.List(context.Background(), queries.ListItemsQuery{
		PageSize: 1,
		Filter: queries.ListPremiumLabelsFilter{
			ClassEquals: "class2",
		},
	})
	s.Require().NoError(err)
	s.Require().Len(pls, 1)
	s.Require().NotEqual(cursor, "")

	//  Test Filters: ClassEquals with error in pagination
	pls, cursor, err = repo.List(context.Background(), queries.ListItemsQuery{
		PageSize:   1,
		PageCursor: "invalidCursor",
		Filter: queries.ListPremiumLabelsFilter{
			ClassEquals: "class2",
		},
	})
	s.Require().Error(err)
	s.Require().Len(pls, 0)
	s.Require().Equal(cursor, "")

	//  Test Filters: RegistrationAmountEquals
	pls, cursor, err = repo.List(context.Background(), queries.ListItemsQuery{
		PageSize: 10,
		Filter: queries.ListPremiumLabelsFilter{
			RegistrationAmountEquals: "100",
		},
	})
	s.Require().NoError(err)
	s.Require().Len(pls, 2)
	s.Require().Equal(cursor, "")

	//  Test Filters: RenewalAmountEquals
	pls, cursor, err = repo.List(context.Background(), queries.ListItemsQuery{
		PageSize: 10,
		Filter: queries.ListPremiumLabelsFilter{
			RenewalAmountEquals: "200",
		},
	})
	s.Require().NoError(err)
	s.Require().Len(pls, 2)
	s.Require().Equal(cursor, "")

	//  Test Filters: TransferAmountEquals
	pls, cursor, err = repo.List(context.Background(), queries.ListItemsQuery{
		PageSize: 10,
		Filter: queries.ListPremiumLabelsFilter{
			TransferAmountEquals: "300",
		},
	})
	s.Require().NoError(err)
	s.Require().Len(pls, 3)
	s.Require().Equal(cursor, "")

	//  Test Filters: RestoreAmountEquals
	pls, cursor, err = repo.List(context.Background(), queries.ListItemsQuery{
		PageSize: 10,
		Filter: queries.ListPremiumLabelsFilter{
			RestoreAmountEquals: "4000",
		},
	})
	s.Require().NoError(err)
	s.Require().Len(pls, 1)
	s.Require().Equal(cursor, "")

	// Test invalidFilterType
	pls, cursor, err = repo.List(context.Background(), queries.ListItemsQuery{
		PageSize: 10,
		Filter: queries.ListRegistrarsFilter{
			ClidLike: "dummy",
		},
	})
	s.Require().Error(err)
	s.Require().Len(pls, 0)
	s.Require().Equal(cursor, "")

	// Delete one of the premium labels
	err = repo.DeleteByLabelListAndCurrency(context.Background(), "myLabel", s.listName, "USD")
	s.Require().NoError(err)

	// List again
	pls, _, err = repo.List(context.Background(), queries.ListItemsQuery{
		PageSize: 10,
	})
	s.Require().NoError(err)
	s.Require().Len(pls, 2)

	// Delete the rest
	err = repo.DeleteByLabelListAndCurrency(context.Background(), "myLabel2", s.listName, "USD")
	s.Require().NoError(err)
	err = repo.DeleteByLabelListAndCurrency(context.Background(), "superdomain", s.listName, "PEN")
	s.Require().NoError(err)

	// List again
	pls, _, err = repo.List(context.Background(), queries.ListItemsQuery{
		PageSize: 10,
	})
	s.Require().NoError(err)
	s.Require().Len(pls, 0)
}
