package main

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/binary"
	"io"
	"math/big"
	"net"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeServer is enough of an EPP server for the tool: it greets, accepts
// one password, answers checks, and lets each drop name be created after
// it has been refused releaseAfter times.
type fakeServer struct {
	addr         string
	releaseAfter int
	mu           sync.Mutex
	tries        map[string]int
	taken        map[string]bool
	logins       int
	closeEvery   int // end a session after this many commands (0: never)
}

func selfSigned(t *testing.T) tls.Certificate {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	tmpl := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "localhost"},
		NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour), DNSNames: []string{"localhost"}}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	require.NoError(t, err)
	return tls.Certificate{Certificate: [][]byte{der}, PrivateKey: key}
}

func startFake(t *testing.T, f *fakeServer) *fakeServer {
	t.Helper()
	f.tries, f.taken = map[string]int{}, map[string]bool{}
	l, err := tls.Listen("tcp", "127.0.0.1:0", &tls.Config{Certificates: []tls.Certificate{selfSigned(t)}, ClientAuth: tls.RequireAnyClientCert, MinVersion: tls.VersionTLS12})
	require.NoError(t, err)
	t.Cleanup(func() { _ = l.Close() })
	f.addr = l.Addr().String()
	go func() {
		for {
			c, err := l.Accept()
			if err != nil {
				return
			}
			go f.serve(c)
		}
	}()
	return f
}

func writeFrame(w io.Writer, s string) {
	var hdr [4]byte
	binary.BigEndian.PutUint32(hdr[:], uint32(len(s)+4)) //nolint:gosec // G115: the fake's frames are a few KB
	_, _ = w.Write(hdr[:])
	_, _ = w.Write([]byte(s))
}

func readFrame(r io.Reader) (string, error) {
	var hdr [4]byte
	if _, err := io.ReadFull(r, hdr[:]); err != nil {
		return "", err
	}
	body := make([]byte, binary.BigEndian.Uint32(hdr[:])-4)
	_, err := io.ReadFull(r, body)
	return string(body), err
}

func respond(code int, msg, resData string) string {
	return `<epp xmlns="urn:ietf:params:xml:ns:epp-1.0"><response><result code="` + strconv.Itoa(code) + `"><msg>` + msg + `</msg></result>` + resData + `</response></epp>`
}

var (
	nameRE  = regexp.MustCompile(`<domain:name>([^<]+)</domain:name>`)
	loginRE = regexp.MustCompile(`<pw>([^<]+)</pw>`)
)

func (f *fakeServer) serve(c net.Conn) {
	defer c.Close()
	writeFrame(c, `<epp xmlns="urn:ietf:params:xml:ns:epp-1.0"><greeting/></epp>`)
	n := 0
	for {
		in, err := readFrame(c)
		if err != nil {
			return
		}
		n++
		switch {
		case strings.Contains(in, "<login>"):
			f.mu.Lock()
			f.logins++
			f.mu.Unlock()
			if m := loginRE.FindStringSubmatch(in); len(m) < 2 || m[1] != "secret-pw" {
				writeFrame(c, respond(2200, "Authentication error", ""))
				continue
			}
			writeFrame(c, respond(1000, "ok", ""))
		case strings.Contains(in, "<logout/>"):
			writeFrame(c, respond(1500, "bye", ""))
			return
		case strings.Contains(in, "<domain:check"):
			var cds bytes.Buffer
			for _, m := range nameRE.FindAllStringSubmatch(in, -1) {
				cds.WriteString(`<domain:cd><domain:name avail="1">` + m[1] + `</domain:name></domain:cd>`)
			}
			writeFrame(c, respond(1000, "ok", `<resData><domain:chkData xmlns:domain="urn:ietf:params:xml:ns:domain-1.0">`+cds.String()+`</domain:chkData></resData>`))
		case strings.Contains(in, "<domain:create"):
			name := nameRE.FindStringSubmatch(in)[1]
			f.mu.Lock()
			f.tries[name]++
			var code int
			switch {
			case f.taken[name]:
				code = 2302
			case f.releaseAfter > 0 && f.tries[name] <= f.releaseAfter:
				code = 2302
			default:
				f.taken[name], code = true, 1000
			}
			f.mu.Unlock()
			if code == 2302 {
				writeFrame(c, `<epp xmlns="urn:ietf:params:xml:ns:epp-1.0"><response><result code="2302"><msg>Object exists</msg><extValue><value><undef/></value><reason>failed-creates; scope=registrar; suspended-for-ms=0</reason></extValue></result></response></epp>`)
			} else {
				writeFrame(c, respond(code, "ok", ""))
			}
		default:
			writeFrame(c, respond(2000, "unknown", ""))
		}
		if f.closeEvery > 0 && n%f.closeEvery == 0 {
			writeFrame(c, respond(2502, "closing", ""))
			return
		}
	}
}

func testOptions(t *testing.T, f *fakeServer, scenario string) options {
	return options{
		Addr: f.addr, TLS: &tls.Config{Certificates: []tls.Certificate{selfSigned(t)}, InsecureSkipVerify: true}, //nolint:gosec // the fake's self-signed certificate
		Login: "registrar1", Password: "secret-pw", Scenario: scenario, Sessions: 3,
		Duration: 500 * time.Millisecond, Timeout: 5 * time.Second, Reconnect: true,
		CheckNames: 5, TLD: "test", Create: createSpec{AuthInfo: "x"},
	}
}

func TestCheckScenario(t *testing.T) {
	f := startFake(t, &fakeServer{})
	rep, err := run(context.Background(), testOptions(t, f, "check"))
	require.NoError(t, err)
	cr := rep.Commands["domain:check"]
	require.NotNil(t, cr)
	assert.Positive(t, cr.Count)
	assert.Equal(t, cr.Count, cr.Codes[1000])
	assert.Positive(t, cr.PerSecond)
	assert.GreaterOrEqual(t, cr.P99, cr.P50)
	assert.Equal(t, 3, rep.Commands["login"].Codes[1000])
	assert.Zero(t, rep.Errors)
}

// Sessions race for the names; each is won once, after the server has
// refused it while still registered, and the losers' 2302s are counted.
func TestDropScenario(t *testing.T) {
	f := startFake(t, &fakeServer{releaseAfter: 20})
	o := testOptions(t, f, "drop")
	o.Duration = 10 * time.Second
	o.Names = []string{"alpha.test", "bravo.test", "charlie.test"}
	rep, err := run(context.Background(), o)
	require.NoError(t, err)
	require.Len(t, rep.Names, 3)
	for _, n := range rep.Names {
		assert.True(t, n.Won, n.Name)
		assert.GreaterOrEqual(t, n.Attempts, 21, "%s: 20 refusals, then the create", n.Name)
		assert.Less(t, n.WonAfter, o.Duration)
	}
	cr := rep.Commands["domain:create"]
	assert.Equal(t, 3, cr.Codes[1000])
	assert.GreaterOrEqual(t, cr.Codes[2302], 60)
	assert.Equal(t, cr.Codes[2302], rep.Reasons["failed-creates"], "reasons are counted by kind")
	assert.Less(t, rep.Elapsed, o.Duration, "the run stops once every name is won")

	var out bytes.Buffer
	rep.print(&out)
	assert.Contains(t, out.String(), "names: 3 of 3 won")
}

// A session the server ends is logged in again.
func TestReconnect(t *testing.T) {
	f := startFake(t, &fakeServer{closeEvery: 3})
	o := testOptions(t, f, "check")
	o.Sessions = 1
	rep, err := run(context.Background(), o)
	require.NoError(t, err)
	assert.Positive(t, rep.Closes)
	assert.Greater(t, rep.Commands["login"].Codes[1000], 1)
}

func TestWrongPasswordStopsTheSession(t *testing.T) {
	f := startFake(t, &fakeServer{})
	o := testOptions(t, f, "check")
	o.Password, o.Reconnect = "wrong", false
	rep, err := run(context.Background(), o)
	require.NoError(t, err)
	assert.Equal(t, 3, rep.Commands["login"].Codes[2200])
	assert.Nil(t, rep.Commands["domain:check"])
}

func TestFrames(t *testing.T) {
	f := string(createFrame("a&b.test", createSpec{Registrant: "r1", Contacts: map[string]string{"tech": "t1", "admin": "a1"}, AuthInfo: "p<w"}, "TR-1"))
	assert.Contains(t, f, `<domain:name>a&amp;b.test</domain:name>`, "escaped")
	assert.Contains(t, f, `<domain:pw>p&lt;w</domain:pw>`)
	assert.Less(t, strings.Index(f, `type="admin"`), strings.Index(f, `type="tech"`), "contacts in schema order")
	assert.Contains(t, string(checkFrame([]string{"x.test", "y.test"}, "TR-2")), `<domain:name>y.test</domain:name>`)
	_, err := run(context.Background(), options{Scenario: "drop"})
	assert.ErrorContains(t, err, "needs names")
	_, err = run(context.Background(), options{Scenario: "nonsense"})
	assert.ErrorContains(t, err, "unknown scenario")
}
