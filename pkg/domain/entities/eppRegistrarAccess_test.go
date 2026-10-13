package entities

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net/netip"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

var eppNow = time.Date(2026, 10, 13, 12, 0, 0, 0, time.UTC)

// plainHasher stores "hash:" + the password, so tests can see what was hashed.
type plainHasher struct{}

func (plainHasher) Hash(p string) (string, error) { return "hash:" + p, nil }
func (plainHasher) Verify(enc, p string) (bool, error) {
	return enc == "hash:"+p, nil
}

func newAccess(t *testing.T) *EPPRegistrarAccess {
	t.Helper()
	a, err := NewEPPRegistrarAccess("rar-001")
	require.NoError(t, err)
	_, err = a.AddLogin("rar-001", EPPLoginRoleTransform, "tester", eppNow)
	require.NoError(t, err)
	return a
}

func certPEM(t *testing.T, key any, pub any, notBefore, notAfter time.Time) []byte {
	t.Helper()
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(42),
		Subject:      pkix.Name{CommonName: "epp-client.rar-001.example"},
		NotBefore:    notBefore,
		NotAfter:     notAfter,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, pub, key)
	require.NoError(t, err)
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
}

func ecCert(t *testing.T, notBefore, notAfter time.Time) []byte {
	t.Helper()
	k, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	return certPEM(t, k, &k.PublicKey, notBefore, notAfter)
}

func TestEPPCredentialLifecycle(t *testing.T) {
	lc := newEPPCredentialLifecycle("tester", eppNow)
	require.Equal(t, EPPCredentialActive, lc.State(eppNow))

	later := eppNow.Add(24 * time.Hour)
	require.NoError(t, lc.retire(EPPRetireNormal, &later, "tester", eppNow))
	require.Equal(t, EPPCredentialRetiring, lc.State(eppNow))
	require.True(t, lc.Usable(eppNow))
	require.Equal(t, EPPCredentialRetired, lc.State(later))
	require.False(t, lc.Usable(later))

	// A scheduled retirement can be brought forward until it takes effect.
	require.NoError(t, lc.retire(EPPRetireNormal, &eppNow, "tester", eppNow))
	require.Equal(t, EPPCredentialRetired, lc.State(eppNow))
	require.ErrorIs(t, lc.retire(EPPRetireNormal, &eppNow, "tester", eppNow), ErrEPPCredentialRetired)
}

func TestEPPNormalRetireDefaultsToTheWindow(t *testing.T) {
	lc := newEPPCredentialLifecycle("tester", eppNow)
	require.NoError(t, lc.retire(EPPRetireNormal, nil, "tester", eppNow))
	require.Equal(t, eppNow.Add(DefaultEPPRetireWindow), *lc.RetireAt)
	require.Equal(t, EPPCredentialRetiring, lc.State(eppNow))

	past := eppNow.Add(-time.Hour)
	lc = newEPPCredentialLifecycle("tester", eppNow)
	require.NoError(t, lc.retire(EPPRetireNormal, &past, "tester", eppNow))
	require.Equal(t, eppNow, *lc.RetireAt, "a past time retires now, not in the past")
}

func TestEPPCompromiseRetireIsImmediate(t *testing.T) {
	lc := newEPPCredentialLifecycle("tester", eppNow)
	later := eppNow.Add(time.Hour)
	require.NoError(t, lc.retire(EPPRetireCompromise, &later, "tester", eppNow))
	require.Equal(t, EPPCredentialRetired, lc.State(eppNow), "a compromise cannot be scheduled")
	require.True(t, lc.Compromised())
}

func TestEPPRecordUseOnlyMovesForward(t *testing.T) {
	lc := newEPPCredentialLifecycle("tester", eppNow)
	lc.recordUse(eppNow.Add(time.Minute), "192.0.2.1")
	lc.recordUse(eppNow, "192.0.2.2")
	require.Equal(t, eppNow.Add(time.Minute), *lc.LastUsedAt)
	require.Equal(t, "192.0.2.1", lc.LastUsedFrom)
}

func TestEPPAddLogin(t *testing.T) {
	a := newAccess(t)
	_, err := a.AddLogin("rar-001", EPPLoginRoleReadonly, "tester", eppNow)
	require.ErrorIs(t, err, ErrEPPLoginExists)
	_, err = a.AddLogin("ab", EPPLoginRoleTransform, "tester", eppNow)
	require.ErrorIs(t, err, ErrInvalidEPPCredential)
	_, err = a.AddLogin("rar-001-ro", "admin", "tester", eppNow)
	require.ErrorIs(t, err, ErrInvalidEPPCredential)

	for i := len(a.Logins); i < MaxEPPLoginsPerRegistrar; i++ {
		_, err := a.AddLogin(uuid.NewString()[:12], EPPLoginRoleReadonly, "tester", eppNow)
		require.NoError(t, err)
	}
	_, err = a.AddLogin("one-too-many", EPPLoginRoleReadonly, "tester", eppNow)
	require.ErrorIs(t, err, ErrEPPCredentialLimit)
}

func TestEPPUpdateLogin(t *testing.T) {
	a := newAccess(t)
	off, ro, cap5, tooMany := false, EPPLoginRoleReadonly, 5, MaxEPPSessionCap+1
	l, err := a.UpdateLogin("rar-001", EPPLoginChanges{Enabled: &off, Role: &ro, SessionCap: &cap5}, eppNow.Add(time.Hour))
	require.NoError(t, err)
	require.False(t, l.Enabled)
	require.Equal(t, EPPLoginRoleReadonly, l.Role)
	require.Equal(t, 5, l.SessionCap)
	require.Equal(t, eppNow.Add(time.Hour), l.UpdatedAt)

	_, err = a.UpdateLogin("rar-001", EPPLoginChanges{SessionCap: &tooMany}, eppNow)
	require.ErrorIs(t, err, ErrInvalidEPPCredential)
	_, err = a.UpdateLogin("nobody", EPPLoginChanges{}, eppNow)
	require.ErrorIs(t, err, ErrEPPLoginNotFound)
}

func TestValidateEPPPassword(t *testing.T) {
	for _, ok := range []string{"abc123", "sixteen-chars-ok", "with one space"} {
		require.NoError(t, ValidateEPPPassword(ok), ok)
	}
	for _, bad := range []string{"short", "seventeen-chars-x", " leading", "trailing ", "two  spaces", "tab\there", "naïve-pass"} {
		require.ErrorIs(t, ValidateEPPPassword(bad), ErrInvalidEPPPassword, bad)
	}
}

func TestEPPPasswordCapAndRotation(t *testing.T) {
	a := newAccess(t)
	p1, err := a.AddPassword("rar-001", "first-pw-1", plainHasher{}, "tester", eppNow)
	require.NoError(t, err)
	require.Equal(t, "hash:first-pw-1", p1.Hash)
	_, err = a.AddPassword("rar-001", "second-pw-2", plainHasher{}, "tester", eppNow)
	require.NoError(t, err)

	_, err = a.AddPassword("rar-001", "third-pw-3", plainHasher{}, "tester", eppNow)
	require.ErrorIs(t, err, ErrEPPCredentialLimit, "two passwords may be usable at once")

	// Retiring later still counts until it takes effect.
	later := eppNow.Add(time.Hour)
	_, err = a.RetireCredential(EPPCredentialRef{Kind: EPPCredentialPassword, LoginID: "rar-001", ID: p1.ID}, EPPRetireNormal, &later, "tester", eppNow)
	require.NoError(t, err)
	_, err = a.AddPassword("rar-001", "third-pw-3", plainHasher{}, "tester", eppNow)
	require.ErrorIs(t, err, ErrEPPCredentialLimit)
	_, err = a.AddPassword("rar-001", "third-pw-3", plainHasher{}, "tester", later)
	require.NoError(t, err)

	_, err = a.AddPassword("rar-001", "x", plainHasher{}, "tester", later)
	require.ErrorIs(t, err, ErrInvalidEPPPassword)
}

func TestEPPReplacePassword(t *testing.T) {
	a := newAccess(t)
	p1, err := a.AddPassword("rar-001", "first-pw-1", plainHasher{}, "tester", eppNow)
	require.NoError(t, err)
	_, err = a.AddPassword("rar-001", "second-pw-2", plainHasher{}, "tester", eppNow)
	require.NoError(t, err)

	// <newPW> works at the cap: the count of usable passwords is unchanged.
	p3, err := a.ReplacePassword("rar-001", p1.ID, "third-pw-3", plainHasher{}, "rar-001", eppNow)
	require.NoError(t, err)
	require.Equal(t, EPPCredentialRetired, p1.State(eppNow))
	require.Equal(t, EPPRetireReplaced, p1.RetireKind)
	require.Equal(t, EPPCredentialActive, p3.State(eppNow))

	_, err = a.ReplacePassword("rar-001", p1.ID, "fourth-pw-4", plainHasher{}, "rar-001", eppNow)
	require.ErrorIs(t, err, ErrEPPCredentialRetired)
	_, err = a.ReplacePassword("rar-001", p3.ID, "x", plainHasher{}, "rar-001", eppNow)
	require.ErrorIs(t, err, ErrInvalidEPPPassword)
	_, err = a.RetireCredential(EPPCredentialRef{Kind: EPPCredentialPassword, LoginID: "rar-001", ID: p3.ID}, EPPRetireReplaced, nil, "tester", eppNow)
	require.ErrorIs(t, err, ErrInvalidEPPCredential, "only <newPW> replaces")
}

func TestEPPCertificatePins(t *testing.T) {
	a := newAccess(t)
	pemA := ecCert(t, eppNow.Add(-time.Hour), eppNow.Add(365*24*time.Hour))
	pin, err := a.AddCertificatePin(pemA, nil, "tester", eppNow)
	require.NoError(t, err)
	require.Len(t, pin.SPKISHA256, 64)
	require.Equal(t, "CN=epp-client.rar-001.example", pin.Subject)
	require.True(t, pin.AllowsLogin("anything"))

	_, err = a.AddCertificatePin(pemA, nil, "tester", eppNow)
	require.ErrorIs(t, err, ErrEPPCertificatePinExists)

	// A pin can be in place before its certificate is valid.
	future := ecCert(t, eppNow.Add(24*time.Hour), eppNow.Add(400*24*time.Hour))
	_, err = a.AddCertificatePin(future, []string{"rar-001"}, "tester", eppNow)
	require.NoError(t, err)

	expired := ecCert(t, eppNow.Add(-48*time.Hour), eppNow.Add(-time.Hour))
	_, err = a.AddCertificatePin(expired, nil, "tester", eppNow)
	require.ErrorIs(t, err, ErrInvalidEPPCertificate)

	_, err = a.AddCertificatePin(ecCert(t, eppNow, eppNow.Add(time.Hour)), []string{"nobody"}, "tester", eppNow)
	require.ErrorIs(t, err, ErrEPPLoginNotFound)

	_, err = a.AddCertificatePin(ecCert(t, eppNow, eppNow.Add(time.Hour)), nil, "tester", eppNow)
	require.NoError(t, err)
	_, err = a.AddCertificatePin(ecCert(t, eppNow, eppNow.Add(time.Hour)), nil, "tester", eppNow)
	require.ErrorIs(t, err, ErrEPPCredentialLimit)

	_, err = a.AddCertificatePin([]byte("not pem"), nil, "tester", eppNow)
	require.ErrorIs(t, err, ErrInvalidEPPCertificate)
}

func TestEPPCertificatePinRestriction(t *testing.T) {
	a := newAccess(t)
	_, err := a.AddLogin("rar-001-ro", EPPLoginRoleReadonly, "tester", eppNow)
	require.NoError(t, err)
	pin, err := a.AddCertificatePin(ecCert(t, eppNow, eppNow.Add(time.Hour)), []string{"rar-001-ro", "rar-001-ro"}, "tester", eppNow)
	require.NoError(t, err)
	require.Equal(t, []ClIDType{"rar-001-ro"}, pin.Logins)
	require.True(t, pin.AllowsLogin("rar-001-ro"))
	require.False(t, pin.AllowsLogin("rar-001"))
}

func TestEPPCertificatePinRefusesWeakRSA(t *testing.T) {
	a := newAccess(t)
	k, err := rsa.GenerateKey(rand.Reader, 1024) //nolint:gosec // G403: the weak key is what is being refused
	require.NoError(t, err)
	_, err = a.AddCertificatePin(certPEM(t, k, &k.PublicKey, eppNow, eppNow.Add(time.Hour)), nil, "tester", eppNow)
	require.ErrorIs(t, err, ErrInvalidEPPCertificate)
}

func TestParseEPPAllowedPrefix(t *testing.T) {
	for in, want := range map[string]string{
		"192.0.2.10":           "192.0.2.10/32",
		"192.0.2.77/24":        "192.0.2.0/24",
		" 2001:db8::1 ":        "2001:db8::1/128", //nolint:gocritic // mapKey: the surrounding spaces are the input being tested
		"2001:db8:1::/48":      "2001:db8:1::/48",
		"::ffff:192.0.2.9":     "192.0.2.9/32",
		"::ffff:192.0.2.0/120": "192.0.2.0/24",
	} {
		got, err := ParseEPPAllowedPrefix(in)
		require.NoError(t, err, in)
		require.Equal(t, netip.MustParsePrefix(want), got, in)
	}
	for _, bad := range []string{"", "nonsense", "10.0.0.0/8", "0.0.0.0/0", "2001:db8::/32", "192.0.2.1/33", "fe80::1%eth0"} {
		_, err := ParseEPPAllowedPrefix(bad)
		require.ErrorIs(t, err, ErrInvalidEPPAllowedNetwork, bad)
	}
}

func TestEPPAllowedNetworks(t *testing.T) {
	a := newAccess(t)
	n, err := a.AddAllowedNetwork("192.0.2.0/24", " office ", "tester", eppNow)
	require.NoError(t, err)
	require.Equal(t, "office", n.Note)
	_, err = a.AddAllowedNetwork("192.0.2.9/24", "", "tester", eppNow)
	require.ErrorIs(t, err, ErrEPPAllowedNetworkExists, "the same network after masking")

	_, err = a.RetireCredential(EPPCredentialRef{Kind: EPPCredentialAllowedNetwork, ID: n.ID}, EPPRetireNormal, &eppNow, "tester", eppNow)
	require.NoError(t, err)
	_, err = a.AddAllowedNetwork("192.0.2.0/24", "", "tester", eppNow)
	require.NoError(t, err, "a retired network can be allowed again")

	for i := 1; countUsable(a.AllowedNetworks, eppNow) < MaxEPPAllowedNetworksPerRegistrar; i++ {
		_, err := a.AddAllowedNetwork(netip.AddrFrom4([4]byte{198, 51, byte(i), 1}).String(), "", "tester", eppNow)
		require.NoError(t, err)
	}
	_, err = a.AddAllowedNetwork("203.0.113.1", "", "tester", eppNow)
	require.ErrorIs(t, err, ErrEPPCredentialLimit)
}

func TestEPPRetireAndRecordUseFindEveryKind(t *testing.T) {
	a := newAccess(t)
	pw, err := a.AddPassword("rar-001", "first-pw-1", plainHasher{}, "tester", eppNow)
	require.NoError(t, err)
	pin, err := a.AddCertificatePin(ecCert(t, eppNow, eppNow.Add(time.Hour)), nil, "tester", eppNow)
	require.NoError(t, err)
	net, err := a.AddAllowedNetwork("192.0.2.1", "", "tester", eppNow)
	require.NoError(t, err)

	used := eppNow.Add(time.Minute)
	a.RecordUse([]EPPCredentialUse{{ID: pw.ID, At: used, From: "192.0.2.1"}, {ID: pin.ID, At: used}, {ID: net.ID, At: used}, {ID: uuid.New(), At: used}})
	for _, lc := range []*EPPCredentialLifecycle{&pw.EPPCredentialLifecycle, &pin.EPPCredentialLifecycle, &net.EPPCredentialLifecycle} {
		require.Equal(t, used, *lc.LastUsedAt)
	}

	pwRef := EPPCredentialRef{Kind: EPPCredentialPassword, LoginID: "rar-001", ID: pw.ID}
	pinRef := EPPCredentialRef{Kind: EPPCredentialCertificatePin, ID: pin.ID}
	netRef := EPPCredentialRef{Kind: EPPCredentialAllowedNetwork, ID: net.ID}

	// A reference whose parts disagree finds nothing.
	for _, wrong := range []EPPCredentialRef{
		{Kind: EPPCredentialAllowedNetwork, ID: pin.ID},
		{Kind: EPPCredentialPassword, LoginID: "other-login", ID: pw.ID},
		{Kind: EPPCredentialCertificatePin, ID: uuid.New()},
	} {
		_, err := a.RetireCredential(wrong, EPPRetireNormal, nil, "tester", eppNow)
		require.ErrorIs(t, err, ErrEPPCredentialNotFound, wrong)
	}
	for _, ref := range []EPPCredentialRef{pwRef, pinRef, netRef} {
		lc, err := a.RetireCredential(ref, EPPRetireCompromise, nil, "tester", eppNow)
		require.NoError(t, err)
		require.True(t, lc.Compromised())
	}
}
