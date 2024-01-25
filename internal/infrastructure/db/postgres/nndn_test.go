package postgres

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"

	"github.com/onasunnymorning/domain-os/internal/domain/entities"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type NNDNSuite struct {
	suite.Suite
	repo         *GormNNDNRepository
	tldRepo      *GormTLDRepository
	nndnCount    uint64
	createdNNDNs []string
	createdTLD   string
}

func TestNNDNSuite(t *testing.T) {
	suite.Run(t, new(NNDNSuite))
}

func (s *NNDNSuite) newTestNNDN() *entities.NNDN {
	atomic.AddUint64(&s.nndnCount, 1)
	currentCount := atomic.LoadUint64(&s.nndnCount)
	nndn, _ := entities.NewNNDN(fmt.Sprintf("example-%d.%s", currentCount, s.createdTLD))
	return nndn
}

func (s *NNDNSuite) SetupSuite() {
	db := getTestDB().Begin()
	s.repo = NewGormNNDNRepository(db)
	s.tldRepo = NewGormTLDRepo(db)
	tld, _ := entities.NewTLD("xyz")
	err := s.tldRepo.Create(tld)
	require.NoError(s.T(), err)
	s.createdTLD = tld.Name.String()
}

func (s *NNDNSuite) TearDownSuite() {
	for _, id := range s.createdNNDNs {
		s.repo.DeleteNNDN(context.Background(), id)
	}
	s.tldRepo.DeleteByName(s.createdTLD)

	// Check if any NNDNs are left
	remainingNNDNs, err := s.repo.ListNNDNs(context.Background(), 100, "")
	require.NoError(s.T(), err)
	require.Empty(s.T(), remainingNNDNs, "There are still NNDNs left in the database after cleanup")
}

func (s *NNDNSuite) TestCreateNNDN() {
	nndn := s.newTestNNDN()
	createdNNDN, err := s.repo.CreateNNDN(context.Background(), nndn)
	require.NoError(s.T(), err)
	require.NotNil(s.T(), createdNNDN)

	s.createdNNDNs = append(s.createdNNDNs, createdNNDN.Name.String())
}

func (s *NNDNSuite) TestReadNNDN() {
	nndn := s.newTestNNDN()
	createdNNDN, err := s.repo.CreateNNDN(context.Background(), nndn)
	require.NoError(s.T(), err)
	s.createdNNDNs = append(s.createdNNDNs, createdNNDN.Name.String())

	readNNDN, err := s.repo.GetNNDN(context.Background(), createdNNDN.Name.String())
	require.NoError(s.T(), err)
	require.NotNil(s.T(), readNNDN)
	require.Equal(s.T(), createdNNDN.Name, readNNDN.Name)

}

func (s *NNDNSuite) TestUpdateNNDN() {
	nndn := s.newTestNNDN()
	createdNNDN, err := s.repo.CreateNNDN(context.Background(), nndn)
	require.NoError(s.T(), err)
	s.createdNNDNs = append(s.createdNNDNs, createdNNDN.Name.String())

	createdNNDN.UName = "updated-unicode-Name"
	updatedNNDN, err := s.repo.UpdateNNDN(context.Background(), createdNNDN)
	require.NoError(s.T(), err)

	require.NotNil(s.T(), updatedNNDN)
	require.Equal(s.T(), "updated-unicode-Name", updatedNNDN.UName.String())

}

func (s *NNDNSuite) TestDeleteNNDN() {
	nndn := s.newTestNNDN()
	createdNNDN, err := s.repo.CreateNNDN(context.Background(), nndn)
	require.NoError(s.T(), err)

	err = s.repo.DeleteNNDN(context.Background(), createdNNDN.Name.String())
	require.NoError(s.T(), err)

	_, err = s.repo.GetNNDN(context.Background(), createdNNDN.Name.String())
	require.Error(s.T(), err)
}

func (s *NNDNSuite) TestListNNDNs() {
	var cursor string
	for i := 0; i < 3; i++ {
		nndn := s.newTestNNDN()
		if cursor == "" {
			cursor = nndn.Name.String()
		}
		createdNNDN, err := s.repo.CreateNNDN(context.Background(), nndn)
		require.NoError(s.T(), err)
		s.createdNNDNs = append(s.createdNNDNs, createdNNDN.Name.String())
	}

	nndns, err := s.repo.ListNNDNs(context.Background(), 3, cursor)
	require.NoError(s.T(), err)
	require.Len(s.T(), nndns, 2)
}

func (s *NNDNSuite) TestCreateNNDN_Error() {
	// Create a NNDN with a specific Name
	nndn := s.newTestNNDN()
	createdNNDN, err := s.repo.CreateNNDN(context.Background(), nndn)
	require.NoError(s.T(), err)
	// Cleanup
	s.createdNNDNs = append(s.createdNNDNs, createdNNDN.Name.String())

	// Attempt to create another NNDN with the same Name, expecting an error
	duplicateNNDN := s.newTestNNDN()
	duplicateNNDN.UName = createdNNDN.UName // Set the same UName
	_, err = s.repo.CreateNNDN(context.Background(), duplicateNNDN)
	require.Error(s.T(), err)

}

func (s *NNDNSuite) TestCreateAndUpdateNNDN_Error() {
	// Create the first NNDN
	firstNNDN := s.newTestNNDN()
	createdFirstNNDN, err := s.repo.CreateNNDN(context.Background(), firstNNDN)
	require.NoError(s.T(), err)
	// Cleanup
	s.createdNNDNs = append(s.createdNNDNs, createdFirstNNDN.Name.String())

	// Create the second NNDN
	secondNNDN := s.newTestNNDN()
	createdSecondNNDN, err := s.repo.CreateNNDN(context.Background(), secondNNDN)
	require.NoError(s.T(), err)

	// Attempt to update the second NNDN with the same aName as the first, expecting an error
	createdSecondNNDN.UName = createdFirstNNDN.UName
	_, err = s.repo.UpdateNNDN(context.Background(), createdSecondNNDN)
	require.Error(s.T(), err)

	// Cleanup
	s.createdNNDNs = append(s.createdNNDNs, createdSecondNNDN.Name.String())
}
