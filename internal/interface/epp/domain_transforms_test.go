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

func domainUpdateXML(name, changes, ext string) string {
	if ext != "" {
		ext = "<extension>" + ext + "</extension>"
	}
	return cmdXML(`<update><domain:update xmlns:domain="urn:ietf:params:xml:ns:domain-1.0"><domain:name>`+name+`</domain:name>`+changes+`</domain:update></update>`+ext, "UPD-1")
}

func domainRenewXML(name, curExpDate, period string) string {
	return cmdXML(`<renew><domain:renew xmlns:domain="urn:ietf:params:xml:ns:domain-1.0"><domain:name>`+name+`</domain:name><domain:curExpDate>`+curExpDate+`</domain:curExpDate>`+period+`</domain:renew></renew>`, "REN-1")
}

func domainDeleteXML(name string) string {
	return cmdXML(`<delete><domain:delete xmlns:domain="urn:ietf:params:xml:ns:domain-1.0"><domain:name>`+name+`</domain:name></domain:delete></delete>`, "DEL-1")
}

func secDNSUpdateXML(attrs, inner string) string {
	return `<secDNS:update xmlns:secDNS="urn:ietf:params:xml:ns:secDNS-1.1"` + attrs + `>` + inner + `</secDNS:update>`
}

// fullLoginXML logs registrar1 in with every object service and extension.
func fullLoginXML() string {
	return loginWithServices([]string{nsDomain, nsHost, nsContact}, []string{nsSecDNS, nsRGP})
}

// domainServer is objectServer with registrar1 logged in with DNSSEC and
// RGP too.
func domainServer(t *testing.T) (*testCredentials, *testServer, *client) {
	t.Helper()
	creds, ts, _ := objectServer(t)
	c := dial(t, ts.addr)
	requireCode(t, c.roundTrip(fullLoginXML()), "1000")
	return creds, ts, c
}

// nameservers are a domain info's <domain:hostObj>s.
func nameservers(info *etree.Document) []string {
	return texts(info.FindElements("//infData/ns/hostObj"))
}

func TestDomainUpdate(t *testing.T) {
	creds, _, c := domainServer(t)
	reg := creds.domains.registry
	requireCode(t, c.roundTrip(createXML("upd.test", createOpts{ns: `<domain:ns><domain:hostObj>ns1.registrar1.net</domain:hostObj></domain:ns>`})), "1000")
	update := func(changes, ext string) *etree.Document {
		return c.roundTrip(domainUpdateXML("upd.test", changes, ext))
	}
	info := func() *etree.Document { return c.roundTrip(infoXML("upd.test", "")) }
	add := func(inner string) string { return "<domain:add>" + inner + "</domain:add>" }
	rem := func(inner string) string { return "<domain:rem>" + inner + "</domain:rem>" }
	status := func(s string) string { return `<domain:status s="` + s + `"/>` }
	ns := func(names ...string) string {
		return "<domain:ns><domain:hostObj>" + strings.Join(names, "</domain:hostObj><domain:hostObj>") + "</domain:hostObj></domain:ns>"
	}

	t.Run("statuses", func(t *testing.T) {
		requireCode(t, update(add(status("clientHold")+status("clientRenewProhibited")), ""), "1000")
		assert.Subset(t, statusesOf(info()), []string{"clientHold", "clientRenewProhibited"})
		requireCode(t, update(add(status("clientHold")), ""), "2306")
		requireCode(t, update(add(status("serverHold")), ""), "2306")
		requireCode(t, update(rem(status("clientDeleteProhibited")), ""), "2306")
		requireCode(t, update(rem(status("clientHold")+status("clientRenewProhibited")), ""), "1000")
		assert.NotContains(t, statusesOf(info()), "clientHold")
	})
	t.Run("nameservers", func(t *testing.T) {
		requireCode(t, update(add(ns("ns2.registrar1.net")), ""), "1000")
		assert.ElementsMatch(t, []string{"ns1.registrar1.net", "ns2.registrar1.net"}, nameservers(info()))
		requireCode(t, update(add(ns("ns2.registrar1.net")), ""), "2306")
		requireCode(t, update(add(ns("ns9.nobody.net")), ""), "2303")
		requireCode(t, update(add(ns("ns1.registrar2.net")), ""), "2303")
		requireCode(t, update(add(`<domain:ns><domain:hostAttr><domain:hostName>ns3.registrar1.net</domain:hostName></domain:hostAttr></domain:ns>`), ""), "2306")
		requireCode(t, update(rem(ns("ns1.registrar1.net")), ""), "1000")
		assert.Equal(t, []string{"ns2.registrar1.net"}, nameservers(info()))
		requireCode(t, update(rem(ns("ns1.registrar1.net")), ""), "2306")
	})
	t.Run("contacts and registrant", func(t *testing.T) {
		requireCode(t, update(add(`<domain:contact type="admin">registrar1-c1</domain:contact>`), ""), "1000")
		requireCode(t, update(add(`<domain:contact type="admin">registrar1-c2</domain:contact>`), ""), "2306")
		requireCode(t, update(rem(`<domain:contact type="admin">registrar1-c2</domain:contact>`), ""), "2306")
		requireCode(t, update(add(`<domain:contact type="billing">registrar2-c1</domain:contact>`), ""), "2201")
		requireCode(t, update(add(`<domain:contact type="billing">nobody-1</domain:contact>`), ""), "2303")
		requireCode(t, update(rem(`<domain:contact type="tech">registrar1-c2</domain:contact>`), ""), "2003")
		requireCode(t, update(`<domain:chg><domain:registrant>registrar1-c2</domain:registrant></domain:chg>`, ""), "1000")
		assert.Equal(t, "registrar1-c2", text(info(), "//infData/registrant"))
		requireCode(t, update(`<domain:chg><domain:registrant/></domain:chg>`, ""), "2003")
		requireCode(t, update(rem(`<domain:contact type="admin">registrar1-c1</domain:contact>`), ""), "1000")
	})
	t.Run("authorization information", func(t *testing.T) {
		requireCode(t, update(`<domain:chg><domain:authInfo><domain:pw>New-Auth-2?</domain:pw></domain:authInfo></domain:chg>`, ""), "1000")
		assert.Equal(t, "New-Auth-2?", text(info(), "//infData/authInfo/pw"))
		requireCode(t, update(`<domain:chg><domain:authInfo><domain:null/></domain:authInfo></domain:chg>`, ""), "2306")
		requireCode(t, update(`<domain:chg><domain:authInfo><domain:pw>weak</domain:pw></domain:authInfo></domain:chg>`, ""), "2306")
	})
	t.Run("DNSSEC", func(t *testing.T) {
		ds := dsDataXML(2371, 13, 2, testDigest)
		requireCode(t, update("", secDNSUpdateXML("", "<secDNS:add>"+ds+"</secDNS:add>")), "1000")
		require.Len(t, info().FindElements("//extension/infData/dsData"), 1)
		requireCode(t, update("", secDNSUpdateXML("", "<secDNS:add>"+ds+"</secDNS:add>")), "2306")
		requireCode(t, update("", secDNSUpdateXML("", "<secDNS:add>"+dsDataXML(2372, 5, 2, testDigest)+"</secDNS:add>")), "2306")
		requireCode(t, update("", secDNSUpdateXML("", "<secDNS:add>"+dsDataXML(2372, 13, 2, testDigest[:62])+"</secDNS:add>")), "2005")
		requireCode(t, update("", secDNSUpdateXML(` urgent="true"`, "<secDNS:rem><secDNS:all>true</secDNS:all></secDNS:rem>")), "2102")
		requireCode(t, update("", secDNSUpdateXML("", "<secDNS:chg><secDNS:maxSigLife>604800</secDNS:maxSigLife></secDNS:chg>")), "2102")
		requireCode(t, update("", secDNSUpdateXML("", `<secDNS:add><secDNS:keyData><secDNS:flags>257</secDNS:flags><secDNS:protocol>3</secDNS:protocol><secDNS:alg>13</secDNS:alg><secDNS:pubKey>AQPJ////4Q==</secDNS:pubKey></secDNS:keyData></secDNS:add>`)), "2306")
		requireCode(t, update("", secDNSUpdateXML("", "<secDNS:rem>"+ds+"</secDNS:rem>")), "1000")
		assert.Empty(t, info().FindElements("//extension/infData/dsData"))
		requireCode(t, update("", secDNSUpdateXML("", "<secDNS:rem>"+ds+"</secDNS:rem>")), "2306")
		requireCode(t, update("", secDNSUpdateXML("", "<secDNS:add>"+ds+dsDataXML(2372, 13, 2, testDigest)+"</secDNS:add>")), "1000")
		requireCode(t, update("", secDNSUpdateXML("", "<secDNS:rem><secDNS:all>true</secDNS:all></secDNS:rem>")), "1000")
		assert.Empty(t, info().FindElements("//extension/infData/dsData"))
	})
	t.Run("clientUpdateProhibited refuses every update but one removing it", func(t *testing.T) {
		requireCode(t, update(add(status("clientUpdateProhibited")), ""), "1000")
		requireCode(t, update(add(status("clientHold")), ""), "2304")
		requireCode(t, update(add(status("clientHold"))+rem(status("clientUpdateProhibited")), ""), "1000")
		assert.NotContains(t, statusesOf(info()), "clientUpdateProhibited")
		requireCode(t, update(rem(status("clientHold")), ""), "1000")
	})
	t.Run("serverUpdateProhibited refuses every update", func(t *testing.T) {
		reg.SetDomainStatus("upd.test", func(s *entities.DomainStatus) { s.ServerUpdateProhibited, s.OK = true, false })
		defer reg.SetDomainStatus("upd.test", func(s *entities.DomainStatus) { s.ServerUpdateProhibited, s.OK = false, true })
		requireCode(t, update(add(status("clientHold")), ""), "2304")
	})
	for _, tc := range []struct{ name, frame, code string }{
		{"no add, rem, chg or extension", domainUpdateXML("upd.test", "", ""), "2003"},
		{"another registrar's domain", domainUpdateXML("registered.test", add(status("clientHold")), ""), "2201"},
		{"a domain nobody has", domainUpdateXML("nobody.test", add(status("clientHold")), ""), "2303"},
	} {
		t.Run("refused: "+tc.name, func(t *testing.T) { requireCode(t, c.roundTrip(tc.frame), tc.code) })
	}
}

// statusesOf are a domain info's statuses.
func statusesOf(info *etree.Document) []string {
	var out []string
	for _, el := range info.FindElements("//infData/status") {
		out = append(out, el.SelectAttrValue("s", ""))
	}
	return out
}

// exDate is a domain info's expiry date.
func exDate(t *testing.T, info *etree.Document) time.Time {
	t.Helper()
	d, err := time.Parse(time.RFC3339, text(info, "//infData/exDate"))
	require.NoError(t, err)
	return d
}

func TestDomainRenew(t *testing.T) {
	creds, _, c := domainServer(t)
	reg := creds.domains.registry
	requireCode(t, c.roundTrip(createXML("ren.test", createOpts{period: `<domain:period unit="y">2</domain:period>`})), "1000")
	exp := exDate(t, c.roundTrip(infoXML("ren.test", "")))
	cur := exp.Format("2006-01-02")

	t.Run("refused: a current expiry date that is not the domain's", func(t *testing.T) {
		requireCode(t, c.roundTrip(domainRenewXML("ren.test", exp.AddDate(-1, 0, 0).Format("2006-01-02"), "")), "2306")
	})
	t.Run("refused: past the maximum horizon of ten years", func(t *testing.T) {
		requireCode(t, c.roundTrip(domainRenewXML("ren.test", cur, `<domain:period unit="y">9</domain:period>`)), "2306")
	})
	t.Run("refused: months that are not whole years", func(t *testing.T) {
		requireCode(t, c.roundTrip(domainRenewXML("ren.test", cur, `<domain:period unit="m">13</domain:period>`)), "2306")
	})
	t.Run("renewed: the expiry date moves by the period, in the renew grace period", func(t *testing.T) {
		resp := c.roundTrip(domainRenewXML("ren.test", cur+"Z", `<domain:period unit="m">36</domain:period>`))
		requireCode(t, resp, "1000")
		requireSchemaValid(t, []byte(mustWrite(t, resp)))
		assert.Equal(t, exp.AddDate(3, 0, 0).Format(time.RFC3339), text(resp, "//resData/renData/exDate"))
		info := c.roundTrip(infoXML("ren.test", ""))
		assert.Equal(t, exp.AddDate(3, 0, 0), exDate(t, info))
		var rgp []string
		for _, el := range info.FindElements("//extension/infData/rgpStatus") {
			rgp = append(rgp, el.SelectAttrValue("s", ""))
		}
		assert.Contains(t, rgp, "renewPeriod")
	})
	t.Run("refused: the same renew again, with the old expiry date", func(t *testing.T) {
		requireCode(t, c.roundTrip(domainRenewXML("ren.test", cur, "")), "2306")
	})
	t.Run("refused: clientRenewProhibited", func(t *testing.T) {
		requireCode(t, c.roundTrip(domainUpdateXML("ren.test", `<domain:add><domain:status s="clientRenewProhibited"/></domain:add>`, "")), "1000")
		requireCode(t, c.roundTrip(domainRenewXML("ren.test", exp.AddDate(3, 0, 0).Format("2006-01-02"), "")), "2304")
	})
	t.Run("refused: another registrar's domain", func(t *testing.T) {
		requireCode(t, c.roundTrip(domainRenewXML("registered.test", reg.Domain("registered.test").ExpiryDate.Format("2006-01-02"), "")), "2201")
	})
}

func TestDomainDelete(t *testing.T) {
	_, _, c := domainServer(t)
	t.Run("deleted: pendingDelete, answered 1001", func(t *testing.T) {
		requireCode(t, c.roundTrip(createXML("del.test", createOpts{})), "1000")
		requireCode(t, c.roundTrip(domainDeleteXML("del.test")), "1001")
		assert.Contains(t, statusesOf(c.roundTrip(infoXML("del.test", ""))), "pendingDelete")
		requireCode(t, c.roundTrip(domainDeleteXML("del.test")), "2304")
		requireCode(t, c.roundTrip(domainUpdateXML("del.test", `<domain:add><domain:status s="clientHold"/></domain:add>`, "")), "2304")
	})
	t.Run("refused: clientDeleteProhibited", func(t *testing.T) {
		requireCode(t, c.roundTrip(createXML("keep.test", createOpts{})), "1000")
		requireCode(t, c.roundTrip(domainUpdateXML("keep.test", `<domain:add><domain:status s="clientDeleteProhibited"/></domain:add>`, "")), "1000")
		requireCode(t, c.roundTrip(domainDeleteXML("keep.test")), "2304")
	})
	t.Run("deleted: an update prohibition does not stop a delete (RFC 5731 §2.3)", func(t *testing.T) {
		requireCode(t, c.roundTrip(createXML("locked.test", createOpts{})), "1000")
		requireCode(t, c.roundTrip(domainUpdateXML("locked.test", `<domain:add><domain:status s="clientUpdateProhibited"/></domain:add>`, "")), "1000")
		requireCode(t, c.roundTrip(domainDeleteXML("locked.test")), "1001")
		assert.Subset(t, statusesOf(c.roundTrip(infoXML("locked.test", ""))), []string{"pendingDelete", "clientUpdateProhibited"})
	})
	t.Run("refused: a subordinate host exists", func(t *testing.T) {
		requireCode(t, c.roundTrip(createXML("parent.test", createOpts{})), "1000")
		requireCode(t, c.roundTrip(hostCreateXML("ns1.parent.test", addrXML("v4", testV4))), "1000")
		resp := c.roundTrip(domainDeleteXML("parent.test"))
		requireCode(t, resp, "2305")
		assert.Contains(t, msg(resp), "ns1.parent.test")
		requireCode(t, c.roundTrip(hostDeleteXML("ns1.parent.test")), "1000")
		requireCode(t, c.roundTrip(domainDeleteXML("parent.test")), "1001")
	})
	t.Run("refused: another registrar's domain", func(t *testing.T) {
		requireCode(t, c.roundTrip(domainDeleteXML("registered.test")), "2201")
	})
}

func TestDomainUpdateRefusesARepeatedExtension(t *testing.T) {
	_, _, c := domainServer(t)
	requireCode(t, c.roundTrip(createXML("twice.test", createOpts{ext: secDNSCreate(dsDataXML(2371, 13, 2, testDigest))})), "1000")
	rem := secDNSUpdateXML("", "<secDNS:rem>"+dsDataXML(2371, 13, 2, testDigest)+"</secDNS:rem>")
	add := secDNSUpdateXML("", "<secDNS:add>"+dsDataXML(2372, 13, 2, testDigest)+"</secDNS:add>")
	resp := c.roundTrip(domainUpdateXML("twice.test", "", rem+add))
	requireCode(t, resp, "2001")
	assert.Contains(t, msg(resp), "more than once")
	ds := c.roundTrip(infoXML("twice.test", "")).FindElements("//extension/infData/dsData/keyTag")
	assert.Equal(t, []string{"2371"}, texts(ds), "neither block was applied")
}

func TestHostUnderADomainBeingDeleted(t *testing.T) {
	_, _, c := domainServer(t)
	requireCode(t, c.roundTrip(createXML("going.test", createOpts{})), "1000")
	requireCode(t, c.roundTrip(hostCreateXML("ns1.elsewhere.net")), "1000")
	requireCode(t, c.roundTrip(domainDeleteXML("going.test")), "1001")
	resp := c.roundTrip(hostCreateXML("ns1.going.test", addrXML("v4", testV4)))
	requireCode(t, resp, "2304")
	assert.Contains(t, msg(resp), "being deleted")
	requireCode(t, c.roundTrip(hostUpdateXML("ns1.elsewhere.net", chgXML("ns2.going.test"))), "2304")
}
