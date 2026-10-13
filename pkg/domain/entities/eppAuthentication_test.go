package entities

import (
	"crypto/x509"
	"encoding/pem"
	"net/netip"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type authFixture struct {
	access  *EPPRegistrarAccess
	cert    *x509.Certificate
	pw      *EPPPassword
	pin     *EPPCertificatePin
	network *EPPAllowedNetwork
}

func parsePEM(t *testing.T, b []byte) *x509.Certificate {
	t.Helper()
	block, _ := pem.Decode(b)
	c, err := x509.ParseCertificate(block.Bytes)
	require.NoError(t, err)
	return c
}

func newAuthFixture(t *testing.T) authFixture {
	t.Helper()
	a := newAccess(t)
	certPEM := ecCert(t, eppNow.Add(-time.Hour), eppNow.Add(24*time.Hour))
	pin, err := a.AddCertificatePin(certPEM, nil, "tester", eppNow)
	require.NoError(t, err)
	pw, err := a.AddPassword("rar-001", "first-pw-1", plainHasher{}, "tester", eppNow)
	require.NoError(t, err)
	n, err := a.AddAllowedNetwork("192.0.2.0/24", "", "tester", eppNow)
	require.NoError(t, err)
	return authFixture{access: a, cert: parsePEM(t, certPEM), pw: pw, pin: pin, network: n}
}

func (f authFixture) attempt() EPPLoginAttempt {
	return EPPLoginAttempt{LoginID: "rar-001", Source: netip.MustParseAddr("192.0.2.7"), Certificate: f.cert, CheckSource: true}
}

func TestEPPAdmitAndVerify(t *testing.T) {
	f := newAuthFixture(t)
	adm, err := f.access.Admit(f.attempt(), eppNow)
	require.NoError(t, err)
	require.Equal(t, f.pin, adm.Pin)
	require.Equal(t, f.network, adm.Network)

	pw, err := adm.VerifyPassword("first-pw-1", plainHasher{}, eppNow)
	require.NoError(t, err)
	require.Equal(t, f.pw, pw)
	_, err = adm.VerifyPassword("wrong-pw-1", plainHasher{}, eppNow)
	require.ErrorIs(t, err, ErrEPPPasswordMismatch)

	uses := adm.Uses(pw.ID, "192.0.2.7", eppNow)
	require.Len(t, uses, 3)
}

func TestEPPAdmitRefusals(t *testing.T) {
	other := ecCert(t, eppNow.Add(-time.Hour), eppNow.Add(time.Hour))
	cases := map[string]struct {
		change func(f *authFixture, at *EPPLoginAttempt)
		want   error
	}{
		"unknown login":             {func(_ *authFixture, at *EPPLoginAttempt) { at.LoginID = "nobody" }, ErrEPPUnknownLogin},
		"disabled login":            {func(f *authFixture, _ *EPPLoginAttempt) { f.access.Logins[0].Enabled = false }, ErrEPPLoginDisabled},
		"source outside networks":   {func(_ *authFixture, at *EPPLoginAttempt) { at.Source = netip.MustParseAddr("198.51.100.1") }, ErrEPPSourceNotAllowed},
		"no certificate":            {func(_ *authFixture, at *EPPLoginAttempt) { at.Certificate = nil }, ErrEPPCertificateMissing},
		"certificate not pinned":    {func(_ *authFixture, at *EPPLoginAttempt) { at.Certificate = parsePEM(t, other) }, ErrEPPCertificateNotPinned},
		"pin restricted elsewhere":  {func(f *authFixture, _ *EPPLoginAttempt) { f.pin.Logins = []ClIDType{"rar-001-ro"} }, ErrEPPCertificateNotPinned},
		"certificate expired":       {func(f *authFixture, _ *EPPLoginAttempt) { f.cert.NotAfter = eppNow.Add(-time.Second) }, ErrEPPCertificateOutOfDate},
		"certificate not yet valid": {func(f *authFixture, _ *EPPLoginAttempt) { f.cert.NotBefore = eppNow.Add(time.Second) }, ErrEPPCertificateOutOfDate},
		"pin retired":               {func(f *authFixture, _ *EPPLoginAttempt) { r := eppNow; f.pin.RetireAt = &r }, ErrEPPCertificateNotPinned},
		"network retired":           {func(f *authFixture, _ *EPPLoginAttempt) { r := eppNow; f.network.RetireAt = &r }, ErrEPPSourceNotAllowed},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			f := newAuthFixture(t)
			at := f.attempt()
			tc.change(&f, &at)
			_, err := f.access.Admit(at, eppNow)
			require.ErrorIs(t, err, tc.want)
			require.ErrorIs(t, err, ErrEPPAuthentication)
		})
	}
}

func TestEPPAdmitWithoutSourceCheck(t *testing.T) {
	f := newAuthFixture(t)
	at := f.attempt()
	at.Source = netip.MustParseAddr("198.51.100.1")
	at.CheckSource = false
	adm, err := f.access.Admit(at, eppNow)
	require.NoError(t, err)
	require.Nil(t, adm.Network)
	require.Len(t, adm.Uses(f.pw.ID, "198.51.100.1", eppNow), 2)
}

func TestEPPAdmitUnmapsIPv4InIPv6(t *testing.T) {
	f := newAuthFixture(t)
	at := f.attempt()
	at.Source = netip.MustParseAddr("::ffff:192.0.2.7")
	_, err := f.access.Admit(at, eppNow)
	require.NoError(t, err)
}

func TestEPPVerifySkipsRetiredPasswords(t *testing.T) {
	f := newAuthFixture(t)
	r := eppNow
	f.pw.RetireAt = &r
	adm, err := f.access.Admit(f.attempt(), eppNow)
	require.NoError(t, err)
	_, err = adm.VerifyPassword("first-pw-1", plainHasher{}, eppNow)
	require.ErrorIs(t, err, ErrEPPPasswordMismatch)
}
