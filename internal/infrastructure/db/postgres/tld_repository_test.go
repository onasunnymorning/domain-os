package postgres

import (
	"context"
	"testing"

	"github.com/onasunnymorning/domain-os/internal/application/queries"
	"github.com/onasunnymorning/domain-os/internal/domain/entities"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"gorm.io/gorm"
)

type TLDSuite struct {
	suite.Suite
	db               *gorm.DB
	RegistryOperator *entities.RegistryOperator
}

func TestTLDSuite(t *testing.T) {
	suite.Run(t, new(TLDSuite))
}

func (s *TLDSuite) SetupSuite() {
	s.db = getTestDB()
	NewGormTLDRepo(s.db)

	// Create a Registry Operator
	ro, _ := entities.NewRegistryOperator("TLDSuiteRo", "TLDSuiteRo", "TLDSuiteRo@me.email")
	roRepo := NewGORMRegistryOperatorRepository(s.db)
	_, err := roRepo.Create(context.Background(), ro)
	require.NoError(s.T(), err)
	createdRo, err := roRepo.GetByRyID(context.Background(), ro.RyID.String())
	require.NoError(s.T(), err)
	s.RegistryOperator = createdRo

	createdRo, err = roRepo.GetByRyID(context.Background(), ro.RyID.String())
	require.NoError(s.T(), err)
	s.RegistryOperator = createdRo

}

func (s *TLDSuite) TearDownSuite() {
	if s.RegistryOperator != nil {
		roRepo := NewGORMRegistryOperatorRepository(s.db)
		_ = roRepo.DeleteByRyID(context.Background(), s.RegistryOperator.RyID.String())
	}

}

func (s *TLDSuite) TestCreateTLD() {
	tx := s.db.Begin()
	defer tx.Rollback()
	repo := NewGormTLDRepo(tx)

	tld, _ := entities.NewTLD("com", "TLDSuiteRo")
	err := repo.Create(context.Background(), tld)
	require.NoError(s.T(), err)

	readTLD, err := repo.GetByName(context.Background(), tld.Name.String(), false)
	require.NoError(s.T(), err)
	require.NotNil(s.T(), readTLD)
	require.Equal(s.T(), tld, readTLD)

}

func (s *TLDSuite) TestCreateTLD_Duplicate() {
	tx := s.db.Begin()
	defer tx.Rollback()
	repo := NewGormTLDRepo(tx)

	tld, _ := entities.NewTLD("com", "TLDSuiteRo")
	err := repo.Create(context.Background(), tld)
	require.NoError(s.T(), err)

	// Create a duplicate
	err = repo.Create(context.Background(), tld)
	require.Error(s.T(), err)
}

func (s *TLDSuite) TestListTLD() {
	tx := s.db.Begin()
	defer tx.Rollback()
	repo := NewGormTLDRepo(tx)

	filter := queries.ListTldsFilter{
		RyIDEquals: "TLDSuiteRo",
	}

	tld1, _ := entities.NewTLD("com", filter.RyIDEquals)
	err := repo.Create(context.Background(), tld1)
	require.NoError(s.T(), err)

	tld2, _ := entities.NewTLD("net", filter.RyIDEquals)
	err = repo.Create(context.Background(), tld2)
	require.NoError(s.T(), err)

	// retrieve all tlds
	tlds, _, err := repo.List(context.Background(), queries.ListItemsQuery{PageSize: 25, Filter: filter})
	require.NoError(s.T(), err)
	require.NotNil(s.T(), tlds)
	require.Len(s.T(), tlds, 2)

	// count tlds
	count, err := repo.Count(context.Background(), filter)
	require.NoError(s.T(), err)
	require.Equal(s.T(), int64(2), count)

	// retrieve first page of tlds
	tlds, cursor, err := repo.List(context.Background(), queries.ListItemsQuery{PageSize: 1, Filter: filter})
	require.NoError(s.T(), err)
	require.NotNil(s.T(), tlds)
	require.Len(s.T(), tlds, 1)
	require.NotNil(s.T(), cursor)

	// retrieve last page of tlds
	tlds, cursor, err = repo.List(context.Background(), queries.ListItemsQuery{PageSize: 1, PageCursor: cursor, Filter: filter})
	require.NoError(s.T(), err)
	require.NotNil(s.T(), tlds)
	require.Len(s.T(), tlds, 1)
	require.Equal(s.T(), "", cursor)

	// Test filter by type
	filter.TypeEquals = "generic"
	tlds, _, err = repo.List(context.Background(), queries.ListItemsQuery{PageSize: 25, Filter: filter})
	require.NoError(s.T(), err)
	require.NotNil(s.T(), tlds)
	require.Len(s.T(), tlds, 2)

	// Count with same filter
	count, err = repo.Count(context.Background(), filter)
	require.NoError(s.T(), err)
	require.Equal(s.T(), int64(2), count)

	// Test filter by type counrty-code
	filter.TypeEquals = "country-code"
	tlds, _, err = repo.List(context.Background(), queries.ListItemsQuery{PageSize: 25, Filter: filter})
	require.NoError(s.T(), err)
	require.NotNil(s.T(), tlds)
	require.Len(s.T(), tlds, 0)

	// Count with same filter
	count, err = repo.Count(context.Background(), filter)
	require.NoError(s.T(), err)
	require.Equal(s.T(), int64(0), count)

	// Test filter by name
	filter.TypeEquals = ""
	filter.NameLike = "com"
	tlds, _, err = repo.List(context.Background(), queries.ListItemsQuery{PageSize: 25, Filter: filter})
	require.NoError(s.T(), err)
	require.NotNil(s.T(), tlds)
	require.Len(s.T(), tlds, 1)

	// Count with same filter
	count, err = repo.Count(context.Background(), filter)
	require.NoError(s.T(), err)
	require.Equal(s.T(), int64(1), count)
}

func (s *TLDSuite) TestUpdateTLD() {
	tx := s.db.Begin()
	defer tx.Rollback()
	repo := NewGormTLDRepo(tx)

	tld, _ := entities.NewTLD("com", "TLDSuiteRo")
	err := repo.Create(context.Background(), tld)
	require.NoError(s.T(), err)

	tld.Type = entities.TLDType("country-code")
	err = repo.Update(context.Background(), tld)
	require.NoError(s.T(), err)

	readTLD, err := repo.GetByName(context.Background(), tld.Name.String(), false)
	require.NoError(s.T(), err)
	require.NotNil(s.T(), readTLD)
	require.Equal(s.T(), tld, readTLD)
	require.Equal(s.T(), "country-code", readTLD.Type.String())
}

func (s *TLDSuite) TestGetTLD() {
	tx := s.db.Begin()
	defer tx.Rollback()
	repo := NewGormTLDRepo(tx)

	tld, _ := entities.NewTLD("com", "TLDSuiteRo")
	err := repo.Create(context.Background(), tld)
	require.NoError(s.T(), err)

	readTLD, err := repo.GetByName(context.Background(), tld.Name.String(), false)
	require.NoError(s.T(), err)
	require.NotNil(s.T(), readTLD)
	require.Equal(s.T(), tld, readTLD)

	// Test not found
	readTLD, err = repo.GetByName(context.Background(), "notfound", false)
	require.Error(s.T(), err)
	require.Nil(s.T(), readTLD)
}

func (s *TLDSuite) TestCountTLD() {
	tx := s.db.Begin()
	defer tx.Rollback()
	repo := NewGormTLDRepo(tx)

	tld, _ := entities.NewTLD("com", "TLDSuiteRo")
	err := repo.Create(context.Background(), tld)
	require.NoError(s.T(), err)

	count, err := repo.Count(context.Background(), queries.ListTldsFilter{})
	require.NoError(s.T(), err)
	require.Equal(s.T(), int64(6), count)
}
