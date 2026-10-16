package epp

// ICANN RST v2.0 StandardEPP conformance, per ADR 0015.
//
// Each RST case is one test named after its case ID, and each numbered
// requirement in the case is a subtest. A requirement this server cannot meet
// yet is not left out: it is a subtest that skips with the phase that
// delivers it (rstPending), so `make rst-epp` reads as a readiness
// scoreboard. TestRST_EPPCatalogueIsComplete fails if the pinned spec has a
// case this file does not account for.
//
// The spec, schemas and extension registry are pinned in testdata/rst; see
// testdata/rst/PROVENANCE.md for the release and how to move to a newer one.

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/beevik/etree"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/onasunnymorning/domain-os/internal/config"
	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
)

// rstPending skips a requirement the server does not meet yet. reason names
// the phase of docs/epp/ARCHITECTURE.md that delivers it.
func rstPending(t *testing.T, reason string) {
	t.Helper()
	t.Skipf("RST pending: %s", reason)
}

// rstOperational skips a requirement that is a property of a deployment
// (DNS, firewall, the certificate actually installed), not of this code.
// These belong on the pre-test checklist for an RST run.
func rstOperational(t *testing.T, what string) {
	t.Helper()
	t.Skipf("RST operational, verified per deployment: %s", what)
}

// dialTLS opens a TLS session with cfg, without reading the greeting.
func dialTLS(t *testing.T, addr string, cfg *tls.Config) (*client, error) {
	t.Helper()
	d := &net.Dialer{Timeout: 5 * time.Second}
	conn, err := tls.DialWithDialer(d, "tcp", addr, cfg)
	if err != nil {
		return nil, err
	}
	require.NoError(t, conn.SetDeadline(time.Now().Add(10*time.Second)))
	t.Cleanup(func() { _ = conn.Close() })
	return &client{t: t, conn: conn}, nil
}

// --- epp-01: Service connectivity test ------------------------------------

func TestRST_epp01_ServiceConnectivity(t *testing.T) {
	creds := newTestCredentials(t)
	ts := startServerWith(t, testConfig(), listenLoopback(t), creds)

	t.Run("epp-01/1 A record for the EPP hostname", func(t *testing.T) {
		rstOperational(t, "at least one A record for EPP_SERVER_NAME")
	})

	t.Run("epp-01/2 reachable over IPv6", func(t *testing.T) {
		l, err := net.Listen("tcp6", "[::1]:0")
		if err != nil {
			t.Skipf("no IPv6 loopback on this machine: %v", err)
		}
		v6 := startServerOn(t, testConfig(), l)
		dial(t, v6.addr) // reads and schema-checks the greeting
		// The AAAA record itself is operational, like epp-01/1.
	})

	t.Run("epp-01/3 port 700 open to the RST client ACL", func(t *testing.T) {
		// RST's epp.clientACL becomes the test registrars' allowed networks;
		// loopback stands in for it here. The firewall in front is
		// operational.
		dial(t, ts.addr) // an allowed source is greeted

		closed := newTestCredentials(t)
		closed.update(testRegistrar, func(a *entities.EPPRegistrarAccess) error {
			for _, n := range a.AllowedNetworks {
				if _, err := a.RetireCredential(entities.EPPCredentialRef{Kind: entities.EPPCredentialAllowedNetwork, ID: n.ID}, entities.EPPRetireNormal, ptr(time.Now()), "test", time.Now()); err != nil {
					return err
				}
			}
			return nil
		})
		other := startServerWith(t, testConfig(), listenLoopback(t), closed)
		assertClosedWithoutGreeting(t, dialRaw(t, other.addr))
	})

	t.Run("epp-01/4 TLSv1.2 and TLSv1.3 supported", func(t *testing.T) {
		for _, v := range []uint16{tls.VersionTLS12, tls.VersionTLS13} {
			cfg := clientTLSConfig(t)
			cfg.MinVersion, cfg.MaxVersion = v, v
			c, err := dialTLS(t, ts.addr, cfg)
			require.NoError(t, err, tls.VersionName(v))
			require.NotNil(t, c.read().FindElement("//greeting"), tls.VersionName(v))
		}
	})

	t.Run("epp-01/5 TLSv1.1 and older refused", func(t *testing.T) {
		for _, v := range []uint16{tls.VersionTLS10, tls.VersionTLS11} {
			cfg := clientTLSConfig(t)
			cfg.MinVersion, cfg.MaxVersion = v, v // #nosec G402 -- test-only: offering a forbidden version to prove the server refuses it
			_, err := dialTLS(t, ts.addr, cfg)
			require.Error(t, err, tls.VersionName(v))
			assert.Contains(t, err.Error(), "protocol version", "refused by the server's version check, %s", tls.VersionName(v))
		}
	})

	t.Run("epp-01/6 server certificate from a trusted CA", func(t *testing.T) {
		rstOperational(t, "the installed EPP_TLS_CERT_PATH chains to a CA in RST's epp.tlsCertificateStore (Mozilla's bundle)")
	})

	t.Run("epp-01/7 certificate in date and presented with its chain", func(t *testing.T) {
		ca := newTestCA(t)
		now := time.Now()

		dir := t.TempDir()
		certPath, keyPath := ca.writeServerCert(t, dir, []string{"epp.example.test"}, now.Add(-time.Hour), now.Add(time.Hour))
		cfg := testConfig()
		cfg.TLS = config.EPPTLS{CertPath: certPath, KeyPath: keyPath, ServerName: "epp.example.test", ReloadInterval: time.Minute}
		srv := startServer(t, cfg)
		c := dial(t, srv.addr)
		chain := c.conn.ConnectionState().PeerCertificates
		require.Len(t, chain, 2, "leaf and intermediate are both presented")
		assert.True(t, chain[1].Equal(ca.intermediate))

		expiredDir := t.TempDir()
		certPath, keyPath = ca.writeServerCert(t, expiredDir, []string{"epp.example.test"}, now.Add(-2*time.Hour), now.Add(-time.Hour))
		cfg.TLS.CertPath, cfg.TLS.KeyPath = certPath, keyPath
		_, err := NewServer(testOptions(t, cfg))
		require.Error(t, err, "an expired certificate stops the server at start")
		assert.Contains(t, err.Error(), "expired")

		notYetDir := t.TempDir()
		certPath, keyPath = ca.writeServerCert(t, notYetDir, []string{"epp.example.test"}, now.Add(time.Hour), now.Add(2*time.Hour))
		cfg.TLS.CertPath, cfg.TLS.KeyPath = certPath, keyPath
		_, err = NewServer(testOptions(t, cfg))
		require.Error(t, err, "a certificate not yet valid stops the server at start")
		assert.Contains(t, err.Error(), "not valid until")
	})

	t.Run("epp-01/8 extraneous and disordered client certificates accepted", func(t *testing.T) {
		ca := newTestCA(t)
		leaf, key := ca.leaf(t, nil, time.Now().Add(-time.Hour), time.Now().Add(time.Hour), x509.ExtKeyUsageClientAuth)
		unrelated := newTestCA(t).root
		creds.pin(testRegistrar, leaf)
		for name, chain := range map[string]tls.Certificate{
			"leaf, intermediate, root":      clientChain(leaf, key, ca.intermediate, ca.root),
			"leaf, root, intermediate":      clientChain(leaf, key, ca.root, ca.intermediate),
			"leaf, unrelated, intermediate": clientChain(leaf, key, unrelated, ca.intermediate),
			"leaf only":                     clientChain(leaf, key),
		} {
			cfg := clientTLSConfig(t)
			cfg.Certificates = []tls.Certificate{chain}
			c, err := dialTLS(t, ts.addr, cfg)
			require.NoError(t, err, name)
			require.NotNil(t, c.read().FindElement("//greeting"), name)
			c.login()
		}
		// Only the leaf is pinned (ADR 0014), so the rest of the chain is
		// irrelevant to authentication.
	})

	t.Run("epp-01/9 server name matches a subjectAltName", func(t *testing.T) {
		ca := newTestCA(t)
		now := time.Now()
		cases := []struct {
			sans       []string
			serverName string
			ok         bool
		}{
			{[]string{"epp.example.test"}, "epp.example.test", true},
			{[]string{"*.example.test"}, "epp.example.test", true},
			{[]string{"other.example.test"}, "epp.example.test", false},
			{[]string{"*.test"}, "epp.example.test", false},
		}
		for _, tc := range cases {
			certPath, keyPath := ca.writeServerCert(t, t.TempDir(), tc.sans, now.Add(-time.Hour), now.Add(time.Hour))
			cfg := testConfig()
			cfg.TLS = config.EPPTLS{CertPath: certPath, KeyPath: keyPath, ServerName: tc.serverName, ReloadInterval: time.Minute}
			_, err := NewServer(testOptions(t, cfg))
			if tc.ok {
				assert.NoError(t, err, "SANs %v cover %s", tc.sans, tc.serverName)
			} else {
				assert.ErrorContains(t, err, "EPP_SERVER_NAME", "SANs %v do not cover %s", tc.sans, tc.serverName)
			}
		}
	})

	t.Run("epp-01/10 only RFC 9325 recommended cipher suites under TLSv1.2", func(t *testing.T) {
		recommended := map[uint16]bool{}
		for _, id := range rfc9325CipherSuites {
			recommended[id] = true
		}
		var negotiated []string
		for _, suite := range append(tls.CipherSuites(), tls.InsecureCipherSuites()...) {
			if !supportsTLS12(suite) {
				continue
			}
			cfg := clientTLSConfig(t)
			cfg.MaxVersion = tls.VersionTLS12
			cfg.CipherSuites = []uint16{suite.ID} // #nosec G402 -- test-only: offering each suite in turn to prove which the server accepts
			c, err := dialTLS(t, ts.addr, cfg)
			if err != nil {
				continue
			}
			c.read()
			require.True(t, recommended[suite.ID], "the server negotiated %s, which RFC 9325 does not recommend", suite.Name)
			negotiated = append(negotiated, suite.Name)
		}
		assert.NotEmpty(t, negotiated, "at least one recommended suite works with the server's key type")
	})
}

func supportsTLS12(s *tls.CipherSuite) bool {
	for _, v := range s.SupportedVersions {
		if v == tls.VersionTLS12 {
			return true
		}
	}
	return false
}

// --- epp-02: Protocol conformance test ------------------------------------

// languageTag is a loose BCP 47 shape: enough to catch an empty or garbled
// <lang>, not a full registry check.
var languageTag = regexp.MustCompile(`^[A-Za-z]{2,8}(-[A-Za-z0-9]{1,8})*$`)

func TestRST_epp02_ProtocolConformance(t *testing.T) {
	ts := startServer(t, testConfig())
	c := dial(t, ts.addr)
	greeting := c.roundTrip(`<?xml version="1.0" encoding="UTF-8"?><epp xmlns="urn:ietf:params:xml:ns:epp-1.0"><hello/></epp>`)
	g := greeting.FindElement("/epp/greeting")
	require.NotNil(t, g, "a valid <greeting>; the harness has already validated it against the pinned schemas")
	registry := loadExtensionRegistry(t)

	t.Run("epp-02/1 svID identifies the server", func(t *testing.T) {
		assert.Equal(t, serverID, strings.TrimSpace(g.FindElement("svID").Text()))
	})

	t.Run("epp-02/2 svDate within 30 seconds of now", func(t *testing.T) {
		svDate, err := time.Parse(time.RFC3339, g.FindElement("svDate").Text())
		require.NoError(t, err)
		assert.WithinDuration(t, time.Now(), svDate, 30*time.Second)
	})

	t.Run("epp-02/3 exactly one version, 1.0", func(t *testing.T) {
		versions := g.FindElements("svcMenu/version")
		require.Len(t, versions, 1)
		assert.Equal(t, "1.0", versions[0].Text())
	})

	t.Run("epp-02/4 valid languages, including en", func(t *testing.T) {
		var langs []string
		for _, l := range g.FindElements("svcMenu/lang") {
			assert.Regexp(t, languageTag, l.Text())
			langs = append(langs, l.Text())
		}
		assert.Contains(t, langs, "en")
	})

	t.Run("epp-02/5 domain objURI present", func(t *testing.T) {
		assert.Contains(t, texts(g.FindElements("svcMenu/objURI")), nsDomain)
	})

	t.Run("epp-02/6 every objURI and extURI is in the EPP Extension Registry", func(t *testing.T) {
		for _, uri := range texts(g.FindElements("svcMenu/objURI")) {
			assert.True(t, registry.objects[uri], "objURI %s is not a registered object mapping", uri)
		}
		for _, uri := range texts(g.FindElements("svcMenu/svcExtension/extURI")) {
			assert.True(t, registry.extensions[uri], "extURI %s is not a registered extension", uri)
		}
	})

	extURIs := texts(g.FindElements("svcMenu/svcExtension/extURI"))
	requireExt := func(t *testing.T, uri, pendingReason string) {
		t.Helper()
		if !contains(extURIs, uri) {
			rstPending(t, pendingReason)
		}
	}

	t.Run("epp-02/7 secDNS-1.1 extURI", func(t *testing.T) {
		requireExt(t, "urn:ietf:params:xml:ns:secDNS-1.1", "DNSSEC (RFC 5910) is offered: this should not be pending")
	})
	t.Run("epp-02/7 rgp-1.0 extURI", func(t *testing.T) {
		requireExt(t, "urn:ietf:params:xml:ns:rgp-1.0", "RGP (RFC 3915) is offered: this should not be pending")
	})
	t.Run("epp-02/8 launch-1.0 extURI", func(t *testing.T) {
		requireExt(t, "urn:ietf:params:xml:ns:launch-1.0", "launch phases (RFC 8334) for sunrise and claims; not yet scheduled in docs/epp/ARCHITECTURE.md")
	})

	for _, rec := range []struct{ uri, rfc string }{
		{"urn:ietf:params:xml:ns:epp:secure-authinfo-transfer-1.0", "RFC 9154"},
		{"urn:ietf:params:xml:ns:epp:unhandled-namespaces-1.0", "RFC 9038"},
		{"urn:ietf:params:xml:ns:epp:loginSec-1.0", "RFC 8807"},
		{"urn:ietf:params:xml:ns:changePoll-1.0", "RFC 8590"},
	} {
		t.Run("epp-02 recommended "+rec.rfc, func(t *testing.T) {
			// RST raises EPP_GREETING_RECOMMENDED_EXTENSION_MISSING as a
			// warning, not an error.
			requireExt(t, rec.uri, rec.rfc+" is recommended by RST (warning only); not yet scheduled")
		})
	}
}

// extensionRegistry is ICANN's extraction of URIs from the IANA EPP
// Extension Registry, pinned in testdata/rst/epp-extensions.yaml.
type extensionRegistry struct {
	objects, extensions map[string]bool
}

func loadExtensionRegistry(t *testing.T) extensionRegistry {
	t.Helper()
	raw, err := os.ReadFile("testdata/rst/epp-extensions.yaml")
	require.NoError(t, err)
	var doc struct {
		URIs struct {
			Objects    []struct{ URI string } `yaml:"objects"`
			Extensions []struct{ URI string } `yaml:"extensions"`
		} `yaml:"uris"`
	}
	require.NoError(t, yaml.Unmarshal(raw, &doc))
	reg := extensionRegistry{objects: map[string]bool{}, extensions: map[string]bool{}}
	for _, o := range doc.URIs.Objects {
		reg.objects[o.URI] = true
	}
	for _, e := range doc.URIs.Extensions {
		reg.extensions[e.URI] = true
	}
	require.NotEmpty(t, reg.objects, "the pinned registry parsed")
	require.NotEmpty(t, reg.extensions, "the pinned registry parsed")
	return reg
}

// --- epp-03: Authentication test ------------------------------------------

func TestRST_epp03_Authentication(t *testing.T) {
	creds := newTestCredentials(t)
	ts := startServerWith(t, testConfig(), listenLoopback(t), creds)

	// registrar2 has its own login, password and certificate, like RST's
	// epp.clid02 and epp.clid02DataModel.
	ca := newTestCA(t)
	otherLeaf, otherKey := ca.leaf(t, nil, time.Now().Add(-time.Hour), time.Now().Add(time.Hour), x509.ExtKeyUsageClientAuth)
	creds.update("registrar2", func(a *entities.EPPRegistrarAccess) error {
		if _, err := a.AddLogin("registrar2", entities.EPPLoginRoleTransform, "test", time.Now()); err != nil {
			return err
		}
		if _, err := a.AddPassword("registrar2", "secret-pw-02", fastHash, "test", time.Now()); err != nil {
			return err
		}
		_, err := a.AddAllowedNetwork("127.0.0.1", "loopback", "test", time.Now())
		return err
	})
	creds.pin("registrar2", otherLeaf)

	// loginWith logs in on a fresh session presenting cert, and returns the
	// result code.
	loginWith := func(t *testing.T, cert tls.Certificate, clID, pw string) string {
		cfg := clientTLSConfig(t)
		cfg.Certificates = []tls.Certificate{cert}
		c, err := dialTLS(t, ts.addr, cfg)
		require.NoError(t, err)
		c.read()
		resp := c.roundTrip(loginAsXML(clID, pw, "", "RST-03"))
		return resp.FindElement("//response/result").SelectAttrValue("code", "")
	}
	ours := testClientCertificate(t)

	t.Run("epp-03/1 unknown client ID rejected", func(t *testing.T) {
		assert.Equal(t, "2200", loginWith(t, ours, "zq7rnd0m9x", testPassword))
	})
	t.Run("epp-03/2 wrong password rejected", func(t *testing.T) {
		assert.Equal(t, "2200", loginWith(t, ours, testLogin, "not-the-pw-1"))
	})
	t.Run("epp-03/3 wrong client certificate rejected", func(t *testing.T) {
		unpinned, err := SelfSignedCertificate()
		require.NoError(t, err)
		assert.Equal(t, "2200", loginWith(t, unpinned, testLogin, testPassword))
	})
	t.Run("epp-03/4 another registrar's certificate rejected", func(t *testing.T) {
		theirs := clientChain(otherLeaf, otherKey)
		assert.Equal(t, "2200", loginWith(t, theirs, testLogin, testPassword))
		assert.Equal(t, "1000", loginWith(t, theirs, "registrar2", "secret-pw-02"), "it is a valid certificate, for its own registrar")
	})

	t.Run("epp-03/5 no client certificate rejected", func(t *testing.T) {
		cfg := clientTLSConfig(t)
		cfg.Certificates = nil
		c, err := dialTLS(t, ts.addr, cfg)
		if err == nil {
			// TLS 1.3 reports the server's refusal on the first read.
			_, err = c.conn.Read(make([]byte, 1))
		}
		require.Error(t, err, "a session without a client certificate never reaches <login>")
		assert.Contains(t, err.Error(), "certificate")
	})

	t.Run("epp-03/6 valid client ID, password and certificate accepted", func(t *testing.T) {
		assert.Equal(t, "1000", loginWith(t, ours, testLogin, testPassword))
	})
}

func ptr[T any](v T) *T { return &v }

// --- epp-04: Domain <check> command test ----------------------------------

// The registry behind these tests is fakeDomains: .test is served and
// registered.test stands in for epp.registeredNames. The availability rules
// themselves are tested in services.EPPDomainService's tests.
func TestRST_epp04_DomainCheck(t *testing.T) {
	ts := startServer(t, testConfig())
	c := dial(t, ts.addr)
	c.login()

	// avail checks a name and returns its avail attribute, or the result
	// code if the check was refused.
	avail := func(t *testing.T, name string) string {
		t.Helper()
		resp := c.roundTrip(checkNamesXML(name))
		code := resp.FindElement("//response/result").SelectAttrValue("code", "")
		if code != "1000" {
			return code
		}
		el := resp.FindElement("//resData/chkData/cd/name")
		require.NotNil(t, el, "a <domain:name> for the name checked")
		return el.SelectAttrValue("avail", "")
	}

	t.Run("epp-04 syntactically invalid names are unavailable or refused", func(t *testing.T) {
		for _, name := range []string{
			"-leading.test", "trailing-.test", "under_score.test", "dou..ble.test", "sp ace.test",
			strings.Repeat("x", 64) + ".test", "nodot", "",
		} {
			got := avail(t, name)
			assert.Contains(t, []string{"0", "false", "2001", "2004", "2005"}, got, "%q", name)
		}
	})
	t.Run("epp-04 registered names are unavailable", func(t *testing.T) {
		assert.Equal(t, "0", avail(t, "registered.test"))
	})
	t.Run("epp-04 unregistered names are available", func(t *testing.T) {
		assert.Equal(t, "1", avail(t, "rst"+strings.ToLower(strconv.FormatInt(time.Now().UnixNano(), 36))+".test"))
	})
}

// --- epp-14: Domain <create> command test ---------------------------------

// rstRegistrar2 gives registrar2 (RST's epp.clid02) a login, password,
// allowed network and the test client certificate, so it can log in.
func rstRegistrar2(t *testing.T, creds *testCredentials) {
	t.Helper()
	creds.update("registrar2", func(a *entities.EPPRegistrarAccess) error {
		if _, err := a.AddLogin("registrar2", entities.EPPLoginRoleTransform, "test", time.Now()); err != nil {
			return err
		}
		if _, err := a.AddPassword("registrar2", "secret-pw-02", fastHash, "test", time.Now()); err != nil {
			return err
		}
		if _, err := a.AddCertificatePin(certificatePEM(testClientCertificate(t)), nil, "test", time.Now()); err != nil {
			return err
		}
		_, err := a.AddAllowedNetwork("127.0.0.1", "loopback", "test", time.Now())
		return err
	})
}

func TestRST_epp14_DomainCreate(t *testing.T) {
	creds := newTestCredentials(t)
	rstRegistrar2(t, creds)
	// .thin is a TLD whose phase prohibits a registrant, as
	// general.registryDataModel=minimum would; .test requires one.
	thin := creds.domains.registry.AddTLD("thin", true)
	thin.Policy.ContactDataPolicy.RegistrantContactDataPolicy = entities.ContactDataPolicyTypeProhibited
	thin.Policy.ContactDataPolicy.TechContactDataPolicy = entities.ContactDataPolicyTypeOptional
	creds.domains.registry.Accredit(testRegistrar, "thin")
	ts := startServerWith(t, testConfig(), listenLoopback(t), creds)
	c := dial(t, ts.addr)
	requireCode(t, c.roundTrip(loginWithServices([]string{nsDomain}, []string{nsSecDNS})), "1000")

	refused := func(t *testing.T, frame string) {
		t.Helper()
		code := c.roundTrip(frame).FindElement("//response/result").SelectAttrValue("code", "")
		assert.True(t, strings.HasPrefix(code, "2"), "refused, got %s", code)
	}
	t.Run("epp-14 invalid domain name refused", func(t *testing.T) {
		refused(t, createXML("-invalid-.test", createOpts{}))
		refused(t, createXML("in valid.test", createOpts{}))
	})
	t.Run("epp-14 invalid period refused (1-10 years)", func(t *testing.T) {
		refused(t, createXML("p0.test", createOpts{period: `<domain:period unit="y">0</domain:period>`}))
		refused(t, createXML("p11.test", createOpts{period: `<domain:period unit="y">11</domain:period>`}))
		refused(t, createXML("p121.test", createOpts{period: `<domain:period unit="m">121</domain:period>`}))
	})
	t.Run("epp-14 non-existent host object refused", func(t *testing.T) {
		refused(t, createXML("nohost.test", createOpts{ns: `<domain:ns><domain:hostObj>ns1.nonexistent-rst.net</domain:hostObj></domain:ns>`}))
	})
	t.Run("epp-14 host attributes refused (epp.hostModel = objects)", func(t *testing.T) {
		refused(t, createXML("glue.test", createOpts{ns: `<domain:ns><domain:hostAttr><domain:hostName>ns1.glue.test</domain:hostName></domain:hostAttr></domain:ns>`}))
		refused(t, createXML("glue2.test", createOpts{ns: `<domain:ns><domain:hostAttr><domain:hostName>ns1.glue2.test</domain:hostName><domain:hostAddr ip="v4">192.0.2.1</domain:hostAddr></domain:hostAttr></domain:ns>`}))
	})
	t.Run("epp-14 host objects accepted (epp.hostModel = objects)", func(t *testing.T) {
		requireCode(t, c.roundTrip(createXML("withns.test", createOpts{ns: `<domain:ns><domain:hostObj>ns1.registrar1.net</domain:hostObj></domain:ns>`})), "1000")
	})
	t.Run("epp-14 invalid DNSSEC data refused", func(t *testing.T) {
		for _, ds := range []string{
			fmt.Sprintf(`<secDNS:dsData><secDNS:keyTag>70000</secDNS:keyTag><secDNS:alg>13</secDNS:alg><secDNS:digestType>2</secDNS:digestType><secDNS:digest>%s</secDNS:digest></secDNS:dsData>`, testDigest), // key tag
			dsDataXML(2371, 100, 2, testDigest),           // algorithm
			dsDataXML(2371, 13, 99, testDigest),           // digest type
			dsDataXML(2371, 13, 2, testDigest[:62]),       // malformed digest
			dsDataXML(2371, 13, 2, testDigest[:61]+"XYZ"), // not hex
		} {
			refused(t, createXML("dnssec-bad.test", createOpts{ext: secDNSCreate(ds)}))
		}
	})
	t.Run("epp-14 non-existent registrant refused", func(t *testing.T) {
		refused(t, createXML("noreg1.test", createOpts{contacts: `<domain:registrant>rst-nobody</domain:registrant><domain:contact type="tech">registrar1-c2</domain:contact>`}))
	})
	t.Run("epp-14 missing registrant refused where the data model needs one", func(t *testing.T) {
		refused(t, createXML("noreg2.test", createOpts{contacts: `<domain:contact type="tech">registrar1-c2</domain:contact>`}))
	})
	t.Run("epp-14 registrant refused where the data model has none", func(t *testing.T) {
		refused(t, createXML("thick.thin", createOpts{}))
		requireCode(t, c.roundTrip(createXML("ok.thin", createOpts{contacts: " "})), "1000")
	})
	t.Run("epp-14 empty <pw> accepted (RFC 9154)", func(t *testing.T) {
		rstPending(t, "RFC 9154 is recommended by RST (warning only); not yet scheduled")
	})

	t.Run("epp-14 both registrars create domains", func(t *testing.T) {
		requireCode(t, c.roundTrip(createXML("rst-one.test", createOpts{ext: secDNSCreate(dsDataXML(2371, 13, 2, testDigest))})), "1000")
		c2 := dial(t, ts.addr)
		requireCode(t, c2.roundTrip(loginAsXML("registrar2", "secret-pw-02", "", "RST-14")), "1000")
		requireCode(t, c2.roundTrip(createXML("rst-two.test", createOpts{contacts: `<domain:registrant>registrar2-c1</domain:registrant><domain:contact type="tech">registrar2-c2</domain:contact>`})), "1000")
		info := c2.roundTrip(infoXML("rst-two.test", ""))
		assert.Equal(t, "registrar2", text(info, "//infData/clID"))
		assert.Equal(t, "registrar2", text(info, "//infData/crID"))
	})

	info := c.roundTrip(infoXML("rst-one.test", ""))
	requireCode(t, info, "1000")
	t.Run("epp-14 info: roid is valid", func(t *testing.T) {
		assert.Regexp(t, `^(\w|_){1,80}-\w{1,8}$`, text(info, "//infData/roid"))
	})
	t.Run("epp-14 info: roid ends in the registry's configured repository ID", func(t *testing.T) {
		assert.True(t, strings.HasSuffix(text(info, "//infData/roid"), "-"+testRepositoryID.String()),
			"roid %q, configured repository ID %q", text(info, "//infData/roid"), testRepositoryID)
	})
	t.Run("epp-14 info: roid's repository ID is registered with IANA", func(t *testing.T) {
		rstOperational(t, "ROID_REPOSITORY_ID is the operator's ID in the IANA EPP Repository Identifiers registry (ADR 0017)")
	})
	t.Run("epp-14 info: crDate and exDate present and valid", func(t *testing.T) {
		cr, err := time.Parse(time.RFC3339, text(info, "//infData/crDate"))
		require.NoError(t, err)
		ex, err := time.Parse(time.RFC3339, text(info, "//infData/exDate"))
		require.NoError(t, err)
		assert.WithinDuration(t, time.Now(), cr, time.Minute)
		assert.Equal(t, cr.AddDate(1, 0, 0), ex)
	})
	t.Run("epp-14 info: crID and clID are the creating registrar", func(t *testing.T) {
		assert.Equal(t, testRegistrar, text(info, "//infData/crID"))
		assert.Equal(t, testRegistrar, text(info, "//infData/clID"))
	})
	t.Run("epp-14 info: DNSSEC data present and correct", func(t *testing.T) {
		ds := info.FindElement("//extension/infData/dsData")
		require.NotNil(t, ds)
		assert.Equal(t, "2371", ds.SelectElement("keyTag").Text())
		assert.Equal(t, "13", ds.SelectElement("alg").Text())
		assert.Equal(t, "2", ds.SelectElement("digestType").Text())
		assert.Equal(t, testDigest, ds.SelectElement("digest").Text())
	})
}

// --- epp-17: Service Port consistency test --------------------------------

// Two servers over one registry stand in for two service ports behind one
// name: a domain created through one is answered identically by the other.
func TestRST_epp17_ServicePortConsistency(t *testing.T) {
	creds := newTestCredentials(t)
	a := startServerWith(t, testConfig(), listenLoopback(t), creds)
	cfgB := testConfig()
	cfgB.Sessions.NodeID = "node-b"
	b := startServerWith(t, cfgB, listenLoopback(t), creds)

	ca, cb := dial(t, a.addr), dial(t, b.addr)
	ca.login()
	cb.login()
	requireCode(t, ca.roundTrip(createXML("ports.test", createOpts{ns: `<domain:ns><domain:hostObj>ns1.registrar1.net</domain:hostObj></domain:ns>`})), "1000")

	infData := func(c *client) string {
		resp := c.roundTrip(infoXML("ports.test", ""))
		requireCode(t, resp, "1000")
		doc := etree.NewDocument()
		doc.SetRoot(resp.FindElement("//resData/infData").Copy())
		s, err := doc.WriteToString()
		require.NoError(t, err)
		return s
	}
	t.Run("epp-17 <info> is 1000 and its <infData> identical on every port", func(t *testing.T) {
		assert.Equal(t, infData(ca), infData(cb))
	})
}

// --- epp-16, epp-18, epp-21: domain update, renew and delete ---------------

// The tests follow epp-16's data providers (epp-16-status-data,
// epp-16-ns-data, epp-16-dnssec-data). The registry uses host objects and
// the DS data interface (epp.hostModel = objects, epp.secDNSInterfaces =
// dsData), so the rows for host attributes and key data apply only as
// refusals. RANDHEX(32) is read as 32 octets: the SHA-256 digest that the
// rows' digest type 2 names.

func TestRST_epp16_DomainUpdate(t *testing.T) {
	_, ts, c := domainServer(t)
	name := rstRandChars() + ".test"
	requireCode(t, c.roundTrip(createXML(name, createOpts{})), "1000")
	update := func(changes, ext string) string { return code(c.roundTrip(domainUpdateXML(name, changes, ext))) }
	statusEl := func(add bool, s string) string {
		if add {
			return `<domain:add><domain:status s="` + s + `"/></domain:add>`
		}
		return `<domain:rem><domain:status s="` + s + `"/></domain:rem>`
	}

	t.Run("epp-16 statuses (epp-16-status-data)", func(t *testing.T) {
		for i, row := range []struct {
			add    bool
			status string
			pass   bool
		}{
			{true, "clientDeleteProhibited", true},
			{true, "clientHold", true},
			{true, "clientRenewProhibited", true},
			{true, "clientTransferProhibited", true},
			{true, "clientUpdateProhibited", true},
			{true, "clientHold", false},
			{false, "clientUpdateProhibited", true},
			{true, "linked", false},
			{true, "ok", false},
			{true, "pendingCreate", false},
			{true, "pendingDelete", false},
			{true, "pendingTransfer", false},
			{true, "pendingUpdate", false},
			{true, "serverDeleteProhibited", false},
			{true, "serverRenewProhibited", false},
			{true, "serverTransferProhibited", false},
			{true, "serverUpdateProhibited", false},
			{true, "invalidStatusCode", false},
			{false, "clientDeleteProhibited", true},
			{false, "clientHold", true},
			{false, "clientRenewProhibited", true},
			{false, "clientTransferProhibited", true},
			{false, "invalidStatusCode", false},
		} {
			got := update(statusEl(row.add, row.status), "")
			if row.pass {
				assert.Equal(t, "1000", got, "row %d: add=%v %s", i+1, row.add, row.status)
			} else {
				assert.True(t, isRefusal(got), "row %d: add=%v %s: got %s", i+1, row.add, row.status, got)
			}
		}
	})
	t.Run("epp-16 nameservers (epp-16-ns-data, host objects)", func(t *testing.T) {
		host := "ns1.epp-16.rst." + rstRandChars() + ".icann"
		nsEl := func(add bool) string {
			el := "<domain:ns><domain:hostObj>" + host + "</domain:hostObj></domain:ns>"
			if add {
				return "<domain:add>" + el + "</domain:add>"
			}
			return "<domain:rem>" + el + "</domain:rem>"
		}
		assert.True(t, isRefusal(update(nsEl(true), "")), "a host object that does not exist")
		requireCode(t, c.roundTrip(hostCreateXML(host)), "1000")
		assert.Equal(t, "1000", update(nsEl(true), ""))
		assert.Contains(t, nameservers(c.roundTrip(infoXML(name, ""))), host)
		assert.True(t, isRefusal(update(nsEl(true), "")), "added twice")
		assert.Equal(t, "1000", update(nsEl(false), ""))
		assert.True(t, isRefusal(update(nsEl(false), "")), "removed twice")
		assert.True(t, isRefusal(update(`<domain:add><domain:ns><domain:hostAttr><domain:hostName>ns2.epp-16.rst.`+name+`</domain:hostName><domain:hostAddr ip="v4">208.77.190.196</domain:hostAddr></domain:hostAttr></domain:ns></domain:add>`, "")), "host attributes")
	})
	t.Run("epp-16 registrant", func(t *testing.T) {
		assert.Equal(t, "1000", update(`<domain:chg><domain:registrant>registrar1-c2</domain:registrant></domain:chg>`, ""))
		assert.Equal(t, "registrar1-c2", text(c.roundTrip(infoXML(name, "")), "//infData/registrant"))
	})
	t.Run("epp-16 DNSSEC (epp-16-dnssec-data, DS data)", func(t *testing.T) {
		ds := func(keyTag, alg, digestType, digest string) string {
			return `<secDNS:dsData><secDNS:keyTag>` + keyTag + `</secDNS:keyTag><secDNS:alg>` + alg + `</secDNS:alg><secDNS:digestType>` + digestType +
				`</secDNS:digestType><secDNS:digest>` + digest + `</secDNS:digest>` +
				`<secDNS:keyData><secDNS:flags>257</secDNS:flags><secDNS:protocol>3</secDNS:protocol><secDNS:alg>16</secDNS:alg><secDNS:pubKey>7JCMl8WwNOyFNWF6GBuMlIdtf08Cr1bO/hToZ6xCvKcu4o5ShXBzbCgzTGJHovhoUgj9wsMA1aWA</secDNS:pubKey></secDNS:keyData></secDNS:dsData>`
		}
		addDS := func(d string) string { return secDNSUpdateXML("", "<secDNS:add>"+d+"</secDNS:add>") }
		for _, row := range []struct{ name, ds string }{
			{"an empty key tag", ds("", "16", "2", testDigest)},
			{"key tag -1", ds("-1", "16", "2", testDigest)},
			{"key tag A", ds("A", "16", "2", testDigest)},
			{"algorithm empty", ds("2371", "", "2", testDigest)},
			{"algorithm -1", ds("2371", "-1", "2", testDigest)},
			{"algorithm A", ds("2371", "A", "2", testDigest)},
			{"algorithm 123", ds("2371", "123", "2", testDigest)},
			{"digest type empty", ds("2371", "16", "", testDigest)},
			{"digest type -1", ds("2371", "16", "-1", testDigest)},
			{"digest type A", ds("2371", "16", "A", testDigest)},
			{"digest type 255", ds("2371", "16", "255", testDigest)},
			{"an empty digest", ds("2371", "16", "2", "")},
		} {
			assert.True(t, isRefusal(update("", addDS(row.ds))), row.name)
		}
		valid := ds("2371", "16", "2", testDigest)
		assert.Equal(t, "1000", update("", addDS(valid)))
		assert.Len(t, c.roundTrip(infoXML(name, "")).FindElements("//extension/infData/dsData"), 1)
		assert.Equal(t, "1000", update("", secDNSUpdateXML("", "<secDNS:rem>"+valid+"</secDNS:rem>")))
		assert.True(t, isRefusal(update("", secDNSUpdateXML("", "<secDNS:rem>"+valid+"</secDNS:rem>"))), "removed twice")
		assert.True(t, isRefusal(update("", secDNSUpdateXML("", `<secDNS:rem><secDNS:keyData><secDNS:flags>257</secDNS:flags><secDNS:protocol>3</secDNS:protocol><secDNS:alg>13</secDNS:alg><secDNS:pubKey>uugEQOG7/3zv3rVGq43NQTqfmZxWn4fMIpi6ph7JKEvTRwhtw3aIV4dSk+fgpFFmORrKNznT7AIuJKy6P/Mi1Q==</secDNS:pubKey></secDNS:keyData></secDNS:rem>`))), "key data")
	})
	t.Run("epp-16 <update> of another registrar's domain is 2201", func(t *testing.T) {
		other := registrar2(t, ts)
		requireCode(t, other.roundTrip(domainUpdateXML(name, statusEl(true, "clientHold"), "")), "2201")
	})
}

func TestRST_epp18_DomainRenew(t *testing.T) {
	_, _, c := domainServer(t)
	name := rstRandChars() + ".test"
	requireCode(t, c.roundTrip(createXML(name, createOpts{})), "1000")
	exp := exDate(t, c.roundTrip(infoXML(name, "")))

	t.Run("epp-18 the expiry date moves by the period", func(t *testing.T) {
		requireCode(t, c.roundTrip(domainRenewXML(name, exp.Format("2006-01-02"), `<domain:period unit="y">2</domain:period>`)), "1000")
		exp = exp.AddDate(2, 0, 0)
		assert.Equal(t, exp, exDate(t, c.roundTrip(infoXML(name, ""))))
	})
	t.Run("epp-18 the domain is in its renewPeriod", func(t *testing.T) {
		info := c.roundTrip(infoXML(name, ""))
		var rgp []string
		for _, el := range info.FindElements("//extension/infData/rgpStatus") {
			rgp = append(rgp, el.SelectAttrValue("s", ""))
		}
		assert.Contains(t, rgp, "renewPeriod")
	})
	t.Run("epp-18 a renew past ten years from now is refused", func(t *testing.T) {
		assert.True(t, isRefusal(code(c.roundTrip(domainRenewXML(name, exp.Format("2006-01-02"), `<domain:period unit="y">8</domain:period>`)))))
		assert.Equal(t, exp, exDate(t, c.roundTrip(infoXML(name, ""))), "unchanged")
	})
}

func TestRST_epp21_DomainDelete(t *testing.T) {
	_, _, c := domainServer(t)
	name := rstRandChars() + ".test"
	requireCode(t, c.roundTrip(createXML(name, createOpts{})), "1000")
	got := code(c.roundTrip(domainDeleteXML(name)))
	t.Run("epp-21 <delete> is 1000 or 1001", func(t *testing.T) {
		assert.Contains(t, []string{"1000", "1001"}, got)
	})
	t.Run("epp-21 info: the domain is gone, or pendingDelete", func(t *testing.T) {
		info := c.roundTrip(infoXML(name, ""))
		if code(info) == "2303" {
			return
		}
		assert.Equal(t, "1001", got, "only a pending delete leaves the domain")
		assert.Contains(t, statusesOf(info), "pendingDelete")
	})
}

// --- epp-06 … epp-10: contact objects -------------------------------------

// This registry keeps contacts (general.registryDataModel is not minimum),
// so the contact cases apply. The tests follow the cases' data providers
// (epp-07-data, epp-09-postalinfo-data, epp-09-other-data,
// epp-09-status-data). The registry stores no phone extensions, so the
// operator declares epp.supportedContactElements as org, street, sp, pc,
// voice and fax, without voice:ext and fax:ext; the rows that set them are
// covered by the refusal of an extension. registrar2-c1, sponsored by
// registrar2, stands in for epp.registeredContacts.

// rstContactID stands in for an {AUTO} contact ID: 3 to 16 characters.
func rstContactID() string {
	id := rstRandChars()
	if len(id) > 16 {
		id = id[len(id)-16:]
	}
	return "c" + id[1:]
}

// rstAutoContact is the first row of epp-07-data: every supported element,
// with realistic values.
func rstAutoContact(id string) string {
	return contactCreateXML(id, contactOpts{
		postalInfo: postalInfoXML("int", "Ada Lovelace", "Analytical Engines Ltd.", "London", "GB", "12 St James's Square"),
		voice:      `<contact:voice>+44.2071234567</contact:voice>`,
		fax:        `<contact:fax>+44.2071234568</contact:fax>`,
		email:      "ada@example.com",
	})
}

func TestRST_epp06_ContactCheck(t *testing.T) {
	_, _, c := objectServer(t)
	avail := func(t *testing.T, id string) string {
		t.Helper()
		resp := c.roundTrip(contactCheckXML(id))
		if code := code(resp); code != "1000" {
			return code
		}
		el := resp.FindElement("//resData/chkData/cd/id")
		require.NotNil(t, el, "a <contact:id> for the ID checked")
		return el.SelectAttrValue("avail", "")
	}
	t.Run("epp-06 syntactically invalid IDs are unavailable or refused", func(t *testing.T) {
		for _, id := range []string{"", "ab", strings.Repeat("x", 17)} {
			assert.Contains(t, []string{"0", "false", "2001", "2004", "2005"}, avail(t, id), "%q", id)
		}
	})
	t.Run("epp-06 registered IDs are unavailable", func(t *testing.T) {
		assert.Equal(t, "0", avail(t, "registrar2-c1"))
		assert.Equal(t, "0", avail(t, "registrar1-c1"))
	})
	t.Run("epp-06 unregistered IDs are available", func(t *testing.T) {
		assert.Equal(t, "1", avail(t, rstContactID()))
	})
}

func TestRST_epp07_ContactCreate(t *testing.T) {
	_, _, c := objectServer(t)
	create := func(frame string) string { return code(c.roundTrip(frame)) }

	id := rstContactID()
	t.Run("epp-07 a contact with every supported element accepted", func(t *testing.T) {
		assert.Equal(t, "1000", create(rstAutoContact(id)))
	})
	t.Run("epp-07 info confirms the stored values", func(t *testing.T) {
		info := c.roundTrip(contactInfoXML(id))
		requireCode(t, info, "1000")
		assert.Equal(t, "Ada Lovelace", text(info, `//infData/postalInfo[@type='int']/name`))
		assert.Equal(t, "Analytical Engines Ltd.", text(info, `//infData/postalInfo[@type='int']/org`))
		assert.Equal(t, "12 St James's Square", text(info, `//infData/postalInfo[@type='int']/addr/street`))
		assert.Equal(t, "London", text(info, `//infData/postalInfo[@type='int']/addr/city`))
		assert.Equal(t, "VA", text(info, `//infData/postalInfo[@type='int']/addr/sp`))
		assert.Equal(t, "20166", text(info, `//infData/postalInfo[@type='int']/addr/pc`))
		assert.Equal(t, "GB", text(info, `//infData/postalInfo[@type='int']/addr/cc`))
		assert.Equal(t, "+44.2071234567", text(info, "//infData/voice"))
		assert.Equal(t, "+44.2071234568", text(info, "//infData/fax"))
		assert.Equal(t, "ada@example.com", text(info, "//infData/email"))
	})

	long := strings.Repeat("x", 256)
	pi := func(typ, name, org, city, cc string, streets ...string) contactOpts {
		return contactOpts{postalInfo: postalInfoXML(typ, name, org, city, cc, streets...)}
	}
	for _, row := range []struct {
		name  string
		frame string
	}{
		{"an empty ID", contactCreateXML("", contactOpts{})},
		{"an ID of 17 characters", contactCreateXML(strings.Repeat("a", 17), contactOpts{})},
		{"postal info type tni", contactCreateXML(rstContactID(), pi("tni", "Ada", "", "London", "GB"))},
		{"an empty postal info type", contactCreateXML(rstContactID(), pi("", "Ada", "", "London", "GB"))},
		{"an empty name", contactCreateXML(rstContactID(), pi("int", "", "", "London", "GB"))},
		{"an org of 256 characters", contactCreateXML(rstContactID(), pi("int", "Ada", long, "London", "GB"))},
		{"an empty city", contactCreateXML(rstContactID(), pi("int", "Ada", "", "", "GB"))},
		{"an empty country code", contactCreateXML(rstContactID(), pi("int", "Ada", "", "London", ""))},
		{"an empty voice number", contactCreateXML(rstContactID(), contactOpts{voice: `<contact:voice/>`})},
		{"an empty email", contactCreateXML(rstContactID(), contactOpts{email: " "})},
		{"a street of 256 characters", contactCreateXML(rstContactID(), pi("int", "Ada", "", "London", "GB", long))},
		{"a city of 256 characters", contactCreateXML(rstContactID(), pi("int", "Ada", "", long, "GB"))},
		{"country code XX", contactCreateXML(rstContactID(), pi("int", "Ada", "", "London", "XX"))},
		{"country code U", contactCreateXML(rstContactID(), pi("int", "Ada", "", "London", "U"))},
		{"country code USA", contactCreateXML(rstContactID(), pi("int", "Ada", "", "London", "USA"))},
	} {
		t.Run("epp-07 refused: "+row.name, func(t *testing.T) {
			assert.True(t, isRefusal(create(row.frame)))
		})
	}
	invalidPhones := []string{"+12345", "+1111.1", "55", "+1.", "+1.123456789012345", "+1.a", "+a.1"}
	t.Run("epp-07 refused: invalid voice numbers", func(t *testing.T) {
		for _, v := range invalidPhones {
			assert.True(t, isRefusal(create(contactCreateXML(rstContactID(), contactOpts{voice: `<contact:voice>` + v + `</contact:voice>`}))), v)
		}
	})
	t.Run("epp-07 refused: invalid fax numbers", func(t *testing.T) {
		for _, v := range invalidPhones {
			assert.True(t, isRefusal(create(contactCreateXML(rstContactID(), contactOpts{fax: `<contact:fax>` + v + `</contact:fax>`}))), v)
		}
	})
	t.Run("epp-07 refused: invalid email addresses", func(t *testing.T) {
		for _, e := range []string{"example", "example.example", "@example.com", "@", strings.Repeat("a", 65) + "@example.com"} {
			assert.True(t, isRefusal(create(contactCreateXML(rstContactID(), contactOpts{email: e}))), e)
		}
	})
	t.Run("epp-07 refused: a phone extension, which is not a supported element", func(t *testing.T) {
		assert.True(t, isRefusal(create(contactCreateXML(rstContactID(), contactOpts{voice: `<contact:voice x="1234">+44.2071234567</contact:voice>`}))))
	})
}

func TestRST_epp08_ContactAccessControl(t *testing.T) {
	_, ts, c := objectServer(t)
	id := rstContactID()
	requireCode(t, c.roundTrip(rstAutoContact(id)), "1000")
	other := registrar2(t, ts)
	t.Run("epp-08 <info> by a non-sponsor refused", func(t *testing.T) {
		assert.True(t, isRefusal(code(other.roundTrip(contactInfoXML(id)))))
	})
	t.Run("epp-08 <update> by a non-sponsor refused", func(t *testing.T) {
		assert.True(t, isRefusal(code(other.roundTrip(contactUpdateXML(id, "<contact:chg><contact:email>x@example.com</contact:email></contact:chg>")))))
		assert.True(t, isRefusal(code(other.roundTrip(contactUpdateXML(id, "<contact:add>"+contactStatusXML("clientDeleteProhibited")+"</contact:add>")))))
	})
}

func TestRST_epp09_ContactUpdate(t *testing.T) {
	_, _, c := objectServer(t)
	id := rstContactID()
	requireCode(t, c.roundTrip(rstAutoContact(id)), "1000")
	update := func(changes string) string { return code(c.roundTrip(contactUpdateXML(id, changes))) }
	chg := func(inner string) string { return "<contact:chg>" + inner + "</contact:chg>" }
	long := strings.Repeat("x", 256)

	t.Run("epp-09 postal info (epp-09-postalinfo-data)", func(t *testing.T) {
		for _, typ := range []string{"int", "loc"} {
			assert.Equal(t, "1000", update(chg(postalInfoXML(typ, "Charles Babbage", "Difference Engines Ltd.", "London", "GB", "1 Dorset Street"))), typ)
		}
		for _, row := range []struct{ name, frame string }{
			{"type tni", postalInfoXML("tni", "Charles", "", "London", "GB")},
			{"an empty type", postalInfoXML("", "Charles", "", "London", "GB")},
			{"an empty name", postalInfoXML("int", "", "", "London", "GB")},
			{"an org of 256 characters", postalInfoXML("int", "Charles", long, "London", "GB")},
			{"an empty city", postalInfoXML("int", "Charles", "", "", "GB")},
			{"an empty country code", postalInfoXML("int", "Charles", "", "London", "")},
			{"a street of 256 characters", postalInfoXML("int", "Charles", "", "London", "GB", long)},
			{"a city of 256 characters", postalInfoXML("int", "Charles", "", long, "GB")},
			{"country code XX", postalInfoXML("int", "Charles", "", "London", "XX")},
			{"country code U", postalInfoXML("int", "Charles", "", "London", "U")},
			{"country code USA", postalInfoXML("int", "Charles", "", "London", "USA")},
		} {
			assert.True(t, isRefusal(update(chg(row.frame))), row.name)
		}
	})
	t.Run("epp-09 voice, fax and email (epp-09-other-data)", func(t *testing.T) {
		assert.Equal(t, "1000", update(chg(`<contact:voice>+44.2079876543</contact:voice><contact:fax>+44.2079876544</contact:fax><contact:email>charles@example.com</contact:email>`)))
		for _, v := range []string{"+12345", "+1111.1", "55", "+1.", "+1.123456789012345", "+1.a", "+a.1"} {
			assert.True(t, isRefusal(update(chg(`<contact:voice>`+v+`</contact:voice>`))), "voice %s", v)
			assert.True(t, isRefusal(update(chg(`<contact:fax>`+v+`</contact:fax>`))), "fax %s", v)
		}
		for _, e := range []string{"example", "example.example", "@example.com", "@", strings.Repeat("a", 65) + "@example.com"} {
			assert.True(t, isRefusal(update(chg(`<contact:email>`+e+`</contact:email>`))), "email %s", e)
		}
	})
	t.Run("epp-09 statuses (epp-09-status-data)", func(t *testing.T) {
		for i, row := range []struct {
			add    bool
			status string
			pass   bool
		}{
			{true, "clientUpdateProhibited", true},
			{true, "clientUpdateProhibited", false},
			{false, "clientUpdateProhibited", true},
			{true, "clientDeleteProhibited", true},
			{true, "clientDeleteProhibited", false},
			{true, "clientHold", false},
			{true, "linked", false},
			{true, "ok", false},
			{true, "pendingCreate", false},
			{true, "pendingDelete", false},
			{true, "pendingTransfer", false},
			{true, "pendingUpdate", false},
			{true, "serverDeleteProhibited", false},
			{true, "serverUpdateProhibited", false},
			{true, "invalidStatusCode", false},
			{false, "clientUpdateProhibited", false},
			{false, "clientDeleteProhibited", true},
			{false, "clientDeleteProhibited", false},
			{false, "ok", false},
			{false, "pendingCreate", false},
			{false, "pendingDelete", false},
			{false, "pendingTransfer", false},
			{false, "pendingUpdate", false},
			{false, "serverDeleteProhibited", false},
			{false, "serverUpdateProhibited", false},
			{false, "invalidStatusCode", false},
		} {
			el := "<contact:rem>" + contactStatusXML(row.status) + "</contact:rem>"
			if row.add {
				el = "<contact:add>" + contactStatusXML(row.status) + "</contact:add>"
			}
			got := update(el)
			if row.pass {
				assert.Equal(t, "1000", got, "row %d: add=%v %s", i+1, row.add, row.status)
			} else {
				assert.True(t, isRefusal(got), "row %d: add=%v %s: got %s", i+1, row.add, row.status, got)
			}
		}
	})
	t.Run("epp-09 info confirms the stored values", func(t *testing.T) {
		info := c.roundTrip(contactInfoXML(id))
		requireCode(t, info, "1000")
		for _, typ := range []string{"int", "loc"} {
			assert.Equal(t, "Charles Babbage", text(info, `//infData/postalInfo[@type='`+typ+`']/name`))
			assert.Equal(t, "Difference Engines Ltd.", text(info, `//infData/postalInfo[@type='`+typ+`']/org`))
		}
		assert.Equal(t, "+44.2079876543", text(info, "//infData/voice"))
		assert.Equal(t, "charles@example.com", text(info, "//infData/email"))
		assert.Equal(t, []string{"ok"}, statuses(info))
	})
}

func TestRST_epp10_ContactDelete(t *testing.T) {
	_, _, c := objectServer(t)
	id := rstContactID()
	requireCode(t, c.roundTrip(rstAutoContact(id)), "1000")
	t.Run("epp-10 <delete> is 1000 or 1001", func(t *testing.T) {
		assert.Contains(t, []string{"1000", "1001"}, code(c.roundTrip(contactDeleteXML(id))))
	})
	t.Run("epp-10 info confirms the contact is gone", func(t *testing.T) {
		requireCode(t, c.roundTrip(contactInfoXML(id)), "2303")
	})
}

// --- epp-05, epp-11 … epp-13, epp-15, epp-23 … epp-27: host objects -------

// This registry uses host objects (epp.hostModel = objects), so the host
// cases apply. The tests follow the cases' data providers (epp-11-data,
// epp-13-status-data, epp-13-addr-data, epp-23-data in the pinned release),
// with RST's address prefixes. .test stands in for each TLD in the TLD set,
// .icann for a name outside it, and registered.test, sponsored by
// registrar2, for epp.registeredNames.

// rstRandChars stands in for RANDCHARS(18): a fresh label for each call.
func rstRandChars() string {
	return "r" + strconv.FormatInt(time.Now().UnixNano()+rstRandSeq.Add(1), 36)
}

var rstRandSeq atomic.Int64

// isRefusal reports whether a result code refuses the command (2xxx).
func isRefusal(code string) bool { return strings.HasPrefix(code, "2") }

func TestRST_epp05_HostCheck(t *testing.T) {
	creds, _, c := objectServer(t)
	// epp.registeredNameservers: one host subordinate to each TLD in the
	// set, sponsored by another registrar.
	creds.domains.registry.AddHost("ns1.registered.test", "registrar2", testV4)
	avail := func(t *testing.T, name string) string {
		t.Helper()
		resp := c.roundTrip(hostCheckXML(name))
		if code := code(resp); code != "1000" {
			return code
		}
		el := resp.FindElement("//resData/chkData/cd/name")
		require.NotNil(t, el, "a <host:name> for the name checked")
		return el.SelectAttrValue("avail", "")
	}
	t.Run("epp-05 syntactically invalid hostnames are unavailable or refused", func(t *testing.T) {
		for _, name := range []string{"!.epp-05.test", "-leading.example.net", "dou..ble.example.net", "under_score.example.net", "nodot"} {
			assert.Contains(t, []string{"0", "false", "2001", "2004", "2005"}, avail(t, name), "%q", name)
		}
	})
	t.Run("epp-05 registered hostnames are unavailable", func(t *testing.T) {
		assert.Equal(t, "0", avail(t, "ns1.registered.test"))
	})
	t.Run("epp-05 unregistered hostnames are available", func(t *testing.T) {
		assert.Equal(t, "1", avail(t, "ns1."+rstRandChars()+".test"))
		assert.Equal(t, "1", avail(t, "ns1."+rstRandChars()+".icann"))
	})
}

func TestRST_epp11_HostCreate(t *testing.T) {
	_, _, c := objectServer(t)
	create := func(name string, addrs ...string) string { return code(c.roundTrip(hostCreateXML(name, addrs...))) }

	external := "ns1.epp-11.rst." + rstRandChars() + ".icann"
	t.Run("epp-11 external host without addresses accepted", func(t *testing.T) {
		assert.Equal(t, "1000", create(external))
	})
	t.Run("epp-11 invalid hostname refused", func(t *testing.T) {
		assert.True(t, isRefusal(create("!.epp-11.rst."+rstRandChars()+".icann")))
	})
	parent := rstRandChars() + ".test"
	ns1 := "ns1.epp-11.rst." + parent
	t.Run("epp-11 internal host whose superordinate domain does not exist refused", func(t *testing.T) {
		assert.True(t, isRefusal(create(ns1)))
	})
	requireCode(t, c.roundTrip(createXML(parent, createOpts{})), "1000")
	t.Run("epp-11 internal host with an IPv4 address accepted", func(t *testing.T) {
		assert.Equal(t, "1000", create(ns1, addrXML("v4", "208.77.190.196")))
	})
	ns2 := "ns2.epp-11.rst." + parent
	t.Run("epp-11 invalid IPv4 addresses refused", func(t *testing.T) {
		for _, a := range []string{"", "2602:800:900e:1257::4", "208.77.190", "a.b.c.d", "256.1.1.1", "1.2.3.4.5", "127.0.0.1"} {
			assert.True(t, isRefusal(create(ns2, addrXML("v4", a))), "%q", a)
		}
		assert.True(t, isRefusal(create(ns2, addrXML("v5", "208.77.190.197"))), "ip=v5")
	})
	ns3 := "ns3.epp-11.rst." + parent
	t.Run("epp-11 invalid IPv6 addresses refused", func(t *testing.T) {
		for _, a := range []string{"", "208.77.190.198", "::", "::1", "::X", "NOT::HEX::"} {
			assert.True(t, isRefusal(create(ns3, addrXML("v6", a))), "%q", a)
		}
	})
	t.Run("epp-11 internal host with an IPv6 address accepted", func(t *testing.T) {
		assert.Equal(t, "1000", create(ns3, addrXML("v6", "2602:800:900e:1257::5")))
	})
	t.Run("epp-11 info confirms the stored values", func(t *testing.T) {
		for name, want := range map[string]map[string]string{
			external: {},
			ns1:      {"208.77.190.196": "v4"},
			ns3:      {"2602:800:900e:1257::5": "v6"},
		} {
			info := c.roundTrip(hostInfoXML(name))
			requireCode(t, info, "1000")
			assert.Equal(t, name, text(info, "//infData/name"))
			assert.Equal(t, want, addresses(info), name)
			assert.Equal(t, testRegistrar, text(info, "//infData/clID"))
		}
	})
}

func TestRST_epp12_HostAccessControl(t *testing.T) {
	_, ts, c := objectServer(t)
	parent := rstRandChars() + ".test"
	requireCode(t, c.roundTrip(createXML(parent, createOpts{})), "1000")
	internal, external := "ns1."+parent, "ns1.epp-12.rst."+rstRandChars()+".icann"
	requireCode(t, c.roundTrip(hostCreateXML(internal, addrXML("v4", testV4))), "1000")
	requireCode(t, c.roundTrip(hostCreateXML(external)), "1000")

	other := registrar2(t, ts)
	t.Run("epp-12 <update> by a non-sponsor is 2201", func(t *testing.T) {
		for _, name := range []string{internal, external} {
			for _, changes := range []string{
				addXML(statusXML("clientUpdateProhibited")),
				addXML(addrXML("v4", testV4b)),
				chgXML("ns9.epp-12.rst." + rstRandChars() + ".icann"),
			} {
				requireCode(t, other.roundTrip(hostUpdateXML(name, changes)), "2201")
			}
		}
	})
}

func TestRST_epp13_HostUpdate(t *testing.T) {
	_, _, c := objectServer(t)
	parent := rstRandChars() + ".test"
	requireCode(t, c.roundTrip(createXML(parent, createOpts{})), "1000")
	host := "ns1.epp-13.rst." + parent
	requireCode(t, c.roundTrip(hostCreateXML(host, addrXML("v4", testV4))), "1000")
	update := func(changes string) string { return code(c.roundTrip(hostUpdateXML(host, changes))) }

	t.Run("epp-13 status changes (epp-13-status-data)", func(t *testing.T) {
		for _, row := range []struct {
			add    bool
			status string
			pass   bool
		}{
			{true, "clientUpdateProhibited", true},
			{false, "clientUpdateProhibited", true},
			{true, "clientDeleteProhibited", true},
			{true, "clientHold", false},
			{true, "linked", false},
			{true, "ok", false},
			{true, "pendingCreate", false},
			{true, "pendingDelete", false},
			{true, "pendingTransfer", false},
			{true, "pendingUpdate", false},
			{true, "serverDeleteProhibited", false},
			{true, "serverUpdateProhibited", false},
			{true, "invalidStatusCode", false},
			{false, "clientDeleteProhibited", true},
			{false, "clientHold", false},
			{false, "linked", false},
			{false, "ok", false},
			{false, "pendingCreate", false},
			{false, "pendingDelete", false},
			{false, "pendingTransfer", false},
			{false, "pendingUpdate", false},
			{false, "serverDeleteProhibited", false},
			{false, "serverUpdateProhibited", false},
			{false, "invalidStatusCode", false},
		} {
			changes := remXML(statusXML(row.status))
			if row.add {
				changes = addXML(statusXML(row.status))
			}
			got := update(changes)
			if row.pass {
				assert.Equal(t, "1000", got, "add=%v %s", row.add, row.status)
				if row.add {
					assert.Contains(t, statuses(c.roundTrip(hostInfoXML(host))), row.status)
				}
			} else {
				assert.True(t, isRefusal(got), "add=%v %s: got %s", row.add, row.status, got)
			}
		}
	})
	t.Run("epp-13 address changes (epp-13-addr-data)", func(t *testing.T) {
		for _, row := range []struct {
			add      bool
			ip, addr string
			pass     bool
		}{
			{true, "v4", "208.77.190.192", true},
			{true, "v4", "", false},
			{true, "v4", "2602:800:900e:1257::1", false},
			{true, "v4", "208.77.190", false},
			{true, "v4", "a.b.c.d", false},
			{true, "v4", "256.1.1.1", false},
			{true, "v4", "1.2.3.4.5", false},
			{true, "v5", "208.77.190.193", false},
			{true, "v6", "2602:800:900e:1257::2", true},
			{true, "v6", "208.77.190.194", false},
			{true, "v6", "::", false},
			{true, "v6", "::1", false},
			{true, "v6", "::X", false},
			{true, "v6", "NOT::HEX::", false},
			{false, "v4", "208.77.190.195", false},
			{false, "v6", "2602:800:900e:1257::3", false},
		} {
			changes := remXML(addrXML(row.ip, row.addr))
			if row.add {
				changes = addXML(addrXML(row.ip, row.addr))
			}
			got := update(changes)
			if row.pass {
				assert.Equal(t, "1000", got, "add=%v %s %q", row.add, row.ip, row.addr)
			} else {
				assert.True(t, isRefusal(got), "add=%v %s %q: got %s", row.add, row.ip, row.addr, got)
			}
		}
	})
	t.Run("epp-13 info confirms the stored values", func(t *testing.T) {
		info := c.roundTrip(hostInfoXML(host))
		requireCode(t, info, "1000")
		assert.Equal(t, map[string]string{testV4: "v4", "208.77.190.192": "v4", "2602:800:900e:1257::2": "v6"}, addresses(info))
		assert.Equal(t, []string{"ok"}, statuses(info))
	})
}

func TestRST_epp15_ObjectIntegrity(t *testing.T) {
	_, _, c := objectServer(t)
	host := "ns1.epp-15.rst." + rstRandChars() + ".icann"
	requireCode(t, c.roundTrip(hostCreateXML(host)), "1000")
	requireCode(t, c.roundTrip(createXML(rstRandChars()+".test", createOpts{ns: `<domain:ns><domain:hostObj>` + host + `</domain:hostObj></domain:ns>`})), "1000")
	t.Run("epp-15 <delete> of a linked host object refused", func(t *testing.T) {
		assert.True(t, isRefusal(code(c.roundTrip(hostDeleteXML(host)))))
		requireCode(t, c.roundTrip(hostInfoXML(host)), "1000")
	})
	t.Run("epp-15 <delete> of a linked contact object refused", func(t *testing.T) {
		id := rstContactID()
		requireCode(t, c.roundTrip(rstAutoContact(id)), "1000")
		requireCode(t, c.roundTrip(createXML(rstRandChars()+".test", createOpts{
			contacts: `<domain:registrant>` + id + `</domain:registrant><domain:contact type="tech">` + id + `</domain:contact>`,
		})), "1000")
		assert.True(t, isRefusal(code(c.roundTrip(contactDeleteXML(id)))))
		requireCode(t, c.roundTrip(contactInfoXML(id)), "1000")
	})
}

func TestRST_epp23_HostRename(t *testing.T) {
	creds, _, c := objectServer(t)
	parent := rstRandChars() + ".test"
	requireCode(t, c.roundTrip(createXML(parent, createOpts{})), "1000")
	host := "ns1.epp-23.rst." + parent
	requireCode(t, c.roundTrip(hostCreateXML(host, addrXML("v4", testV4))), "1000")
	rename := func(to string) string {
		got := code(c.roundTrip(hostUpdateXML(host, chgXML(to))))
		if got == "1000" {
			host = to
		}
		return got
	}

	t.Run("epp-23 rename to an invalid hostname refused", func(t *testing.T) {
		assert.True(t, isRefusal(rename("!.invalid")))
	})
	t.Run("epp-23 rename to an external name accepted", func(t *testing.T) {
		assert.Equal(t, "1000", rename("ns1.epp-23.rst."+rstRandChars()+".icann"))
	})
	t.Run("epp-23 rename into another registrar's domain refused", func(t *testing.T) {
		check := c.roundTrip(checkNamesXML("registered.test"))
		require.Equal(t, "0", check.FindElement("//resData/chkData/cd/name").SelectAttrValue("avail", ""), "registered.test exists")
		assert.True(t, isRefusal(rename("ns1.epp-23.rst."+rstRandChars()+".registered.test")))
	})
	newParent := rstRandChars() + ".test"
	t.Run("epp-23 rename into a non-existent domain refused", func(t *testing.T) {
		assert.True(t, isRefusal(rename("ns1.epp-23.rst."+newParent)))
	})
	t.Run("epp-23 rename into the client's own domain accepted", func(t *testing.T) {
		requireCode(t, c.roundTrip(createXML(newParent, createOpts{})), "1000")
		assert.Equal(t, "1000", rename("ns1.epp-23.rst."+newParent))
	})
	t.Run("epp-23 info confirms the new name", func(t *testing.T) {
		info := c.roundTrip(hostInfoXML(host))
		requireCode(t, info, "1000")
		assert.Equal(t, host, text(info, "//infData/name"))
	})
	t.Run("epp-23 rename of an external host linked to another client's objects is 2305", func(t *testing.T) {
		// Through EPP a registrar's domains use only its own hosts; a
		// deposit imported from a registry with shared hosts can link one
		// registrar's host to another's domain.
		external := "ns2.epp-23.rst." + rstRandChars() + ".icann"
		requireCode(t, c.roundTrip(hostCreateXML(external)), "1000")
		reg := creds.domains.registry
		reg.AddDomain(&entities.Domain{Name: entities.DomainName(rstRandChars() + ".test"), ClID: "registrar2",
			RoID: entities.RoidType("23_DOM-" + testRepositoryID), Hosts: []*entities.Host{reg.Host(external, testRegistrar)}})
		requireCode(t, c.roundTrip(hostUpdateXML(external, chgXML("ns3.epp-23.rst."+rstRandChars()+".icann"))), "2305")
	})
}

func TestRST_epp24_HostDelete(t *testing.T) {
	_, _, c := objectServer(t)
	host := "ns1.epp-24.rst." + rstRandChars() + ".icann"
	requireCode(t, c.roundTrip(hostCreateXML(host)), "1000")
	t.Run("epp-24 <delete> is 1000 or 1001", func(t *testing.T) {
		assert.Contains(t, []string{"1000", "1001"}, code(c.roundTrip(hostDeleteXML(host))))
	})
	t.Run("epp-24 info confirms the host is gone", func(t *testing.T) {
		requireCode(t, c.roundTrip(hostInfoXML(host)), "2303")
	})
}

// rstVictimDomain is a domain registrar1 creates and delegates to external
// hosts, under which registrar2 then tries to create hosts (epp-25 to
// epp-27).
func rstVictimDomain(t *testing.T, c *client) string {
	t.Helper()
	ns := "ns1.epp-26.rst." + rstRandChars() + ".icann"
	requireCode(t, c.roundTrip(hostCreateXML(ns)), "1000")
	parent := rstRandChars() + ".test"
	requireCode(t, c.roundTrip(createXML(parent, createOpts{ns: `<domain:ns><domain:hostObj>` + ns + `</domain:hostObj></domain:ns>`})), "1000")
	return parent
}

func TestRST_epp25_SubordinateHostCreate(t *testing.T) {
	_, ts, c := objectServer(t)
	parent := rstVictimDomain(t, c)
	other := registrar2(t, ts)
	t.Run("epp-25 a non-sponsor's subordinate host with addresses refused", func(t *testing.T) {
		assert.True(t, isRefusal(code(other.roundTrip(hostCreateXML("ns1.epp-25."+parent, addrXML("v4", testV4), addrXML("v6", testV6))))))
	})
	t.Run("epp-25 a non-sponsor's subordinate host without addresses: refused, or sponsored by the domain's sponsor", func(t *testing.T) {
		name := "ns2.epp-25." + parent
		got := code(other.roundTrip(hostCreateXML(name)))
		if got == "1000" {
			assert.Equal(t, testRegistrar, text(c.roundTrip(hostInfoXML(name)), "//infData/clID"))
			return
		}
		assert.True(t, isRefusal(got))
	})
}

func TestRST_epp26_WideGlue(t *testing.T) {
	_, ts, c := objectServer(t)
	parent := rstVictimDomain(t, c)
	t.Run("epp-26 a non-sponsor's internal host with glue refused", func(t *testing.T) {
		assert.True(t, isRefusal(code(registrar2(t, ts).roundTrip(hostCreateXML("ns1.epp-26."+parent, addrXML("v4", testV4))))))
	})
}

func TestRST_epp27_GluelessInternalHost(t *testing.T) {
	_, ts, c := objectServer(t)
	parent := rstVictimDomain(t, c)
	t.Run("epp-27 a non-sponsor's glueless internal host refused", func(t *testing.T) {
		// Refused at <create>, so the sponsor never gets to delegate to it.
		assert.True(t, isRefusal(code(registrar2(t, ts).roundTrip(hostCreateXML("ns1.epp-27."+parent)))))
	})
}

// --- Pending: domain transfer --------------------------------------------

// rstPendingCases are the StandardEPP cases with no implementation to test
// yet, each with the phase that delivers it. TestRST_EPPCatalogueIsComplete
// keeps this list and the pinned spec in step.
var rstPendingCases = map[string]string{
	"epp-19": "Phase 4h: domain <transfer> request and approval; pendingTransfer within 120s; RFC 9154 authInfo",
	"epp-20": "Phase 4h: domain <transfer> rejection",
}

func TestRST_EPPPendingCases(t *testing.T) {
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
var rstImplementedCases = []string{
	"epp-01", "epp-02", "epp-03", "epp-04", "epp-05", "epp-06", "epp-07", "epp-08", "epp-09", "epp-10",
	"epp-11", "epp-12", "epp-13", "epp-14", "epp-15", "epp-16", "epp-17", "epp-18", "epp-21", "epp-23", "epp-24",
	"epp-25", "epp-26", "epp-27",
}

// TestRST_EPPCatalogueIsComplete fails when the pinned spec and this file
// disagree: a case in the spec that is neither implemented nor pending, or a
// case here that the spec no longer has. It is what makes moving to a newer
// RST release (testdata/rst/PROVENANCE.md) surface every new case.
func TestRST_EPPCatalogueIsComplete(t *testing.T) {
	raw, err := os.ReadFile("testdata/rst/epp-cases.yaml")
	require.NoError(t, err)
	var spec map[string]any
	require.NoError(t, yaml.Unmarshal(raw, &spec))

	accounted := map[string]bool{}
	for _, id := range rstImplementedCases {
		accounted[id] = true
	}
	for id := range rstPendingCases {
		require.False(t, accounted[id], "%s is both implemented and pending", id)
		accounted[id] = true
	}

	for id := range spec {
		assert.True(t, accounted[id], "RST case %s is in the pinned spec but not in rst_epp_test.go", id)
	}
	for id := range accounted {
		_, ok := spec[id]
		assert.True(t, ok, "%s is accounted for here but is not in the pinned spec", id)
	}
}

func texts(els []*etree.Element) []string {
	out := make([]string, 0, len(els))
	for _, e := range els {
		out = append(out, strings.TrimSpace(e.Text()))
	}
	return out
}

func contains(list []string, v string) bool {
	for _, s := range list {
		if s == v {
			return true
		}
	}
	return false
}
