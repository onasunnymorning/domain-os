package tmch

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var now = time.Date(2026, 11, 4, 12, 0, 0, 0, time.UTC)

const smdrlFile = "1,2026-11-04T00:00:00.0Z\n" +
	"smd-id,insertion-datetime\n" +
	"0000001751373633628125-65535,2026-11-03T12:00:00.0Z\n" +
	"0000001681375789102250-65535,2026-10-30T08:15:30.5Z\n"

const dnlFile = "1,2026-11-04T00:00:00.0Z\n" +
	"DNL,lookup-key,insertion-datetime\n" +
	"example-one,2026110400/2/1/5/dkDMTBXNSG4EMHfjzFhsadE9Rzb9lJ-mnHHZxYMFk-cftvfF,2026-11-03T00:00:00.0Z\n" +
	"xn--mgbh0fb,2026110400/1/1/1/abc,2026-11-01T00:00:00.0Z\n"

func TestParseSMDRL(t *testing.T) {
	l, err := ParseSMDRL(strings.NewReader(smdrlFile), "test", now)
	require.NoError(t, err)
	assert.Equal(t, 1, l.Version)
	assert.Equal(t, time.Date(2026, 11, 4, 0, 0, 0, 0, time.UTC), l.GeneratedAt)
	require.Len(t, l.Entries, 2)
	assert.True(t, l.Revoked("0000001751373633628125-65535"))
	assert.Equal(t, time.Date(2026, 10, 30, 8, 15, 30, 500_000_000, time.UTC), l.Entries[0].RevokedAt)
	assert.Equal(t, "test", l.Source)

	empty, err := ParseSMDRL(strings.NewReader("1,2026-11-04T00:00:00.0Z\nsmd-id,insertion-datetime\n"), "test", now)
	require.NoError(t, err, "a list with no revocations")
	assert.Empty(t, empty.Entries)

	for name, tc := range map[string]struct {
		file string
		want error
	}{
		"empty file":        {"", ErrEmptyList},
		"one line":          {"1,2026-11-04T00:00:00.0Z\n", ErrEmptyList},
		"bad version":       {"x,2026-11-04T00:00:00.0Z\nsmd-id,insertion-datetime\n", ErrListHeader},
		"bad generation":    {"1,yesterday\nsmd-id,insertion-datetime\n", ErrListHeader},
		"wrong columns":     {"1,2026-11-04T00:00:00.0Z\nDNL,lookup-key,insertion-datetime\n", ErrListHeader},
		"three fields":      {"1,2026-11-04T00:00:00.0Z\nsmd-id,insertion-datetime\na-1,2026-11-03T12:00:00.0Z,extra\n", ErrListRecord},
		"bad timestamp":     {"1,2026-11-04T00:00:00.0Z\nsmd-id,insertion-datetime\na-1,noon\n", ErrListRecord},
		"version 2":         {"2,2026-11-04T00:00:00.0Z\nsmd-id,insertion-datetime\n", entities.ErrTMCHListVersion},
		"duplicate entries": {"1,2026-11-04T00:00:00.0Z\nsmd-id,insertion-datetime\na-1,2026-11-03T12:00:00.0Z\na-1,2026-11-03T12:00:00.0Z\n", entities.ErrTMCHEntryDuplicate},
	} {
		_, err := ParseSMDRL(strings.NewReader(tc.file), "test", now)
		assert.ErrorIs(t, err, tc.want, name)
	}
}

func TestParseDNL(t *testing.T) {
	l, err := ParseDNL(strings.NewReader(dnlFile), "test", now)
	require.NoError(t, err)
	require.Len(t, l.Entries, 2)
	e := l.Lookup("Example-One")
	require.NotNil(t, e)
	assert.Equal(t, "2026110400/2/1/5/dkDMTBXNSG4EMHfjzFhsadE9Rzb9lJ-mnHHZxYMFk-cftvfF", e.LookupKey)
	assert.Equal(t, time.Date(2026, 11, 3, 0, 0, 0, 0, time.UTC), e.InsertedAt)
	assert.NotNil(t, l.Lookup("xn--mgbh0fb"))
	assert.Nil(t, l.Lookup("example-two"))

	_, err = ParseDNL(strings.NewReader(smdrlFile), "test", now)
	assert.ErrorIs(t, err, ErrListHeader, "an SMDRL is not a DNL list")
	_, err = ParseDNL(strings.NewReader("1,2026-11-04T00:00:00.0Z\nDNL,lookup-key,insertion-datetime\nok,key\n"), "test", now)
	assert.ErrorIs(t, err, ErrListRecord)
	_, err = ParseDNL(strings.NewReader("1,2026-11-04T00:00:00.0Z\nDNL,lookup-key,insertion-datetime\n-bad,key,2026-11-03T00:00:00.0Z\n"), "test", now)
	assert.ErrorIs(t, err, entities.ErrTMCHEntryInvalid)
}

func caPEM(t *testing.T) []byte {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	tmpl := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "TMCH Root CA"}, NotBefore: now.AddDate(-1, 0, 0), NotAfter: now.AddDate(9, 0, 0), IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	require.NoError(t, err)
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
}

func TestFeed(t *testing.T) {
	var sawAuth []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, pass, ok := r.BasicAuth()
		if ok {
			sawAuth = append(sawAuth, r.URL.Path+":"+user+":"+pass)
		} else {
			sawAuth = append(sawAuth, r.URL.Path+":none")
		}
		switch r.URL.Path {
		case "/tmch.crt":
			_, _ = w.Write(caPEM(t))
		case "/smdrl/smdrl-latest.csv":
			if !ok {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			_, _ = w.Write([]byte(smdrlFile))
		case "/dnl/dnl-latest.csv":
			_, _ = w.Write([]byte(dnlFile))
		case "/huge":
			_, _ = w.Write([]byte(strings.Repeat("x", 2048)))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	cfg := Config{CertificatesURL: srv.URL + "/tmch.crt", SMDRLURL: srv.URL + "/smdrl/smdrl-latest.csv", DNLURL: srv.URL + "/dnl/dnl-latest.csv", Username: "registry", Password: "pw", MaxBytes: 1024}
	feed := NewFeed(cfg, nil)
	feed.now = func() time.Time { return now }
	assert.True(t, feed.Configured())
	assert.False(t, NewFeed(Config{SMDRLURL: "x"}, nil).Configured())

	ctx := context.Background()
	certs, err := feed.FetchCertificates(ctx)
	require.NoError(t, err)
	assert.Len(t, certs.Certificates, 1)
	assert.Equal(t, cfg.CertificatesURL, certs.Source)
	assert.Equal(t, now, certs.FetchedAt)

	smdrl, err := feed.FetchSMDRL(ctx)
	require.NoError(t, err)
	assert.Len(t, smdrl.Entries, 2)

	dnl, err := feed.FetchDNL(ctx)
	require.NoError(t, err)
	assert.Len(t, dnl.Entries, 2)
	assert.Equal(t, cfg.DNLURL, dnl.Source)

	assert.Equal(t, []string{"/tmch.crt:none", "/smdrl/smdrl-latest.csv:registry:pw", "/dnl/dnl-latest.csv:registry:pw"}, sawAuth, "the CA bundle is public; the lists are fetched with the registry's credentials")

	_, err = NewFeed(Config{CertificatesURL: srv.URL + "/missing"}, nil).FetchCertificates(ctx)
	assert.ErrorIs(t, err, ErrFetch)
	_, err = NewFeed(Config{SMDRLURL: srv.URL + "/smdrl/smdrl-latest.csv"}, nil).FetchSMDRL(ctx)
	assert.ErrorIs(t, err, ErrFetch, "401 without credentials")
	_, err = NewFeed(Config{}, nil).FetchDNL(ctx)
	assert.ErrorIs(t, err, ErrNotConfigured)
	_, err = NewFeed(Config{DNLURL: srv.URL + "/huge", MaxBytes: 1024}, nil).FetchDNL(ctx)
	assert.ErrorIs(t, err, ErrTooLarge)
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	_, err = feed.FetchDNL(cancelled)
	assert.ErrorIs(t, err, ErrFetch)
}
