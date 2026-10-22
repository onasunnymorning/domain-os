package epp

import (
	"testing"
	"time"

	"github.com/beevik/etree"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The transfer code a test registrar sets: random enough for RFC 9154.
const testTransferCode = "Tr4nsf3r-C0de-Xq9#Lm2!" // #nosec G101 -- a test fixture's authInfo, not a credential

func infoWithCodeXML(name, pw string) string {
	return cmdXML(`<info><domain:info xmlns:domain="urn:ietf:params:xml:ns:domain-1.0"><domain:name>`+name+`</domain:name>`+
		`<domain:authInfo><domain:pw>`+pw+`</domain:pw></domain:authInfo></domain:info></info>`, "INF-9154")
}

func TestRFC9154_AuthInfo(t *testing.T) {
	creds, ts, c := domainServer(t)
	requireCode(t, c.roundTrip(createXML("code.test", createOpts{pw: testTransferCode,
		ns: `<domain:ns><domain:hostObj>ns1.registrar1.net</domain:hostObj></domain:ns>`})), "1000")
	other := registrar2(t, ts)

	t.Run("the greeting offers secure-authinfo-transfer", func(t *testing.T) {
		g := c.roundTrip(`<?xml version="1.0" encoding="UTF-8"?><epp xmlns="urn:ietf:params:xml:ns:epp-1.0"><hello/></epp>`)
		assert.Contains(t, texts(g.FindElements("//svcMenu/svcExtension/extURI")), nsSecureAuthInfo)
	})
	t.Run("the sponsor is told a code is set, never what it is (§5.3)", func(t *testing.T) {
		info := c.roundTrip(infoXML("code.test", ""))
		require.NotNil(t, info.FindElement("//infData/authInfo/pw"))
		assert.Empty(t, text(info, "//infData/authInfo/pw"))
		assert.NotContains(t, mustWrite(t, info), testTransferCode)
	})
	t.Run("another registrar without a code: 2201 (ADR 0006)", func(t *testing.T) {
		requireCode(t, other.roundTrip(infoXML("code.test", "")), "2201")
	})
	t.Run("another registrar with the code: the limited view (§5.3)", func(t *testing.T) {
		info := other.roundTrip(infoWithCodeXML("code.test", testTransferCode))
		requireCode(t, info, "1000")
		requireSchemaValid(t, []byte(mustWrite(t, info)))
		assert.Equal(t, "code.test", text(info, "//infData/name"))
		assert.NotEmpty(t, text(info, "//infData/roid"))
		assert.Equal(t, testRegistrar, text(info, "//infData/clID"))
		assert.NotEmpty(t, text(info, "//infData/exDate"))
		for _, hidden := range []string{"registrant", "contact", "ns", "host", "authInfo", "crID", "upID"} {
			assert.Nil(t, info.FindElement("//infData/"+hidden), "the limited view has no <domain:%s>", hidden)
		}
		assert.Nil(t, info.FindElement("//response/extension"), "nor extensions")
	})
	t.Run("another registrar with a wrong or empty code: 2202 (§4.4)", func(t *testing.T) {
		requireCode(t, other.roundTrip(infoWithCodeXML("code.test", testTransferCode+"x")), "2202")
		requireCode(t, other.roundTrip(infoWithCodeXML("code.test", "")), "2202")
	})
	t.Run("an unset code: the sponsor sees no <authInfo>, and nothing matches (§4.4)", func(t *testing.T) {
		requireCode(t, c.roundTrip(domainUpdateXML("code.test", `<domain:chg><domain:authInfo><domain:pw/></domain:authInfo></domain:chg>`, "")), "1000")
		assert.Nil(t, c.roundTrip(infoXML("code.test", "")).FindElement("//infData/authInfo"))
		requireCode(t, other.roundTrip(infoWithCodeXML("code.test", testTransferCode)), "2202")
		requireCode(t, other.roundTrip(infoWithCodeXML("code.test", "")), "2202")
	})
	t.Run("a code set again matches again", func(t *testing.T) {
		requireCode(t, c.roundTrip(domainUpdateXML("code.test", `<domain:chg><domain:authInfo><domain:pw>`+testTransferCode+`</domain:pw></domain:authInfo></domain:chg>`, "")), "1000")
		requireCode(t, other.roundTrip(infoWithCodeXML("code.test", testTransferCode)), "1000")
	})
	t.Run("a code not random enough is refused: 2202 (§5.2)", func(t *testing.T) {
		resp := c.roundTrip(domainUpdateXML("code.test", `<domain:chg><domain:authInfo><domain:pw>Secret-Auth-1!</domain:pw></domain:authInfo></domain:chg>`, ""))
		requireCode(t, resp, "2202")
		assert.Contains(t, msg(resp), "128 bits")
	})
	t.Run("<domain:ext> authorization information is refused, and the code kept: 2102", func(t *testing.T) {
		// <domain:ext> holds any element a loaded schema declares (strict
		// processing); an unknown one is a syntax error (2001) already.
		ext := `<domain:authInfo><domain:ext><host:delete xmlns:host="urn:ietf:params:xml:ns:host-1.0"><host:name>ns.example.net</host:name></host:delete></domain:ext></domain:authInfo>`
		requireCode(t, c.roundTrip(domainUpdateXML("code.test", `<domain:chg>`+ext+`</domain:chg>`, "")), "2102")
		requireCode(t, other.roundTrip(infoWithCodeXML("code.test", testTransferCode)), "1000")
		infoExt := func(c *client) *etree.Document {
			return c.roundTrip(cmdXML(`<info><domain:info xmlns:domain="urn:ietf:params:xml:ns:domain-1.0">`+
				`<domain:name>code.test</domain:name>`+ext+`</domain:info></info>`, "INF-EXT"))
		}
		requireCode(t, infoExt(other), "2102")
		requireCode(t, infoExt(c), "2102")
		requireCode(t, c.roundTrip(cmdXML(`<create><domain:create xmlns:domain="urn:ietf:params:xml:ns:domain-1.0">`+
			`<domain:name>ext.test</domain:name><domain:registrant>reg1</domain:registrant>`+ext+
			`</domain:create></create>`, "CRE-EXT")), "2102")
	})
	t.Run("the code never reaches the audit log", func(t *testing.T) {
		require.Eventually(t, func() bool { return len(creds.audit.All()) > 0 }, 5*time.Second, 50*time.Millisecond)
		for _, tx := range creds.audit.All() {
			assert.NotContains(t, tx.Request, testTransferCode, "svTRID %s", tx.SvTRID)
			assert.NotContains(t, tx.Response, testTransferCode, "svTRID %s", tx.SvTRID)
		}
	})
}
