package epp

import (
	"context"
	"crypto/tls"
	"encoding/binary"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/beevik/etree"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/require"

	"github.com/onasunnymorning/domain-os/internal/config"
)

// testConfig is a configuration for in-process tests: a self-signed
// certificate, the production limits, and timeouts short enough to test.
func testConfig() config.EPP {
	return config.EPP{
		TLS: config.EPPTLS{SelfSigned: true, ReloadInterval: time.Minute},
		Limits: config.EPPLimits{
			MaxConnections: 100,
			MaxHandshakes:  100,
			MaxFrameBytes:  64 * 1024,
			XMLMaxDepth:    32,
			XMLMaxElements: 2000,
		},
		Timeouts: config.EPPTimeouts{
			Handshake:  2 * time.Second,
			Login:      5 * time.Second,
			Idle:       5 * time.Second,
			Read:       2 * time.Second,
			Write:      2 * time.Second,
			MaxSession: time.Minute,
			Drain:      2 * time.Second,
		},
		Auth: config.EPPAuth{
			RequireAllowlist:    true,
			RefreshInterval:     time.Minute,
			MaxConcurrentVerify: 4,
			MaxLoginAttempts:    3,
		},
		Sessions: config.EPPSessions{
			NodeID:    "node-a",
			Lease:     10 * time.Second,
			Heartbeat: time.Second,
			PerSource: 20,
		},
		Commands: config.EPPCommands{CreatesInFlight: 10, CreateQueueWait: 5 * time.Second},
	}
}

// testServer is a running server on a loopback port.
type testServer struct {
	*Server
	addr string
	ops  *httptest.Server
	done chan error
}

// startServer runs a server built from cfg on an IPv4 loopback port until
// the test ends.
func startServer(t testing.TB, cfg config.EPP) *testServer {
	t.Helper()
	return startServerOn(t, cfg, listenLoopback(t))
}

func listenLoopback(t testing.TB) net.Listener {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	return l
}

// testOptions are server options with the default test credentials, for
// tests that build a server without running it.
func testOptions(t testing.TB, cfg config.EPP) Options {
	t.Helper()
	return newTestCredentials(t).options(cfg, func() string { return "x" })
}

// startServerOn runs a server built from cfg on l until the test ends, with
// the default test credentials (newTestCredentials).
func startServerOn(t testing.TB, cfg config.EPP, l net.Listener) *testServer {
	t.Helper()
	return startServerWith(t, cfg, l, newTestCredentials(t))
}

// startServerWith runs a server that authenticates against creds.
func startServerWith(t testing.TB, cfg config.EPP, l net.Listener, creds *testCredentials) *testServer {
	t.Helper()
	var n atomic.Int64
	srv, err := NewServer(creds.options(cfg, func() string { return fmt.Sprintf("%s-%d", cfg.Sessions.NodeID, n.Add(1)) }))
	require.NoError(t, err)
	creds.server = srv

	ts := &testServer{Server: srv, addr: l.Addr().String(), ops: httptest.NewServer(srv.OpsHandler()), done: make(chan error, 1)}
	go func() { ts.done <- srv.Serve(l) }()
	require.Eventually(t, srv.Ready, 2*time.Second, 5*time.Millisecond, "server becomes ready")

	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = srv.Shutdown(ctx)
		<-ts.done
		ts.ops.Close()
	})
	return ts
}

// counter reads a labelled counter's current value.
func (ts *testServer) counter(t testing.TB, name string, labels ...string) float64 {
	t.Helper()
	vecs := map[string]*prometheus.CounterVec{
		"connections":    ts.metrics.connections,
		"commands":       ts.metrics.commands,
		"framesRejected": ts.metrics.framesRejected,
		"logins":         ts.metrics.logins,
		"loginFailures":  ts.metrics.loginFailures,
	}
	vec, ok := vecs[name]
	require.True(t, ok, "unknown counter %q", name)
	var m dto.Metric
	require.NoError(t, vec.WithLabelValues(labels...).Write(&m))
	return m.GetCounter().GetValue()
}

// counterOf reads one series of a counter vector.
func counterOf(t testing.TB, vec *prometheus.CounterVec, labels ...string) float64 {
	t.Helper()
	var m dto.Metric
	require.NoError(t, vec.WithLabelValues(labels...).Write(&m))
	return m.GetCounter().GetValue()
}

// counterValue reads an unlabelled counter's current value.
func counterValue(t testing.TB, c prometheus.Counter) float64 {
	t.Helper()
	var m dto.Metric
	require.NoError(t, c.Write(&m))
	return m.GetCounter().GetValue()
}

// gauge reads a gauge's current value.
func gauge(t testing.TB, g prometheus.Gauge) float64 {
	t.Helper()
	var m dto.Metric
	require.NoError(t, g.Write(&m))
	return m.GetGauge().GetValue()
}

// getOps fetches an ops endpoint and returns its status and body.
func (ts *testServer) getOps(t testing.TB, path string) (int, string) {
	t.Helper()
	resp, err := http.Get(ts.ops.URL + path) // #nosec G107 -- test-only: a local httptest server
	require.NoError(t, err)
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	return resp.StatusCode, string(body)
}

// client is a minimal EPP client for tests.
type client struct {
	t    testing.TB
	conn *tls.Conn
}

// dial connects and reads the greeting.
func dial(t testing.TB, addr string) *client {
	t.Helper()
	c := dialNoGreeting(t, addr)
	greeting := c.read()
	require.NotNil(t, greeting.FindElement("//greeting"), "the server greets on connect")
	return c
}

func dialNoGreeting(t testing.TB, addr string) *client {
	t.Helper()
	conn, err := tls.Dial("tcp", addr, clientTLSConfig(t))
	require.NoError(t, err)
	require.NoError(t, conn.SetDeadline(time.Now().Add(10*time.Second)))
	t.Cleanup(func() { _ = conn.Close() })
	return &client{t: t, conn: conn}
}

// send writes one RFC 5734 data unit: a 4-byte big-endian length that
// counts itself, then the XML.
func (c *client) send(xml string) {
	c.t.Helper()
	require.NoError(c.t, c.trySend(xml))
}

// trySend writes a frame and returns the write error, for frames the server
// may close the connection on before it has read them in full.
func (c *client) trySend(xml string) error {
	frame := make([]byte, 4+len(xml))
	binary.BigEndian.PutUint32(frame, uint32(len(frame))) // #nosec G115 -- test frames are far below 4 GiB
	copy(frame[4:], xml)
	_, err := c.conn.Write(frame)
	return err
}

func (c *client) read() *etree.Document {
	c.t.Helper()
	header := make([]byte, 4)
	_, err := io.ReadFull(c.conn, header)
	require.NoError(c.t, err)
	body := make([]byte, binary.BigEndian.Uint32(header)-4)
	_, err = io.ReadFull(c.conn, body)
	require.NoError(c.t, err)
	requireSchemaValid(c.t, body) // every frame the server sends, in every test
	doc := etree.NewDocument()
	require.NoError(c.t, doc.ReadFromBytes(body))
	return doc
}

// roundTrip sends xml and returns the response.
func (c *client) roundTrip(xml string) *etree.Document {
	c.t.Helper()
	c.send(xml)
	return c.read()
}

// login logs in and requires success.
func (c *client) login() {
	c.t.Helper()
	requireCode(c.t, c.roundTrip(loginXML("ABC-1")), "1000")
}

// requireClosed asserts the server has closed the connection.
func (c *client) requireClosed() {
	c.t.Helper()
	require.NoError(c.t, c.conn.SetReadDeadline(time.Now().Add(5*time.Second)))
	_, err := c.conn.Read(make([]byte, 1))
	require.Error(c.t, err, "the server closed the connection")
	require.NotContains(c.t, err.Error(), "timeout", "closed by the server, not by our read deadline")
}

func requireCode(t testing.TB, doc *etree.Document, code string) {
	t.Helper()
	result := doc.FindElement("//response/result")
	require.NotNil(t, result, "response has a result")
	require.Equal(t, code, result.SelectAttrValue("code", ""), "result code; msg: %s", result.FindElement("msg").Text())
}

func text(doc *etree.Document, path string) string {
	if el := doc.FindElement(path); el != nil {
		return el.Text()
	}
	return ""
}

func loginXML(clTRID string) string {
	return loginAsXML(testLogin, testPassword, "", clTRID)
}

// loginAsXML is a <login> for clID with pw and, unless empty, newPW.
func loginAsXML(clID, pw, newPW, clTRID string) string {
	var np string
	if newPW != "" {
		np = "<newPW>" + newPW + "</newPW>"
	}
	return cmdXML(`<login><clID>`+clID+`</clID><pw>`+pw+`</pw>`+np+`
<options><version>1.0</version><lang>en</lang></options>
<svcs><objURI>urn:ietf:params:xml:ns:domain-1.0</objURI></svcs></login>`, clTRID)
}

func domainCheckXML(clTRID string) string {
	return cmdXML(`<check><domain:check xmlns:domain="urn:ietf:params:xml:ns:domain-1.0">
<domain:name>example.test</domain:name></domain:check></check>`, clTRID)
}

func cmdXML(inner, clTRID string) string {
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="no"?>
<epp xmlns="urn:ietf:params:xml:ns:epp-1.0"><command>`)
	b.WriteString(inner)
	if clTRID != "" {
		b.WriteString("<clTRID>" + clTRID + "</clTRID>")
	}
	b.WriteString("</command></epp>")
	return b.String()
}

// dialRaw opens a plain TCP connection: no TLS, no greeting.
func dialRaw(t testing.TB, addr string) net.Conn {
	t.Helper()
	conn, err := net.Dial("tcp", addr)
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })
	return conn
}

// assertClosedWithoutGreeting asserts the server closes conn without
// sending anything.
func assertClosedWithoutGreeting(t testing.TB, conn net.Conn) {
	t.Helper()
	require.NoError(t, conn.SetReadDeadline(time.Now().Add(5*time.Second)))
	n, err := conn.Read(make([]byte, 1))
	require.Error(t, err, "the server closed the connection")
	require.Zero(t, n, "nothing was sent before closing")
	require.NotContains(t, err.Error(), "timeout", "closed by the server, not by our read deadline")
}

func discardLogger() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }
