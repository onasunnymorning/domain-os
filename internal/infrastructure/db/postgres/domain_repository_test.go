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
	db      *gorm.DB
	rarClid string
	tld     string
}

func TestDomainSuite(t *testing.T) {
	suite.Run(t, new(DomainSuite))
}

func (s *DomainSuite) SetupSuite() {
	s.db = setupTestDB()
	NewGormTLDRepo(s.db)

	// Create a registrar
	rar, _ := entities.NewRegistrar("domaintestRar", "goBro Inc.", "email@gobro.com", 199)
	repo := NewGormRegistrarRepository(s.db)
	createdRar, err := repo.Create(context.Background(), rar)
	s.Require().NoError(err)
	s.Require().NotNil(createdRar)
	s.rarClid = createdRar.ClID.String()

	// Create a TLD
	tld, _ := entities.NewTLD("domaintesttld")
	tldRepo := NewGormTLDRepo(s.db)
	err = tldRepo.Create(tld)
	s.Require().NoError(err)
	s.tld = tld.Name.String()
}

func (s *DomainSuite) TearDownSuite() {
	if s.tld != "" {
		repo := NewGormTLDRepo(s.db)
		_ = repo.DeleteByName(s.tld)
	}
	if s.rarClid != "" {
		repo := NewGormRegistrarRepository(s.db)
		_ = repo.Delete(context.Background(), s.rarClid)
	}
}

// func (s *DomainSuite) TestDomainRepository_CreateDomain() {
// 	tx := s.db.Begin()
// 	defer tx.Rollback()
// 	repo := NewDomainRepository(tx)

// 	// Create a domain
// 	domain, err := entities.NewDomain("1234_DOM-APEX", "geoff.domaintesttld", "STr0mgP@ZZ")
// 	s.Require().NoError(err)
// 	domain.ClID = "domaintestRar"
// 	createdDomain, err := repo.CreateDomain(context.Background(), domain)
// 	s.Require().NoError(err)
// 	s.Require().NotNil(createdDomain)
// 	s.Require().Equal(domain.Name, createdDomain.Name)
// 	s.Require().Equal(domain.ClID, createdDomain.ClID)
// 	s.Require().Equal(domain.AuthInfo, createdDomain.AuthInfo)

// }
