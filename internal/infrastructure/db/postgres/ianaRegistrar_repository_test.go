package postgres

import (
	"context"
	"testing"

	"github.com/onasunnymorning/domain-os/internal/domain/entities"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"gorm.io/gorm"
)

func TestIANARar_TableName(t *testing.T) {
	s := IANARegistrar{}
	if s.TableName() != "iana_registrars" {
		t.Errorf("Expected iana_registrars, got %s", s.TableName())
	}
}

type IANARarSuite struct {
	suite.Suite
	db *gorm.DB
}

func TestIANARarSuite(t *testing.T) {
	suite.Run(t, new(IANARarSuite))
}

func (s *IANARarSuite) SetupSuite() {
	s.db = setupTestDB()
	NewGormTLDRepo(s.db)
}

func (s *IANARarSuite) TestUpdateAll() {
	tx := s.db.Begin()
	defer tx.Rollback()
	repo := NewIANARegistrarRepository(tx)

	rars := []*entities.IANARegistrar{
		{
			GurID:   1234,
			Name:    "registrar1",
			Status:  "ok",
			RdapURL: "https://rdapURL",
		},
		{
			GurID:   1235,
			Name:    "regsirar2",
			Status:  "terminated",
			RdapURL: "https://rdapURL2",
		},
	}

	err := repo.UpdateAll(rars)
	require.Nil(s.T(), err)
}

func (s *IANARarSuite) TestList() {
	tx := s.db.Begin()
	defer tx.Rollback()
	repo := NewIANARegistrarRepository(tx)

	rars := []*entities.IANARegistrar{
		{
			GurID:   1234,
			Name:    "gomamma",
			Status:  "ok",
			RdapURL: "https://rdapURL",
		},
		{
			GurID:   1235,
			Name:    "gobro",
			Status:  "terminated",
			RdapURL: "https://rdapURL2",
		},
	}

	err := repo.UpdateAll(rars)
	require.Nil(s.T(), err)

	list, err := repo.List(context.Background(), 25, "", "", "")
	require.Nil(s.T(), err)
	require.Equal(s.T(), 2, len(list))

	list, err = repo.List(context.Background(), 25, "1234", "bro", "")
	require.Nil(s.T(), err)
	require.Equal(s.T(), 1, len(list))
	require.Equal(s.T(), rars[1], list[0])
}

func (s *IANARarSuite) TestGetByGurID() {
	tx := s.db.Begin()
	defer tx.Rollback()
	repo := NewIANARegistrarRepository(tx)

	rars := []*entities.IANARegistrar{
		{
			GurID:   1234,
			Name:    "registrar1",
			Status:  "ok",
			RdapURL: "https://rdapURL",
		},
		{
			GurID:   1235,
			Name:    "regsirar2",
			Status:  "terminated",
			RdapURL: "https://rdapURL2",
		},
	}

	err := repo.UpdateAll(rars)
	require.Nil(s.T(), err)

	rar, err := repo.GetByGurID(context.Background(), 1234)
	require.Nil(s.T(), err)
	require.Equal(s.T(), rars[0], rar)
}
