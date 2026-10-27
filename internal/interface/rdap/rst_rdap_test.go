package rdap

// ICANN RST v2.0 StandardRDAP conformance, per ADR 0015.
//
// Each RST case will be one test named after its case ID. Until a case can be
// tested it is listed in rstPendingCases with the phase of
// docs/rdap/ARCHITECTURE.md that delivers it, so `make rst-rdap` reads as a
// readiness scoreboard from the first PR. TestRST_RDAPCatalogueIsComplete
// fails if the pinned spec has a case this file does not account for.
//
// The spec is pinned in testdata/rst; see testdata/rst/PROVENANCE.md for the
// release and how to move to a newer one.

import (
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/onasunnymorning/domain-os/internal/infrastructure/tlsconfig"
)

// rstPending skips a requirement the server does not meet yet. reason names
// the phase of docs/rdap/ARCHITECTURE.md that delivers it.
func rstPending(t *testing.T, reason string) {
	t.Helper()
	t.Skipf("RST pending: %s", reason)
}

// rstOperational skips a requirement that is a property of a deployment
// (DNS, the certificate actually installed), not of this code. These belong
// on the pre-test checklist in docs/rdap/ARCHITECTURE.md.
func rstOperational(t *testing.T, what string) {
	t.Helper()
	t.Skipf("RST operational, verified per deployment: %s", what)
}

// randomLabel is a label no fixture holds, as RST generates them.
func randomLabel(t *testing.T) string {
	t.Helper()
	b := make([]byte, 8)
	_, err := rand.Read(b)
	require.NoError(t, err)
	return "rst-" + hex.EncodeToString(b)
}

// assertNotFound checks what rdap-08 to rdap-10 require of a lookup for
// something that does not exist: 404, a valid RDAP error object if there is
// a body, and the CORS header.
func assertNotFound(t *testing.T, ts *testServer, path string) {
	t.Helper()
	resp, body := ts.do(t, http.MethodGet, path)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode, path)
	assert.Equal(t, "*", resp.Header.Get("Access-Control-Allow-Origin"), path)
	if len(body) > 0 {
		e := decodeError(t, body)
		assert.Equal(t, http.StatusNotFound, e.ErrorCode)
	}
}

// rstServer serves the TLD "example" under the registry-wide base URL.
func rstServer(t *testing.T) *testServer {
	t.Helper()
	return startServer(t, testConfig("https://rdap.example.test/"), &fakeLookups{
		domains:     map[string]bool{"registered.example": true},
		nameservers: map[string]bool{"ns1.registered.example": true},
		registrars:  map[int]bool{9995: true},
	})
}

// --- rdap-04: Help query test ---------------------------------------------

func TestRST_rdap04_HelpQuery(t *testing.T) {
	ts := rstServer(t)
	for _, base := range []string{"", "example/"} {
		t.Run("rdap-04 help at base URL /"+base, func(t *testing.T) {
			resp, body := ts.do(t, http.MethodGet, base+"help")
			require.Equal(t, http.StatusOK, resp.StatusCode)
			assert.Equal(t, contentType, resp.Header.Get("Content-Type"))
			assert.Equal(t, "*", resp.Header.Get("Access-Control-Allow-Origin"))
			var help helpResponse
			require.NoError(t, json.Unmarshal(body, &help))
			assert.Contains(t, help.RDAPConformance, "rdap_level_0")
			assert.Contains(t, help.RDAPConformance, "icann_rdap_response_profile_1")
			assert.Contains(t, help.RDAPConformance, "icann_rdap_technical_implementation_guide_1")
			require.NotEmpty(t, help.Notices, "the Terms of Use notice")
		})
	}
	t.Run("rdap-04 validated by the RDAP Conformance Tool", func(t *testing.T) {
		rstPending(t, "Phase 6, the conformance tool pinned and run by this suite")
	})
}

// --- rdap-08 to rdap-10: non-existent objects -----------------------------

func TestRST_rdap08_NonExistentDomain(t *testing.T) {
	ts := rstServer(t)
	t.Run("rdap-08 a random domain is 404 with an error object and CORS", func(t *testing.T) {
		assertNotFound(t, ts, "example/domain/"+randomLabel(t)+".example")
		assertNotFound(t, ts, "domain/"+randomLabel(t)+".example")
	})
}

func TestRST_rdap09_NonExistentNameserver(t *testing.T) {
	ts := rstServer(t)
	t.Run("rdap-09 a random internal host is 404", func(t *testing.T) {
		assertNotFound(t, ts, "example/nameserver/ns1."+randomLabel(t)+".example")
	})
	t.Run("rdap-09 a random external host is 404", func(t *testing.T) {
		assertNotFound(t, ts, "example/nameserver/ns1."+randomLabel(t)+".net")
	})
}

func TestRST_rdap10_NonExistentEntity(t *testing.T) {
	ts := rstServer(t)
	t.Run("rdap-10 a random entity handle is 404", func(t *testing.T) {
		assertNotFound(t, ts, "example/entity/"+randomLabel(t))
		assertNotFound(t, ts, "example/entity/424242")
	})
}

// --- rdap-91: TLS version conformance check -------------------------------

func TestRST_rdap91_TLS(t *testing.T) {
	ts := rstServer(t)
	addr := ts.url[len("https://") : len(ts.url)-1]
	dial := func(cfg *tls.Config) error {
		cfg.InsecureSkipVerify = true // #nosec G402 -- test-only: the server presents a certificate generated for this test
		conn, err := tls.Dial("tcp", addr, cfg)
		if err == nil {
			_ = conn.Close()
		}
		return err
	}

	t.Run("rdap-91/1 TLS 1.2 and 1.3 supported", func(t *testing.T) {
		assert.NoError(t, dial(&tls.Config{MinVersion: tls.VersionTLS12, MaxVersion: tls.VersionTLS12})) // #nosec G402 -- test-only: pinning TLS 1.2 to prove it is offered
		assert.NoError(t, dial(&tls.Config{MinVersion: tls.VersionTLS13, MaxVersion: tls.VersionTLS13}))
	})
	t.Run("rdap-91/2 TLS 1.1 and earlier refused", func(t *testing.T) {
		// #nosec G402 -- test-only: proving the server refuses TLS 1.1
		assert.Error(t, dial(&tls.Config{MinVersion: tls.VersionTLS10, MaxVersion: tls.VersionTLS11}))
	})
	t.Run("rdap-91/3 certificate issued by a trusted CA", func(t *testing.T) {
		rstOperational(t, "the certificate installed for RDAP_SERVER_NAME is from a CA browsers trust")
	})
	t.Run("rdap-91/4 certificate not expired, chain presented", func(t *testing.T) {
		// An expired certificate is refused at start and on reload
		// (tlsconfig); whether the deployed file holds its chain is not
		// something this code can know.
		rstOperational(t, "the certificate file holds the leaf and every intermediate")
	})
	t.Run("rdap-91/5 a certificate not covering the server name is refused", func(t *testing.T) {
		cert, err := tlsconfig.SelfSigned("other.example.test")
		require.NoError(t, err)
		key, err := x509.MarshalPKCS8PrivateKey(cert.PrivateKey)
		require.NoError(t, err)
		dir := t.TempDir()
		cfg := testConfig("https://rdap.example.test/")
		cfg.TLS.SelfSigned = false
		cfg.TLS.CertPath, cfg.TLS.KeyPath = filepath.Join(dir, "rdap.crt"), filepath.Join(dir, "rdap.key")
		require.NoError(t, os.WriteFile(cfg.TLS.KeyPath, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: key}), 0o600))
		require.NoError(t, os.WriteFile(cfg.TLS.CertPath, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: cert.Certificate[0]}), 0o600))
		_, err = NewServer(Options{Config: cfg, Lookups: &fakeLookups{}})
		assert.ErrorContains(t, err, "RDAP_SERVER_NAME")

		// With no server name, the base URL's host is checked instead.
		cfg.TLS.ServerName = ""
		_, err = NewServer(Options{Config: cfg, Lookups: &fakeLookups{}})
		assert.ErrorContains(t, err, `"rdap.example.test"`)
		cfg.BaseURL = "https://other.example.test/"
		_, err = NewServer(Options{Config: cfg, Lookups: &fakeLookups{}})
		assert.NoError(t, err, "the certificate covers the base URL's host")
	})
	t.Run("rdap-91/6 only RFC 9325 cipher suites on TLS 1.2", func(t *testing.T) {
		// The test certificate has an RSA key, so the ECDSA suites cannot be
		// negotiated here; that the server offers no suite outside the list
		// is checked in tlsconfig.
		for _, suite := range tlsconfig.RFC9325CipherSuites {
			if !strings.Contains(tls.CipherSuiteName(suite), "_RSA_") {
				continue
			}
			assert.NoError(t, dial(&tls.Config{MinVersion: tls.VersionTLS12, MaxVersion: tls.VersionTLS12, CipherSuites: []uint16{suite}}), tls.CipherSuiteName(suite)) // #nosec G402 -- test-only: pinning TLS 1.2 to test one suite
		}
		cbc := []uint16{tls.TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA, tls.TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA}
		assert.Error(t, dial(&tls.Config{MinVersion: tls.VersionTLS12, MaxVersion: tls.VersionTLS12, CipherSuites: cbc}), "CBC suites are not offered") // #nosec G402 -- test-only: proving the server refuses CBC suites
	})
}

// rstPendingCases are the StandardRDAP cases with no implementation to test
// yet, each with the phase that delivers it. TestRST_RDAPCatalogueIsComplete
// keeps this list and the pinned spec in step.
var rstPendingCases = map[string]string{
	"rdap-01": "Phase 2, domain lookup; graded by the conformance tool from Phase 6",
	"rdap-02": "Phase 3, nameserver lookup; graded by the conformance tool from Phase 6",
	"rdap-03": "Phase 3, registrar entity lookup; graded by the conformance tool from Phase 6",
	"rdap-05": "Phase 2, HEAD on a domain",
	"rdap-06": "Phase 3, HEAD on a nameserver",
	"rdap-07": "Phase 3, HEAD on an entity",
	"rdap-92": "Phase 7, identical responses over IPv4 and IPv6 (per deployment)",
}

func TestRST_RDAPPendingCases(t *testing.T) {
	ids := make([]string, 0, len(rstPendingCases))
	for id := range rstPendingCases {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		t.Run(id, func(t *testing.T) { rstPending(t, rstPendingCases[id]) })
	}
}

// rstImplementedCases have their own TestRST_ function in this file.
var rstImplementedCases = []string{"rdap-04", "rdap-08", "rdap-09", "rdap-10", "rdap-91"}

// TestRST_RDAPCatalogueIsComplete fails when the pinned spec and this file
// disagree: a case in the spec that is neither implemented nor pending, or a
// case here that the spec no longer has. It is what makes moving to a newer
// RST release (testdata/rst/PROVENANCE.md) surface every new case.
func TestRST_RDAPCatalogueIsComplete(t *testing.T) {
	raw, err := os.ReadFile("testdata/rst/rdap-cases.yaml")
	require.NoError(t, err)
	var spec map[string]any
	require.NoError(t, yaml.Unmarshal(raw, &spec))
	require.NotEmpty(t, spec)

	accounted := map[string]bool{}
	for _, id := range rstImplementedCases {
		accounted[id] = true
	}
	for id := range rstPendingCases {
		require.False(t, accounted[id], "%s is both implemented and pending", id)
		accounted[id] = true
	}

	for id := range spec {
		assert.True(t, accounted[id], "RST case %s is in the pinned spec but not in rst_rdap_test.go", id)
	}
	for id := range accounted {
		_, ok := spec[id]
		assert.True(t, ok, "%s is accounted for here but is not in the pinned spec", id)
	}
}
