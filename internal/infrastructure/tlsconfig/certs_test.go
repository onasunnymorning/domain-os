package tlsconfig

import (
	"crypto/x509"
	"encoding/pem"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func writePair(t *testing.T, commonName string) (certPath, keyPath string) {
	t.Helper()
	cert, err := SelfSigned(commonName)
	require.NoError(t, err)
	key, err := x509.MarshalPKCS8PrivateKey(cert.PrivateKey)
	require.NoError(t, err)
	dir := t.TempDir()
	certPath, keyPath = filepath.Join(dir, "tls.crt"), filepath.Join(dir, "tls.key")
	require.NoError(t, os.WriteFile(keyPath, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: key}), 0o600))
	require.NoError(t, os.WriteFile(certPath, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: cert.Certificate[0]}), 0o600))
	return certPath, keyPath
}

func TestFileCertSource(t *testing.T) {
	certPath, keyPath := writePair(t, "rdap.example.test")

	t.Run("loads a certificate covering the server name and reports its expiry", func(t *testing.T) {
		var notAfter time.Time
		cs, err := NewFileCertSource(FileOptions{
			CertPath: certPath, KeyPath: keyPath, ServerName: "rdap.example.test", ServerNameSetting: "RDAP_SERVER_NAME",
			Hooks: Hooks{Loaded: func(t time.Time) { notAfter = t }},
		})
		require.NoError(t, err)
		assert.Equal(t, cs.NotAfter(), notAfter)
		got, err := cs.GetCertificate(nil)
		require.NoError(t, err)
		assert.NotNil(t, got.Leaf)
	})

	t.Run("refuses a certificate that does not cover the server name", func(t *testing.T) {
		_, err := NewFileCertSource(FileOptions{CertPath: certPath, KeyPath: keyPath, ServerName: "other.example.test", ServerNameSetting: "RDAP_SERVER_NAME"})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "RDAP_SERVER_NAME")
	})

	t.Run("refuses an expired certificate", func(t *testing.T) {
		cs := &CertSource{}
		leaf := &x509.Certificate{NotBefore: time.Now().Add(-2 * time.Hour), NotAfter: time.Now().Add(-time.Hour)}
		assert.ErrorContains(t, cs.check(leaf, time.Now()), "expired")
	})
}

func TestServerOffersOnlyRFC9325Suites(t *testing.T) {
	cert, err := SelfSigned("rdap.example.test")
	require.NoError(t, err)
	cs, err := NewStaticCertSource(cert, Hooks{})
	require.NoError(t, err)
	cfg := Server(cs)
	assert.Equal(t, RFC9325CipherSuites, cfg.CipherSuites)
	assert.EqualValues(t, 0x0303, cfg.MinVersion) // TLS 1.2
}
