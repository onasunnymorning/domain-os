package epp

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSessionLifecycle(t *testing.T) {
	ts := startServer(t, testConfig())
	c := dial(t, ts.addr)

	login := c.roundTrip(loginXML("LOGIN-1"))
	requireCode(t, login, "1000")
	assert.Equal(t, "LOGIN-1", text(login, "//trID/clTRID"), "clTRID is echoed")

	check := c.roundTrip(domainCheckXML("CHECK-1"))
	requireCode(t, check, "1000")
	assert.NotNil(t, check.FindElement("//resData/chkData"))
	assert.Equal(t, "CHECK-1", text(check, "//trID/clTRID"))
	assert.NotEqual(t, text(login, "//trID/svTRID"), text(check, "//trID/svTRID"), "every response gets its own svTRID")

	requireCode(t, c.roundTrip(cmdXML("<logout/>", "LOGOUT-1")), "1500")
	c.requireClosed()

	assert.Equal(t, 1.0, ts.counter(t, "logins", loginSuccess))
	assert.Equal(t, 1.0, ts.counter(t, "commands", testRegistrar, "domain:check", "1000"))
	assert.Eventually(t, func() bool { return ts.sessions.len() == 0 }, 2*time.Second, 10*time.Millisecond, "the session is deregistered on close")
	assert.Equal(t, 0.0, gauge(t, ts.metrics.sessionsActive))
}

func TestResponseOmitsClTRIDWhenNoneWasSent(t *testing.T) {
	ts := startServer(t, testConfig())
	c := dial(t, ts.addr)
	resp := c.roundTrip(loginXML(""))
	requireCode(t, resp, "1000")
	assert.Nil(t, resp.FindElement("//trID/clTRID"))
	assert.NotEmpty(t, text(resp, "//trID/svTRID"))
}

func TestHelloAnswersWithGreeting(t *testing.T) {
	ts := startServer(t, testConfig())
	c := dial(t, ts.addr)
	resp := c.roundTrip(`<?xml version="1.0" encoding="UTF-8"?><epp xmlns="urn:ietf:params:xml:ns:epp-1.0"><hello/></epp>`)
	require.NotNil(t, resp.FindElement("//greeting/svID"))
}

func TestStateMachine(t *testing.T) {
	ts := startServer(t, testConfig())

	t.Run("a command before login is a use error and the session stays open", func(t *testing.T) {
		c := dial(t, ts.addr)
		requireCode(t, c.roundTrip(domainCheckXML("CHK-1")), "2002")
		requireCode(t, c.roundTrip(cmdXML("<logout/>", "OUT-1")), "2002")
		c.login()
	})

	t.Run("a second login is a use error", func(t *testing.T) {
		c := dial(t, ts.addr)
		c.login()
		requireCode(t, c.roundTrip(loginXML("ABC-2")), "2002")
		requireCode(t, c.roundTrip(domainCheckXML("CHK-2")), "1000")
	})
}

func TestUnboundCommands(t *testing.T) {
	ts := startServer(t, testConfig())
	c := dial(t, ts.addr)
	c.login()

	renew := cmdXML(`<renew><domain:renew xmlns:domain="urn:ietf:params:xml:ns:domain-1.0"><domain:name>example.test</domain:name><domain:curExpDate>2030-01-01</domain:curExpDate></domain:renew></renew>`, "REN-1")
	requireCode(t, c.roundTrip(renew), "2101")
	requireCode(t, c.roundTrip(cmdXML("<frobnicate/>", "FRB-1")), "2000")
	requireCode(t, c.roundTrip(domainCheckXML("CHK-3")), "1000")
	assert.Equal(t, 1.0, ts.counter(t, "commands", testRegistrar, "domain:renew", "2101"))
}

func TestFramesRejectedBeforeDispatch(t *testing.T) {
	ts := startServer(t, testConfig())
	c := dial(t, ts.addr)

	cases := []struct {
		name, reason, xml string
	}{
		{"malformed XML", rejectSyntax, `<epp xmlns="urn:ietf:params:xml:ns:epp-1.0"><command>`},
		{"not EPP", rejectSyntax, `<html><body/></html>`},
		{"DOCTYPE", rejectDoctype, `<?xml version="1.0"?><!DOCTYPE epp [<!ENTITY x "y">]><epp xmlns="urn:ietf:params:xml:ns:epp-1.0"><hello/></epp>`},
		{"too deep", rejectDepth, `<epp xmlns="urn:ietf:params:xml:ns:epp-1.0">` + strings.Repeat("<a>", 40) + strings.Repeat("</a>", 40) + `</epp>`},
		{"too many elements", rejectElements, `<epp xmlns="urn:ietf:params:xml:ns:epp-1.0"><command>` + strings.Repeat("<x/>", 2100) + `</command></epp>`},
		{"clTRID too long", rejectClTRID, cmdXML("<logout/>", strings.Repeat("x", 65))},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			before := ts.counter(t, "framesRejected", tc.reason)
			requireCode(t, c.roundTrip(tc.xml), "2001")
			assert.Equal(t, before+1, ts.counter(t, "framesRejected", tc.reason))
		})
	}

	c.login() // every rejection left the session usable
}

func TestOversizeFrameClosesTheSession(t *testing.T) {
	cfg := testConfig()
	cfg.Limits.MaxFrameBytes = 4096
	ts := startServer(t, cfg)
	c := dial(t, ts.addr)
	// The server closes as soon as it has read the header, so the rest of the
	// write may hit a closed connection (broken pipe, reset). That is the
	// behaviour under test, not a failure; the close is what is asserted.
	_ = c.trySend(cmdXML("<logout/>", "BIG-"+strings.Repeat("x", 5000)))
	c.requireClosed()
}

func TestLoginTimeout(t *testing.T) {
	cfg := testConfig()
	cfg.Timeouts.Login = 200 * time.Millisecond
	ts := startServer(t, cfg)

	idle := dial(t, ts.addr)
	idle.requireClosed()
	assert.Equal(t, 1.0, ts.counter(t, "connections", outcomeLoginTimeout))

	ok := dial(t, ts.addr)
	ok.login()
	time.Sleep(400 * time.Millisecond) // past the login timeout
	requireCode(t, ok.roundTrip(domainCheckXML("CHK-4")), "1000")
}

func TestNodeConnectionCap(t *testing.T) {
	cfg := testConfig()
	cfg.Limits.MaxConnections = 2
	cfg.Limits.MaxHandshakes = 2
	ts := startServer(t, cfg)

	first := dial(t, ts.addr)
	second := dial(t, ts.addr)

	third := dialRaw(t, ts.addr)
	assertClosedWithoutGreeting(t, third)
	assert.Equal(t, 1.0, ts.counter(t, "connections", outcomeNodeCap))

	first.login()
	second.login()
}

func TestHandshakeTimeout(t *testing.T) {
	cfg := testConfig()
	cfg.Timeouts.Handshake = 200 * time.Millisecond
	ts := startServer(t, cfg)

	start := time.Now()
	silent := dialRaw(t, ts.addr) // TCP only: never starts TLS
	assertClosedWithoutGreeting(t, silent)
	assert.Less(t, time.Since(start), cfg.Timeouts.Read, "closed at the handshake timeout, not the read timeout")
	assert.Eventually(t, func() bool { return ts.counter(t, "connections", outcomeHandshakeFailed) == 1 }, time.Second, 10*time.Millisecond)
	assert.Equal(t, 0.0, gauge(t, ts.metrics.handshakesActive), "the handshake slot is released")
}

func TestDrain(t *testing.T) {
	ts := startServer(t, testConfig())
	c := dial(t, ts.addr)
	c.login()

	code, _ := ts.getOps(t, "/readyz")
	assert.Equal(t, 200, code)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	require.NoError(t, ts.Shutdown(ctx), "an idle session drains without the timeout")

	c.requireClosed()
	code, _ = ts.getOps(t, "/readyz")
	assert.Equal(t, 503, code, "not ready while draining")
	code, _ = ts.getOps(t, "/healthz")
	assert.Equal(t, 200, code, "still alive while draining")
	select {
	case err := <-ts.done:
		require.NoError(t, err)
		ts.done <- nil // for the cleanup
	case <-time.After(2 * time.Second):
		t.Fatal("Serve did not return after drain")
	}
}

func TestMetricsEndpoint(t *testing.T) {
	ts := startServer(t, testConfig())
	c := dial(t, ts.addr)
	c.login()

	code, body := ts.getOps(t, "/metrics")
	require.Equal(t, 200, code)
	for _, series := range []string{
		`epp_sessions_active 1`,
		`epp_connections_total{outcome="accepted"} 1`,
		`epp_logins_total{result="success"} 1`,
		`epp_commands_total{clid="registrar1",code="1000",command="login"} 1`,
		`epp_command_duration_seconds_bucket`,
		`epp_frame_bytes_bucket`,
		`epp_server_cert_not_after_timestamp_seconds`,
		`go_goroutines`,
	} {
		assert.Contains(t, body, series)
	}
}
