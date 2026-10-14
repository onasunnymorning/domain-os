package epp

import (
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
}

func createXML(name string, o createOpts) string {
	if o.contacts == "" {
		o.contacts = `<domain:registrant>registrar1-c1</domain:registrant><domain:contact type="tech">registrar1-c2</domain:contact>`
	}
	if o.pw == "" {
		o.pw = "Secret-Auth-1!"
	}
	return cmdXML(`<create><domain:create xmlns:domain="urn:ietf:params:xml:ns:domain-1.0">
<domain:name>`+name+`</domain:name>`+o.period+o.ns+o.contacts+`
<domain:authInfo><domain:pw>`+o.pw+`</domain:pw></domain:authInfo></domain:create></create>`, "CRE-1")
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
	assert.Equal(t, "Secret-Auth-1!", text(info, "//infData/authInfo/pw"), "the sponsor sees its authorization information")
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
		assert.NotContains(t, create.Request, "Secret-Auth-1!")
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
		{"an authInfo that breaks the password rules", createXML("weak.test", createOpts{pw: "weak"}), "2306", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp := c.roundTrip(tc.frame)
			requireCode(t, resp, tc.code)
			assert.Contains(t, msg(resp), tc.msgPart)
		})
	}
	assert.Empty(t, creds.domains.registry.Registered, "none of them registered anything")

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
