package epp

import (
	"context"
	"crypto/x509"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
)

func TestFailedLoginsCloseTheSession(t *testing.T) {
	ts := startServer(t, testConfig())
	c := dial(t, ts.addr)
	bad := loginAsXML(testLogin, "not-the-pw-1", "", "BAD")

	requireCode(t, c.roundTrip(bad), "2200")
	requireCode(t, c.roundTrip(bad), "2200")
	requireCode(t, c.roundTrip(bad), "2501") // the third attempt, testConfig's limit
	c.requireClosed()

	assert.Equal(t, 3.0, ts.counter(t, "logins", loginFailure))
	assert.Equal(t, 3.0, ts.counter(t, "loginFailures", "wrong_password"))
}

func TestEveryRefusalReadsTheSame(t *testing.T) {
	ts := startServer(t, testConfig())
	unknown := dial(t, ts.addr).roundTrip(loginAsXML("nobody-here", testPassword, "", "REF-1"))
	wrongPW := dial(t, ts.addr).roundTrip(loginAsXML(testLogin, "not-the-pw-1", "", "REF-2"))
	requireCode(t, unknown, "2200")
	requireCode(t, wrongPW, "2200")
	assert.Equal(t, text(unknown, "//result/msg"), text(wrongPW, "//result/msg"), "a client cannot tell an unknown login from a wrong password")
	assert.Equal(t, 1.0, ts.counter(t, "loginFailures", "unknown_login"))
}

func TestLockoutSpansSessions(t *testing.T) {
	ts := startServer(t, testConfig())
	bad := loginAsXML(testLogin, "not-the-pw-1", "", "BAD")

	// The test lockout locks after 5 failures; a session allows 3.
	first := dial(t, ts.addr)
	for i := 0; i < 3; i++ {
		first.roundTrip(bad)
	}
	first.requireClosed()
	second := dial(t, ts.addr)
	second.roundTrip(bad)
	second.roundTrip(bad)

	requireCode(t, second.roundTrip(loginXML("GOOD")), "2501")
	second.requireClosed()
	assert.Equal(t, 1.0, ts.counter(t, "logins", loginLocked))
}

func TestNewPasswordReplacesTheOldOne(t *testing.T) {
	ts := startServer(t, testConfig())
	c := dial(t, ts.addr)
	requireCode(t, c.roundTrip(loginAsXML(testLogin, testPassword, "rotated-pw-02", "NEW")), "1000")

	requireCode(t, dial(t, ts.addr).roundTrip(loginXML("OLD")), "2200")
	requireCode(t, dial(t, ts.addr).roundTrip(loginAsXML(testLogin, "rotated-pw-02", "", "NEW")), "1000")
}

func TestInvalidNewPasswordLeavesTheSessionLoggedOut(t *testing.T) {
	ts := startServer(t, testConfig())
	c := dial(t, ts.addr)
	// Within the schema's 6 to 16 characters, but not printable ASCII.
	requireCode(t, c.roundTrip(loginAsXML(testLogin, testPassword, "pässwörd-01", "NEW")), "2306")
	requireCode(t, c.roundTrip(domainCheckXML("CHK")), "2002") // still not logged in
	// Outside the schema, it is a syntax error before the password rules.
	requireCode(t, c.roundTrip(loginAsXML(testLogin, testPassword, "short", "NEW")), "2001")
	c.login()
}

func TestCompromiseEndsLiveSessions(t *testing.T) {
	creds := newTestCredentials(t)
	ts := startServerWith(t, testConfig(), listenLoopback(t), creds)

	victim := dial(t, ts.addr)
	victim.login()
	bystander := dial(t, ts.addr)
	bystander.login()

	// A normal retirement leaves open sessions alone.
	pin := creds.access(testRegistrar).CertificatePins[0]
	network := creds.access(testRegistrar).AllowedNetworks[0]
	creds.update(testRegistrar, func(a *entities.EPPRegistrarAccess) error {
		_, err := a.RetireCredential(entities.EPPCredentialRef{Kind: entities.EPPCredentialAllowedNetwork, ID: network.ID}, entities.EPPRetireNormal, ptr(time.Now()), "test", time.Now())
		return err
	})
	creds.refresh()
	requireCode(t, victim.roundTrip(domainCheckXML("STILL-OK")), "1000")

	// A compromise ends every session that used the credential.
	creds.update(testRegistrar, func(a *entities.EPPRegistrarAccess) error {
		_, err := a.RetireCredential(entities.EPPCredentialRef{Kind: entities.EPPCredentialCertificatePin, ID: pin.ID}, entities.EPPRetireCompromise, nil, "test", time.Now())
		return err
	})
	creds.refresh()
	victim.requireClosed()
	bystander.requireClosed()
	assert.Equal(t, 2.0, counterValue(t, ts.metrics.sessionsEnded.WithLabelValues(endedCompromised)))
}

func TestCredentialStoreOutage(t *testing.T) {
	creds := newTestCredentials(t)
	ts := startServerWith(t, testConfig(), listenLoopback(t), creds)
	c := dial(t, ts.addr)

	creds.store.Fail(errors.New("connection refused"))
	requireCode(t, c.roundTrip(loginXML("DOWN")), "2400")
	assert.Equal(t, 1.0, ts.counter(t, "logins", loginError))

	creds.store.Fail(nil)
	c.login() // an outage is not a failed attempt, and the session can retry
}

func TestNotReadyUntilTheAllowlistLoads(t *testing.T) {
	creds := newTestCredentials(t)
	creds.store.Fail(errors.New("database starting"))
	srv, err := NewServer(creds.options(testConfig(), func() string { return "x" }))
	require.NoError(t, err)
	creds.server = srv
	l := listenLoopback(t)
	done := make(chan error, 1)
	go func() { done <- srv.Serve(l) }()
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = srv.Shutdown(ctx)
		<-done
	})

	require.Eventually(t, srv.serving.Load, 2*time.Second, 5*time.Millisecond)
	assert.False(t, srv.Ready(), "no allowlist yet")
	assertClosedWithoutGreeting(t, dialRaw(t, l.Addr().String()))

	creds.store.Fail(nil)
	creds.refresh()
	assert.True(t, srv.Ready())
	dial(t, l.Addr().String())
}

func TestLoginRecordsWhoTheSessionIs(t *testing.T) {
	ts := startServer(t, testConfig())
	c := dial(t, ts.addr)
	c.login()

	sessions := ts.sessions.all()
	require.Len(t, sessions, 1)
	assert.Equal(t, testRegistrar, sessions[0].registrar())
	assert.Equal(t, testLogin, sessions[0].loggedInAs())

	leaf, err := x509.ParseCertificate(testClientCertificate(t).Certificate[0])
	require.NoError(t, err)
	assert.Equal(t, float64(leaf.NotAfter.Unix()),
		gauge(t, ts.metrics.clientCertNotAfter.WithLabelValues(testRegistrar, testLogin)))
}

func TestCompromisingAReplacedPasswordEndsItsSessions(t *testing.T) {
	creds := newTestCredentials(t)
	ts := startServerWith(t, testConfig(), listenLoopback(t), creds)
	old := creds.access(testRegistrar).Logins[0].Passwords[0]

	c := dial(t, ts.addr)
	requireCode(t, c.roundTrip(loginAsXML(testLogin, testPassword, "rotated-pw-02", "NEW")), "1000")

	// The old password was retired by <newPW>, then turns out to have leaked.
	creds.update(testRegistrar, func(a *entities.EPPRegistrarAccess) error {
		_, err := a.RetireCredential(entities.EPPCredentialRef{Kind: entities.EPPCredentialPassword, LoginID: testLogin, ID: old.ID}, entities.EPPRetireCompromise, nil, "test", time.Now())
		return err
	})
	creds.refresh()
	c.requireClosed()
}
