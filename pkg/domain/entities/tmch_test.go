package entities

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var tmchNow = time.Date(2026, 11, 4, 12, 0, 0, 0, time.UTC)

// testCertPEM issues a self-signed certificate for the tests; isCA and the
// validity window are what the bundle rules read.
func testCertPEM(t *testing.T, cn string, isCA bool, notBefore, notAfter time.Time) []byte {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	tmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(time.Now().UnixNano()),
		Subject:               pkix.Name{CommonName: cn},
		NotBefore:             notBefore,
		NotAfter:              notAfter,
		IsCA:                  isCA,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	require.NoError(t, err)
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
}

func TestNewTMCHCertificates(t *testing.T) {
	valid := testCertPEM(t, "TMCH Root CA", true, tmchNow.AddDate(-1, 0, 0), tmchNow.AddDate(9, 0, 0))
	expired := testCertPEM(t, "TMCH Old CA", true, tmchNow.AddDate(-10, 0, 0), tmchNow.AddDate(-1, 0, 0))
	leaf := testCertPEM(t, "a signer", false, tmchNow.AddDate(-1, 0, 0), tmchNow.AddDate(1, 0, 0))

	t.Run("a bundle with a valid CA, an expired one kept beside it", func(t *testing.T) {
		bundle := append(append([]byte("# comment before the first block\n"), expired...), valid...)
		c, err := NewTMCHCertificates(bundle, "https://ca.icann.org/tmch.crt", tmchNow)
		require.NoError(t, err)
		require.Len(t, c.Certificates, 2)
		assert.Equal(t, "CN=TMCH Old CA", c.Certificates[0].Subject)
		assert.Equal(t, tmchNow, c.FetchedAt)
		assert.True(t, c.ValidAt(tmchNow))
		assert.False(t, c.ValidAt(tmchNow.AddDate(10, 0, 0)))
		assert.Equal(t, tmchNow.AddDate(-1, 0, 0), c.GeneratedAt(), "the latest NotBefore")
		assert.Equal(t, tmchNow.AddDate(9, 0, 0), c.ExpiresAt())
		s := c.Status()
		assert.Equal(t, TMCHSetCertificates, s.Set)
		assert.Equal(t, 2, s.Entries)
		require.NotNil(t, s.ExpiresAt)
		assert.Equal(t, c.ExpiresAt(), *s.ExpiresAt)
		assert.Contains(t, c.Certificates[1].PEM, "BEGIN CERTIFICATE")
		assert.Len(t, c.Fingerprint(), 64)
		assert.Equal(t, c.Fingerprint(), s.Fingerprint)
		only, err := NewTMCHCertificates(valid, "test", tmchNow)
		require.NoError(t, err)
		assert.NotEqual(t, c.Fingerprint(), only.Fingerprint(), "a bundle that dropped a certificate has another fingerprint")
	})
	t.Run("refusals", func(t *testing.T) {
		for name, tc := range map[string]struct {
			bundle []byte
			want   error
		}{
			"empty":                {[]byte(""), ErrTMCHNoCertificate},
			"no certificate block": {[]byte("-----BEGIN PUBLIC KEY-----\nAAAA\n-----END PUBLIC KEY-----\n"), ErrTMCHNoCertificate},
			"garbage block":        {[]byte("-----BEGIN CERTIFICATE-----\nAAAA\n-----END CERTIFICATE-----\n"), ErrTMCHCertificateInvalid},
			"not a CA":             {leaf, ErrTMCHCertificateNotCA},
			"all expired":          {expired, ErrTMCHCertificatesExpired},
		} {
			_, err := NewTMCHCertificates(tc.bundle, "test", tmchNow)
			assert.ErrorIs(t, err, tc.want, name)
		}
	})
}

func TestNewSMDRevocationList(t *testing.T) {
	gen := tmchNow.Add(-2 * time.Hour)
	entries := []SMDRevocation{
		{SMDID: "0000001751373633628125-65535", RevokedAt: gen.Add(-time.Hour)},
		{SMDID: " 0000001681375789102250-65535 ", RevokedAt: gen.Add(-2 * time.Hour)},
	}
	l, err := NewSMDRevocationList(1, gen, entries, "https://ry.marksdb.org/smdrl/smdrl-latest.csv", tmchNow)
	require.NoError(t, err)
	assert.Equal(t, "0000001681375789102250-65535", l.Entries[0].SMDID, "sorted and trimmed")
	assert.True(t, l.Revoked("0000001751373633628125-65535"))
	assert.True(t, l.Revoked(" 0000001681375789102250-65535"))
	assert.False(t, l.Revoked("0000009999999999999999-65535"))
	assert.False(t, (&SMDRevocationList{}).Revoked("anything"), "an empty list revokes nothing")
	s := l.Status()
	assert.Equal(t, TMCHSetSMDRL, s.Set)
	assert.Equal(t, 2, s.Entries)
	assert.Equal(t, gen, s.GeneratedAt)
	assert.Equal(t, 2*time.Hour, s.Age(tmchNow))
	assert.Equal(t, time.Duration(0), s.SinceFetch(tmchNow))
	assert.Nil(t, s.ExpiresAt)

	for name, tc := range map[string]struct {
		version int
		gen     time.Time
		entries []SMDRevocation
		want    error
	}{
		"version 2":               {2, gen, entries, ErrTMCHListVersion},
		"no generation":           {1, time.Time{}, entries, ErrTMCHListGenerated},
		"only bad rows":           {1, gen, []SMDRevocation{{SMDID: "", RevokedAt: gen}, {SMDID: "x-1"}}, ErrTMCHEntriesRejected},
		"more rejected than kept": {1, gen, []SMDRevocation{{SMDID: "x-1", RevokedAt: gen}, {SMDID: "x-1", RevokedAt: gen}, {SMDID: "x-1", RevokedAt: gen}}, ErrTMCHEntriesRejected},
	} {
		_, err := NewSMDRevocationList(tc.version, tc.gen, tc.entries, "test", tmchNow)
		assert.ErrorIs(t, err, tc.want, name)
	}
	// A bad row beside good ones is left out and counted, not fatal: one
	// bad row must not keep a day's revocations from being stored.
	tolerant, err := NewSMDRevocationList(1, gen, append(entries, SMDRevocation{SMDID: "", RevokedAt: gen}, SMDRevocation{SMDID: "0000001751373633628125-65535", RevokedAt: gen}), "test", tmchNow)
	require.NoError(t, err)
	assert.Len(t, tolerant.Entries, 2)
	assert.Equal(t, 2, tolerant.Rejected, "an empty ID and a duplicate")
	assert.Equal(t, 2, tolerant.Status().Rejected)
	empty, err := NewSMDRevocationList(1, gen, nil, "test", tmchNow)
	require.NoError(t, err, "an empty list is a list: nothing revoked")
	assert.Empty(t, empty.Entries)
}

func TestNewDNLList(t *testing.T) {
	gen := tmchNow.Add(-26 * time.Hour)
	entries := []DNLEntry{
		{Label: "Example-One", LookupKey: "2013041500/2/1/5/dkDMTBXNSG4EMHfjzFhsadE9Rzb9lJ-mnHHZxYMFk-cftvfF", InsertedAt: gen},
		{Label: "xn--mgbh0fb", LookupKey: "2013041500/1/1/1/abc", InsertedAt: gen},
		{Label: "alpha", LookupKey: "2013041500/3/1/1/def", InsertedAt: gen},
	}
	l, err := NewDNLList(1, gen, entries, "https://ry.marksdb.org/dnl/dnl-latest.csv", tmchNow)
	require.NoError(t, err)
	assert.Equal(t, []string{"alpha", "example-one", "xn--mgbh0fb"}, []string{l.Entries[0].Label, l.Entries[1].Label, l.Entries[2].Label}, "lower-cased and sorted")
	require.NotNil(t, l.Lookup("EXAMPLE-ONE"))
	assert.Equal(t, "2013041500/2/1/5/dkDMTBXNSG4EMHfjzFhsadE9Rzb9lJ-mnHHZxYMFk-cftvfF", l.Lookup("example-one").LookupKey)
	assert.Nil(t, l.Lookup("beta"))
	assert.Nil(t, (&DNLList{}).Lookup("alpha"))
	s := l.Status()
	assert.Equal(t, TMCHSetDNL, s.Set)
	assert.Equal(t, 3, s.Entries)
	assert.Equal(t, 26*time.Hour, s.Age(tmchNow))

	// Rows that are not entries are left out and counted; a label this
	// registry could never register loses nothing by being left off.
	bad := []DNLEntry{
		{Label: "-bad", LookupKey: "k", InsertedAt: gen},
		{Label: "ok", LookupKey: "", InsertedAt: gen},
		{Label: "ok", LookupKey: "k"},
		{Label: "a b", LookupKey: "k", InsertedAt: gen},
		{Label: "ALPHA", LookupKey: "k2", InsertedAt: gen}, // a duplicate of alpha
	}
	tolerant, err := NewDNLList(1, gen, append(append([]DNLEntry{}, entries...), bad[0], bad[4]), "test", tmchNow)
	require.NoError(t, err)
	assert.Len(t, tolerant.Entries, 3)
	assert.Equal(t, 2, tolerant.Rejected, "an invalid label and a duplicate")
	assert.Equal(t, "2013041500/3/1/1/def", tolerant.Lookup("alpha").LookupKey, "the first listing of a label is the one kept")
	for name, rows := range map[string][]DNLEntry{
		"only bad rows":           bad,
		"more rejected than kept": append(append([]DNLEntry{}, entries...), bad...), // 5 of 8
	} {
		_, err = NewDNLList(1, gen, rows, "test", tmchNow)
		assert.ErrorIs(t, err, ErrTMCHEntriesRejected, name)
	}
	_, err = NewDNLList(0, gen, nil, "test", tmchNow)
	assert.ErrorIs(t, err, ErrTMCHListVersion)
}

func TestTMCHSetStatus_Supersedes(t *testing.T) {
	gen := tmchNow
	stored := &TMCHSetStatus{Set: TMCHSetDNL, GeneratedAt: gen}
	assert.True(t, TMCHSetStatus{GeneratedAt: gen}.Supersedes(nil), "nothing stored")
	assert.True(t, TMCHSetStatus{GeneratedAt: gen.Add(time.Hour)}.Supersedes(stored), "a later generation")
	assert.False(t, TMCHSetStatus{GeneratedAt: gen}.Supersedes(stored), "the same generation")
	assert.False(t, TMCHSetStatus{GeneratedAt: gen.Add(-time.Hour)}.Supersedes(stored), "an older generation never replaces a newer one")
	bundle := &TMCHSetStatus{Set: TMCHSetCertificates, GeneratedAt: gen, Fingerprint: "a"}
	assert.True(t, TMCHSetStatus{GeneratedAt: gen, Fingerprint: "b"}.Supersedes(bundle), "a bundle with other contents at the same generation (a certificate dropped)")
	assert.False(t, TMCHSetStatus{GeneratedAt: gen, Fingerprint: "a"}.Supersedes(bundle))
	assert.False(t, TMCHSetStatus{GeneratedAt: gen.Add(-time.Hour), Fingerprint: "b"}.Supersedes(bundle), "an older bundle does not")
}

func TestTMCHStatus_Set(t *testing.T) {
	s := TMCHStatus{DNL: &TMCHSetStatus{Set: TMCHSetDNL, Entries: 4}}
	got, err := s.Set(TMCHSetDNL)
	require.NoError(t, err)
	assert.Equal(t, 4, got.Entries)
	_, err = s.Set(TMCHSetSMDRL)
	assert.ErrorIs(t, err, ErrTMCHDataMissing)
	_, err = s.Set("marks")
	assert.ErrorIs(t, err, ErrTMCHSetUnknown)
}

func TestNewTMCHRefreshEvent(t *testing.T) {
	status := TMCHSetStatus{Set: TMCHSetSMDRL, GeneratedAt: tmchNow, FetchedAt: tmchNow, Entries: 12, Source: "test"}
	ev, err := NewTMCHRefreshEvent("domain-os/worker", status)
	require.NoError(t, err)
	assert.Equal(t, EventTMCHSMDRLRefreshed, ev.Type)
	assert.Equal(t, TMCHSetSMDRL, ev.Subject)
	assert.Equal(t, "domain-os/worker", ev.Source)
	assert.Equal(t, status, ev.Data)
	assert.Contains(t, ev.Description, "12 entries")
	for set, typ := range map[string]string{TMCHSetCertificates: EventTMCHCertificatesRefreshed, TMCHSetDNL: EventTMCHDNLRefreshed} {
		ev, err := NewTMCHRefreshEvent("domain-os/api", TMCHSetStatus{Set: set})
		require.NoError(t, err)
		assert.Equal(t, typ, ev.Type)
	}
	_, err = NewTMCHRefreshEvent("domain-os/api", TMCHSetStatus{Set: "marks"})
	assert.ErrorIs(t, err, ErrTMCHSetUnknown)
}
