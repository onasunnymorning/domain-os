package postgres

import (
	"context"
	"fmt"
	"net/netip"
	"testing"
	"time"

	"github.com/onasunnymorning/domain-os/internal/domain/entities"
	"github.com/stretchr/testify/suite"
	"gorm.io/gorm"
)

type DomainSuite struct {
	suite.Suite
	db        *gorm.DB
	rarClid   string
	tld       string
	contactID string
	hosts     []*entities.Host
	ry        *entities.RegistryOperator
}

func TestDomainSuite(t *testing.T) {
	suite.Run(t, new(DomainSuite))
}

func (s *DomainSuite) SetupSuite() {
	s.db = setupTestDB()

	// Create a registrar
	rar, _ := entities.NewRegistrar("domaintestRar", "goBro Inc.", "email@gobro.com", 199, getValidRegistrarPostalInfoArr())
	repo := NewGormRegistrarRepository(s.db)
	createdRar, err := repo.Create(context.Background(), rar)
	s.Require().NoError(err)
	s.Require().NotNil(createdRar)
	s.rarClid = createdRar.ClID.String()

	// Create a Registry Operator
	ro, _ := entities.NewRegistryOperator("DomainSuiteRy", "DomainSuiteRy", "me@my.email")
	roRepo := NewGORMRegistryOperatorRepository(s.db)
	_, err = roRepo.Create(context.Background(), ro)
	s.Require().NoError(err)
	createdRo, err := roRepo.GetByRyID(context.Background(), ro.RyID.String())
	s.Require().NoError(err)
	s.ry = createdRo

	// Create a TLD
	tld, _ := entities.NewTLD("domaintesttld", "DomainSuiteRy")
	tldRepo := NewGormTLDRepo(s.db)
	err = tldRepo.Create(context.Background(), tld)
	s.Require().NoError(err)
	s.tld = tld.Name.String()

	// Create a 5 more TLDs
	for i := 0; i < 5; i++ {
		tld, _ := entities.NewTLD(fmt.Sprintf("domaintesttld%d", i), "DomainSuiteRy")
		err = tldRepo.Create(context.Background(), tld)
		s.Require().NoError(err)
	}

	// Create a contact
	contact, err := entities.NewContact("myTestContact007", "1234567899_CONT-APEX", "my@email.me", "st0NGp@ZZ", string(rar.ClID))
	s.Require().NoError(err)
	contactRepo := NewContactRepository(s.db)
	createdContact, err := contactRepo.CreateContact(context.Background(), contact)
	s.Require().NoError(err)
	s.Require().NotNil(createdContact)
	s.contactID = createdContact.ClID.String()

	// Create some hosts
	hostRepo := NewGormHostRepository(s.db)
	for i := 0; i < 3; i++ {
		host, err := entities.NewHost("ns"+fmt.Sprint(i)+".apex.domains", fmt.Sprint(i)+"1234_HOST-APEX", "domaintestRar")
		s.Require().NoError(err)
		s.Require().NotNil(host)

		// set as in-bailiwick for testing GLUE records in real life the domain layer will take care of this
		host.InBailiwick = true

		createdHost, err := hostRepo.CreateHost(context.Background(), host)
		s.Require().NoError(err)
		s.Require().NotNil(createdHost)

		s.hosts = append(s.hosts, createdHost)
	}

	// Add IPs to the hosts
	hostAddressRepo := NewGormHostAddressRepository(s.db)
	for i, host := range s.hosts {
		// create a netip.Addr
		a, err := netip.ParseAddr(fmt.Sprintf("192.168.1.%d", i))
		s.Require().NoError(err)

		// add one ip to each host
		ho_roid_int, err := host.RoID.Int64()
		s.Require().NoError(err)
		_, err = hostAddressRepo.CreateHostAddress(context.Background(), ho_roid_int, &a)
		s.Require().NoError(err)
	}

}

func (s *DomainSuite) TearDownSuite() {
	if s.tld != "" {
		repo := NewGormTLDRepo(s.db)
		_ = repo.DeleteByName(context.Background(), s.tld)
	}
	if s.rarClid != "" {
		repo := NewGormRegistrarRepository(s.db)
		_ = repo.Delete(context.Background(), s.rarClid)
	}
	if s.hosts != nil {
		repo := NewGormHostRepository(s.db)
		for _, host := range s.hosts {
			hoRoID, _ := host.RoID.Int64()
			_ = repo.DeleteHostByRoid(context.Background(), hoRoID)
		}
	}
	if s.ry != nil {
		repo := NewGORMRegistryOperatorRepository(s.db)
		_ = repo.DeleteByRyID(context.Background(), s.ry.RyID.String())
	}

}

func (s *DomainSuite) TestDomainRepository_CreateDomain() {
	tx := s.db.Begin()
	defer tx.Rollback()
	repo := NewDomainRepository(tx)

	// Create a domain
	domain, err := entities.NewDomain("1234_DOM-APEX", "geoff.domaintesttld", "GoMamma", "STr0mgP@ZZ")
	s.Require().NoError(err)
	domain.ClID = "domaintestRar"
	domain.RegistrantID = "myTestContact007"
	domain.AdminID = "myTestContact007"
	domain.TechID = "myTestContact007"
	domain.BillingID = "myTestContact007"
	createdDomain, err := repo.CreateDomain(context.Background(), domain)
	s.Require().NoError(err)
	s.Require().NotNil(createdDomain)
	s.Require().Equal(domain.Name, createdDomain.Name)
	s.Require().Equal(domain.ClID, createdDomain.ClID)
	s.Require().Equal(domain.AuthInfo, createdDomain.AuthInfo)
	s.Require().NotNil(createdDomain.RoID)

	// Create the same domains again should result in an error
	_, err = repo.CreateDomain(context.Background(), createdDomain)
	s.Require().Error(err)

}

func (s *DomainSuite) TestDomainRepository_CreateDomainWithHosts() {
	tx := s.db.Begin()
	defer tx.Rollback()
	repo := NewDomainRepository(tx)

	// Create a domain
	domain, err := entities.NewDomain("1234_DOM-APEX", "geoff.domaintesttld", "GoMamma", "STr0mgP@ZZ")
	s.Require().NoError(err)
	domain.ClID = "domaintestRar"
	domain.RegistrantID = "myTestContact007"
	domain.AdminID = "myTestContact007"
	domain.TechID = "myTestContact007"
	domain.BillingID = "myTestContact007"
	// Add some hosts
	domain.Hosts = s.hosts
	// Set active
	domain.Status.Inactive = false

	// Create the domain in the db
	createdDomain, err := repo.CreateDomain(context.Background(), domain)
	s.Require().NoError(err)
	s.Require().NotNil(createdDomain)
	s.Require().Equal(domain.Name, createdDomain.Name)
	s.Require().Equal(domain.ClID, createdDomain.ClID)
	s.Require().Equal(domain.AuthInfo, createdDomain.AuthInfo)
	s.Require().NotNil(createdDomain.RoID)

	// Retrieve the domain and check if the hosts are there
	retrievedDomain, err := repo.GetDomainByName(context.Background(), createdDomain.Name.String(), true)
	s.Require().NoError(err)
	s.Require().NotNil(retrievedDomain)
	s.Require().Equal(domain.Name, retrievedDomain.Name)
	s.Require().Equal(len(domain.Hosts), len(retrievedDomain.Hosts))

	// Retrieve the NS records that result from the domain having hosts
	rr, err := repo.GetActiveDomainsWithHosts(context.Background(), s.tld)
	s.Require().NoError(err)
	s.Require().Equal(len(domain.Hosts), len(rr))

	// Count the domains
	count, err := repo.Count(context.Background())
	s.Require().NoError(err)
	s.Require().Equal(int64(1), count)

	// try and delete the domain with hosts associated, should fail
	err = repo.DeleteDomainByName(context.Background(), createdDomain.Name.String())
	s.Require().Error(err)
}

func (s *DomainSuite) TestDomainRepository_GetGlue() {
	tx := s.db.Begin()
	defer tx.Rollback()
	repo := NewDomainRepository(tx)

	// add IPs to the hosts
	for i, host := range s.hosts {
		// create a net.IP
		a, err := netip.ParseAddr(fmt.Sprintf("192.168.1.%d", i))
		s.Require().NoError(err)

		// append it to the host
		host.Addresses = append(host.Addresses, a)
	}

	// Create a domain
	domain, err := entities.NewDomain("1234_DOM-APEX", "geoff.domaintesttld", "GoMamma", "STr0mgP@ZZ")
	s.Require().NoError(err)
	domain.ClID = "domaintestRar"
	domain.RegistrantID = "myTestContact007"
	domain.AdminID = "myTestContact007"
	domain.TechID = "myTestContact007"
	domain.BillingID = "myTestContact007"
	// Add some hosts
	domain.Hosts = s.hosts
	// Set active
	domain.Status.Inactive = false

	// Create the domain
	createdDomain, err := repo.CreateDomain(context.Background(), domain)
	s.Require().NoError(err)
	s.Require().NotNil(createdDomain)
	s.Require().Equal(domain.Name, createdDomain.Name)
	s.Require().Equal(domain.ClID, createdDomain.ClID)
	s.Require().Equal(domain.AuthInfo, createdDomain.AuthInfo)
	s.Require().NotNil(createdDomain.RoID)

	// Retrieve the domain and check if the hosts are there
	retrievedDomain, err := repo.GetDomainByName(context.Background(), createdDomain.Name.String(), true)
	s.Require().NoError(err)
	s.Require().NotNil(retrievedDomain)
	s.Require().Equal(domain.Name, retrievedDomain.Name)
	s.Require().Equal(len(domain.Hosts), len(retrievedDomain.Hosts))

	// Retrieve the NS records that result from the domain having hosts
	rr, err := repo.GetActiveDomainsWithHosts(context.Background(), s.tld)
	s.Require().NoError(err)
	s.Require().Equal(len(domain.Hosts), len(rr))

	// Retrieve the GLUE records that result from the domain having hosts
	glue, err := repo.GetActiveDomainGlue(context.Background(), s.tld)
	s.Require().NoError(err)
	s.Require().Equal(len(domain.Hosts), len(glue))

	// try and delete the domain with hosts associated, should fail
	err = repo.DeleteDomainByName(context.Background(), createdDomain.Name.String())
	s.Require().Error(err)
}

func (s *DomainSuite) TestDomainRepository_AddAndRemoveHosts() {
	tx := s.db.Begin()
	defer tx.Rollback()
	repo := NewDomainRepository(tx)

	// Create a domain
	domain, err := entities.NewDomain("1234_DOM-APEX", "geoff.domaintesttld", "GoMamma", "STr0mgP@ZZ")
	s.Require().NoError(err)
	domain.ClID = "domaintestRar"
	domain.RegistrantID = "myTestContact007"
	domain.AdminID = "myTestContact007"
	domain.TechID = "myTestContact007"
	domain.BillingID = "myTestContact007"
	domain.Status.Inactive = false // in real life the domain layer will take care of this

	// Create the domain
	createdDomain, err := repo.CreateDomain(context.Background(), domain)
	s.Require().NoError(err)
	s.Require().NotNil(createdDomain)
	s.Require().Equal(domain.Name, createdDomain.Name)
	s.Require().Equal(domain.ClID, createdDomain.ClID)
	s.Require().Equal(domain.AuthInfo, createdDomain.AuthInfo)
	s.Require().NotNil(createdDomain.RoID)

	// Add some hosts
	for _, host := range s.hosts {
		hoRoID, _ := host.RoID.Int64()
		domRoID, _ := createdDomain.RoID.Int64()
		err = repo.AddHostToDomain(context.Background(), domRoID, hoRoID)
		s.Require().NoError(err)
	}

	// Retrieve the domain and check if the hosts are there
	retrievedDomain, err := repo.GetDomainByName(context.Background(), createdDomain.Name.String(), true)
	s.Require().NoError(err)
	s.Require().NotNil(retrievedDomain)
	s.Require().Equal(domain.Name, retrievedDomain.Name)
	s.Require().Equal(len(s.hosts), len(retrievedDomain.Hosts))
	s.Require().Equal(domain.Status.Inactive, retrievedDomain.Status.Inactive)

	// Remove the hosts
	for _, host := range s.hosts {
		hoRoID, _ := host.RoID.Int64()
		domRoID, _ := createdDomain.RoID.Int64()
		err = repo.RemoveHostFromDomain(context.Background(), domRoID, hoRoID)
		s.Require().NoError(err)
	}

	// Retrieve the domain and check if the hosts are there
	retrievedDomain, err = repo.GetDomainByName(context.Background(), createdDomain.Name.String(), true)
	s.Require().NoError(err)
	s.Require().NotNil(retrievedDomain)
	s.Require().Equal(domain.Name, retrievedDomain.Name)
	s.Require().Equal(0, len(retrievedDomain.Hosts))

	// Check if the hosts still exist
	for _, host := range s.hosts {
		hoRoID, _ := host.RoID.Int64()
		_, err = NewGormHostRepository(tx).GetHostByRoid(context.Background(), hoRoID)
		s.Require().NoError(err)
	}
}
func (s *DomainSuite) TestDomainRepository_GetDomainByName() {
	tx := s.db.Begin()
	defer tx.Rollback()
	repo := NewDomainRepository(tx)

	// Create a domain
	domain, err := entities.NewDomain("1234_DOM-APEX", "geoff.domaintesttld", "GoMamma", "STr0mgP@ZZ")
	s.Require().NoError(err)
	domain.ClID = "domaintestRar"
	domain.RegistrantID = "myTestContact007"
	domain.AdminID = "myTestContact007"
	domain.TechID = "myTestContact007"
	domain.BillingID = "myTestContact007"
	createdDomain, err := repo.CreateDomain(context.Background(), domain)
	s.Require().NoError(err)
	s.Require().NotNil(createdDomain)

	// Get the domain
	foundDomain, err := repo.GetDomainByName(context.Background(), domain.Name.String(), false)
	s.Require().NoError(err)
	s.Require().NotNil(foundDomain)
	s.Require().Equal(createdDomain.Name, foundDomain.Name)
	s.Require().Equal(createdDomain.ClID, foundDomain.ClID)
	s.Require().Equal(createdDomain.AuthInfo, foundDomain.AuthInfo)
	s.Require().Equal(createdDomain.RoID, foundDomain.RoID)

	// Try get a domain that doesn't exist
	_, err = repo.GetDomainByName(context.Background(), "nonexistent.domaintesttld", false)
	s.Require().Error(err)

}

func (s *DomainSuite) TestDomainRepository_GetDomainByRoID() {
	tx := s.db.Begin()
	defer tx.Rollback()
	repo := NewDomainRepository(tx)

	// Create a domain
	domain, err := entities.NewDomain("1234_DOM-APEX", "geoff.domaintesttld", "GoMamma", "STr0mgP@ZZ")
	s.Require().NoError(err)
	domain.ClID = "domaintestRar"
	domain.RegistrantID = "myTestContact007"
	domain.AdminID = "myTestContact007"
	domain.TechID = "myTestContact007"
	domain.BillingID = "myTestContact007"
	createdDomain, err := repo.CreateDomain(context.Background(), domain)
	s.Require().NoError(err)
	s.Require().NotNil(createdDomain)

	// Get the domain
	roid, _ := createdDomain.RoID.Int64()
	foundDomain, err := repo.GetDomainByID(context.Background(), roid, false)
	s.Require().NoError(err)
	s.Require().NotNil(foundDomain)
	s.Require().Equal(createdDomain.Name, foundDomain.Name)
	s.Require().Equal(createdDomain.ClID, foundDomain.ClID)
	s.Require().Equal(createdDomain.AuthInfo, foundDomain.AuthInfo)
	s.Require().Equal(createdDomain.RoID, foundDomain.RoID)
}

func (s *DomainSuite) TestDomainRepository_CreateWithHostAndRetrieve() {
	tx := s.db.Begin()
	defer tx.Rollback()
	repo := NewDomainRepository(tx)

	// Create a domain
	domain, err := entities.NewDomain("1234_DOM-APEX", "geoff.domaintesttld", "GoMamma", "STr0mgP@ZZ")
	s.Require().NoError(err)
	domain.ClID = "domaintestRar"
	domain.RegistrantID = "myTestContact007"
	domain.AdminID = "myTestContact007"
	domain.TechID = "myTestContact007"
	domain.BillingID = "myTestContact007"
	domain.Hosts = s.hosts
	createdDomain, err := repo.CreateDomain(context.Background(), domain)
	s.Require().NoError(err)
	s.Require().NotNil(createdDomain)

	// Get the domain
	roid, _ := createdDomain.RoID.Int64()
	foundDomain, err := repo.GetDomainByID(context.Background(), roid, true) // preaload hosts to ensure that if they were accidentally created, the test will fail
	s.Require().NoError(err)
	s.Require().NotNil(foundDomain)
	s.Require().Equal(createdDomain.Name, foundDomain.Name)
	s.Require().Equal(createdDomain.ClID, foundDomain.ClID)
	s.Require().Equal(createdDomain.AuthInfo, foundDomain.AuthInfo)
	s.Require().Equal(createdDomain.RoID, foundDomain.RoID)
	s.Require().Equal(len(s.hosts), len(foundDomain.Hosts)) // it should fail here if the hosts were created
}

func (s *DomainSuite) TestDomainRepository_UpdateDomain() {
	tx := s.db.Begin()
	defer tx.Rollback()
	repo := NewDomainRepository(tx)

	// Create a domain
	domain, err := entities.NewDomain("1234_DOM-APEX", "geoff.domaintesttld", "GoMamma", "STr0mgP@ZZ")
	s.Require().NoError(err)
	domain.ClID = "domaintestRar"
	domain.RegistrantID = "myTestContact007"
	domain.AdminID = "myTestContact007"
	domain.TechID = "myTestContact007"
	domain.BillingID = "myTestContact007"
	createdDomain, err := repo.CreateDomain(context.Background(), domain)
	s.Require().NoError(err)
	s.Require().NotNil(createdDomain)

	// Update the domain
	createdDomain.AuthInfo = "newAuthInfo"
	updatedDomain, err := repo.UpdateDomain(context.Background(), createdDomain)
	s.Require().NoError(err)
	s.Require().NotNil(updatedDomain)
	s.Require().Equal(createdDomain.Name, updatedDomain.Name)
	s.Require().Equal(createdDomain.ClID, updatedDomain.ClID)
	s.Require().Equal(createdDomain.AuthInfo, updatedDomain.AuthInfo)
	s.Require().Equal(createdDomain.RoID, updatedDomain.RoID)
}

func (s *DomainSuite) TestDomainRepository_DeleteDomain() {
	tx := s.db.Begin()
	defer tx.Rollback()
	repo := NewDomainRepository(tx)

	// Create a domain
	domain, err := entities.NewDomain("1234_DOM-APEX", "geoff.domaintesttld", "GoMamma", "STr0mgP@ZZ")
	s.Require().NoError(err)
	domain.ClID = "domaintestRar"
	domain.RegistrantID = "myTestContact007"
	domain.AdminID = "myTestContact007"
	domain.TechID = "myTestContact007"
	domain.BillingID = "myTestContact007"
	createdDomain, err := repo.CreateDomain(context.Background(), domain)
	s.Require().NoError(err)
	s.Require().NotNil(createdDomain)

	// Delete the domain
	roid, _ := createdDomain.RoID.Int64()
	err = repo.DeleteDomainByID(context.Background(), roid)
	s.Require().NoError(err)

	// Ensure the domain was deleted
	_, err = repo.GetDomainByID(context.Background(), roid, false)
	s.Require().Error(err)

	err = repo.DeleteDomainByID(context.Background(), roid)
	s.Require().NoError(err)

	// Ensure the domain was deleted
	_, err = repo.GetDomainByID(context.Background(), roid, false)
	s.Require().Error(err)
}

func (s *DomainSuite) TestDomainRepository_ListDomains() {
	tx := s.db.Begin()
	defer tx.Rollback()
	repo := NewDomainRepository(tx)

	// Create a domain
	domain, err := entities.NewDomain("1234_DOM-APEX", "geoff.domaintesttld", "GoMamma", "STr0mgP@ZZ")
	s.Require().NoError(err)
	domain.ClID = "domaintestRar"
	domain.RegistrantID = "myTestContact007"
	domain.AdminID = "myTestContact007"
	domain.TechID = "myTestContact007"
	domain.BillingID = "myTestContact007"
	_, err = repo.CreateDomain(context.Background(), domain)
	s.Require().NoError(err)

	// Create a second domain
	domain, err = entities.NewDomain("12345_DOM-APEX", "de.domaintesttld", "GoMamma", "STr0mgP@ZZ")
	s.Require().NoError(err)
	domain.ClID = "domaintestRar"
	domain.RegistrantID = "myTestContact007"
	domain.AdminID = "myTestContact007"
	domain.TechID = "myTestContact007"
	domain.BillingID = "myTestContact007"
	createdDomain2, err := repo.CreateDomain(context.Background(), domain)
	s.Require().NoError(err)

	// Create a third domain
	domain, err = entities.NewDomain("123456_DOM-APEX", "prins.domaintesttld", "GoMamma", "STr0mgP@ZZ")
	s.Require().NoError(err)
	domain.ClID = "domaintestRar"
	domain.RegistrantID = "myTestContact007"
	domain.AdminID = "myTestContact007"
	domain.TechID = "myTestContact007"
	domain.BillingID = "myTestContact007"
	createdDomain3, err := repo.CreateDomain(context.Background(), domain)
	s.Require().NoError(err)

	// List all three
	domains, err := repo.ListDomains(context.Background(), 25, "")
	s.Require().NoError(err)
	s.Require().Equal(3, len(domains))

	// List 2
	domains, err = repo.ListDomains(context.Background(), 2, "")
	s.Require().NoError(err)
	s.Require().Equal(2, len(domains))

	// list the last one
	domains, err = repo.ListDomains(context.Background(), 25, createdDomain2.RoID.String())
	s.Require().NoError(err)
	s.Require().Equal(1, len(domains))
	s.Require().Equal(createdDomain3.RoID, domains[0].RoID)

	// Use a bad roid objectidentifier
	_, err = repo.ListDomains(context.Background(), 25, "1234_CONT-APEX")
	s.Require().ErrorIs(err, entities.ErrInvalidRoid)

	// Use a bad roid int64
	_, err = repo.ListDomains(context.Background(), 25, "ABCD_DOM-APEX")
	s.Require().Error(err)
}

// UpdateDomainWith hosts checks if a domain that has host associations is updated doesn't lose the hosts.
func (s *DomainSuite) TestDomainRepository_UpdateDomainWithHosts() {
	tx := s.db.Begin()
	defer tx.Rollback()
	repo := NewDomainRepository(tx)

	// Create a domain
	domain, err := entities.NewDomain("1234_DOM-APEX", "geoff.domaintesttld", "GoMamma", "STr0mgP@ZZ")
	s.Require().NoError(err)
	domain.ClID = "domaintestRar"
	domain.RegistrantID = "myTestContact007"
	domain.AdminID = "myTestContact007"
	domain.TechID = "myTestContact007"
	domain.BillingID = "myTestContact007"
	domain.Hosts = s.hosts
	createdDomain, err := repo.CreateDomain(context.Background(), domain)
	s.Require().NoError(err)
	s.Require().NotNil(createdDomain)
	s.Require().Equal(len(s.hosts), len(createdDomain.Hosts))

	// Retrieve the domains without hosts
	retrievedDomain, err := repo.GetDomainByName(context.Background(), createdDomain.Name.String(), false)
	s.Require().NoError(err)
	s.Require().NotNil(retrievedDomain)
	s.Require().Equal(0, len(retrievedDomain.Hosts))

	// Update the domain
	createdDomain.AuthInfo = "newAu123$th"
	updatedDomain, err := repo.UpdateDomain(context.Background(), createdDomain)
	s.Require().NoError(err)
	s.Require().NotNil(updatedDomain)
	s.Require().Equal(createdDomain.Name, updatedDomain.Name)
	s.Require().Equal(createdDomain.ClID, updatedDomain.ClID)
	s.Require().Equal(createdDomain.AuthInfo, updatedDomain.AuthInfo)
	s.Require().Equal(createdDomain.RoID, updatedDomain.RoID)

	// Retrieve the domain with hosts
	retrievedDomain, err = repo.GetDomainByName(context.Background(), createdDomain.Name.String(), true)
	s.Require().NoError(err)
	s.Require().NotNil(retrievedDomain)
	s.Require().Equal(len(s.hosts), len(retrievedDomain.Hosts))
	s.Require().Equal("newAu123$th", retrievedDomain.AuthInfo.String())
}

func (s *DomainSuite) TestDomainRepository_ListExpiringDomains() {
	// Create a couple of domains with different expiry dates
	tx := s.db.Begin()
	defer tx.Rollback()

	repo := NewDomainRepository(tx)

	// Create 3 domains
	expecteddomains := make([]*entities.Domain, 3)
	for i := 0; i < 3; i++ {
		// Create a domain
		roid := fmt.Sprintf("1234%d_DOM-APEX", i)
		name := fmt.Sprintf("geoff-%d.domaintesttld%d", i, i)
		domain, err := entities.NewDomain(roid, name, "GoMamma", "STr0mgP@ZZ")
		s.Require().NoError(err)
		domain.ClID = "domaintestRar"
		domain.TLDName = entities.DomainName(fmt.Sprintf("domaintesttld%d", i))
		domain.RegistrantID = "myTestContact007"
		domain.AdminID = "myTestContact007"
		domain.TechID = "myTestContact007"
		domain.BillingID = "myTestContact007"
		// Set the expiry date to be in 1, 2, 3 days
		domain.ExpiryDate = time.Now().AddDate(0, 0, i+1).UTC()

		createdDomain, err := repo.CreateDomain(context.Background(), domain)
		s.Require().NoError(err)
		s.Require().NotNil(createdDomain)

		expecteddomains[i] = createdDomain
	}

	// List domains that are expiring in 2 days
	domains, err := repo.ListExpiringDomains(context.Background(), time.Now().AddDate(0, 0, 2), 25, "domaintestRar", "", "")
	s.Require().NoError(err)
	s.Require().Equal(2, len(domains))

	// List domains that are expiring in 3 days
	domains, err = repo.ListExpiringDomains(context.Background(), time.Now().AddDate(0, 0, 3), 25, "domaintestRar", "", "")
	s.Require().NoError(err)
	s.Require().Equal(3, len(domains))

	// List the domains for a specific registrar
	domains, err = repo.ListExpiringDomains(context.Background(), time.Now().AddDate(0, 0, 3), 25, "domaintestRar", "", "")
	s.Require().NoError(err)
	s.Require().Equal(3, len(domains))

	// List the domains for a specific registrar and tld
	domains, err = repo.ListExpiringDomains(context.Background(), time.Now().AddDate(0, 0, 3), 25, "domaintestRar", "domaintesttld1", "")
	s.Require().NoError(err)
	s.Require().Equal(1, len(domains))

	// Test the count endpoint while we are here
	count, err := repo.CountExpiringDomains(context.Background(), time.Now().AddDate(0, 0, 3), "domaintestRar", "")
	s.Require().NoError(err)
	s.Require().Equal(int64(3), count)

	count, err = repo.CountExpiringDomains(context.Background(), time.Now().AddDate(0, 0, 3), "domaintestRar", "idontexist")
	s.Require().NoError(err)
	s.Require().Equal(int64(0), count)

	count, err = repo.CountExpiringDomains(context.Background(), time.Now().AddDate(0, 0, 3), "domaintestRar", "")
	s.Require().NoError(err)
	s.Require().Equal(int64(3), count)

	count, err = repo.CountExpiringDomains(context.Background(), time.Now().AddDate(0, 0, 3), "idontexist", "")
	s.Require().NoError(err)
	s.Require().Equal(int64(0), count)

	// Now add a cursor and list the last domain
	domains, err = repo.ListExpiringDomains(context.Background(), time.Now().AddDate(0, 0, 3), 25, "domaintestRar", "", expecteddomains[1].RoID.String())
	s.Require().NoError(err)
	s.Require().Equal(1, len(domains))

	// Cause an error due to invalid roid
	_, err = repo.ListExpiringDomains(context.Background(), time.Now().AddDate(0, 0, 3), 25, "domaintestRar", "", "1234_CONT-APEX")
	s.Require().ErrorIs(err, entities.ErrInvalidRoid)

	// Cause an error due to invalid roid int64
	_, err = repo.ListExpiringDomains(context.Background(), time.Now().AddDate(0, 0, 3), 25, "domaintestRar", "", "ABCD_DOM-APEX")
	s.Require().Error(err)

}

func (s *DomainSuite) TestDomainRepository_ListPurgeableDomains() {
	// Create a couple of domains with different expiry dates
	tx := s.db.Begin()
	defer tx.Rollback()

	repo := NewDomainRepository(tx)

	// Create 3 domains
	expecteddomains := make([]*entities.Domain, 3)
	for i := 0; i < 3; i++ {
		// Create a domain
		roid := fmt.Sprintf("1234%d_DOM-APEX", i)
		name := fmt.Sprintf("geoff-%d.domaintesttld", i)
		domain, err := entities.NewDomain(roid, name, "GoMamma", "STr0mgP@ZZ")
		s.Require().NoError(err)
		domain.ClID = "domaintestRar"
		domain.TLDName = "domaintesttld"
		domain.RegistrantID = "myTestContact007"
		domain.AdminID = "myTestContact007"
		domain.TechID = "myTestContact007"
		domain.BillingID = "myTestContact007"
		// Set the expiry date to be in 1, 2, 3 days
		domain.RGPStatus.PurgeDate = time.Now().AddDate(0, 0, i+1).UTC()
		// Set the domain to be pending delete
		domain.Status.PendingDelete = true

		createdDomain, err := repo.CreateDomain(context.Background(), domain)
		s.Require().NoError(err)
		s.Require().NotNil(createdDomain)

		expecteddomains[i] = createdDomain
	}

	// List domains that are pending delete (0 as of now)
	domains, err := repo.ListPurgeableDomains(context.Background(), time.Now().AddDate(0, 0, 0), 25, "domaintestRar", "", "domaintesttld")
	s.Require().NoError(err)
	s.Require().Equal(0, len(domains))

	// List domains that are pending delete
	domains, err = repo.ListPurgeableDomains(context.Background(), time.Now().AddDate(0, 0, 5), 25, "domaintestRar", "", "")
	s.Require().NoError(err)
	s.Require().Equal(3, len(domains))

	// Test the count endpoint while we are here
	count, err := repo.CountPurgeableDomains(context.Background(), time.Now().AddDate(0, 0, 0), "domaintestRar", "domaintesttld")
	s.Require().NoError(err)
	s.Require().Equal(int64(0), count)

	count, err = repo.CountPurgeableDomains(context.Background(), time.Now().AddDate(0, 0, 5), "domaintestRar", "")
	s.Require().NoError(err)
	s.Require().Equal(int64(3), count)

	count, err = repo.CountPurgeableDomains(context.Background(), time.Now().AddDate(0, 0, 5), "idontexist", "")
	s.Require().NoError(err)
	s.Require().Equal(int64(0), count)

	// Now add a cursor and list the last domain
	domains, err = repo.ListPurgeableDomains(context.Background(), time.Now().AddDate(0, 0, 5), 25, "domaintestRar", expecteddomains[1].RoID.String(), "domaintesttld")
	s.Require().NoError(err)
	s.Require().Equal(1, len(domains))

	// Cause an error due to invalid roid
	_, err = repo.ListPurgeableDomains(context.Background(), time.Now().AddDate(0, 0, 0), 25, "domaintestRar", "1234_CONT-APEX", "domaintesttld")
	s.Require().ErrorIs(err, entities.ErrInvalidRoid)

	// Cause an error due to invalid roid int64
	_, err = repo.ListPurgeableDomains(context.Background(), time.Now().AddDate(0, 0, 0), 25, "domaintestRar", "ABCD_DOM-APEX", "domaintesttld")
	s.Require().Error(err)

}

func (s *DomainSuite) TestGetInt64RoidFromDomainRoidString() {
	// Test with a valid DOMAIN_ROID_ID
	roidString := "1234_DOM-APEX"
	expectedRoid := int64(1234)
	roid, err := getInt64RoidFromDomainRoidString(roidString)
	s.Require().NoError(err)
	s.Require().Equal(expectedRoid, roid)

	// Test with an empty string
	roidString = ""
	expectedRoid = int64(0)
	roid, err = getInt64RoidFromDomainRoidString(roidString)
	s.Require().NoError(err)
	s.Require().Equal(expectedRoid, roid)

	// Test with an invalid DOMAIN_ROID_ID
	roidString = "1234_CONT-APEX"
	roid, err = getInt64RoidFromDomainRoidString(roidString)
	s.Require().ErrorIs(err, entities.ErrInvalidRoid)
	s.Require().Equal(int64(0), roid)

	// Test with an invalid roid int64
	roidString = "ABCD_DOM-APEX"
	roid, err = getInt64RoidFromDomainRoidString(roidString)
	s.Require().Error(err)
	s.Require().Equal(int64(0), roid)
}

func (s *DomainSuite) TestDomainRepository_ListRestoredDomains() {
	// Create a couple of domains with different expiry dates
	tx := s.db.Begin()
	defer tx.Rollback()

	repo := NewDomainRepository(tx)

	// Create 3 domains
	expecteddomains := make([]*entities.Domain, 3)
	for i := 0; i < 3; i++ {
		// Create a domain
		roid := fmt.Sprintf("1234%d_DOM-APEX", i)
		name := fmt.Sprintf("geoff-%d.domaintesttld", i)
		domain, err := entities.NewDomain(roid, name, "GoMamma", "STr0mgP@ZZ")
		s.Require().NoError(err)
		domain.ClID = "domaintestRar"
		domain.TLDName = "domaintesttld"
		domain.RegistrantID = "myTestContact007"
		domain.AdminID = "myTestContact007"
		domain.TechID = "myTestContact007"
		domain.BillingID = "myTestContact007"
		// Set the domain to be pending restore
		domain.Status.PendingRestore = true

		createdDomain, err := repo.CreateDomain(context.Background(), domain)
		s.Require().NoError(err)
		s.Require().NotNil(createdDomain)

		expecteddomains[i] = createdDomain
	}

	// List domains that are pending restore (3)
	domains, err := repo.ListRestoredDomains(context.Background(), 25, "domaintestRar", "domaintesttld", "")
	s.Require().NoError(err)
	s.Require().Equal(3, len(domains))

	// List domains that are pending restore without tld
	domains, err = repo.ListRestoredDomains(context.Background(), 25, "domaintestRar", "", "")
	s.Require().NoError(err)
	s.Require().Equal(3, len(domains))

	// List domains that are pending restore without registrar
	domains, err = repo.ListRestoredDomains(context.Background(), 25, "", "", "")
	s.Require().NoError(err)
	s.Require().Equal(3, len(domains))

	// Test the count endpoint while we are here
	count, err := repo.CountRestoredDomains(context.Background(), "domaintestRar", "domaintesttld")
	s.Require().NoError(err)
	s.Require().Equal(int64(3), count)

	count, err = repo.CountRestoredDomains(context.Background(), "domaintestRar", "")
	s.Require().NoError(err)
	s.Require().Equal(int64(3), count)

	count, err = repo.CountRestoredDomains(context.Background(), "", "")
	s.Require().NoError(err)
	s.Require().Equal(int64(3), count)

	count, err = repo.CountRestoredDomains(context.Background(), "idontexist", "")
	s.Require().NoError(err)
	s.Require().Equal(int64(0), count)

	count, err = repo.CountRestoredDomains(context.Background(), "", "idontexist")
	s.Require().NoError(err)
	s.Require().Equal(int64(0), count)

	// Now add a cursor and list the last domain
	domains, err = repo.ListRestoredDomains(context.Background(), 25, "domaintestRar", "domaintesttld", expecteddomains[1].RoID.String())
	s.Require().NoError(err)
	s.Require().Equal(1, len(domains))

	// Cause an error due to invalid roid
	_, err = repo.ListRestoredDomains(context.Background(), 25, "domaintestRar", "domaintesttld", "1234_CONT-APEX")
	s.Require().ErrorIs(err, entities.ErrInvalidRoid)

	// Cause an error due to invalid roid int64
	_, err = repo.ListRestoredDomains(context.Background(), 25, "domaintestRar", "domaintesttld", "ABCD_DOM-APEX")
	s.Require().Error(err)

}
