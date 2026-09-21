package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
	"github.com/stretchr/testify/suite"
	"gorm.io/gorm"
)

// RegistryScopeSuite proves the SQL half of operator isolation: two operators,
// one TLD each, and every scoped delete tried from the wrong side first. The
// wrong operator gets not-found and the row is still there afterwards; the
// right operator deletes it. See ADR-0006 and #415.
type RegistryScopeSuite struct {
	suite.Suite
	db         *gorm.DB
	ownerScope entities.RegistryScope
	otherScope entities.RegistryScope
}

const (
	rsOwnerRy  = "RsOwnerRy"
	rsOtherRy  = "RsOtherRy"
	rsOwnerTLD = "rsowned"
	rsOtherTLD = "rsother"
	rsRar      = "rsScopeRar"
)

func TestRegistryScopeSuite(t *testing.T) {
	suite.Run(t, new(RegistryScopeSuite))
}

func (s *RegistryScopeSuite) SetupSuite() {
	s.db = setupTestDB()
	s.Require().NoError(AutoMigrate(s.db))
	ctx := context.Background()
	s.TearDownSuite()

	owner, err := entities.NewOperatorID(rsOwnerRy)
	s.Require().NoError(err)
	other, err := entities.NewOperatorID(rsOtherRy)
	s.Require().NoError(err)
	s.ownerScope = entities.OperatorRegistryScope(owner)
	s.otherScope = entities.OperatorRegistryScope(other)

	roRepo := NewGORMRegistryOperatorRepository(s.db)
	tldRepo := NewGormTLDRepo(s.db)
	for ry, tld := range map[string]string{rsOwnerRy: rsOwnerTLD, rsOtherRy: rsOtherTLD} {
		ro, err := entities.NewRegistryOperator(ry, ry, "ops@"+tld+".example")
		s.Require().NoError(err)
		_, err = roRepo.Create(ctx, ro)
		s.Require().NoError(err)
		t, err := entities.NewTLD(tld, ry)
		s.Require().NoError(err)
		s.Require().NoError(tldRepo.Create(ctx, t))
	}

	rar, err := entities.NewRegistrar(rsRar, "rsScopeRarName", "rs@gobro.com", 8299, getValidRegistrarPostalInfoArr())
	s.Require().NoError(err)
	_, err = NewGormRegistrarRepository(s.db).Create(ctx, rar)
	s.Require().NoError(err)
}

func (s *RegistryScopeSuite) TearDownSuite() {
	ctx := context.Background()
	s.db.Exec(`DELETE FROM domains WHERE cl_id = ?`, rsRar)
	s.db.Exec(`DELETE FROM nndns WHERE tld_name IN ?`, []string{rsOwnerTLD, rsOtherTLD})
	s.db.Exec(`DELETE FROM phases WHERE tld_name IN ?`, []string{rsOwnerTLD, rsOtherTLD})
	s.db.Exec(`DELETE FROM tlds WHERE name IN ?`, []string{rsOwnerTLD, rsOtherTLD})
	_ = NewGormRegistrarRepository(s.db).Delete(ctx, rsRar)
	roRepo := NewGORMRegistryOperatorRepository(s.db)
	_ = roRepo.DeleteByRyID(ctx, rsOwnerRy)
	_ = roRepo.DeleteByRyID(ctx, rsOtherRy)
}

func (s *RegistryScopeSuite) TestDomainDeleteIsConfinedToTheOperator() {
	tx := s.db.Begin()
	defer tx.Rollback()
	repo := NewDomainRepository(tx)
	ctx := context.Background()

	d, err := entities.NewDomain("7101_DOM-APEX", "kept."+rsOwnerTLD, rsRar, "STr0mgP@ZZ")
	s.Require().NoError(err)
	created, err := repo.Create(ctx, d)
	s.Require().NoError(err)
	roid, _ := created.RoID.Int64()

	s.Require().ErrorIs(repo.DeleteDomainByName(ctx, s.otherScope, "kept."+rsOwnerTLD), entities.ErrDomainNotFound)
	s.Require().ErrorIs(repo.DeleteDomainByID(ctx, s.otherScope, roid), entities.ErrDomainNotFound)
	_, err = repo.GetDomainByName(ctx, "kept."+rsOwnerTLD, false)
	s.Require().NoError(err, "another operator's delete must leave the domain in place")

	s.Require().NoError(repo.DeleteDomainByName(ctx, s.ownerScope, "kept."+rsOwnerTLD))
	_, err = repo.GetDomainByName(ctx, "kept."+rsOwnerTLD, false)
	s.Require().Error(err, "the owner's delete must remove it")
}

func (s *RegistryScopeSuite) TestNNDNDeleteIsConfinedToTheOperator() {
	tx := s.db.Begin()
	defer tx.Rollback()
	repo := NewGormNNDNRepository(tx)
	ctx := context.Background()

	n, err := entities.NewNNDN("reserved." + rsOwnerTLD)
	s.Require().NoError(err)
	_, err = repo.CreateNNDN(ctx, n)
	s.Require().NoError(err)

	s.Require().ErrorIs(repo.DeleteNNDN(ctx, s.otherScope, "reserved."+rsOwnerTLD), entities.ErrNNDNNotFound)
	_, err = repo.GetNNDN(ctx, "reserved."+rsOwnerTLD)
	s.Require().NoError(err)

	s.Require().NoError(repo.DeleteNNDN(ctx, s.ownerScope, "reserved."+rsOwnerTLD))
}

func (s *RegistryScopeSuite) TestPhaseDeleteIsConfinedToTheOperator() {
	tx := s.db.Begin()
	defer tx.Rollback()
	repo := NewGormPhaseRepository(tx)
	ctx := context.Background()

	p, err := entities.NewPhase("Future", "Launch", time.Now().UTC().AddDate(0, 1, 0))
	s.Require().NoError(err)
	p.TLDName = rsOwnerTLD
	_, err = repo.CreatePhase(ctx, p)
	s.Require().NoError(err)

	s.Require().ErrorIs(repo.DeletePhaseByTLDAndName(ctx, s.otherScope, rsOwnerTLD, "Future"), entities.ErrPhaseNotFound)
	_, err = repo.GetPhaseByTLDAndName(ctx, rsOwnerTLD, "Future")
	s.Require().NoError(err)

	s.Require().NoError(repo.DeletePhaseByTLDAndName(ctx, s.ownerScope, rsOwnerTLD, "Future"))
}

func (s *RegistryScopeSuite) TestTLDDeleteIsConfinedToTheOperator() {
	tx := s.db.Begin()
	defer tx.Rollback()
	repo := NewGormTLDRepo(tx)
	ctx := context.Background()

	s.Require().ErrorIs(repo.DeleteByName(ctx, s.otherScope, rsOwnerTLD), entities.ErrTLDNotFound)
	_, err := repo.GetByName(ctx, rsOwnerTLD, false)
	s.Require().NoError(err, "another operator must not be able to delete this TLD")

	s.Require().NoError(repo.DeleteByName(ctx, s.ownerScope, rsOwnerTLD))
}

// A zero scope is not "everything": the repository refuses it outright.
func (s *RegistryScopeSuite) TestZeroScopeIsRefused() {
	ctx := context.Background()
	s.Require().ErrorIs(NewDomainRepository(s.db).DeleteDomainByName(ctx, entities.RegistryScope{}, "kept."+rsOwnerTLD), entities.ErrInvalidRegistryScope)
	s.Require().ErrorIs(NewGormTLDRepo(s.db).DeleteByName(ctx, entities.RegistryScope{}, rsOwnerTLD), entities.ErrInvalidRegistryScope)
}
