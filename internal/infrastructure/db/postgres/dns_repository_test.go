package postgres

import (
	"context"
	"fmt"
	"testing"

	"github.com/onasunnymorning/domain-os/internal/domain/entities"
	"github.com/stretchr/testify/suite"
	"gorm.io/gorm"
)

type DNSSuite struct {
	suite.Suite
	db        *gorm.DB
	rarClid   string
	tld       string
	contactID string
	domains   []*entities.Domain
	hosts     []*entities.Host
}

func TestDNSSuite(t *testing.T) {
	suite.Run(t, new(DNSSuite))
}

func (s *DNSSuite) SetupSuite() {
	s.db = setupTestDB()

	// Create a registrar
	rar, _ := entities.NewRegistrar("GoBro", "goBroooo Inc.", "email@gobro.com", 189, getValidRegistrarPostalInfoArr())
	repo := NewGormRegistrarRepository(s.db)
	createdRar, err := repo.Create(context.Background(), rar)
	s.Require().NoError(err)
	s.Require().NotNil(createdRar)
	s.rarClid = createdRar.ClID.String()

	// Create a TLD
	tld, _ := entities.NewTLD("windy")
	tldRepo := NewGormTLDRepo(s.db)
	err = tldRepo.Create(context.Background(), tld)
	s.Require().NoError(err)
	s.tld = tld.Name.String()

	// Create a contact
	contact, err := entities.NewContact("myTC007", "123456789_CONT-APEX", "my@email.me", "st0NGp@ZZ", string(rar.ClID))
	s.Require().NoError(err)
	contactRepo := NewContactRepository(s.db)
	createdContact, err := contactRepo.CreateContact(context.Background(), contact)
	s.Require().NoError(err)
	s.Require().NotNil(createdContact)
	s.contactID = createdContact.ClID.String()

	// Create some hosts
	hostRepo := NewGormHostRepository(s.db)
	for i := 0; i < 3; i++ {
		host, err := entities.NewHost("ns"+fmt.Sprint(i)+".windy.domains", fmt.Sprint(i)+"5674_HOST-APEX", "GoBro")
		s.Require().NoError(err)
		s.Require().NotNil(host)

		createdHost, err := hostRepo.CreateHost(context.Background(), host)
		s.Require().NoError(err)
		s.Require().NotNil(createdHost)

		s.hosts = append(s.hosts, createdHost)
	}

	// Create some domains
	domainRepo := NewDomainRepository(s.db)
	for i := 0; i < 3; i++ {
		domain, err := entities.NewDomain(fmt.Sprintf("%d1234456_DOM-APEX", i), "domain"+fmt.Sprint(i)+".windy", createdRar.ClID.String(), "st0NGp@ZZ")
		s.Require().NoError(err)
		s.Require().NotNil(domain)

		domain.RegistrantID = createdContact.ID
		domain.AdminID = createdContact.ID
		domain.TechID = createdContact.ID
		domain.BillingID = createdContact.ID

		createdDomain, err := domainRepo.CreateDomain(context.Background(), domain)
		s.Require().NoError(err)
		s.Require().NotNil(createdDomain)

		s.domains = append(s.domains, createdDomain)
	}

}

func (s *DNSSuite) TearDownSuite() {
	if s.tld != "" {
		repo := NewGormTLDRepo(s.db)
		_ = repo.DeleteByName(context.Background(), s.tld)
	}
	if s.rarClid != "" {
		repo := NewGormRegistrarRepository(s.db)
		_ = repo.Delete(context.Background(), s.rarClid)
	}
	for i, dom := range s.domains {
		repo := NewDomainRepository(s.db)
		dom_roid, err := dom.RoID.Int64()
		s.Require().NoError(err)
		ho_roid, err := s.hosts[i].RoID.Int64()
		s.Require().NoError(err)
		err = repo.RemoveHostFromDomain(context.Background(), dom_roid, ho_roid)
		s.Require().NoError(err)
	}
	if s.hosts != nil {
		repo := NewGormHostRepository(s.db)
		for _, host := range s.hosts {
			hoRoID, _ := host.RoID.Int64()
			_ = repo.DeleteHostByRoid(context.Background(), hoRoID)
		}
	}
	if s.domains != nil {
		repo := NewDomainRepository(s.db)
		for _, dom := range s.domains {
			_ = repo.DeleteDomainByName(context.Background(), dom.Name.String())
		}
	}
}

// // If there are no domains with hosts (we haven't linked them in the setup), we should get nil
// func (s *DNSSuite) TestGetNSRecordsPerTLD_domains_have_no_hosts() {
// 	repo := NewDNSRepository(s.db)
// 	nsRecords, err := repo.GetNSRecordsPerTLD(context.Background(), s.tld)
// 	for _, ns := range nsRecords {
// 		fmt.Println(ns)
// 	}
// 	s.Require().NoError(err)
// 	s.Require().Nil(nsRecords)
// }

// // Test if we link the domains with hosts, we should get the NS records
// func (s *DNSSuite) TestGetNSRecordsPerTLD_domains_have_hosts() {
// 	// Link the domains with the hosts
// 	domainRepo := NewDomainRepository(s.db)
// 	for i, dom := range s.domains {
// 		dom_roid, err := dom.RoID.Int64()
// 		s.Require().NoError(err)
// 		host_roid, err := s.hosts[i].RoID.Int64()
// 		s.Require().NoError(err)
// 		err = domainRepo.AddHostToDomain(context.Background(), dom_roid, host_roid)
// 		s.Require().NoError(err)
// 	}

// 	repo := NewDNSRepository(s.db)
// 	nsRecords, err := repo.GetNSRecordsPerTLD(context.Background(), s.tld)
// 	for _, ns := range nsRecords {
// 		fmt.Println(ns)
// 	}
// 	s.Require().NoError(err)
// 	s.Require().NotNil(nsRecords)
// 	s.Require().Equal(3, len(nsRecords))
// }
