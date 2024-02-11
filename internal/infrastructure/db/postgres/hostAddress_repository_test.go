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

type HostAddressSuite struct {
	suite.Suite
	db       *gorm.DB
	rarClid  string
	hostRoid int64
}

func TestHostAddressSuite(t *testing.T) {
	suite.Run(t, new(HostAddressSuite))
}

func (s *HostAddressSuite) SetupSuite() {
	s.db = setupTestDB()
	NewGormTLDRepo(s.db)

	// Create a registrar
	rar, _ := entities.NewRegistrar("199-myrar", "goBro Inc.", "email@gobro.com", 199)
	rarRepo := NewGormRegistrarRepository(s.db)
	createdRar, _ := rarRepo.Create(context.Background(), rar)
	s.rarClid = createdRar.ClID.String()

	// Create a host
	t := time.Now().UTC()
	host := getValidHost(s.rarClid, &t)
	hostRepo := NewGormHostRepository(s.db)
	createdHost, _ := hostRepo.CreateHost(context.Background(), host)
	s.hostRoid, _ = createdHost.RoID.Int64()
}

func (s *HostAddressSuite) TearDownSuite() {
	if s.hostRoid != 0 {
		hostRepo := NewGormHostRepository(s.db)
		_ = hostRepo.DeleteHostByRoid(context.Background(), s.hostRoid)
	}
	if s.rarClid != "" {
		rarRepo := NewGormRegistrarRepository(s.db)
		_ = rarRepo.Delete(context.Background(), s.rarClid)
	}
}

func (s *HostAddressSuite) TestCreateHostAddress() {
	tx := s.db.Begin()
	defer tx.Rollback()
	repo := NewGormHostAddressRepository(tx)

	a, _ := netip.ParseAddr("195.238.2.21")
	createdAddr, err := repo.CreateHostAddress(context.Background(), s.hostRoid, &a)
	s.Require().NoError(err)
	s.Require().NotNil(createdAddr)
}

func (s *HostAddressSuite) TestGetHostAddressesByHostRoid() {
	tx := s.db.Begin()
	defer tx.Rollback()
	repo := NewGormHostAddressRepository(tx)

	ips := []string{"195.238.2.21", "195.238.2.22", "2001:db8:85a3::8a2e:370:7334"}
	for _, ip := range ips {
		a, _ := netip.ParseAddr(ip)
		_, _ = repo.CreateHostAddress(context.Background(), s.hostRoid, &a)
	}

	addrs, err := repo.GetHostAddressesByHostRoid(context.Background(), s.hostRoid)
	s.Require().NoError(err)
	s.Require().Len(addrs, 3)
}

func (s *HostAddressSuite) TestDeleteHostAddressByHostRoidAndAddress() {
	tx := s.db.Begin()
	defer tx.Rollback()
	repo := NewGormHostAddressRepository(tx)

	ips := []string{"195.238.2.21", "195.238.2.22", "2001:db8:85a3::8a2e:370:7334"}
	for _, ip := range ips {
		a, _ := netip.ParseAddr(ip)
		_, _ = repo.CreateHostAddress(context.Background(), s.hostRoid, &a)
	}

	addrs, err := repo.GetHostAddressesByHostRoid(context.Background(), s.hostRoid)
	s.Require().NoError(err)
	s.Require().Len(addrs, 3)

	// Delete one of the addresses
	err = repo.DeleteHostAddressByHostRoidAndAddress(context.Background(), s.hostRoid, &addrs[0])
	s.Require().NoError(err)

	// Verify the address was deleted
	addrs, err = repo.GetHostAddressesByHostRoid(context.Background(), s.hostRoid)
	s.Require().NoError(err)
	s.Require().Len(addrs, 2)

	// Delete the rest of the addresses
	for _, addr := range addrs {
		err = repo.DeleteHostAddressByHostRoidAndAddress(context.Background(), s.hostRoid, &addr)
		s.Require().NoError(err)
	}

	// Verify all addresses were deleted
	addrs, err = repo.GetHostAddressesByHostRoid(context.Background(), s.hostRoid)
	s.Require().NoError(err)
	s.Require().Len(addrs, 0)
}
