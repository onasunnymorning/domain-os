package postgres

import (
	"context"
	"fmt"
	"testing"

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

	// Create a TLD
	tld, _ := entities.NewTLD("domaintesttld")
	tldRepo := NewGormTLDRepo(s.db)
	err = tldRepo.Create(context.Background(), tld)
	s.Require().NoError(err)
	s.tld = tld.Name.String()

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

		createdHost, err := hostRepo.CreateHost(context.Background(), host)
		s.Require().NoError(err)
		s.Require().NotNil(createdHost)

		s.hosts = append(s.hosts, createdHost)
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
