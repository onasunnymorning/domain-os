package epp

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/pires/go-proxyproto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onasunnymorning/domain-os/internal/config"
)

func TestProxyProtocol(t *testing.T) {
	cfg := testConfig()
	cfg.Proxy.Enabled = true
	cfg.Proxy.TrustedCIDRs = []string{"127.0.0.1/32"}
	ts := startServer(t, cfg)

	t.Run("the client address comes from the PROXY header", func(t *testing.T) {
		conn := dialRaw(t, ts.addr)
		header := proxyproto.HeaderProxyFromAddrs(2,
			&net.TCPAddr{IP: net.ParseIP("203.0.113.7"), Port: 40000},
			&net.TCPAddr{IP: net.ParseIP("192.0.2.1"), Port: 700})
		_, err := header.WriteTo(conn)
		require.NoError(t, err)

		c := tlsOver(t, conn)
		greeting := c.read()
		require.NotNil(t, greeting.FindElement("//greeting"))

		sessions := ts.sessions.all()
		require.Len(t, sessions, 1)
		assert.Equal(t, "203.0.113.7", sessions[0].clientIP)
	})

	t.Run("a connection without a PROXY header is refused", func(t *testing.T) {
		conn := dialRaw(t, ts.addr)
		c := tls.Client(conn, clientTLSConfig(t))
		require.NoError(t, c.SetDeadline(time.Now().Add(5*time.Second)))
		assert.Error(t, c.Handshake())
	})
}

func TestProxyProtocolRefusesUntrustedPeers(t *testing.T) {
	cfg := testConfig()
	cfg.Proxy.Enabled = true
	cfg.Proxy.TrustedCIDRs = []string{"10.0.0.0/8"} // not loopback
	ts := startServer(t, cfg)

	conn := dialRaw(t, ts.addr)
	assertClosedWithoutGreeting(t, conn)
	assert.Equal(t, 0, ts.sessions.len())
}

func TestServerCertificateReload(t *testing.T) {
	dir := t.TempDir()
	certPath, keyPath := filepath.Join(dir, "epp.crt"), filepath.Join(dir, "epp.key")
	first := writeKeyPair(t, certPath, keyPath, time.Now())

	cfg := testConfig()
	cfg.TLS = config.EPPTLS{CertPath: certPath, KeyPath: keyPath, ReloadInterval: 20 * time.Millisecond}
	ts := startServer(t, cfg)

	c := dial(t, ts.addr)
	assert.True(t, bytes.Equal(first, peerCert(c)), "the server presents the certificate on disk")

	t.Run("a broken file keeps the current certificate", func(t *testing.T) {
		require.NoError(t, os.WriteFile(certPath, []byte("not a certificate"), 0o600))
		touch(t, certPath, time.Now().Add(time.Second))
		require.Eventually(t, func() bool { return counterValue(t, ts.metrics.certReloadErrors) >= 1 }, 2*time.Second, 10*time.Millisecond)
		assert.True(t, bytes.Equal(first, peerCert(dial(t, ts.addr))))
	})

	t.Run("a renewed certificate is picked up without dropping live sessions", func(t *testing.T) {
		second := writeKeyPair(t, certPath, keyPath, time.Now().Add(2*time.Second))
		require.Eventually(t, func() bool {
			return bytes.Equal(second, peerCert(dial(t, ts.addr)))
		}, 2*time.Second, 20*time.Millisecond, "new handshakes get the new certificate")

		c.login() // the session opened before the reload is unaffected
	})
}

// tlsOver starts a client TLS session on an existing connection.
func tlsOver(t *testing.T, conn net.Conn) *client {
	t.Helper()
	c := tls.Client(conn, clientTLSConfig(t))
	require.NoError(t, c.SetDeadline(time.Now().Add(5*time.Second)))
	return &client{t: t, conn: c}
}

func clientTLSConfig(t *testing.T) *tls.Config {
	t.Helper()
	cert, err := SelfSignedCertificate()
	require.NoError(t, err)
	return &tls.Config{
		Certificates:       []tls.Certificate{cert},
		InsecureSkipVerify: true, // #nosec G402 -- test-only: the server presents a certificate generated in this test
		MinVersion:         tls.VersionTLS12,
	}
}

func peerCert(c *client) []byte {
	return c.conn.ConnectionState().PeerCertificates[0].Raw
}

// writeKeyPair writes a fresh self-signed certificate and key as PEM, sets
// both files' modification time to mtime, and returns the certificate's DER.
func writeKeyPair(t *testing.T, certPath, keyPath string, mtime time.Time) []byte {
	t.Helper()
	cert, err := SelfSignedCertificate()
	require.NoError(t, err)
	key, err := x509.MarshalPKCS8PrivateKey(cert.PrivateKey)
	require.NoError(t, err)
	// Key first: a reload that sees only the new certificate fails to pair
	// and is retried on the next tick.
	require.NoError(t, os.WriteFile(keyPath, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: key}), 0o600))
	require.NoError(t, os.WriteFile(certPath, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: cert.Certificate[0]}), 0o600))
	touch(t, keyPath, mtime)
	touch(t, certPath, mtime)
	return cert.Certificate[0]
}

func touch(t *testing.T, path string, mtime time.Time) {
	t.Helper()
	require.NoError(t, os.Chtimes(path, mtime, mtime))
}
