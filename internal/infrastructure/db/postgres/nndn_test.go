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
		nndn.Reason = "list-reason"
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

	// Filter with NameLike
	nndns, cursor, err := repo.ListNNDNs(context.Background(), queries.ListItemsQuery{
		PageSize: 25,
		Filter:   queries.ListNndnsFilter{NameLike: "example"},
	})
	require.NoError(s.T(), err)
	require.Len(s.T(), nndns, 3)
	require.Equal(s.T(), "", cursor)

	// Count with same filter
	count, err := repo.Count(context.Background(), queries.ListNndnsFilter{NameLike: "example"})
	require.NoError(s.T(), err)
	require.Equal(s.T(), 3, count)

	// Filter With TldEquals
	nndns, cursor, err = repo.ListNNDNs(context.Background(), queries.ListItemsQuery{
		PageSize: 25,
		Filter:   queries.ListNndnsFilter{TldEquals: s.tld},
	})
	require.NoError(s.T(), err)
	require.Len(s.T(), nndns, 3)
	require.Equal(s.T(), "", cursor)

	// Count with same filter
	count, err = repo.Count(context.Background(), queries.ListNndnsFilter{TldEquals: s.tld})
	require.NoError(s.T(), err)
	require.Equal(s.T(), 3, count)

	// Filter with TldEquals zero results
	nndns, cursor, err = repo.ListNNDNs(context.Background(), queries.ListItemsQuery{
		PageSize: 25,
		Filter:   queries.ListNndnsFilter{TldEquals: "non-existent-tld"},
	})
	require.NoError(s.T(), err)
	require.Len(s.T(), nndns, 0)
	require.Equal(s.T(), "", cursor)

	// count with same filter
	count, err = repo.Count(context.Background(), queries.ListNndnsFilter{TldEquals: "non-existent-tld"})
	require.NoError(s.T(), err)
	require.Equal(s.T(), 0, count)

	// Filter with ReasonEquals
	nndns, cursor, err = repo.ListNNDNs(context.Background(), queries.ListItemsQuery{
		PageSize: 25,
		Filter:   queries.ListNndnsFilter{ReasonEquals: "list-reason"},
	})
	require.NoError(s.T(), err)
	require.Len(s.T(), nndns, 3)
	require.Equal(s.T(), "", cursor)

	// Count with same filter
	count, err = repo.Count(context.Background(), queries.ListNndnsFilter{ReasonEquals: "list-reason"})
	require.NoError(s.T(), err)
	require.Equal(s.T(), 3, count)

	// Filter with ReasonLike
	nndns, cursor, err = repo.ListNNDNs(context.Background(), queries.ListItemsQuery{
		PageSize: 25,
		Filter:   queries.ListNndnsFilter{ReasonLike: "list"},
	})
	require.NoError(s.T(), err)
	require.Len(s.T(), nndns, 3)
	require.Equal(s.T(), "", cursor)

	// Count with same filter
	count, err = repo.Count(context.Background(), queries.ListNndnsFilter{ReasonLike: "list"})
	require.NoError(s.T(), err)
	require.Equal(s.T(), 3, count)

	// Filter with ReasonLike and pagination
	nndns, cursor, err = repo.ListNNDNs(context.Background(), queries.ListItemsQuery{
		PageSize: 2,
		Filter:   queries.ListNndnsFilter{ReasonLike: "list"},
	})
	require.NoError(s.T(), err)
	require.Len(s.T(), nndns, 2)
	require.NotEqual(s.T(), "", cursor)

	// Filter with ReasonLike and pagination + cursor to get next page
	nndns, cursor, err = repo.ListNNDNs(context.Background(), queries.ListItemsQuery{
		PageSize:   2,
		PageCursor: cursor,
		Filter:     queries.ListNndnsFilter{ReasonLike: "list"},
	})
	require.NoError(s.T(), err)
	require.Len(s.T(), nndns, 1)
	require.Equal(s.T(), "", cursor)

	// Invalid filter type
	_, _, err = repo.ListNNDNs(context.Background(), queries.ListItemsQuery{
		PageSize: 25,
		Filter: queries.ListTldsFilter{
			NameLike: "example",
		},
	})
	require.Error(s.T(), err)

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
