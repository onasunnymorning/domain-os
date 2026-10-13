package postgres

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"math/big"
	"testing"
	"time"

	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
	"github.com/stretchr/testify/suite"
	"gorm.io/gorm"
)

type eppTestHasher struct{}

func (eppTestHasher) Hash(p string) (string, error)      { return "hash:" + p, nil }
func (eppTestHasher) Verify(enc, p string) (bool, error) { return enc == "hash:"+p, nil }

type EPPCredentialSuite struct {
	suite.Suite
	db  *gorm.DB
	now time.Time
}

func TestEPPCredentialSuite(t *testing.T) {
	suite.Run(t, new(EPPCredentialSuite))
}

func (s *EPPCredentialSuite) SetupSuite() {
	s.db = setupTestDB()
	s.Require().NoError(AutoMigrate(s.db))
	s.now = time.Date(2026, 10, 13, 12, 0, 0, 0, time.UTC)
}

// begin starts a transaction that is rolled back after the test.
func (s *EPPCredentialSuite) begin() *gorm.DB {
	tx := s.db.Begin()
	s.T().Cleanup(func() { tx.Rollback() })
	return tx
}

func (s *EPPCredentialSuite) registrar(tx *gorm.DB, clid string, gurid int) {
	rar, err := entities.NewRegistrar(clid, clid+" Inc.", clid+"@example.com", gurid, getValidRegistrarPostalInfoArr())
	s.Require().NoError(err)
	_, err = NewGormRegistrarRepository(tx).Create(context.Background(), rar)
	s.Require().NoError(err)
}

func (s *EPPCredentialSuite) certPEM() []byte {
	k, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	s.Require().NoError(err)
	tmpl := &x509.Certificate{SerialNumber: big.NewInt(7), Subject: pkix.Name{CommonName: "client"}, NotBefore: s.now.Add(-time.Hour), NotAfter: s.now.Add(24 * time.Hour)}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &k.PublicKey, k)
	s.Require().NoError(err)
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
}

func (s *EPPCredentialSuite) TestRoundTrip() {
	tx := s.begin()
	ctx := context.Background()
	repo := NewEPPCredentialRepository(tx)
	s.registrar(tx, "epp-rt-rar", 9101)

	empty, err := repo.GetAccess(ctx, "epp-rt-rar")
	s.Require().NoError(err)
	s.Empty(empty.Logins)

	later := s.now.Add(time.Hour)
	saved, err := repo.UpdateAccess(ctx, "epp-rt-rar", func(a *entities.EPPRegistrarAccess) ([]entities.DomainEvent, error) {
		if _, err := a.AddLogin("epp-rt-login", entities.EPPLoginRoleTransform, "tester", s.now); err != nil {
			return nil, err
		}
		pw, err := a.AddPassword("epp-rt-login", "first-pw-1", eppTestHasher{}, "tester", s.now)
		if err != nil {
			return nil, err
		}
		if _, err := a.RetireCredential(entities.EPPCredentialRef{Kind: entities.EPPCredentialPassword, LoginID: "epp-rt-login", ID: pw.ID}, entities.EPPRetireNormal, &later, "tester", s.now); err != nil {
			return nil, err
		}
		if _, err := a.AddCertificatePin(s.certPEM(), []string{"epp-rt-login"}, "tester", s.now); err != nil {
			return nil, err
		}
		if _, err := a.AddAllowedNetwork("2001:db8:1::/48", "dc1", "tester", s.now); err != nil {
			return nil, err
		}
		return []entities.DomainEvent{entities.NewDomainEvent("test", "epp.test", "epp-rt-rar", "", map[string]string{"k": "v"})}, nil
	})
	s.Require().NoError(err)

	got, err := repo.GetAccessByLogin(ctx, "epp-rt-login")
	s.Require().NoError(err)
	s.Equal(saved, got)
	s.Equal("hash:first-pw-1", got.Logins[0].Passwords[0].Hash)
	s.Equal(entities.EPPCredentialRetiring, got.Logins[0].Passwords[0].State(s.now))
	s.Equal([]entities.ClIDType{"epp-rt-login"}, got.CertificatePins[0].Logins)
	s.Equal("2001:db8:1::/48", got.AllowedNetworks[0].Prefix.String())

	var events int64
	s.Require().NoError(tx.Model(&DomainEventRecord{}).Where("subject = ? AND type = ?", "epp-rt-rar", "epp.test").Count(&events).Error)
	s.EqualValues(1, events)

	// A later change updates rows in place.
	used := s.now.Add(time.Minute)
	_, err = repo.UpdateAccess(ctx, "epp-rt-rar", func(a *entities.EPPRegistrarAccess) ([]entities.DomainEvent, error) {
		a.RecordUse([]entities.EPPCredentialUse{{ID: a.AllowedNetworks[0].ID, At: used, From: "2001:db8:1::5"}})
		return nil, nil
	})
	s.Require().NoError(err)
	got, err = repo.GetAccess(ctx, "epp-rt-rar")
	s.Require().NoError(err)
	s.Equal(used, *got.AllowedNetworks[0].LastUsedAt)
	s.Equal("2001:db8:1::5", got.AllowedNetworks[0].LastUsedFrom)
}

func (s *EPPCredentialSuite) TestFailedChangeSavesNothing() {
	tx := s.begin()
	ctx := context.Background()
	repo := NewEPPCredentialRepository(tx)
	s.registrar(tx, "epp-fail-rar", 9102)

	boom := errors.New("boom")
	_, err := repo.UpdateAccess(ctx, "epp-fail-rar", func(a *entities.EPPRegistrarAccess) ([]entities.DomainEvent, error) {
		if _, err := a.AddLogin("epp-fail-login", entities.EPPLoginRoleTransform, "tester", s.now); err != nil {
			return nil, err
		}
		return nil, boom
	})
	s.Require().ErrorIs(err, boom)
	_, err = repo.GetAccessByLogin(ctx, "epp-fail-login")
	s.ErrorIs(err, entities.ErrEPPLoginNotFound)
}

func (s *EPPCredentialSuite) TestLoginIDsAreUniqueAcrossRegistrars() {
	tx := s.begin()
	ctx := context.Background()
	repo := NewEPPCredentialRepository(tx)
	s.registrar(tx, "epp-a-rar", 9103)
	s.registrar(tx, "epp-b-rar", 9104)

	add := func(clid string) error {
		_, err := repo.UpdateAccess(ctx, clid, func(a *entities.EPPRegistrarAccess) ([]entities.DomainEvent, error) {
			_, err := a.AddLogin("epp-shared", entities.EPPLoginRoleTransform, "tester", s.now)
			return nil, err
		})
		return err
	}
	s.Require().NoError(add("epp-a-rar"))
	s.Require().ErrorIs(add("epp-b-rar"), entities.ErrEPPLoginExists)

	got, err := repo.GetAccessByLogin(ctx, "epp-shared")
	s.Require().NoError(err)
	s.Equal(entities.ClIDType("epp-a-rar"), got.ClID, "the first registrar keeps its login")
}

func (s *EPPCredentialSuite) TestUnknownRegistrar() {
	tx := s.begin()
	repo := NewEPPCredentialRepository(tx)
	_, err := repo.GetAccess(context.Background(), "no-such-rar")
	s.ErrorIs(err, entities.ErrRegistrarNotFound)
	_, err = repo.UpdateAccess(context.Background(), "no-such-rar", func(*entities.EPPRegistrarAccess) ([]entities.DomainEvent, error) { return nil, nil })
	s.ErrorIs(err, entities.ErrRegistrarNotFound)
}
