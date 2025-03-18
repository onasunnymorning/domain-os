package postgres

import (
	"context"
	"testing"

	"github.com/onasunnymorning/domain-os/internal/application/queries"
	"github.com/onasunnymorning/domain-os/internal/domain/entities"
	"github.com/stretchr/testify/suite"
	"gorm.io/gorm"
)

type RySuite struct {
	suite.Suite
	db     *gorm.DB
	ryRepo *RegistryOperatorRepository
}

func TestRySuite(t *testing.T) {
	suite.Run(t, new(RySuite))
}

func (s *RySuite) SetupSuite() {
	s.db = setupTestDB()
	s.ryRepo = NewGORMRegistryOperatorRepository(s.db)
}

func (s *RySuite) TestCreateRy() {
	tx := s.db.Begin()
	defer tx.Rollback()
	repo := NewGORMRegistryOperatorRepository(tx)

	ry, _ := entities.NewRegistryOperator("ra-dix", "Radix Inc.", "s@radix.com")
	createdRy, err := repo.Create(context.Background(), ry)
	s.Require().NoError(err)
	s.Require().NotNil(createdRy)

	// Try and create a duplicate
	createdRy, err = repo.Create(context.Background(), ry)
	s.Require().Error(err)
	s.Require().Nil(createdRy)
}

func (s *RySuite) TestGetByRyID() {
	tx := s.db.Begin()
	defer tx.Rollback()
	repo := NewGORMRegistryOperatorRepository(tx)

	ry, _ := entities.NewRegistryOperator("ra-dix", "Radix Inc.", "s@radix.com")
	createdRy, err := repo.Create(context.Background(), ry)
	s.Require().NoError(err)
	s.Require().NotNil(createdRy)

	fetchedRy, err := repo.GetByRyID(context.Background(), "ra-dix")
	s.Require().NoError(err)
	s.Require().NotNil(fetchedRy)
	s.Require().Equal(createdRy, fetchedRy)

	// Try and fetch a non-existent registry operator
	fetchedRy, err = repo.GetByRyID(context.Background(), "non-existent")
	s.Require().Error(err)
	s.Require().Nil(fetchedRy)
}

func (s *RySuite) TestUpdateRy() {
	tx := s.db.Begin()
	defer tx.Rollback()
	repo := NewGORMRegistryOperatorRepository(tx)

	ry, _ := entities.NewRegistryOperator("ra-dix", "Radix Inc.", "s@radix.com")
	createdRy, err := repo.Create(context.Background(), ry)
	s.Require().NoError(err)
	s.Require().NotNil(createdRy)

	createdRy.Name = "Radix Inc. Ltd."
	updatedRy, err := repo.Update(context.Background(), createdRy)
	s.Require().NoError(err)
	s.Require().NotNil(updatedRy)
	s.Require().Equal(updatedRy.Name, "Radix Inc. Ltd.")
}

func (s *RySuite) TestDeleteRy() {
	tx := s.db.Begin()
	defer tx.Rollback()
	repo := NewGORMRegistryOperatorRepository(tx)

	ry, _ := entities.NewRegistryOperator("ra-dix", "Radix Inc.", "s@radix.com")
	createdRy, err := repo.Create(context.Background(), ry)
	s.Require().NoError(err)
	s.Require().NotNil(createdRy)

	err = repo.DeleteByRyID(context.Background(), "ra-dix")
	s.Require().NoError(err)

	// Try and fetch the deleted registry operator
	fetchedRy, err := repo.GetByRyID(context.Background(), "ra-dix")
	s.Require().ErrorIs(err, entities.ErrRegistryOperatorNotFound)
	s.Require().Nil(fetchedRy)

	// Try and delete a non-existent registry operator
	err = repo.DeleteByRyID(context.Background(), "non-existent")
	s.Require().NoError(err)
}

func (s *RySuite) TestListRos() {
	tx := s.db.Begin()
	defer tx.Rollback()
	repo := NewGORMRegistryOperatorRepository(tx)

	ry1, _ := entities.NewRegistryOperator("ra-dix", "Radix Inc.", "s@radix.com")
	createdRy1, err := repo.Create(context.Background(), ry1)
	s.Require().NoError(err)
	s.Require().NotNil(createdRy1)

	ry2, _ := entities.NewRegistryOperator("xyz", "XYZ Inc.", "d@xyz.com")
	createdRy2, err := repo.Create(context.Background(), ry2)
	s.Require().NoError(err)
	s.Require().NotNil(createdRy2)

	ry3, _ := entities.NewRegistryOperator("abc", "ABC LLC", "me@abx.com")
	createdRy3, err := repo.Create(context.Background(), ry3)
	s.Require().NoError(err)
	s.Require().NotNil(createdRy3)

	ros, _, err := repo.List(context.Background(), queries.ListItemsQuery{
		PageSize: 2,
	})
	s.Require().NoError(err)
	s.Require().Len(ros, 2)

	ros, _, err = repo.List(context.Background(), queries.ListItemsQuery{
		PageSize: 25,
	})
	s.Require().NoError(err)
	s.Require().GreaterOrEqual(len(ros), 3)

	err = repo.DeleteByRyID(context.Background(), "ra-dix")
	s.Require().NoError(err)

	ros, _, err = repo.List(context.Background(), queries.ListItemsQuery{
		PageSize: 25,
	})
	s.Require().NoError(err)
	s.Require().GreaterOrEqual(len(ros), 2)

	err = repo.DeleteByRyID(context.Background(), "xyz")
	s.Require().NoError(err)

	ros, _, err = repo.List(context.Background(), queries.ListItemsQuery{
		PageSize: 25,
	})
	s.Require().NoError(err)
	s.Require().GreaterOrEqual(len(ros), 1)

	err = repo.DeleteByRyID(context.Background(), "abc")
	s.Require().NoError(err)

	ros, _, err = repo.List(context.Background(), queries.ListItemsQuery{
		PageSize: 25,
	})
	s.Require().NoError(err)
	s.Require().GreaterOrEqual(len(ros), 0)

}

func (s *RySuite) TestListRosFilters() {
	tx := s.db.Begin()
	defer tx.Rollback()
	repo := NewGORMRegistryOperatorRepository(tx)

	ry1, err := entities.NewRegistryOperator("ra-dix-listro", "Radix Inc.", "s@radix.com")
	s.Require().NoError(err)
	createdRy1, err := repo.Create(context.Background(), ry1)
	s.Require().NoError(err)
	s.Require().NotNil(createdRy1)

	ry2, err := entities.NewRegistryOperator("xyz-listro", "XYZ Inc.", "d@xyz.com")
	s.Require().NoError(err)
	createdRy2, err := repo.Create(context.Background(), ry2)
	s.Require().NoError(err)
	s.Require().NotNil(createdRy2)

	ry3, err := entities.NewRegistryOperator("abc-listro", "ABC LLC", "me@abx.com")
	s.Require().NoError(err)
	createdRy3, err := repo.Create(context.Background(), ry3)
	s.Require().NoError(err)
	s.Require().NotNil(createdRy3)

	// Pagesize = 2
	ros, cursor, err := repo.List(context.Background(), queries.ListItemsQuery{
		PageSize: 2,
		Filter: queries.ListRegistryOperatorsFilter{
			RyidLike: "listro",
		},
	})
	s.Require().NoError(err)
	s.Require().Len(ros, 2)
	s.Require().NotEmpty(ros[1].RyID)
	s.Require().NotEqual("", cursor)
	s.Require().Equal(ros[1].RyID.String(), cursor)

	// Pagesize = 2, cursor set
	ros, newCursor, err := repo.List(context.Background(), queries.ListItemsQuery{
		PageSize:   2,
		PageCursor: cursor,
		Filter: queries.ListRegistryOperatorsFilter{
			RyidLike: "listro",
		},
	})
	s.Require().NoError(err)
	s.Require().Len(ros, 1)
	s.Require().Empty(newCursor)

	// Test filtering RyIDLike
	ros, _, err = repo.List(context.Background(), queries.ListItemsQuery{
		PageSize: 25,
		Filter: queries.ListRegistryOperatorsFilter{
			RyidLike: "dix-listro",
		},
	})
	s.Require().NoError(err)
	s.Require().Len(ros, 1)
	s.Require().Equal(ros[0].RyID.String(), "ra-dix-listro")

	// Count with same filter
	count, err := repo.Count(context.Background(), queries.ListRegistryOperatorsFilter{
		RyidLike: "dix-listro",
	})
	s.Require().NoError(err)
	s.Require().Equal(int64(1), count)

	// Test filtering NameLike
	ros, _, err = repo.List(context.Background(), queries.ListItemsQuery{
		PageSize: 25,
		Filter: queries.ListRegistryOperatorsFilter{
			EmailLike: "xyz",
			RyidLike:  "listro",
		},
	})
	s.Require().NoError(err)
	s.Require().Len(ros, 1)
	s.Require().Equal(ros[0].Email, "d@xyz.com")

	// Count with same filter
	count, err = repo.Count(context.Background(), queries.ListRegistryOperatorsFilter{
		EmailLike: "xyz",
		RyidLike:  "listro",
	})
	s.Require().NoError(err)
	s.Require().Equal(int64(1), count)

	// Test filtering NameLike and pagesize
	ros, newCursor, err = repo.List(context.Background(), queries.ListItemsQuery{
		PageSize: 2,
		Filter: queries.ListRegistryOperatorsFilter{
			NameLike: "Inc",
			RyidLike: "listro",
		},
	})
	s.Require().NoError(err)
	s.Require().Len(ros, 2)
	s.Require().Empty(newCursor)

	err = repo.DeleteByRyID(context.Background(), "ra-dix-listro")
	s.Require().NoError(err)

	err = repo.DeleteByRyID(context.Background(), "xyz-listro")
	s.Require().NoError(err)

	err = repo.DeleteByRyID(context.Background(), "abc-listro")
	s.Require().NoError(err)

}
