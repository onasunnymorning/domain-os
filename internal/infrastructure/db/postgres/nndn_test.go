package postgres

import (
	"context"
	"fmt"
	"testing"

	"gorm.io/gorm"

	"github.com/onasunnymorning/domain-os/internal/application/queries"
	"github.com/onasunnymorning/domain-os/internal/domain/entities"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type NNDNSuite struct {
	suite.Suite
	db  *gorm.DB
	tld string
	ry  *entities.RegistryOperator
}

func TestNNDNSuite(t *testing.T) {
	suite.Run(t, new(NNDNSuite))
}

func (s *NNDNSuite) SetupSuite() {
	s.db = getTestDB()
	tldRepo := NewGormTLDRepo(s.db)

	// Create a Registry Operator
	ro, _ := entities.NewRegistryOperator("NNDNSuiteRo", "NNDNSuiteRo", "NNDNSuiteRo@my.email")
	roRepo := NewGORMRegistryOperatorRepository(s.db)
	_, err := roRepo.Create(context.Background(), ro)
	require.NoError(s.T(), err)
	createdRo, err := roRepo.GetByRyID(context.Background(), ro.RyID.String())
	require.NoError(s.T(), err)
	s.ry = createdRo

	tld, _ := entities.NewTLD("nndntld", "NNDNSuiteRo")
	err = tldRepo.Create(context.Background(), tld)
	require.NoError(s.T(), err)
	s.tld = tld.Name.String()
}

func (s *NNDNSuite) TearDownSuite() {
	if s.tld != "" {
		tldRepo := NewGormTLDRepo(s.db)
		err := tldRepo.DeleteByName(context.Background(), s.tld)
		require.NoError(s.T(), err)
	}
	if s.ry != nil {
		roRepo := NewGORMRegistryOperatorRepository(s.db)
		err := roRepo.DeleteByRyID(context.Background(), s.ry.RyID.String())
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
	nndn.Reason = "test-reason"
	createdNNDN, err := repo.CreateNNDN(context.Background(), nndn)
	require.NoError(s.T(), err)

	readNNDN, err := repo.GetNNDN(context.Background(), createdNNDN.Name.String())
	require.NoError(s.T(), err)
	require.NotNil(s.T(), readNNDN)
	require.Equal(s.T(), createdNNDN.Name, readNNDN.Name)
	require.Equal(s.T(), createdNNDN.Reason, readNNDN.Reason)
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

	nndns, _, err := repo.ListNNDNs(context.Background(), queries.ListItemsQuery{
		PageSize:   3,
		PageCursor: createdNNDNs[0],
	})
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
