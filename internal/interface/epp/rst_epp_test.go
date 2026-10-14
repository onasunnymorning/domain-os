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

// --- epp-05 … epp-27: object commands -------------------------------------

// rstPendingCases are the StandardEPP cases with no implementation to test
// yet, each with the phase that delivers it. TestRST_EPPCatalogueIsComplete
// keeps this list and the pinned spec in step.
var rstPendingCases = map[string]string{
	"epp-05": "object commands epic: host <check>",
	"epp-06": "object commands epic: contact <check> (if the registry data model needs contacts)",
	"epp-07": "object commands epic: contact <create>",
	"epp-08": "object commands epic: contact <info>/<update> refused to non-sponsors with 2201 (ADR 0006)",
	"epp-09": "object commands epic: contact <update>",
	"epp-10": "object commands epic: contact <delete>",
	"epp-11": "object commands epic: host <create>",
	"epp-12": "object commands epic: host <update> refused to non-sponsors with 2201 (ADR 0006)",
	"epp-13": "object commands epic: host <update>",
	"epp-15": "object commands epic: deleting a linked host or contact is refused",
	"epp-16": "object commands epic: domain <update> (statuses, nameservers, registrant, DNSSEC); 2201 to non-sponsors",
	"epp-18": "object commands epic: domain <renew>",
	"epp-19": "object commands epic: domain <transfer> request and approval; pendingTransfer within 120s; RFC 9154 authInfo",
	"epp-20": "object commands epic: domain <transfer> rejection",
	"epp-21": "object commands epic: domain <delete> and pendingDelete/purge",
	"epp-23": "object commands epic: host rename rules, including 2305 for external hosts linked to other sponsors",
	"epp-24": "object commands epic: host <delete>",
	"epp-25": "object commands epic: subordinate host <create>",
	"epp-26": "object commands epic: wide-glue host <create> refused to non-sponsors of the parent domain",
	"epp-27": "object commands epic: glueless internal host access control",
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
var rstImplementedCases = []string{"epp-01", "epp-02", "epp-03", "epp-04", "epp-14", "epp-17"}

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
