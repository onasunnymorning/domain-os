package postgres

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"gorm.io/gorm"

	"github.com/onasunnymorning/domain-os/internal/domain/entities"
)

func TestToDBTld(t *testing.T) {
	tld, err := entities.NewTLD("com")
	if err != nil {
		t.Fatal(err)
	}

	dbtld := ToDBTLD(tld)

	require.Equal(t, tld.Name.String(), dbtld.Name, "TLD Name mismatch")
	require.Equal(t, tld.Type.String(), dbtld.Type, "TLD Type mismatch")
	require.Equal(t, tld.UName.String(), dbtld.UName, "TLD UName mismatch")
	require.Equal(t, tld.CreatedAt, dbtld.CreatedAt, "TLD CreatedAt mismatch")
	require.Equal(t, tld.UpdatedAt, dbtld.UpdatedAt, "TLD UpdatedAt mismatch")
}

func TestFromDBTld(t *testing.T) {
	dbtld := &TLD{
		Name:      "com",
		Type:      "generic",
		UName:     "com",
		CreatedAt: entities.RoundTime(time.Now().UTC()),
		UpdatedAt: entities.RoundTime(time.Now().UTC()),
	}

	tld := FromDBTLD(dbtld)

	require.Equal(t, dbtld.Name, tld.Name.String(), "TLD Name mismatch")
	require.Equal(t, dbtld.Type, tld.Type.String(), "TLD Type mismatch")
	require.Equal(t, dbtld.UName, tld.UName.String(), "TLD UName mismatch")
	require.Equal(t, dbtld.CreatedAt, tld.CreatedAt, "TLD CreatedAt mismatch")
	require.Equal(t, dbtld.UpdatedAt, tld.UpdatedAt, "TLD UpdatedAt mismatch")
}

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
	err := repo.Create(tld)
	require.NoError(s.T(), err)

	readTLD, err := repo.GetByName(tld.Name.String())
	require.NoError(s.T(), err)
	require.NotNil(s.T(), readTLD)
	require.Equal(s.T(), tld, readTLD)

}

func (s *TLDSuite) TestCreateTLD_Duplicate() {
	tx := s.db.Begin()
	defer tx.Rollback()
	repo := NewGormTLDRepo(tx)

	tld, _ := entities.NewTLD("com")
	err := repo.Create(tld)
	require.NoError(s.T(), err)

	// Create a duplicate
	err = repo.Create(tld)
	require.Error(s.T(), err)
}

func (s *TLDSuite) TestListTLD() {
	tx := s.db.Begin()
	defer tx.Rollback()
	repo := NewGormTLDRepo(tx)

	tld1, _ := entities.NewTLD("com")
	err := repo.Create(tld1)
	require.NoError(s.T(), err)

	tld2, _ := entities.NewTLD("net")
	err = repo.Create(tld2)
	require.NoError(s.T(), err)

	tlds, err := repo.List(2, "")
	require.NoError(s.T(), err)
	require.NotNil(s.T(), tlds)
	require.Len(s.T(), tlds, 2)
}

func (s *TLDSuite) TestUpdateTLD() {
	tx := s.db.Begin()
	defer tx.Rollback()
	repo := NewGormTLDRepo(tx)

	tld, _ := entities.NewTLD("com")
	err := repo.Create(tld)
	require.NoError(s.T(), err)

	tld.Type = entities.TLDType("country-code")
	err = repo.Update(tld)
	require.NoError(s.T(), err)

	readTLD, err := repo.GetByName(tld.Name.String())
	require.NoError(s.T(), err)
	require.NotNil(s.T(), readTLD)
	require.Equal(s.T(), tld, readTLD)
	require.Equal(s.T(), "country-code", readTLD.Type.String())
}
