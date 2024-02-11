package postgres

import (
	"context"
	"net/netip"
	"testing"
	"time"

	"github.com/onasunnymorning/domain-os/internal/domain/entities"
	"github.com/stretchr/testify/suite"
	"gorm.io/gorm"
)

type HostSuite struct {
	suite.Suite
	db      *gorm.DB
	rarClid string
}

func TestHostSuite(t *testing.T) {
	suite.Run(t, new(HostSuite))
}

func (s *HostSuite) SetupSuite() {
	s.db = setupTestDB()
	NewGormTLDRepo(s.db)

	// Create a registrar
	rar, _ := entities.NewRegistrar("199-myrar", "goBro Inc.", "email@gobro.com", 199)
	repo := NewGormRegistrarRepository(s.db)
	createdRar, _ := repo.Create(context.Background(), rar)
	s.rarClid = createdRar.ClID.String()
}

func (s *HostSuite) TearDownSuite() {
	if s.rarClid != "" {
		repo := NewGormRegistrarRepository(s.db)
		_ = repo.Delete(context.Background(), s.rarClid)
	}
}

func (s *HostSuite) TestCreateHost() {
	tx := s.db.Begin()
	defer tx.Rollback()
	repo := NewGormHostRepository(tx)

	t := time.Now().UTC()
	host := getValidHost("199-myrar", &t)
	host.ClID = entities.ClIDType(s.rarClid)
	a, _ := netip.ParseAddr("195.238.2.21")
	host.Addresses = append(host.Addresses, a)

	createdHost, err := repo.CreateHost(context.Background(), host)
	s.Require().NoError(err)
	s.Require().NotNil(createdHost)
	s.Require().Equal(0, len(createdHost.Addresses))
}

func (s *HostSuite) TestCreateHost_Duplicate() {
	tx := s.db.Begin()
	defer tx.Rollback()
	repo := NewGormHostRepository(tx)

	t := time.Now().UTC()
	host := getValidHost("199-myrar", &t)
	host.ClID = entities.ClIDType(s.rarClid)

	createdHost, err := repo.CreateHost(context.Background(), host)
	s.Require().NoError(err)
	s.Require().NotNil(createdHost)

	// Create a duplicate
	createdHost, err = repo.CreateHost(context.Background(), host)
	s.Require().Error(err)
	s.Require().Nil(createdHost)
}

func (s *HostSuite) TestReadHost() {
	tx := s.db.Begin()
	defer tx.Rollback()
	repo := NewGormHostRepository(tx)

	t := time.Now().UTC()
	host := getValidHost("199-myrar", &t)
	host.ClID = entities.ClIDType(s.rarClid)

	createdHost, err := repo.CreateHost(context.Background(), host)
	s.Require().NoError(err)
	s.Require().NotNil(createdHost)

	roidInt, _ := host.RoID.Int64()
	readHost, err := repo.GetHostByRoid(context.Background(), roidInt)
	s.Require().NoError(err)
	s.Require().NotNil(readHost)
	s.Require().Equal(createdHost, readHost)
}

func (s *HostSuite) TestUpdateHost() {
	tx := s.db.Begin()
	defer tx.Rollback()
	repo := NewGormHostRepository(tx)

	t := time.Now().UTC()
	host := getValidHost("199-myrar", &t)
	host.ClID = entities.ClIDType(s.rarClid)

	createdHost, err := repo.CreateHost(context.Background(), host)
	s.Require().NoError(err)
	s.Require().NotNil(createdHost)

	createdHost.Name = "Updated Host Name"
	updatedHost, err := repo.UpdateHost(context.Background(), createdHost)
	s.Require().NoError(err)
	s.Require().NotNil(updatedHost)
	s.Require().Equal(entities.DomainName("Updated Host Name"), updatedHost.Name)
}

func (s *HostSuite) TestDeleteHost() {
	tx := s.db.Begin()
	defer tx.Rollback()
	repo := NewGormHostRepository(tx)

	t := time.Now().UTC()
	host := getValidHost("199-myrar", &t)
	host.ClID = entities.ClIDType(s.rarClid)

	createdHost, err := repo.CreateHost(context.Background(), host)
	s.Require().NoError(err)
	s.Require().NotNil(createdHost)

	roidInd, _ := host.RoID.Int64()
	err = repo.DeleteHostByRoid(context.Background(), roidInd)
	s.Require().NoError(err)

	_, err = repo.GetHostByRoid(context.Background(), roidInd)
	s.Require().Error(err)

	err = repo.DeleteHostByRoid(context.Background(), roidInd)
	s.Require().NoError(err)

	_, err = repo.GetHostByRoid(context.Background(), roidInd)
	s.Require().Error(err)
}
