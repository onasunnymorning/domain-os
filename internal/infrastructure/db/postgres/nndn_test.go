package postgres

import (
	"context"
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
	err := tldRepo.Create(tld)
	require.NoError(s.T(), err)
	s.tld = tld.Name.String()
}

func (s *NNDNSuite) TearDownSuite() {
	if s.tld != "" {
		tldRepo := NewGormTLDRepo(s.db)
		err := tldRepo.DeleteByName(s.tld)
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

//func (s *NNDNSuite) TestReadNNDN() {
//	nndn := s.newTestNNDN()
//	createdNNDN, err := s.repo.CreateNNDN(context.Background(), nndn)
//	require.NoError(s.T(), err)
//	s.createdNNDNs = append(s.createdNNDNs, createdNNDN.Name.String())
//
//	readNNDN, err := s.repo.GetNNDN(context.Background(), createdNNDN.Name.String())
//	require.NoError(s.T(), err)
//	require.NotNil(s.T(), readNNDN)
//	require.Equal(s.T(), createdNNDN.Name, readNNDN.Name)
//
//}
//
//func (s *NNDNSuite) TestUpdateNNDN() {
//	nndn := s.newTestNNDN()
//	createdNNDN, err := s.repo.CreateNNDN(context.Background(), nndn)
//	require.NoError(s.T(), err)
//	s.createdNNDNs = append(s.createdNNDNs, createdNNDN.Name.String())
//
//	createdNNDN.UName = "updated-unicode-Name"
//	updatedNNDN, err := s.repo.UpdateNNDN(context.Background(), createdNNDN)
//	require.NoError(s.T(), err)
//
//	require.NotNil(s.T(), updatedNNDN)
//	require.Equal(s.T(), "updated-unicode-Name", updatedNNDN.UName.String())
//
//}
//
//func (s *NNDNSuite) TestDeleteNNDN() {
//	nndn := s.newTestNNDN()
//	createdNNDN, err := s.repo.CreateNNDN(context.Background(), nndn)
//	require.NoError(s.T(), err)
//
//	err = s.repo.DeleteNNDN(context.Background(), createdNNDN.Name.String())
//	require.NoError(s.T(), err)
//
//	_, err = s.repo.GetNNDN(context.Background(), createdNNDN.Name.String())
//	require.Error(s.T(), err)
//}
//
//func (s *NNDNSuite) TestListNNDNs() {
//	var cursor string
//	for i := 0; i < 3; i++ {
//		nndn := s.newTestNNDN()
//		if cursor == "" {
//			cursor = nndn.Name.String()
//		}
//		createdNNDN, err := s.repo.CreateNNDN(context.Background(), nndn)
//		require.NoError(s.T(), err)
//		s.createdNNDNs = append(s.createdNNDNs, createdNNDN.Name.String())
//	}
//
//	nndns, err := s.repo.ListNNDNs(context.Background(), 3, cursor)
//	require.NoError(s.T(), err)
//	require.Len(s.T(), nndns, 2)
//}
//
//func (s *NNDNSuite) TestCreateNNDN_Error() {
//	// Create a NNDN with a specific Name
//	nndn := s.newTestNNDN()
//	createdNNDN, err := s.repo.CreateNNDN(context.Background(), nndn)
//	require.NoError(s.T(), err)
//	// Cleanup
//	s.createdNNDNs = append(s.createdNNDNs, createdNNDN.Name.String())
//
//	// Attempt to create another NNDN with the same Name, expecting an error
//	duplicateNNDN := s.newTestNNDN()
//	duplicateNNDN.UName = createdNNDN.UName // Set the same UName
//	_, err = s.repo.CreateNNDN(context.Background(), duplicateNNDN)
//	require.Error(s.T(), err)
//
//}
//
//func (s *NNDNSuite) TestCreateAndUpdateNNDN_Error() {
//	// Create the first NNDN
//	firstNNDN := s.newTestNNDN()
//	createdFirstNNDN, err := s.repo.CreateNNDN(context.Background(), firstNNDN)
//	require.NoError(s.T(), err)
//	// Cleanup
//	s.createdNNDNs = append(s.createdNNDNs, createdFirstNNDN.Name.String())
//
//	// Create the second NNDN
//	secondNNDN := s.newTestNNDN()
//	createdSecondNNDN, err := s.repo.CreateNNDN(context.Background(), secondNNDN)
//	require.NoError(s.T(), err)
//
//	// Attempt to update the second NNDN with the same aName as the first, expecting an error
//	createdSecondNNDN.UName = createdFirstNNDN.UName
//	_, err = s.repo.UpdateNNDN(context.Background(), createdSecondNNDN)
//	require.Error(s.T(), err)
//
//	// Cleanup
//	s.createdNNDNs = append(s.createdNNDNs, createdSecondNNDN.Name.String())
//}
