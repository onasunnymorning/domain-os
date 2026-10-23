package epp

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/beevik/etree"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
)

// createOpts are the parts of a <domain:create> a test varies.
type createOpts struct {
	period   string // e.g. `<domain:period unit="y">2</domain:period>`
	ns       string // the <domain:ns> element, whole
	contacts string // registrant and contact elements
	pw       string
	ext      string // the <extension> element's content
}

func createXML(name string, o createOpts) string {
	if o.contacts == "" {
		o.contacts = `<domain:registrant>registrar1-c1</domain:registrant><domain:contact type="tech">registrar1-c2</domain:contact>`
	}
	if o.pw == "" {
		o.pw = "Tr4nsf3r-C0de-Xq9#Lm2!"
	}
	ext := ""
	if o.ext != "" {
		ext = "<extension>" + o.ext + "</extension>"
	}
	return cmdXML(`<create><domain:create xmlns:domain="urn:ietf:params:xml:ns:domain-1.0">
<domain:name>`+name+`</domain:name>`+o.period+o.ns+o.contacts+`
<domain:authInfo><domain:pw>`+o.pw+`</domain:pw></domain:authInfo></domain:create></create>`+ext, "CRE-1")
}

func infoXML(name, hosts string) string {
	attr := ""
	if hosts != "" {
		attr = ` hosts="` + hosts + `"`
	}
	return cmdXML(`<info><domain:info xmlns:domain="urn:ietf:params:xml:ns:domain-1.0"><domain:name`+attr+`>`+name+`</domain:name></domain:info></info>`, "INF-1")
}

// msg is a response's result message.
func msg(doc *etree.Document) string { return text(doc, "//response/result/msg") }

func TestDomainCreateThenInfo(t *testing.T) {
	creds := newTestCredentials(t)
	ts := startServerWith(t, testConfig(), listenLoopback(t), creds)
	c := dial(t, ts.addr)
	requireCode(t, c.roundTrip(loginWithServices([]string{nsDomain}, []string{nsRGP})), "1000")

	resp := c.roundTrip(createXML("new.test", createOpts{
		period: `<domain:period unit="y">2</domain:period>`,
		ns:     `<domain:ns><domain:hostObj>ns1.registrar1.net</domain:hostObj><domain:hostObj>NS2.registrar1.net</domain:hostObj></domain:ns>`,
		contacts: `<domain:registrant>registrar1-c1</domain:registrant><domain:contact type="admin">registrar1-c1</domain:contact>
<domain:contact type="tech">registrar1-c2</domain:contact>`,
	}))
	requireCode(t, resp, "1000")
	assert.Equal(t, "new.test", text(resp, "//resData/creData/name"))
	crDate, err := time.Parse(time.RFC3339, text(resp, "//resData/creData/crDate"))
	require.NoError(t, err)
	exDate, err := time.Parse(time.RFC3339, text(resp, "//resData/creData/exDate"))
	require.NoError(t, err)
	assert.Equal(t, crDate.AddDate(2, 0, 0), exDate)

	info := c.roundTrip(infoXML("new.test", ""))
	requireCode(t, info, "1000")
	inf := info.FindElement("//resData/infData")
	require.NotNil(t, inf)
	assert.Equal(t, "new.test", text(info, "//infData/name"))
	assert.Regexp(t, `^\w+_DOM-\w+$`, text(info, "//infData/roid"))
	assert.Equal(t, "registrar1-c1", text(info, "//infData/registrant"))
	contacts := map[string]string{}
	for _, el := range inf.SelectElements("contact") {
		contacts[el.SelectAttrValue("type", "")] = el.Text()
	}
	assert.Equal(t, map[string]string{"admin": "registrar1-c1", "tech": "registrar1-c2"}, contacts)
	assert.Equal(t, []string{"ns1.registrar1.net", "ns2.registrar1.net"}, texts(inf.FindElements("ns/hostObj")))
	assert.Equal(t, testRegistrar, text(info, "//infData/clID"))
	assert.Equal(t, testRegistrar, text(info, "//infData/crID"))
	assert.Equal(t, text(resp, "//resData/creData/crDate"), text(info, "//infData/crDate"))
	assert.Equal(t, text(resp, "//resData/creData/exDate"), text(info, "//infData/exDate"))
	require.NotNil(t, info.FindElement("//infData/authInfo/pw"), "the sponsor is told a code is set (RFC 9154 §5.3)")
	assert.Empty(t, text(info, "//infData/authInfo/pw"), "but never sees it")
	var statuses []string
	for _, el := range inf.SelectElements("status") {
		statuses = append(statuses, el.SelectAttrValue("s", ""))
	}
	assert.Contains(t, statuses, "ok")
	assert.NotContains(t, statuses, "inactive", "it has nameservers")
	rgp := info.FindElement("//extension/infData/rgpStatus")
	require.NotNil(t, rgp, "logged in with RGP: the grace period is reported")
	assert.Equal(t, entities.RGPStatusAddPeriod, rgp.SelectAttrValue("s", ""))

	t.Run("the hosts attribute selects nameservers and subordinate hosts", func(t *testing.T) {
		creds.domains.registry.AddHost("ns1.new.test", testRegistrar)
		all := c.roundTrip(infoXML("new.test", "all"))
		assert.Len(t, all.FindElements("//infData/ns/hostObj"), 2)
		assert.Equal(t, []string{"ns1.new.test"}, texts(all.FindElements("//infData/host")))
		del := c.roundTrip(infoXML("new.test", "del"))
		assert.Len(t, del.FindElements("//infData/ns/hostObj"), 2)
		assert.Empty(t, del.FindElements("//infData/host"))
		sub := c.roundTrip(infoXML("new.test", "sub"))
		assert.Nil(t, sub.FindElement("//infData/ns"))
		assert.Len(t, sub.FindElements("//infData/host"), 1)
		none := c.roundTrip(infoXML("new.test", "none"))
		assert.Nil(t, none.FindElement("//infData/ns"))
		assert.Empty(t, none.FindElements("//infData/host"))
	})

	t.Run("without RGP at login, no RGP statuses", func(t *testing.T) {
		plain := dial(t, ts.addr)
		plain.login()
		resp := plain.roundTrip(infoXML("new.test", ""))
		requireCode(t, resp, "1000")
		assert.Nil(t, resp.FindElement("//response/extension"))
	})

	t.Run("the create is audited before its response, with the authInfo redacted", func(t *testing.T) {
		var create *entities.EPPTransaction
		for _, tx := range creds.audit.All() {
			if tx.Command == "domain:create" {
				create = &tx
			}
		}
		require.NotNil(t, create, "a transform's record is written before it is answered")
		assert.Equal(t, []string{"new.test"}, create.Objects)
		assert.NotContains(t, create.Request, "Tr4nsf3r-C0de-Xq9#Lm2!")
		assert.Contains(t, create.Response, "<domain:creData")
	})
}

func TestDomainInfoOnlyForTheSponsor(t *testing.T) {
	ts := startServer(t, testConfig())
	c := dial(t, ts.addr)
	c.login()

	resp := c.roundTrip(infoXML("registered.test", ""))
	requireCode(t, resp, "2201")
	assert.NotContains(t, msg(resp), "registrar2", "the sponsor is not disclosed")
	requireCode(t, c.roundTrip(infoXML("absent.test", "")), "2303")
	requireCode(t, c.roundTrip(infoXML("-bad-.test", "")), "2005")
}

func TestDomainCreateRefusals(t *testing.T) {
	creds := newTestCredentials(t)
	ts := startServerWith(t, testConfig(), listenLoopback(t), creds)
	c := dial(t, ts.addr)
	c.login()

	cases := []struct {
		name, frame, code, msgPart string
	}{
		{"a period in months that is not whole years", createXML("m13.test", createOpts{period: `<domain:period unit="m">13</domain:period>`}), "2306", "whole years"},
		{"a period over the maximum", createXML("y11.test", createOpts{period: `<domain:period unit="y">11</domain:period>`}), "2004", "1 to 10 years"},
		{"a period of zero, which the schema refuses", createXML("y0.test", createOpts{period: `<domain:period unit="y">0</domain:period>`}), "2001", "period"},
		{"host attributes", createXML("attr.test", createOpts{ns: `<domain:ns><domain:hostAttr><domain:hostName>ns1.attr.test</domain:hostName></domain:hostAttr></domain:ns>`}), "2306", "host objects"},
		{"two contacts of one type", createXML("two.test", createOpts{contacts: `<domain:registrant>registrar1-c1</domain:registrant>
<domain:contact type="tech">registrar1-c1</domain:contact><domain:contact type="tech">registrar1-c2</domain:contact>`}), "2306", "one contact of each type"},
		{"a contact without a type", createXML("untyped.test", createOpts{contacts: `<domain:registrant>registrar1-c1</domain:registrant><domain:contact>registrar1-c2</domain:contact>`}), "2003", "type"},
		{"no registrant, which the phase requires", createXML("noreg.test", createOpts{contacts: `<domain:contact type="tech">registrar1-c2</domain:contact>`}), "2003", "registrant"},
		{"an invalid name", createXML("-bad-.test", createOpts{}), "2005", ""},
		{"a registered name", createXML("registered.test", createOpts{}), "2302", ""},
		{"a reserved name", createXML("reserved.test", createOpts{}), "2308", ""},
		{"a TLD that is not open", createXML("x.closed", createOpts{}), "2306", "not open"},
		{"a TLD not served", createXML("x.example", createOpts{}), "2306", "not served"},
		{"an unknown contact", createXML("nocontact.test", createOpts{contacts: `<domain:registrant>nobody-1</domain:registrant><domain:contact type="tech">registrar1-c2</domain:contact>`}), "2303", "nobody-1"},
		{"another registrar's contact", createXML("theirs.test", createOpts{contacts: `<domain:registrant>registrar2-c1</domain:registrant><domain:contact type="tech">registrar1-c2</domain:contact>`}), "2201", "registrar2-c1"},
		{"an unknown host", createXML("nohost.test", createOpts{ns: `<domain:ns><domain:hostObj>ns9.nowhere.net</domain:hostObj></domain:ns>`}), "2303", "ns9.nowhere.net"},
		{"another registrar's host", createXML("theirhost.test", createOpts{ns: `<domain:ns><domain:hostObj>ns1.registrar2.net</domain:hostObj></domain:ns>`}), "2303", "ns1.registrar2.net"},
		{"an authInfo that is not random enough (RFC 9154)", createXML("weak.test", createOpts{pw: "Secret-Auth-1!"}), "2202", "128 bits"}, // #nosec G101 -- a deliberately weak test authInfo, not a credential
		{"a host object named twice", createXML("twice.test", createOpts{ns: `<domain:ns><domain:hostObj>ns1.registrar1.net</domain:hostObj><domain:hostObj>NS1.registrar1.net</domain:hostObj></domain:ns>`}), "2306", "ns1.registrar1.net is named more than once"},
		{"an invalid name wins over a bad period, host attributes and repeated contacts", createXML("-bad-.test", createOpts{
			period: `<domain:period unit="m">13</domain:period>`,
			ns:     `<domain:ns><domain:hostAttr><domain:hostName>ns1.x.test</domain:hostName></domain:hostAttr></domain:ns>`,
			contacts: `<domain:registrant>registrar1-c1</domain:registrant><domain:contact type="tech">registrar1-c1</domain:contact>
<domain:contact type="tech">registrar1-c2</domain:contact>`,
		}), "2005", ""},
		{"accreditation wins over a bad period", createXML("x.unaccredited", createOpts{period: `<domain:period unit="m">13</domain:period>`}), "2201", "accredited"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp := c.roundTrip(tc.frame)
			requireCode(t, resp, tc.code)
			assert.Contains(t, msg(resp), tc.msgPart)
		})
	}
	assert.Empty(t, creds.domains.registry.Registered, "none of them registered anything")

	t.Run("the authInfo is stored as the schema reads it", func(t *testing.T) {
		// A tab in a normalizedString is a space: the code validated is the
		// one hashed, and the one that matches.
		requireCode(t, c.roundTrip(createXML("tab.test", createOpts{pw: "Tr4nsf3r\tC0de-Xq9#Lm2!"})), "1000") // #nosec G101 -- a test fixture's authInfo, not a credential
		stored := creds.domains.registry.Domain("tab.test").AuthInfo
		assert.True(t, stored.Matches("Tr4nsf3r C0de-Xq9#Lm2!"))
		assert.False(t, stored.Matches("Tr4nsf3r\tC0de-Xq9#Lm2!"))
	})
	t.Run("months in whole years are accepted", func(t *testing.T) {
		resp := c.roundTrip(createXML("m24.test", createOpts{period: `<domain:period unit="m">24</domain:period>`}))
		requireCode(t, resp, "1000")
		d := creds.domains.registry.Domain("m24.test")
		require.NotNil(t, d)
		assert.Equal(t, d.CreatedAt.AddDate(2, 0, 0), d.ExpiryDate)
	})
	t.Run("no period is one year", func(t *testing.T) {
		requireCode(t, c.roundTrip(createXML("default.test", createOpts{})), "1000")
		d := creds.domains.registry.Domain("default.test")
		assert.Equal(t, d.CreatedAt.AddDate(1, 0, 0), d.ExpiryDate)
	})
	t.Run("a registry failure is 2400 without detail", func(t *testing.T) {
		creds.domains.registry.Fail(errorString("connection reset by peer"))
		resp := c.roundTrip(createXML("down.test", createOpts{}))
		requireCode(t, resp, "2400")
		assert.False(t, strings.Contains(msg(resp), "connection reset"))
		creds.domains.registry.Fail(nil)
	})
}

type errorString string

func (e errorString) Error() string { return string(e) }

const testDigest = "49FD46E6C4B45C55D4AC69CBD3CD34AC1AFE51DE3DC8B27C3F72E84BC2E1D3F4"

// secDNSCreate is a <secDNS:create> holding dsData elements.
func secDNSCreate(inner string) string {
	return `<secDNS:create xmlns:secDNS="urn:ietf:params:xml:ns:secDNS-1.1">` + inner + `</secDNS:create>`
}

func dsDataXML(keyTag, alg, digestType int, digest string) string {
	return fmt.Sprintf(`<secDNS:dsData><secDNS:keyTag>%d</secDNS:keyTag><secDNS:alg>%d</secDNS:alg>`+
		`<secDNS:digestType>%d</secDNS:digestType><secDNS:digest>%s</secDNS:digest></secDNS:dsData>`, keyTag, alg, digestType, digest)
}

func TestDomainCreateWithDNSSEC(t *testing.T) {
	creds := newTestCredentials(t)
	ts := startServerWith(t, testConfig(), listenLoopback(t), creds)
	c := dial(t, ts.addr)
	requireCode(t, c.roundTrip(loginWithServices([]string{nsDomain}, []string{nsSecDNS})), "1000")

	resp := c.roundTrip(createXML("signed.test", createOpts{ext: secDNSCreate(
		dsDataXML(2371, 13, 2, strings.ToLower(testDigest)) + dsDataXML(15288, 8, 4, strings.Repeat("AB", 48)),
	)}))
	requireCode(t, resp, "1000")
	d := creds.domains.registry.Domain("signed.test")
	require.NotNil(t, d)
	assert.Equal(t, []entities.DSRecord{
		{KeyTag: 2371, Algorithm: 13, DigestType: 2, Digest: testDigest},
		{KeyTag: 15288, Algorithm: 8, DigestType: 4, Digest: strings.Repeat("AB", 48)},
	}, d.DSRecords)

	info := c.roundTrip(infoXML("signed.test", ""))
	requireCode(t, info, "1000")
	ds := info.FindElements("//extension/infData/dsData")
	require.Len(t, ds, 2, "a session that logged in with secDNS sees the DS data")
	assert.Equal(t, "2371", text(info, "//extension/infData/dsData/keyTag"))
	assert.Equal(t, testDigest, ds[0].SelectElement("digest").Text())

	t.Run("a session without secDNS sees none, and cannot send it", func(t *testing.T) {
		plain := dial(t, ts.addr)
		plain.login()
		assert.Nil(t, plain.roundTrip(infoXML("signed.test", "")).FindElement("//extension"))
		requireCode(t, plain.roundTrip(createXML("nosec.test", createOpts{ext: secDNSCreate(dsDataXML(1, 13, 2, testDigest))})), "2103")
	})

	t.Run("refusals", func(t *testing.T) {
		nine := ""
		for i := range 9 {
			nine += dsDataXML(i+1, 13, 2, testDigest)
		}
		for _, tc := range []struct{ name, ext, code, part string }{
			{"an algorithm the registry does not accept (RSASHA1)", secDNSCreate(dsDataXML(1, 5, 2, testDigest)), "2306", "algorithm"},
			{"an unassigned algorithm", secDNSCreate(dsDataXML(1, 99, 2, testDigest)), "2306", "algorithm"},
			{"a SHA-1 digest", secDNSCreate(dsDataXML(1, 13, 1, strings.Repeat("AB", 20))), "2306", "digest type"},
			{"a digest of the wrong length", secDNSCreate(dsDataXML(1, 13, 2, testDigest[:40])), "2005", "32 octets"},
			{"a digest that is not hex, which the schema refuses", secDNSCreate(dsDataXML(1, 13, 2, "XYZ")), "2001", "digest"},
			{"maxSigLife", secDNSCreate(`<secDNS:maxSigLife>604800</secDNS:maxSigLife>` + dsDataXML(1, 13, 2, testDigest)), "2102", "maxSigLife"},
			{"the key data interface", secDNSCreate(`<secDNS:keyData><secDNS:flags>257</secDNS:flags><secDNS:protocol>3</secDNS:protocol><secDNS:alg>13</secDNS:alg><secDNS:pubKey>AQPJ////4Q==</secDNS:pubKey></secDNS:keyData>`), "2306", "DS data"},
			{"more than eight DS records", secDNSCreate(nine), "2306", "at most 8"},
			{"the same DS record twice", secDNSCreate(dsDataXML(1, 13, 2, testDigest) + dsDataXML(1, 13, 2, strings.ToLower(testDigest))), "2306", "duplicate"},
		} {
			t.Run(tc.name, func(t *testing.T) {
				resp := c.roundTrip(createXML("refused.test", createOpts{ext: tc.ext}))
				requireCode(t, resp, tc.code)
				assert.Contains(t, msg(resp), tc.part)
			})
		}
		assert.Nil(t, creds.domains.registry.Domain("refused.test"))
	})
}
