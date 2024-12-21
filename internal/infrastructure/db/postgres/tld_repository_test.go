package postgres

import (
	"context"
	"testing"

	"github.com/onasunnymorning/domain-os/internal/domain/entities"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"gorm.io/gorm"
)

type TLDSuite struct {
	suite.Suite
	db *gorm.DB
}

func TestTLDSuite(t *testing.T) {
	suite.Run(t, new(TLDSuite))
}

func (s *TLDSuite) SetupSuite() {
	s.db = getTestDB()
	NewGormTLDRepo(s.db)
}

func (s *TLDSuite) TestCreateTLD() {
	tx := s.db.Begin()
	defer tx.Rollback()
	repo := NewGormTLDRepo(tx)

	tld, _ := entities.NewTLD("com")
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

	tld, _ := entities.NewTLD("com")
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

	tld1, _ := entities.NewTLD("com")
	err := repo.Create(context.Background(), tld1)
	require.NoError(s.T(), err)

	tld2, _ := entities.NewTLD("net")
	err = repo.Create(context.Background(), tld2)
	require.NoError(s.T(), err)

	tlds, err := repo.List(context.Background(), 2, "")
	require.NoError(s.T(), err)
	require.NotNil(s.T(), tlds)
	require.Len(s.T(), tlds, 2)
}

func (s *TLDSuite) TestUpdateTLD() {
	tx := s.db.Begin()
	defer tx.Rollback()
	repo := NewGormTLDRepo(tx)

	tld, _ := entities.NewTLD("com")
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

	tld, _ := entities.NewTLD("com")
	err := repo.Create(context.Background(), tld)
	require.NoError(s.T(), err)

	readTLD, err := repo.GetByName(context.Background(), tld.Name.String(), false)
	require.NoError(s.T(), err)
	require.NotNil(s.T(), readTLD)
	require.Equal(s.T(), tld, readTLD)

	// Test not found
	readTLD, err = repo.GetByName(context.Background(), "notfound", false)
	require.Error(s.T(), err)
}

func (s *TLDSuite) TestCountTLD() {
	tx := s.db.Begin()
	defer tx.Rollback()
	repo := NewGormTLDRepo(tx)

	tld, _ := entities.NewTLD("com")
	err := repo.Create(context.Background(), tld)
	require.NoError(s.T(), err)

	count, err := repo.Count(context.Background())
	require.NoError(s.T(), err)
	require.Equal(s.T(), int64(6), count)
}
