package epp

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// testCA is a two-level CA for certificate tests: a root and an
// intermediate that signs leaves, as a public CA would.
type testCA struct {
	root, intermediate       *x509.Certificate
	rootKey, intermediateKey *ecdsa.PrivateKey
}

func newTestCA(t *testing.T) *testCA {
	t.Helper()
	ca := &testCA{}
	ca.root, ca.rootKey = issue(t, &x509.Certificate{
		Subject: pkix.Name{CommonName: "Test Root CA"}, IsCA: true, BasicConstraintsValid: true,
		KeyUsage: x509.KeyUsageCertSign, NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(24 * time.Hour),
	}, nil, nil)
	ca.intermediate, ca.intermediateKey = issue(t, &x509.Certificate{
		Subject: pkix.Name{CommonName: "Test Intermediate CA"}, IsCA: true, BasicConstraintsValid: true,
		KeyUsage: x509.KeyUsageCertSign, NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(24 * time.Hour),
	}, ca.root, ca.rootKey)
	return ca
}

// leaf issues a certificate from the intermediate, for dnsNames, valid
// between notBefore and notAfter.
func (ca *testCA) leaf(t *testing.T, dnsNames []string, notBefore, notAfter time.Time, usage x509.ExtKeyUsage) (*x509.Certificate, *ecdsa.PrivateKey) {
	t.Helper()
	return issue(t, &x509.Certificate{
		Subject: pkix.Name{CommonName: "leaf"}, DNSNames: dnsNames,
		KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{usage},
		NotBefore: notBefore, NotAfter: notAfter,
	}, ca.intermediate, ca.intermediateKey)
}

// writeServerCert writes a leaf and the intermediate to certPath, and the
// leaf's key to keyPath, as a deployment would.
func (ca *testCA) writeServerCert(t *testing.T, dir string, dnsNames []string, notBefore, notAfter time.Time) (certPath, keyPath string) {
	t.Helper()
	leaf, key := ca.leaf(t, dnsNames, notBefore, notAfter, x509.ExtKeyUsageServerAuth)
	certPath, keyPath = filepath.Join(dir, "epp.crt"), filepath.Join(dir, "epp.key")
	pemCerts := append(pemCert(leaf), pemCert(ca.intermediate)...)
	keyDER, err := x509.MarshalPKCS8PrivateKey(key)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(keyPath, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER}), 0o600))
	require.NoError(t, os.WriteFile(certPath, pemCerts, 0o600))
	return certPath, keyPath
}

func issue(t *testing.T, tmpl *x509.Certificate, parent *x509.Certificate, parentKey *ecdsa.PrivateKey) (*x509.Certificate, *ecdsa.PrivateKey) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	serial, err := rand.Int(rand.Reader, big.NewInt(1<<62))
	require.NoError(t, err)
	tmpl.SerialNumber = serial
	if parent == nil {
		parent, parentKey = tmpl, key
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, parent, &key.PublicKey, parentKey)
	require.NoError(t, err)
	cert, err := x509.ParseCertificate(der)
	require.NoError(t, err)
	return cert, key
}

func pemCert(c *x509.Certificate) []byte {
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: c.Raw})
}

// clientChain builds a client certificate whose chain is the leaf followed
// by others, in the order given.
func clientChain(leaf *x509.Certificate, key *ecdsa.PrivateKey, others ...*x509.Certificate) tls.Certificate {
	chain := [][]byte{leaf.Raw}
	for _, c := range others {
		chain = append(chain, c.Raw)
	}
	return tls.Certificate{Certificate: chain, PrivateKey: key, Leaf: leaf}
}
