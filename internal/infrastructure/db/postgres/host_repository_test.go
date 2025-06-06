package postgres

import (
	"context"
	"net/netip"
	"testing"
	"time"

	"github.com/onasunnymorning/domain-os/internal/application/queries"
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

func (s *HostSuite) TestBulkCreateHosts() {
	tx := s.db.Begin()
	defer tx.Rollback()
	repo := NewGormHostRepository(tx)

	t := time.Now().UTC()
	host1 := getValidHost("hostSuiteRar", &t)
	host1.ClID = entities.ClIDType(s.rarClid)
	host2 := getValidHost("hostSuiteRar", &t)
	host2.ClID = entities.ClIDType(s.rarClid)

	hosts := []*entities.Host{host1, host2}

	err := repo.BulkCreate(context.Background(), hosts)
	s.Require().NoError(err)

	// Verify that both hosts were created
	roidInt1, _ := host1.RoID.Int64()
	readHost1, err := repo.GetHostByRoid(context.Background(), roidInt1)
	s.Require().NoError(err)
	s.Require().NotNil(readHost1)
	s.Require().Equal(host1.Name, readHost1.Name)

	roidInt2, _ := host2.RoID.Int64()
	readHost2, err := repo.GetHostByRoid(context.Background(), roidInt2)
	s.Require().NoError(err)
	s.Require().NotNil(readHost2)
	s.Require().Equal(host2.Name, readHost2.Name)
}

func (s *HostSuite) TestBulkCreateHosts_Duplicate() {
	tx := s.db.Begin()
	defer tx.Rollback()
	repo := NewGormHostRepository(tx)

	t := time.Now().UTC()
	host1 := getValidHost("hostSuiteRar1", &t)
	host1.ClID = entities.ClIDType(s.rarClid)
	host2 := getValidHost("hostSuiteRar1", &t) // Duplicate name
	host2.ClID = entities.ClIDType(s.rarClid)

	hosts := []*entities.Host{host1, host2}

	err := repo.BulkCreate(context.Background(), hosts)
	s.Require().Error(err)

	// Verify that no hosts were created
	roidInt1, _ := host1.RoID.Int64()
	readHost1, err := repo.GetHostByRoid(context.Background(), roidInt1)
	s.Require().Error(err)
	s.Require().Nil(readHost1)

	roidInt2, _ := host2.RoID.Int64()
	readHost2, err := repo.GetHostByRoid(context.Background(), roidInt2)
	s.Require().Error(err)
	s.Require().Nil(readHost2)
}

func (s *HostSuite) SetupSuite() {
	s.db = setupTestDB()
	NewGormTLDRepo(s.db)

	// Create a registrar
	rar, err := entities.NewRegistrar("hostSuiteRar", "hostSuiteRar", "email@gobro.com", 199, getValidRegistrarPostalInfoArr())
	s.Require().NoError(err)
	s.Require().NotNil(rar)
	repo := NewGormRegistrarRepository(s.db)
	createdRar, err := repo.Create(context.Background(), rar)
	s.Require().NoError(err)
	s.Require().NotNil(createdRar)
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
	host := getValidHost("hostSuiteRar", &t)
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
	host := getValidHost("hostSuiteRar", &t)
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
	host := getValidHost("hostSuiteRar", &t)
	host.ClID = entities.ClIDType(s.rarClid)

	createdHost, err := repo.CreateHost(context.Background(), host)
	s.Require().NoError(err)
	s.Require().NotNil(createdHost)

	roidInt, _ := host.RoID.Int64()
	readHost, err := repo.GetHostByRoid(context.Background(), roidInt)
	s.Require().NoError(err)
	s.Require().NotNil(readHost)
	s.Require().Equal(createdHost.Name, readHost.Name)
	s.Require().Equal(createdHost.ClID, readHost.ClID)
	s.Require().Equal(createdHost.CrRr, readHost.CrRr)
	s.Require().Equal(createdHost.UpRr, readHost.UpRr)
	s.Require().Equal(createdHost.InBailiwick, readHost.InBailiwick)
	s.Require().Equal(createdHost.Status.ServerDeleteProhibited, readHost.Status.ServerDeleteProhibited)
	s.Require().Equal(createdHost.Status, readHost.Status)
	s.Require().Equal(createdHost.RoID, readHost.RoID)

}

func (s *HostSuite) TestUpdateHost() {
	tx := s.db.Begin()
	defer tx.Rollback()
	repo := NewGormHostRepository(tx)

	t := time.Now().UTC()
	host := getValidHost("hostSuiteRar", &t)
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
	host := getValidHost("hostSuiteRar", &t)
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

func (s *HostSuite) TestListHosts() {
	tx := s.db.Begin()
	defer tx.Rollback()
	repo := NewGormHostRepository(tx)

	t := time.Now().UTC()
	host := getValidHost("hostSuiteRar", &t)
	host.ClID = entities.ClIDType(s.rarClid)

	// Create host 1
	createdHost1, err := repo.CreateHost(context.Background(), host)
	s.Require().NoError(err)
	s.Require().NotNil(createdHost1)

	// Create host 2
	host.RoID = entities.RoidType("123456_HOST-APEX")
	host.Name = "newhostname.example.com"
	createdHost2, err := repo.CreateHost(context.Background(), host)
	s.Require().NoError(err)
	s.Require().NotNil(createdHost2)

	// Create host 3
	host.RoID = entities.RoidType("1234567_HOST-APEX")
	host.Name = "newhostname.exmaple.net"
	createdHost3, err := repo.CreateHost(context.Background(), host)
	s.Require().NoError(err)
	s.Require().NotNil(createdHost3)

	// List them all
	hosts, _, err := repo.ListHosts(context.Background(), queries.ListItemsQuery{PageSize: 25})
	s.Require().NoError(err)
	s.Require().GreaterOrEqual(len(hosts), 3) // tests run in parallel so we can't be sure of the exact number

	// Limit to 2
	hosts, _, err = repo.ListHosts(context.Background(), queries.ListItemsQuery{PageSize: 2})
	s.Require().NoError(err)
	s.Require().Equal(2, len(hosts))

	// Wrong roid object type
	hosts, _, err = repo.ListHosts(context.Background(), queries.ListItemsQuery{PageSize: 2, PageCursor: "123456_CONTACT-APEX"})
	s.Require().ErrorIs(err, entities.ErrInvalidRoid)
	s.Require().Nil(hosts)

	// Roid first part not an int64
	hosts, _, err = repo.ListHosts(context.Background(), queries.ListItemsQuery{PageSize: 2, PageCursor: "abcd_HOST-APEX"})
	s.Require().Error(err)
	s.Require().Nil(hosts)

}
func (s *HostSuite) TestGetHostByNameAndClID() {
	tx := s.db.Begin()
	defer tx.Rollback()
	repo := NewGormHostRepository(tx)

	t := time.Now().UTC()
	host := getValidHost("hostSuiteRar", &t)
	host.ClID = entities.ClIDType(s.rarClid)

	createdHost, err := repo.CreateHost(context.Background(), host)
	s.Require().NoError(err)
	s.Require().NotNil(createdHost)

	readHost, err := repo.GetHostByNameAndClID(context.Background(), createdHost.Name.String(), createdHost.ClID.String())
	s.Require().NoError(err)
	s.Require().NotNil(readHost)
	s.Require().Equal(createdHost.Name, readHost.Name)
	s.Require().Equal(createdHost.ClID, readHost.ClID)
	s.Require().Equal(createdHost.CrRr, readHost.CrRr)
	s.Require().Equal(createdHost.UpRr, readHost.UpRr)
	s.Require().Equal(createdHost.InBailiwick, readHost.InBailiwick)
	s.Require().Equal(createdHost.Status.ServerDeleteProhibited, readHost.Status.ServerDeleteProhibited)
	s.Require().Equal(createdHost.Status, readHost.Status)
	s.Require().Equal(createdHost.RoID, readHost.RoID)
}
