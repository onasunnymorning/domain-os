package postgres

import (
	"context"
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
}

func TestDomainSuite(t *testing.T) {
	suite.Run(t, new(DomainSuite))
}

func (s *DomainSuite) SetupSuite() {
	s.db = setupTestDB()
	NewGormTLDRepo(s.db)

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
