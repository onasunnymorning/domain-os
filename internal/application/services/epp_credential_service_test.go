package services

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"math/big"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/onasunnymorning/domain-os/internal/appcontext"
	"github.com/onasunnymorning/domain-os/internal/application/commands"
	"github.com/onasunnymorning/domain-os/internal/infrastructure/memstore"
	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
	"github.com/onasunnymorning/domain-os/pkg/domain/repositories"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type prefixHasher struct{}

func (prefixHasher) Hash(p string) (string, error)    { return "hash:" + p, nil }
func (prefixHasher) Verify(e, p string) (bool, error) { return e == "hash:"+p, nil }

var eppSvcNow = time.Date(2026, 10, 13, 12, 0, 0, 0, time.UTC)

func newEPPCredentialTestService(t *testing.T) (*EPPCredentialService, *memstore.EPPCredentials, *repositories.MockRegistrarRepository) {
	t.Helper()
	repo := memstore.NewEPPCredentials("rar-001", "rar-002")
	rars := &repositories.MockRegistrarRepository{}
	svc := NewEPPCredentialService(repo, rars, prefixHasher{})
	svc.now = func() time.Time { return eppSvcNow }
	return svc, repo, rars
}

func eppAdminCtx() context.Context {
	return appcontext.WithTraceID(appcontext.WithUserID(context.Background(), "admin@example.com"), "trace-1")
}

func TestEPPCredentialServiceCreateLogin(t *testing.T) {
	svc, repo, rars := newEPPCredentialTestService(t)
	ctx := eppAdminCtx()

	// The registrar's own ClID needs no lookup.
	l, err := svc.CreateLogin(ctx, "rar-001", commands.CreateEPPLoginCommand{LoginID: "rar-001"})
	require.NoError(t, err)
	require.Equal(t, entities.EPPLoginRoleTransform, l.Role, "role defaults to transform")
	require.Equal(t, "admin@example.com", l.CreatedBy)

	rars.On("GetByClID", mock.Anything, "rar-001-ro", false).Return((*entities.Registrar)(nil), entities.ErrRegistrarNotFound)
	_, err = svc.CreateLogin(ctx, "rar-001", commands.CreateEPPLoginCommand{LoginID: "rar-001-ro", Role: "readonly"})
	require.NoError(t, err)

	rars.On("GetByClID", mock.Anything, "rar-002", false).Return(&entities.Registrar{ClID: "rar-002"}, nil)
	_, err = svc.CreateLogin(ctx, "rar-001", commands.CreateEPPLoginCommand{LoginID: "rar-002"})
	require.ErrorIs(t, err, entities.ErrEPPLoginExists, "a login may not be named after another registrar")

	require.Len(t, repo.Events(), 2)
	e := repo.Events()[0]
	require.Equal(t, entities.EventEPPLoginCreated, e.Type)
	require.Equal(t, "rar-001", e.Subject)
	require.Equal(t, "admin@example.com", e.Actor)
	require.Equal(t, "trace-1", e.TraceID)
}

func TestEPPCredentialServiceGeneratesPasswords(t *testing.T) {
	svc, repo, _ := newEPPCredentialTestService(t)
	ctx := eppAdminCtx()
	_, err := svc.CreateLogin(ctx, "rar-001", commands.CreateEPPLoginCommand{LoginID: "rar-001"})
	require.NoError(t, err)

	pw, generated, err := svc.AddPassword(ctx, "rar-001", "rar-001", commands.AddEPPPasswordCommand{})
	require.NoError(t, err)
	require.Len(t, generated, entities.MaxEPPPasswordLength)
	require.NoError(t, entities.ValidateEPPPassword(generated))
	require.Equal(t, "hash:"+generated, pw.Hash)

	_, given, err := svc.AddPassword(ctx, "rar-001", "rar-001", commands.AddEPPPasswordCommand{Password: "chosen-pw-1"})
	require.NoError(t, err)
	require.Empty(t, given, "a supplied password is not echoed back")

	for _, e := range repo.Events() {
		raw, err := json.Marshal(e)
		require.NoError(t, err)
		require.NotContains(t, string(raw), generated, "events never carry a password")
		require.NotContains(t, string(raw), "chosen-pw-1")
		require.NotContains(t, string(raw), "hash:")
	}
}

func TestEPPCredentialServicePinsNetworksAndRetires(t *testing.T) {
	svc, repo, _ := newEPPCredentialTestService(t)
	ctx := eppAdminCtx()

	pin, err := svc.AddCertificatePin(ctx, "rar-001", commands.AddEPPCertificatePinCommand{CertificatePEM: string(testClientCertPEM(t))})
	require.NoError(t, err)
	net, err := svc.AddAllowedNetwork(ctx, "rar-001", commands.AddEPPAllowedNetworkCommand{Network: "192.0.2.10", Note: "primary"})
	require.NoError(t, err)
	require.Equal(t, "192.0.2.10/32", net.Prefix.String())

	at := eppSvcNow.Add(48 * time.Hour)
	pinRef := entities.EPPCredentialRef{Kind: entities.EPPCredentialCertificatePin, ID: pin.ID}
	netRef := entities.EPPCredentialRef{Kind: entities.EPPCredentialAllowedNetwork, ID: net.ID}
	require.NoError(t, svc.RetireCredential(ctx, "rar-001", pinRef, commands.RetireEPPCredentialCommand{At: &at}))
	a, err := svc.GetAccess(ctx, "rar-001")
	require.NoError(t, err)
	require.Equal(t, entities.EPPCredentialRetiring, a.CertificatePins[0].State(eppSvcNow))
	require.NoError(t, svc.RetireCredential(ctx, "rar-001", netRef, commands.RetireEPPCredentialCommand{Kind: "compromise"}))

	err = svc.RetireCredential(ctx, "rar-001", entities.EPPCredentialRef{Kind: entities.EPPCredentialCertificatePin, ID: uuid.New()}, commands.RetireEPPCredentialCommand{})
	require.ErrorIs(t, err, entities.ErrEPPCredentialNotFound)
	err = svc.RetireCredential(ctx, "rar-001", netRef, commands.RetireEPPCredentialCommand{Kind: "whenever"})
	require.ErrorIs(t, err, entities.ErrInvalidEPPCredential)

	types := make([]string, 0, len(repo.Events()))
	for _, e := range repo.Events() {
		types = append(types, e.Type)
	}
	require.Equal(t, []string{"epp.certificate_pin.added", "epp.allowed_network.added", "epp.certificate_pin.retired", "epp.allowed_network.retired"}, types)
	data := repo.Events()[2].Data.(entities.EPPCredentialEventData)
	require.Equal(t, at, *data.RetireAt)
	require.Equal(t, entities.EPPRetireNormal, data.RetireKind)
}

func TestEPPCredentialServiceUnknownRegistrar(t *testing.T) {
	svc, _, _ := newEPPCredentialTestService(t)
	_, err := svc.AddAllowedNetwork(eppAdminCtx(), "rar-999", commands.AddEPPAllowedNetworkCommand{Network: "192.0.2.1"})
	require.ErrorIs(t, err, entities.ErrRegistrarNotFound)
}

func testClientCertPEM(t *testing.T) []byte {
	t.Helper()
	k, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	tmpl := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "client"}, NotBefore: eppSvcNow.Add(-time.Hour), NotAfter: eppSvcNow.Add(365 * 24 * time.Hour)}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &k.PublicKey, k)
	require.NoError(t, err)
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
}
