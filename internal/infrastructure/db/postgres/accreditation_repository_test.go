package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
	"github.com/stretchr/testify/suite"
	"gorm.io/gorm"
)

type AccreditationSuite struct {
	suite.Suite
	db  *gorm.DB
	rar *entities.Registrar
	tld *entities.TLD
	ry  *entities.RegistryOperator
}

func TestAccreditationSuite(t *testing.T) {
	suite.Run(t, new(AccreditationSuite))
}

func (s *AccreditationSuite) SetupSuite() {
	s.db = setupTestDB()

	rarRepo := NewGormRegistrarRepository(s.db)
	_ = rarRepo.Delete(context.Background(), "199-myrar")

	tldRepo := NewGormTLDRepo(s.db)
	_ = tldRepo.DeleteByName(context.Background(), "apex")

	roRepo := NewGORMRegistryOperatorRepository(s.db)
	_ = roRepo.DeleteByRyID(context.Background(), "apex")

	// Create a registrar
	rar, err := entities.NewRegistrar("199-myrar", "accreditationRarName", "email@gobro.com", 199, getValidRegistrarPostalInfoArr())
	if err != nil {
		s.T().Fatal(err)
	}
	createdRar, err := rarRepo.Create(context.Background(), rar)
	if err != nil {
		s.T().Fatal(err)
	}
	s.rar = createdRar

	// Create a Registry Operator
	ro, err := entities.NewRegistryOperator("apex", "apex", "me@my.email")
	if err != nil {
		s.T().Fatal(err)
	}
	_, err = roRepo.Create(context.Background(), ro)
	if err != nil {
		s.T().Fatal(err)
	}
	createdRo, err := roRepo.GetByRyID(context.Background(), ro.RyID.String())
	if err != nil {
		s.T().Fatal(err)
	}
	s.ry = createdRo

	// Create a TLD
	tld, err := entities.NewTLD("apex", createdRo.RyID.String())
	if err != nil {
		s.T().Fatal(err)
	}
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
	if s.ry != nil {
		ryRepo := NewGORMRegistryOperatorRepository(s.db)
		_ = ryRepo.DeleteByRyID(context.Background(), s.ry.RyID.String())
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

	// Insert a mock domain for this registrar and TLD
	err = tx.Exec("INSERT INTO domains (ro_id, name, cl_id, tld_name, expiry_date, auth_info) VALUES (?, ?, ?, ?, ?, ?)",
		2001, "domain-accred-test.com", s.rar.ClID.String(), s.tld.Name.String(), time.Now(), "auth").Error
	s.Require().NoError(err)

	tlds, err := repo.ListRegistrarTLDs(context.Background(), 10, "", s.rar.ClID.String())
	s.Require().NoError(err)
	s.Require().Len(tlds, 1)
	s.Require().Equal(1, tlds[0].DomainCount)

	// Delete the accreditation
	err = repo.DeleteAccreditation(context.Background(), s.tld.Name.String(), s.rar.ClID.String())
	s.Require().NoError(err)

	tlds, err = repo.ListRegistrarTLDs(context.Background(), 10, "", s.rar.ClID.String())
	s.Require().NoError(err)
	s.Require().Len(tlds, 0)
}
