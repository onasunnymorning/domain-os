package services

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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
	"github.com/onasunnymorning/domain-os/pkg/domain/queries"
)

var tmchNow = time.Date(2026, 11, 4, 12, 0, 0, 0, time.UTC)

// fakeTMCHRepo keeps the three sets in memory and records the events given
// with each replacement.
type fakeTMCHRepo struct {
	certs  *entities.TMCHCertificates
	smdrl  *entities.SMDRevocationList
	dnl    *entities.DNLList
	events []entities.DomainEvent
	fail   map[string]error
}

func (r *fakeTMCHRepo) ReplaceCertificates(_ context.Context, b *entities.TMCHCertificates, ev []entities.DomainEvent) error {
	if err := r.fail[entities.TMCHSetCertificates]; err != nil {
		return err
	}
	r.certs, r.events = b, append(r.events, ev...)
	return nil
}
func (r *fakeTMCHRepo) GetCertificates(context.Context) (*entities.TMCHCertificates, error) {
	if r.certs == nil {
		return nil, entities.ErrTMCHDataMissing
	}
	return r.certs, nil
}
func (r *fakeTMCHRepo) ReplaceSMDRL(_ context.Context, l *entities.SMDRevocationList, ev []entities.DomainEvent) error {
	if err := r.fail[entities.TMCHSetSMDRL]; err != nil {
		return err
	}
	r.smdrl, r.events = l, append(r.events, ev...)
	return nil
}
func (r *fakeTMCHRepo) IsSMDRevoked(_ context.Context, id string) (bool, error) {
	return r.smdrl != nil && r.smdrl.Revoked(id), nil
}
func (r *fakeTMCHRepo) ReplaceDNL(_ context.Context, l *entities.DNLList, ev []entities.DomainEvent) error {
	if err := r.fail[entities.TMCHSetDNL]; err != nil {
		return err
	}
	r.dnl, r.events = l, append(r.events, ev...)
	return nil
}
func (r *fakeTMCHRepo) LookupDNL(_ context.Context, label string) (*entities.DNLEntry, error) {
	if r.dnl == nil {
		return nil, nil
	}
	return r.dnl.Lookup(label), nil
}
func (r *fakeTMCHRepo) Status(context.Context) (*entities.TMCHStatus, error) {
	s := &entities.TMCHStatus{}
	if r.certs != nil {
		st := r.certs.Status()
		s.Certificates = &st
	}
	if r.smdrl != nil {
		st := r.smdrl.Status()
		s.SMDRL = &st
	}
	if r.dnl != nil {
		st := r.dnl.Status()
		s.DNL = &st
	}
	return s, nil
}

// fakeTMCHFeed answers each fetch with what the test put in it.
type fakeTMCHFeed struct {
	configured bool
	certs      *entities.TMCHCertificates
	smdrl      *entities.SMDRevocationList
	dnl        *entities.DNLList
	err        map[string]error
}

func (f *fakeTMCHFeed) Configured() bool { return f.configured }
func (f *fakeTMCHFeed) FetchCertificates(context.Context) (*entities.TMCHCertificates, error) {
	return f.certs, f.err[entities.TMCHSetCertificates]
}
func (f *fakeTMCHFeed) FetchSMDRL(context.Context) (*entities.SMDRevocationList, error) {
	return f.smdrl, f.err[entities.TMCHSetSMDRL]
}
func (f *fakeTMCHFeed) FetchDNL(context.Context) (*entities.DNLList, error) {
	return f.dnl, f.err[entities.TMCHSetDNL]
}

func tmchBundle(t *testing.T, notBefore time.Time) *entities.TMCHCertificates {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	tmpl := &x509.Certificate{SerialNumber: big.NewInt(notBefore.Unix()), Subject: pkix.Name{CommonName: "TMCH CA"}, NotBefore: notBefore, NotAfter: notBefore.AddDate(10, 0, 0), IsCA: true, BasicConstraintsValid: true}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	require.NoError(t, err)
	b, err := entities.NewTMCHCertificates(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), "ca", tmchNow)
	require.NoError(t, err)
	return b
}

func tmchFeed(t *testing.T, gen time.Time) *fakeTMCHFeed {
	t.Helper()
	smdrl, err := entities.NewSMDRevocationList(1, gen, []entities.SMDRevocation{{SMDID: "1-65535", RevokedAt: gen}}, "smdrl", tmchNow)
	require.NoError(t, err)
	dnl, err := entities.NewDNLList(1, gen, []entities.DNLEntry{{Label: "example", LookupKey: "k", InsertedAt: gen}}, "dnl", tmchNow)
	require.NoError(t, err)
	return &fakeTMCHFeed{configured: true, certs: tmchBundle(t, tmchNow.AddDate(-2, 0, 0)), smdrl: smdrl, dnl: dnl, err: map[string]error{}}
}

func outcomes(r *queries.TMCHRefreshResult) map[string]string {
	m := map[string]string{}
	for _, s := range r.Sets {
		m[s.Set] = s.Outcome
	}
	return m
}

func TestTMCHService_Refresh(t *testing.T) {
	ctx := context.Background()
	gen := tmchNow.Add(-6 * time.Hour)

	t.Run("no feed", func(t *testing.T) {
		svc := NewTMCHService(&fakeTMCHRepo{}, nil, "domain-os/worker")
		assert.False(t, svc.Configured())
		_, err := svc.Refresh(ctx)
		assert.ErrorIs(t, err, entities.ErrTMCHNotConfigured)
		svc = NewTMCHService(&fakeTMCHRepo{}, &fakeTMCHFeed{configured: false}, "domain-os/worker")
		_, err = svc.Refresh(ctx)
		assert.ErrorIs(t, err, entities.ErrTMCHNotConfigured)
		status, err := svc.Status(ctx)
		require.NoError(t, err, "status answers without a feed")
		assert.Nil(t, status.DNL)
	})

	t.Run("first refresh stores every set with its event", func(t *testing.T) {
		repo := &fakeTMCHRepo{}
		svc := NewTMCHService(repo, tmchFeed(t, gen), "domain-os/worker")
		svc.now = func() time.Time { return tmchNow }
		result, err := svc.Refresh(ctx)
		require.NoError(t, err)
		assert.Equal(t, map[string]string{"certificates": "refreshed", "smdrl": "refreshed", "dnl": "refreshed"}, outcomes(result))
		assert.Equal(t, 3, result.Refreshed())
		assert.Equal(t, 0, result.Failed())
		assert.Equal(t, tmchNow, result.StartedAt)
		require.Len(t, repo.events, 3)
		assert.Equal(t, entities.EventTMCHCertificatesRefreshed, repo.events[0].Type)
		assert.Equal(t, entities.EventTMCHSMDRLRefreshed, repo.events[1].Type)
		assert.Equal(t, entities.EventTMCHDNLRefreshed, repo.events[2].Type)
		assert.Equal(t, "domain-os/worker", repo.events[2].Source)
		revoked, _ := repo.IsSMDRevoked(ctx, "1-65535")
		assert.True(t, revoked)

		// The same generation again: nothing written, nothing emitted.
		result, err = svc.Refresh(ctx)
		require.NoError(t, err)
		assert.Equal(t, map[string]string{"certificates": "unchanged", "smdrl": "unchanged", "dnl": "unchanged"}, outcomes(result))
		assert.Len(t, repo.events, 3)
		assert.Equal(t, 0, result.Refreshed())
		require.NotNil(t, result.Sets[2].Status, "an unchanged set still reports what is stored")
		assert.Equal(t, 1, result.Sets[2].Status.Entries)
	})

	t.Run("a newer generation replaces, an older one does not", func(t *testing.T) {
		repo := &fakeTMCHRepo{}
		feed := tmchFeed(t, gen)
		svc := NewTMCHService(repo, feed, "domain-os/worker")
		_, err := svc.Refresh(ctx)
		require.NoError(t, err)

		newer, err := entities.NewDNLList(1, gen.Add(24*time.Hour), nil, "dnl", tmchNow)
		require.NoError(t, err)
		feed.dnl = newer
		older, err := entities.NewSMDRevocationList(1, gen.Add(-24*time.Hour), nil, "smdrl", tmchNow)
		require.NoError(t, err)
		feed.smdrl = older
		result, err := svc.Refresh(ctx)
		require.NoError(t, err)
		assert.Equal(t, map[string]string{"certificates": "unchanged", "smdrl": "unchanged", "dnl": "refreshed"}, outcomes(result))
		assert.Equal(t, 0, repo.dnl.Status().Entries, "the newer, empty DNL list is what is stored")
		assert.Equal(t, 1, repo.smdrl.Status().Entries, "the older SMDRL did not replace the stored one")
		assert.Len(t, repo.events, 4)
	})

	t.Run("one set failing does not stop the others", func(t *testing.T) {
		repo := &fakeTMCHRepo{fail: map[string]error{entities.TMCHSetDNL: errors.New("disk full")}}
		feed := tmchFeed(t, gen)
		feed.err[entities.TMCHSetSMDRL] = errors.New("401 Unauthorized")
		svc := NewTMCHService(repo, feed, "domain-os/worker")
		result, err := svc.Refresh(ctx)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "smdrl: 401 Unauthorized")
		assert.Contains(t, err.Error(), "dnl: store: disk full")
		assert.Equal(t, map[string]string{"certificates": "refreshed", "smdrl": "failed", "dnl": "failed"}, outcomes(result))
		assert.Equal(t, 2, result.Failed())
		assert.NotNil(t, repo.certs)
		assert.Nil(t, repo.smdrl)
		assert.Nil(t, repo.dnl)
		assert.Len(t, repo.events, 1)
		assert.Nil(t, result.Sets[1].Status, "nothing stored for a set that never loaded")
	})
}
