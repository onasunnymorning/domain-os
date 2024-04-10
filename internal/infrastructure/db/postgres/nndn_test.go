package postgres

import (
	"context"
	"fmt"
	"gorm.io/gorm"
	"testing"

	"github.com/onasunnymorning/domain-os/internal/domain/entities"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type NNDNSuite struct {
	suite.Suite
	db  *gorm.DB
	tld string
}

func TestNNDNSuite(t *testing.T) {
	suite.Run(t, new(NNDNSuite))
}

func (s *NNDNSuite) SetupSuite() {
	s.db = getTestDB()
	tldRepo := NewGormTLDRepo(s.db)
	tld, _ := entities.NewTLD("nndntld")
	err := tldRepo.Create(context.Background(), tld)
	require.NoError(s.T(), err)
	s.tld = tld.Name.String()
}

func (s *NNDNSuite) TearDownSuite() {
	if s.tld != "" {
		tldRepo := NewGormTLDRepo(s.db)
		err := tldRepo.DeleteByName(context.Background(), s.tld)
		require.NoError(s.T(), err)
	}
}

func (s *NNDNSuite) TestCreateNNDN() {
	tx := s.db.Begin()
	defer tx.Rollback()
	repo := NewGormNNDNRepository(tx)

	nndn, _ := entities.NewNNDN("example." + s.tld)
	createdNNDN, err := repo.CreateNNDN(context.Background(), nndn)
	require.NoError(s.T(), err)
	require.NotNil(s.T(), createdNNDN)
}

func (s *NNDNSuite) TestReadNNDN() {
	tx := s.db.Begin()
	defer tx.Rollback()
	repo := NewGormNNDNRepository(tx)

	nndn, _ := entities.NewNNDN("example." + s.tld)
	createdNNDN, err := repo.CreateNNDN(context.Background(), nndn)
	require.NoError(s.T(), err)

	readNNDN, err := repo.GetNNDN(context.Background(), createdNNDN.Name.String())
	require.NoError(s.T(), err)
	require.NotNil(s.T(), readNNDN)
	require.Equal(s.T(), createdNNDN.Name, readNNDN.Name)
}

func (s *NNDNSuite) TestUpdateNNDN() {
	tx := s.db.Begin()
	defer tx.Rollback()
	repo := NewGormNNDNRepository(tx)

	nndn, _ := entities.NewNNDN("example." + s.tld)
	createdNNDN, err := repo.CreateNNDN(context.Background(), nndn)
	require.NoError(s.T(), err)

	createdNNDN.UName = "updated-unicode-name"
	updatedNNDN, err := repo.UpdateNNDN(context.Background(), createdNNDN)
	require.NoError(s.T(), err)

	require.NotNil(s.T(), updatedNNDN)
	require.Equal(s.T(), "updated-unicode-name", updatedNNDN.UName.String())
}

func (s *NNDNSuite) TestDeleteNNDN() {
	tx := s.db.Begin()
	defer tx.Rollback()
	repo := NewGormNNDNRepository(tx)

	nndn, _ := entities.NewNNDN("example." + s.tld)
	createdNNDN, err := repo.CreateNNDN(context.Background(), nndn)
	require.NoError(s.T(), err)

	err = repo.DeleteNNDN(context.Background(), createdNNDN.Name.String())
	require.NoError(s.T(), err)

	_, err = repo.GetNNDN(context.Background(), createdNNDN.Name.String())
	require.Error(s.T(), err)
}

func (s *NNDNSuite) TestListNNDNs() {
	tx := s.db.Begin()
	defer tx.Rollback()
	repo := NewGormNNDNRepository(tx)

	var createdNNDNs []string
	for i := 0; i < 3; i++ {
		nndn, _ := entities.NewNNDN(fmt.Sprintf("list%dexample.%s", i, s.tld))
		createdNNDN, err := repo.CreateNNDN(context.Background(), nndn)
		require.NoError(s.T(), err)
		createdNNDNs = append(createdNNDNs, createdNNDN.Name.String())
	}

	nndns, err := repo.ListNNDNs(context.Background(), 3, createdNNDNs[0])
	require.NoError(s.T(), err)
	require.Len(s.T(), nndns, 2)
}

func (s *NNDNSuite) TestCreateNNDN_Error() {
	tx := s.db.Begin()
	defer tx.Rollback()
	repo := NewGormNNDNRepository(tx)

	nndn, _ := entities.NewNNDN("example." + s.tld)
	_, err := repo.CreateNNDN(context.Background(), nndn)
	require.NoError(s.T(), err)

	duplicateNNDN, _ := entities.NewNNDN("example." + s.tld)
	_, err = repo.CreateNNDN(context.Background(), duplicateNNDN)
	require.Error(s.T(), err)
}
