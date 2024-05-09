package postgres

import (
	"context"
	"testing"

	"github.com/onasunnymorning/domain-os/internal/domain/entities"
	"github.com/stretchr/testify/suite"
	"gorm.io/gorm"
)

type AccreditationSuite struct {
	suite.Suite
	db  *gorm.DB
	rar *entities.Registrar
	tld *entities.TLD
}

func TestAccreditationSuite(t *testing.T) {
	suite.Run(t, new(AccreditationSuite))
}

func (s *AccreditationSuite) SetupSuite() {
	s.db = setupTestDB()

	// Create a registrar
	rar, err := entities.NewRegistrar("199-myrar", "goBro Inc.", "email@gobro.com", 199, getValidRegistrarPostalInfoArr())
	if err != nil {
		s.T().Fatal(err)
	}
	rarRepo := NewGormRegistrarRepository(s.db)
	createdRar, err := rarRepo.Create(context.Background(), rar)
	if err != nil {
		s.T().Fatal(err)
	}
	s.rar = createdRar

	// Create a TLD
	tld, err := entities.NewTLD("apex")
	if err != nil {
		s.T().Fatal(err)
	}
	tldRepo := NewGormTLDRepo(s.db)
	err = tldRepo.Create(context.Background(), tld)
	if err != nil {
		s.T().Fatal(err)
	}
	createdTLD, err := tldRepo.GetByName(context.Background(), tld.Name.String(), false)
	s.tld = createdTLD
}

func (s *AccreditationSuite) TearDownSuite() {
	if s.rar != nil {
		rarRepo := NewGormRegistrarRepository(s.db)
		_ = rarRepo.Delete(context.Background(), s.rar.ClID.String())
	}
	if s.tld != nil {
		tldRepo := NewGormTLDRepo(s.db)
		_ = tldRepo.DeleteByName(context.Background(), s.tld.Name.String())
	}
}

func (s *AccreditationSuite) TestCreateAccreditation() {
	tx := s.db.Begin()
	defer tx.Rollback()
	repo := NewAccreditationRepository(tx)

	err := repo.CreateAccreditation(context.Background(), s.tld.Name.String(), s.rar.ClID.String())
	s.Require().NoError(err)

}

func (s *AccreditationSuite) TestDeleteAccreditation_Idempotent() {
	tx := s.db.Begin()
	defer tx.Rollback()
	repo := NewAccreditationRepository(tx)

	err := repo.DeleteAccreditation(context.Background(), s.tld.Name.String(), s.rar.ClID.String())
	s.Require().NoError(err)

}

func (s *AccreditationSuite) TestListTLDRegistrars() {
	tx := s.db.Begin()
	defer tx.Rollback()
	repo := NewAccreditationRepository(tx)

	err := repo.CreateAccreditation(context.Background(), s.tld.Name.String(), s.rar.ClID.String())
	s.Require().NoError(err)

	rars, err := repo.ListTLDRegistrars(context.Background(), 10, "", s.tld.Name.String())
	s.Require().NoError(err)
	s.Require().Len(rars, 1)

	// Delete the accreditation
	err = repo.DeleteAccreditation(context.Background(), s.tld.Name.String(), s.rar.ClID.String())
	s.Require().NoError(err)

	rars, err = repo.ListTLDRegistrars(context.Background(), 10, "", s.tld.Name.String())
	s.Require().NoError(err)
	s.Require().Len(rars, 0)
}

func (s *AccreditationSuite) TestListRegistrarTLDs() {
	tx := s.db.Begin()
	defer tx.Rollback()
	repo := NewAccreditationRepository(tx)

	err := repo.CreateAccreditation(context.Background(), s.tld.Name.String(), s.rar.ClID.String())
	s.Require().NoError(err)

	tlds, err := repo.ListRegistrarTLDs(context.Background(), 10, "", s.rar.ClID.String())
	s.Require().NoError(err)
	s.Require().Len(tlds, 1)

	// Delete the accreditation
	err = repo.DeleteAccreditation(context.Background(), s.tld.Name.String(), s.rar.ClID.String())
	s.Require().NoError(err)

	tlds, err = repo.ListRegistrarTLDs(context.Background(), 10, "", s.rar.ClID.String())
	s.Require().NoError(err)
	s.Require().Len(tlds, 0)
}
