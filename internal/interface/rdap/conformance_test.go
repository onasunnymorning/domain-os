package rdap

// The RDAP Conformance Tool (rdapct) is what RST grades rdap-01 to rdap-04
// with. This file runs the pinned tool, with RST's pinned configuration
// (testdata/rst/rdapct_config.json), against the server in-process, as the
// EPP suite runs xmllint: it skips locally when Java or the tool is missing,
// and fails in CI.
//
// Getting the tool: `make rdapct` downloads the pinned jar into
// .cache/rdapct, checks its checksum, and has it fetch the IANA datasets it
// validates against once, so the test itself runs with
// --use-local-datasets and needs no network.
//
// Without a local Java, RDAPCT_DOCKER=1 runs the tool in a Temurin image
// (`make rst-rdap-docker`).

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"log/slog"
	"math/big"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/onasunnymorning/domain-os/internal/application/interfaces"
	"github.com/onasunnymorning/domain-os/internal/infrastructure/tlsconfig"
	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
)

// The pinned tool: release v3.1.0 of github.com/icann/rdap-conformance-tool.
const (
	rdapctJar    = "rdapct-3.1.0.jar"
	rdapctSHA256 = "e0d0d2002e8d2e933426c25712be5137bc629d3b081e042d6ffdcc42309e1a42"
	// rdapctImage runs the tool when RDAPCT_DOCKER is set.
	rdapctImage = "eclipse-temurin:21.0.5_11-jre"
)

// rdapct is a way to run the conformance tool.
type rdapct struct {
	dir    string // .cache/rdapct: the jar, datasets, results
	docker bool
	host   string // the host name the tool reaches the server by
}

// findRDAPCT returns the tool, or skips (fails in CI) when it is missing.
func findRDAPCT(t *testing.T) *rdapct {
	t.Helper()
	missing := func(format string, args ...any) *rdapct {
		msg := fmt.Sprintf(format, args...) + "; run `make rdapct`"
		if os.Getenv("CI") != "" {
			t.Fatalf("RDAP Conformance Tool required in CI: %s", msg)
		}
		t.Skipf("RDAP Conformance Tool not available: %s", msg)
		return nil
	}
	dir, err := filepath.Abs("../../../.cache/rdapct")
	require.NoError(t, err)
	jar, err := os.ReadFile(filepath.Join(dir, rdapctJar)) // #nosec G304 -- test-only: the pinned tool's fixed path
	if err != nil {
		return missing("%s not found in %s", rdapctJar, dir)
	}
	sum := sha256.Sum256(jar)
	if hex.EncodeToString(sum[:]) != rdapctSHA256 {
		t.Fatalf("%s does not have the pinned checksum; delete it and run `make rdapct`", rdapctJar)
	}
	if _, err := os.Stat(filepath.Join(dir, "datasets")); err != nil && os.Getenv("RDAPCT_FETCH_DATASETS") == "" {
		return missing("the tool's IANA datasets are not in %s/datasets", dir)
	}
	r := &rdapct{dir: dir, host: "localhost"}
	if os.Getenv("RDAPCT_DOCKER") != "" {
		r.docker, r.host = true, "host.docker.internal"
		return r
	}
	// macOS ships a java stub that fails without a runtime; ask it.
	if err := exec.Command("java", "-version").Run(); err != nil {
		return missing("no Java runtime (%v); install Java 21 or set RDAPCT_DOCKER=1", err)
	}
	return r
}

// command runs tool (java, or sh for the trust store) with args in the tool's directory.
// paths are host directories the tool must see, mounted at the same path in
// Docker.
func (r *rdapct) command(tool string, paths []string, args ...string) *exec.Cmd {
	if !r.docker {
		cmd := exec.Command(tool, args...) // #nosec G204 -- test-only: java or sh with the test's own arguments
		cmd.Dir = r.dir
		return cmd
	}
	dargs := []string{"run", "--rm", "-v", r.dir + ":" + r.dir, "-w", r.dir}
	for _, p := range paths {
		dargs = append(dargs, "-v", p+":"+p)
	}
	dargs = append(dargs, rdapctImage, tool)
	return exec.Command("docker", append(dargs, args...)...) // #nosec G204 -- test-only: docker with the test's own arguments
}

// conformanceCA issues the test server a certificate for host from a CA the
// tool is told to trust, as a deployment's certificate would chain to a CA
// it trusts already.
//
// The tool checks revocation, so the CA also publishes an empty CRL, served
// over HTTP on the loopback, which the server certificate names.
func conformanceCA(t *testing.T, host string) (tls.Certificate, []byte) {
	t.Helper()
	crlListener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	crlURL := fmt.Sprintf("http://%s:%d/ca.crl", host, crlListener.Addr().(*net.TCPAddr).Port)
	caKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	caTmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "domain-os RDAP conformance test CA"},
		NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(24 * time.Hour),
		IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
	}
	caDER, err := x509.CreateCertificate(rand.Reader, caTmpl, caTmpl, &caKey.PublicKey, caKey)
	require.NoError(t, err)
	ca, err := x509.ParseCertificate(caDER)
	require.NoError(t, err)

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(2), Subject: pkix.Name{CommonName: host},
		DNSNames: []string{host}, IPAddresses: []net.IP{net.ParseIP("127.0.0.1")},
		NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(24 * time.Hour),
		KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		CRLDistributionPoints: []string{crlURL},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, ca, &key.PublicKey, caKey)
	require.NoError(t, err)

	crl, err := x509.CreateRevocationList(rand.Reader, &x509.RevocationList{
		Number: big.NewInt(1), ThisUpdate: time.Now().Add(-time.Hour), NextUpdate: time.Now().Add(24 * time.Hour),
	}, ca, caKey)
	require.NoError(t, err)
	crlServer := &http.Server{ReadHeaderTimeout: 5 * time.Second, Handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/pkix-crl")
		_, _ = w.Write(crl)
	})}
	go func() { _ = crlServer.Serve(crlListener) }()
	t.Cleanup(func() { _ = crlServer.Close() })
	return tls.Certificate{Certificate: [][]byte{der, caDER}, PrivateKey: key}, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: caDER})
}

// conformanceRun is one run of the tool against one URL.
type conformanceRun struct {
	Errors   []rdapctResult
	Warnings []rdapctResult
}

type rdapctResult struct {
	Code    int    `json:"code"`
	Value   string `json:"value"`
	Message string `json:"message"`
}

// startConformanceServer serves rec, the nameserver and the registrar over
// TLS on the loopback, with a CA-signed certificate for the tool's host
// name, and returns the base URL the tool reaches it at.
func startConformanceServer(t *testing.T, r *rdapct, lookups *fakeLookups, tmp string) string {
	t.Helper()
	cert, caPEM := conformanceCA(t, r.host)

	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	base := fmt.Sprintf("https://%s:%d/", r.host, l.Addr().(*net.TCPAddr).Port)
	cfg := testConfig(base)
	// Debug, so a failed handshake or a malformed request is logged.
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
	srv, err := NewServer(Options{Config: cfg, Lookups: lookups, Logger: logger})
	require.NoError(t, err)
	srv.certs, err = tlsconfig.NewStaticCertSource(cert, tlsconfig.Hooks{})
	require.NoError(t, err)
	srv.http.TLSConfig = tlsconfig.Server(srv.certs)
	served := make(chan error, 1)
	go func() { served <- srv.Serve(l) }()
	t.Cleanup(func() { _ = srv.Shutdown(t.Context()); <-served })
	require.Eventually(t, srv.Ready, 2*time.Second, 5*time.Millisecond)

	// The tool trusts the test CA through a trust store holding the JVM's
	// own CAs as well, so it can still fetch IANA's datasets when asked to.
	require.NoError(t, os.WriteFile(filepath.Join(tmp, "ca.pem"), caPEM, 0o600))
	script := `set -e
java_home="${JAVA_HOME:-$(dirname "$(dirname "$(readlink -f "$(command -v java)")")")}"
cp "$java_home/lib/security/cacerts" "$1/trust.p12"
chmod u+w "$1/trust.p12"
keytool -importcert -noprompt -alias rdap-test-ca -file "$1/ca.pem" -keystore "$1/trust.p12" -storepass changeit`
	out, err := r.command("sh", []string{tmp}, "-c", script, "sh", tmp).CombinedOutput()
	require.NoError(t, err, "trust store: %s", out)
	return base
}

// runConformance runs the tool against url and reads its results file.
func runConformance(t *testing.T, r *rdapct, tmp, url string, flags ...string) conformanceRun {
	t.Helper()
	results := filepath.Join(tmp, fmt.Sprintf("results-%d.json", time.Now().UnixNano()))
	var jvm []string
	if os.Getenv("RDAPCT_VERBOSE") == "tls" {
		jvm = append(jvm, "-Djavax.net.debug=ssl:handshake")
	}
	args := []string{
		"-Djavax.net.ssl.trustStore=" + filepath.Join(tmp, "trust.p12"),
		"-Djavax.net.ssl.trustStorePassword=changeit",
		// The test CA has a CRL and no OCSP responder.
		"-Dcom.sun.security.enableCRLDP=true",
	}
	args = append(append(jvm, args...), []string{
		"-jar", filepath.Join(r.dir, rdapctJar),
		"-c", mustAbs(t, "testdata/rst/rdapct_config.json"),
		"--results-file", results,
		"--use-rdap-profile-february-2024",
		// The test server listens on the IPv4 loopback only; RST queries
		// both families of a deployment (rdap-92).
		"--no-ipv6-queries",
	}...)
	if os.Getenv("RDAPCT_FETCH_DATASETS") == "" {
		args = append(args, "--use-local-datasets")
	}
	if os.Getenv("RDAPCT_VERBOSE") != "" {
		args = append(args, "--logging", "DEBUG")
	}
	args = append(append(args, flags...), url)
	out, err := r.command("java", []string{tmp, mustAbs(t, "testdata/rst")}, args...).CombinedOutput()
	if os.Getenv("RDAPCT_VERBOSE") != "" {
		t.Logf("rdapct output:\n%s", out)
	}
	raw, readErr := os.ReadFile(results) // #nosec G304 -- test-only: the results file this run named
	require.NoError(t, readErr, "the tool wrote no results (exit %v): %s", err, out)

	var file struct {
		Results struct {
			Error   []rdapctResult `json:"error"`
			Warning []rdapctResult `json:"warning"`
		} `json:"results"`
	}
	require.NoError(t, json.Unmarshal(raw, &file), "results: %s", raw)
	return conformanceRun{Errors: file.Results.Error, Warnings: file.Results.Warning}
}

func mustAbs(t *testing.T, p string) string {
	t.Helper()
	abs, err := filepath.Abs(p)
	require.NoError(t, err)
	return abs
}

// requireConformant fails with every error the tool found, and logs its
// warnings.
func requireConformant(t *testing.T, run conformanceRun) {
	t.Helper()
	for _, w := range run.Warnings {
		t.Logf("rdapct warning %d: %s (%s)", w.Code, w.Message, w.Value)
	}
	if len(run.Errors) == 0 {
		return
	}
	var b strings.Builder
	for _, e := range run.Errors {
		fmt.Fprintf(&b, "\n  %d: %s\n     value: %s", e.Code, e.Message, e.Value)
	}
	t.Fatalf("the RDAP Conformance Tool found %d errors:%s", len(run.Errors), b.String())
}

// TestRST_RDAPConformanceTool grades the server's responses as RST does
// for rdap-01 to rdap-04. Each case's own test (rst_rdap_test.go) checks the
// response's shape without the tool.
func TestRST_RDAPConformanceTool(t *testing.T) {
	r := findRDAPCT(t)
	tmp := t.TempDir()
	thick, thin := fullDomain(), thickToThin()
	lookups := &fakeLookups{
		domains: map[string]*interfaces.RDAPDomain{string(thick.Domain.Name): thick, string(thin.Domain.Name): thin},
		nameservers: map[string]*interfaces.RDAPNameserver{
			"ns1.xn--bcher-kva.example": testNameserver("ns1.xn--bcher-kva.example"),
		},
		registrars: map[int]*entities.Registrar{9995: testRegistrar()},
	}
	base := startConformanceServer(t, r, lookups, tmp)

	t.Run("rdap-01 a thick domain", func(t *testing.T) {
		requireConformant(t, runConformance(t, r, tmp, base+"example/domain/"+string(thick.Domain.Name), "--gtld-registry"))
	})
	t.Run("rdap-01 a domain of a thin TLD", func(t *testing.T) {
		requireConformant(t, runConformance(t, r, tmp, base+"example/domain/"+string(thin.Domain.Name), "--gtld-registry", "--thin"))
	})
	t.Run("rdap-02 a nameserver", func(t *testing.T) {
		requireConformant(t, runConformance(t, r, tmp, base+"example/nameserver/ns1.xn--bcher-kva.example", "--gtld-registry"))
	})
	t.Run("rdap-03 a registrar", func(t *testing.T) {
		requireConformant(t, runConformance(t, r, tmp, base+"example/entity/9995", "--gtld-registry"))
	})
	t.Run("rdap-04 help", func(t *testing.T) {
		requireConformant(t, runConformance(t, r, tmp, base+"example/help", "--gtld-registry"))
	})
}
