package epp

import (
	"strings"
	"testing"

	"github.com/beevik/etree"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onasunnymorning/domain-os/internal/application/services"
	"github.com/onasunnymorning/domain-os/internal/infrastructure/memstore"
	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
)

// newTestHosts is the real services.EPPHostService over reg, minting ROIDs
// from it, so host tests share the registry the domain tests use.
func newTestHosts(reg *memstore.Registry) *services.EPPHostService {
	return services.NewEPPHostService(services.EPPHostServiceDeps{
		Hosts: reg.Hosts(), Domains: reg.Domains(), TLDs: reg.TLDs(),
		Roids: services.NewRoidService(reg, testRepositoryID),
	})
}

// Addresses from the prefixes RST's data providers use.
const (
	testV4  = "208.77.190.196"
	testV4b = "208.77.190.197"
	testV6  = "2602:800:900e:1257::5"
)

func hostCmdXML(verb, inner string) string {
	return cmdXML(`<`+verb+`><host:`+verb+` xmlns:host="urn:ietf:params:xml:ns:host-1.0">`+inner+`</host:`+verb+`></`+verb+`>`, "HOST-1")
}

func hostCheckXML(names ...string) string {
	var b strings.Builder
	for _, n := range names {
		b.WriteString("<host:name>" + n + "</host:name>")
	}
	return hostCmdXML("check", b.String())
}

func hostInfoXML(name string) string { return hostCmdXML("info", "<host:name>"+name+"</host:name>") }

func hostDeleteXML(name string) string {
	return hostCmdXML("delete", "<host:name>"+name+"</host:name>")
}

// hostCreateXML is a <host:create> for name with the given <host:addr>
// elements.
func hostCreateXML(name string, addrs ...string) string {
	return hostCmdXML("create", "<host:name>"+name+"</host:name>"+strings.Join(addrs, ""))
}

// hostUpdateXML is a <host:update> of name; changes is its <host:add>,
// <host:rem> and <host:chg> elements.
func hostUpdateXML(name, changes string) string {
	return hostCmdXML("update", "<host:name>"+name+"</host:name>"+changes)
}

func addrXML(ip, addr string) string {
	if ip == "" {
		return "<host:addr>" + addr + "</host:addr>"
	}
	return `<host:addr ip="` + ip + `">` + addr + "</host:addr>"
}

func statusXML(s string) string { return `<host:status s="` + s + `"/>` }

func addXML(parts ...string) string { return "<host:add>" + strings.Join(parts, "") + "</host:add>" }
func remXML(parts ...string) string { return "<host:rem>" + strings.Join(parts, "") + "</host:rem>" }
func chgXML(name string) string {
	return "<host:chg><host:name>" + name + "</host:name></host:chg>"
}

// code is a response's result code.
func code(doc *etree.Document) string {
	return doc.FindElement("//response/result").SelectAttrValue("code", "")
}

// hostLoginXML logs in as clID with the domain and host services.
func hostLoginXML(clID, pw string) string {
	return cmdXML(`<login><clID>`+clID+`</clID><pw>`+pw+`</pw>
<options><version>1.0</version><lang>en</lang></options>
<svcs><objURI>`+nsDomain+`</objURI><objURI>`+nsHost+`</objURI></svcs></login>`, "HOST-LOGIN")
}

// hostServer starts a server whose registry has, besides newTestDomains'
// fixtures, mine.test sponsored by registrar1, and logs registrar1 in with
// the host service. registrar2 can log in too (rstRegistrar2).
func hostServer(t *testing.T) (*testCredentials, *testServer, *client) {
	t.Helper()
	creds := newTestCredentials(t)
	rstRegistrar2(t, creds)
	reg := creds.domains.registry
	reg.AddDomain(&entities.Domain{Name: "mine.test", ClID: testRegistrar, RoID: entities.RoidType("2_DOM-" + testRepositoryID)})
	reg.AddDomain(&entities.Domain{Name: "also-mine.test", ClID: testRegistrar, RoID: entities.RoidType("3_DOM-" + testRepositoryID)})
	ts := startServerWith(t, testConfig(), listenLoopback(t), creds)
	c := dial(t, ts.addr)
	requireCode(t, c.roundTrip(hostLoginXML(testLogin, testPassword)), "1000")
	return creds, ts, c
}

// registrar2 logs registrar2 in with the host service.
func registrar2(t *testing.T, ts *testServer) *client {
	t.Helper()
	c := dial(t, ts.addr)
	requireCode(t, c.roundTrip(hostLoginXML("registrar2", "secret-pw-02")), "1000")
	return c
}

func TestHostCreateInfoDelete(t *testing.T) {
	_, _, c := hostServer(t)

	t.Run("an external host, without addresses", func(t *testing.T) {
		resp := c.roundTrip(hostCreateXML("NS1.Example.NET"))
		requireCode(t, resp, "1000")
		assert.Equal(t, "ns1.example.net", text(resp, "//resData/creData/name"), "the name as stored: lower case")
		assert.NotEmpty(t, text(resp, "//resData/creData/crDate"))

		info := c.roundTrip(hostInfoXML("ns1.example.net"))
		requireCode(t, info, "1000")
		assert.Equal(t, "ns1.example.net", text(info, "//infData/name"))
		assert.Regexp(t, `^\d+_HOST-EPPTEST$`, text(info, "//infData/roid"))
		assert.Equal(t, []string{"ok"}, statuses(info))
		assert.Empty(t, info.FindElements("//infData/addr"))
		assert.Equal(t, testRegistrar, text(info, "//infData/clID"))
		assert.Equal(t, testRegistrar, text(info, "//infData/crID"))
		assert.NotEmpty(t, text(info, "//infData/crDate"))
		requireSchemaValid(t, []byte(mustWrite(t, info)))
	})

	t.Run("an internal host, with IPv4 and IPv6 addresses", func(t *testing.T) {
		requireCode(t, c.roundTrip(hostCreateXML("ns1.mine.test", addrXML("", testV4), addrXML("v6", strings.ToUpper(testV6)))), "1000")
		info := c.roundTrip(hostInfoXML("ns1.mine.test"))
		requireCode(t, info, "1000")
		assert.Equal(t, map[string]string{testV4: "v4", testV6: "v6"}, addresses(info), "an IPv6 address as stored: canonical")
		requireSchemaValid(t, []byte(mustWrite(t, info)))
	})

	t.Run("an internal host without addresses", func(t *testing.T) {
		requireCode(t, c.roundTrip(hostCreateXML("ns2.mine.test")), "1000")
	})

	t.Run("delete", func(t *testing.T) {
		requireCode(t, c.roundTrip(hostDeleteXML("ns1.example.net")), "1000")
		requireCode(t, c.roundTrip(hostInfoXML("ns1.example.net")), "2303")
	})
}

func TestHostCreateRefusals(t *testing.T) {
	_, _, c := hostServer(t)
	for _, tc := range []struct {
		name, frame, code, says string
	}{
		{"an invalid name", hostCreateXML("!.example.net"), "2005", "invalid host name"},
		{"a name of one label", hostCreateXML("localhost"), "2005", "invalid host name"},
		{"an address on an external host", hostCreateXML("ns9.example.net", addrXML("v4", testV4)), "2306", "takes no addresses"},
		{"an unregistered superordinate domain", hostCreateXML("ns1.nobody.test", addrXML("v4", testV4)), "2303", "nobody.test"},
		{"another registrar's superordinate domain, with glue", hostCreateXML("ns1.registered.test", addrXML("v4", testV4)), "2201", "registered.test"},
		{"another registrar's superordinate domain, without glue", hostCreateXML("ns1.registered.test"), "2201", "registered.test"},
		{"an IPv6 address as v4", hostCreateXML("ns3.mine.test", addrXML("v4", testV6)), "2005", "not IPv4"},
		{"an IPv4 address as v6", hostCreateXML("ns3.mine.test", addrXML("v6", testV4)), "2005", "not IPv6"},
		{"an incomplete IPv4 address", hostCreateXML("ns3.mine.test", addrXML("v4", "208.77.190")), "2005", "invalid IP"},
		{"an IPv4 address out of range", hostCreateXML("ns3.mine.test", addrXML("v4", "256.1.1.1")), "2005", "invalid IP"},
		{"an IPv4 address of five parts", hostCreateXML("ns3.mine.test", addrXML("v4", "1.2.3.4.5")), "2005", "invalid IP"},
		{"an IPv4 address of letters", hostCreateXML("ns3.mine.test", addrXML("v4", "a.b.c.d")), "2005", "invalid IP"},
		{"an IPv6 address that is not hex", hostCreateXML("ns3.mine.test", addrXML("v6", "NOT::HEX::")), "2005", "invalid IP"},
		{"IPv4 loopback", hostCreateXML("ns3.mine.test", addrXML("v4", "127.0.0.1")), "2306", "public unicast"},
		{"a private IPv4 address", hostCreateXML("ns3.mine.test", addrXML("v4", "10.1.2.3")), "2306", "public unicast"},
		{"IPv6 unspecified, shorter than the schema allows", hostCreateXML("ns3.mine.test", addrXML("v6", "::")), "2001", ""},
		{"IPv6 unspecified, written out", hostCreateXML("ns3.mine.test", addrXML("v6", "0::0")), "2306", "public unicast"},
		{"IPv6 loopback", hostCreateXML("ns3.mine.test", addrXML("v6", "::1")), "2306", "public unicast"},
		{"an IPv4 address written as IPv6", hostCreateXML("ns3.mine.test", addrXML("v6", "::ffff:"+testV4)), "2306", "public unicast"},
		{"the same address twice", hostCreateXML("ns3.mine.test", addrXML("v4", testV4), addrXML("v4", testV4)), "2306", "duplicate"},
		{"an IP version the schema does not have", hostCreateXML("ns3.mine.test", addrXML("v5", testV4)), "2001", ""},
		{"a host the registrar has", hostCreateXML("ns1.registrar1.net"), "2302", "already exists"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			resp := c.roundTrip(tc.frame)
			requireCode(t, resp, tc.code)
			assert.Contains(t, msg(resp), tc.says)
		})
	}
	requireCode(t, c.roundTrip(hostInfoXML("ns3.mine.test")), "2303")
}

func TestHostCheck(t *testing.T) {
	_, _, c := hostServer(t)
	resp := c.roundTrip(hostCheckXML("ns1.registrar1.net", "ns1.registrar2.net", "ns1.registered.test", "ns1.nobody.test", "ns1.mine.test", "!.test"))
	requireCode(t, resp, "1000")
	requireSchemaValid(t, []byte(mustWrite(t, resp)))
	type answer struct{ avail, reason string }
	got := map[string]answer{}
	for _, cd := range resp.FindElements("//resData/chkData/cd") {
		name := cd.SelectElement("name")
		reason := ""
		if r := cd.SelectElement("reason"); r != nil {
			reason = r.Text()
		}
		got[name.Text()] = answer{name.SelectAttrValue("avail", ""), reason}
	}
	assert.Equal(t, map[string]answer{
		"ns1.registrar1.net":  {"0", "In use"},
		"ns1.registrar2.net":  {"1", ""}, // another registrar's: this one can have its own
		"ns1.registered.test": {"0", "Superordinate domain not yours"},
		"ns1.nobody.test":     {"1", ""}, // nobody has it; create needs the domain first
		"ns1.mine.test":       {"1", ""},
		"!.test":              {"0", "Invalid host name"},
	}, got)

	t.Run("more names than EPP_MAX_CHECK_NAMES", func(t *testing.T) {
		names := make([]string, testConfig().Commands.MaxCheckNames+1)
		for i := range names {
			names[i] = "ns1.registrar1.net"
		}
		requireCode(t, c.roundTrip(hostCheckXML(names...)), "2306")
	})
}

func TestHostSponsorship(t *testing.T) {
	_, ts, c := hostServer(t)
	theirs := "ns9.theirs.net"
	requireCode(t, registrar2(t, ts).roundTrip(hostCreateXML(theirs)), "1000")
	for _, tc := range []struct{ name, frame, code string }{
		{"info on another registrar's host", hostInfoXML(theirs), "2201"},
		{"update of another registrar's host", hostUpdateXML(theirs, addXML(statusXML("clientDeleteProhibited"))), "2201"},
		{"delete of another registrar's host", hostDeleteXML(theirs), "2201"},
		{"info on a host nobody has", hostInfoXML("ns9.nobody.net"), "2303"},
		{"update of a host nobody has", hostUpdateXML("ns9.nobody.net", addXML(statusXML("clientDeleteProhibited"))), "2303"},
		{"delete of a host nobody has", hostDeleteXML("ns9.nobody.net"), "2303"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			resp := c.roundTrip(tc.frame)
			requireCode(t, resp, tc.code)
			assert.NotContains(t, msg(resp), "registrar2", "the sponsor is not disclosed")
		})
	}
	t.Run("a registrar can have its own host by the same name", func(t *testing.T) {
		requireCode(t, c.roundTrip(hostCreateXML(theirs)), "1000")
		requireCode(t, c.roundTrip(hostInfoXML(theirs)), "1000")
	})
}

func TestHostUpdate(t *testing.T) {
	creds, _, c := hostServer(t)
	reg := creds.domains.registry
	requireCode(t, c.roundTrip(hostCreateXML("ns1.mine.test", addrXML("v4", testV4))), "1000")
	update := func(changes string) *etree.Document { return c.roundTrip(hostUpdateXML("ns1.mine.test", changes)) }
	info := func() *etree.Document { return c.roundTrip(hostInfoXML("ns1.mine.test")) }

	t.Run("addresses are added and removed", func(t *testing.T) {
		requireCode(t, update(addXML(addrXML("v6", testV6), addrXML("v4", testV4b))+remXML(addrXML("v4", testV4))), "1000")
		assert.Equal(t, map[string]string{testV4b: "v4", testV6: "v6"}, addresses(info()))
		assert.Equal(t, testRegistrar, text(info(), "//infData/upID"))
	})
	t.Run("client statuses are added and removed", func(t *testing.T) {
		requireCode(t, update(addXML(statusXML("clientDeleteProhibited"))), "1000")
		assert.Equal(t, []string{"clientDeleteProhibited"}, statuses(info()))
		requireCode(t, update(remXML(statusXML("clientDeleteProhibited"))), "1000")
		assert.Equal(t, []string{"ok"}, statuses(info()))
	})
	for _, tc := range []struct{ name, changes, code string }{
		{"adding a status the server sets", addXML(statusXML("serverUpdateProhibited")), "2306"},
		{"adding linked", addXML(statusXML("linked")), "2306"},
		{"adding ok", addXML(statusXML("ok")), "2306"},
		{"adding a pending status", addXML(statusXML("pendingDelete")), "2306"},
		{"removing a status the server sets", remXML(statusXML("serverDeleteProhibited")), "2306"},
		{"removing a status the host does not have", remXML(statusXML("clientDeleteProhibited")), "2306"},
		{"removing an address the host does not have", remXML(addrXML("v4", "208.77.190.195")), "2306"},
		{"adding an address the host has", addXML(addrXML("v6", testV6)), "2306"},
		{"adding loopback", addXML(addrXML("v6", "::1")), "2306"},
		{"adding an IPv6 address as v4", addXML(addrXML("v4", testV6)), "2005"},
		{"a status the schema does not have", addXML(statusXML("clientHold")), "2001"},
	} {
		t.Run("refused: "+tc.name, func(t *testing.T) {
			before := statuses(info())
			requireCode(t, update(tc.changes), tc.code)
			assert.Equal(t, before, statuses(info()), "nothing changed")
		})
	}
	t.Run("one refused change refuses the whole update", func(t *testing.T) {
		requireCode(t, update(addXML(addrXML("v4", "208.77.190.198"))+remXML(addrXML("v4", "208.77.190.199"))), "2306")
		assert.NotContains(t, addresses(info()), "208.77.190.198")
	})
	t.Run("clientUpdateProhibited refuses every update but one removing it", func(t *testing.T) {
		requireCode(t, update(addXML(statusXML("clientUpdateProhibited"))), "1000")
		requireCode(t, update(addXML(addrXML("v4", "208.77.190.200"))), "2304")
		requireCode(t, update(addXML(addrXML("v4", "208.77.190.200"))+remXML(statusXML("clientUpdateProhibited"))), "1000")
		assert.Equal(t, []string{"ok"}, statuses(info()))
		assert.Contains(t, addresses(info()), "208.77.190.200")
	})
	t.Run("serverUpdateProhibited refuses every update", func(t *testing.T) {
		reg.SetHostStatus("ns1.mine.test", testRegistrar, func(s *entities.HostStatus) { s.ServerUpdateProhibited, s.OK = true, false })
		defer reg.SetHostStatus("ns1.mine.test", testRegistrar, func(s *entities.HostStatus) { s.ServerUpdateProhibited, s.OK = false, true })
		requireCode(t, update(addXML(statusXML("clientDeleteProhibited"))), "2304")
	})
	t.Run("a pending action refuses every update", func(t *testing.T) {
		reg.SetHostStatus("ns1.mine.test", testRegistrar, func(s *entities.HostStatus) { s.PendingDelete, s.OK = true, false })
		defer reg.SetHostStatus("ns1.mine.test", testRegistrar, func(s *entities.HostStatus) { s.PendingDelete, s.OK = false, true })
		requireCode(t, update(addXML(statusXML("clientDeleteProhibited"))), "2304")
	})
}

func TestHostRename(t *testing.T) {
	creds, _, c := hostServer(t)
	reg := creds.domains.registry
	requireCode(t, c.roundTrip(hostCreateXML("ns1.mine.test", addrXML("v4", testV4))), "1000")
	requireCode(t, c.roundTrip(hostCreateXML("ns2.mine.test", addrXML("v4", testV4b))), "1000")

	for _, tc := range []struct{ name, to, code string }{
		{"to an invalid name", "!.invalid", "2005"},
		{"under another registrar's domain", "ns1.registered.test", "2201"},
		{"under an unregistered domain", "ns1.nobody.test", "2303"},
		{"onto another of the registrar's hosts", "ns2.mine.test", "2302"},
	} {
		t.Run("refused: "+tc.name, func(t *testing.T) {
			requireCode(t, c.roundTrip(hostUpdateXML("ns1.mine.test", chgXML(tc.to))), tc.code)
			requireCode(t, c.roundTrip(hostInfoXML("ns1.mine.test")), "1000")
		})
	}
	t.Run("under another of the registrar's domains", func(t *testing.T) {
		requireCode(t, c.roundTrip(hostUpdateXML("ns1.mine.test", chgXML("ns1.also-mine.test"))), "1000")
		requireCode(t, c.roundTrip(hostInfoXML("ns1.mine.test")), "2303")
		info := c.roundTrip(hostInfoXML("ns1.also-mine.test"))
		requireCode(t, info, "1000")
		assert.Equal(t, map[string]string{testV4: "v4"}, addresses(info), "it keeps its addresses")
	})
	t.Run("out of this registry's TLDs: it loses its addresses", func(t *testing.T) {
		requireCode(t, c.roundTrip(hostUpdateXML("ns1.also-mine.test", chgXML("ns1.rst-rename.icann"))), "1000")
		info := c.roundTrip(hostInfoXML("ns1.rst-rename.icann"))
		requireCode(t, info, "1000")
		assert.Empty(t, addresses(info))
	})
	t.Run("out of this registry's TLDs, adding an address: refused", func(t *testing.T) {
		requireCode(t, c.roundTrip(hostUpdateXML("ns2.mine.test", chgXML("ns2.rst-rename.icann"))), "1000")
		requireCode(t, c.roundTrip(hostUpdateXML("ns2.rst-rename.icann", addXML(addrXML("v4", testV4)))), "2306")
	})
	t.Run("a nameserver of another registrar's domain: 2305", func(t *testing.T) {
		host := reg.Host("ns1.rst-rename.icann", testRegistrar)
		require.NotNil(t, host)
		reg.AddDomain(&entities.Domain{Name: "imported.test", ClID: "registrar2", RoID: entities.RoidType("4_DOM-" + testRepositoryID), Hosts: []*entities.Host{host}})
		requireCode(t, c.roundTrip(hostUpdateXML("ns1.rst-rename.icann", chgXML("ns3.rst-rename.icann"))), "2305")
	})
}

func TestHostLinked(t *testing.T) {
	_, _, c := hostServer(t)
	requireCode(t, c.roundTrip(hostCreateXML("ns1.mine.test", addrXML("v4", testV4))), "1000")
	requireCode(t, c.roundTrip(createXML("linked.test", createOpts{
		ns: `<domain:ns><domain:hostObj>ns1.registrar1.net</domain:hostObj><domain:hostObj>ns1.mine.test</domain:hostObj></domain:ns>`,
	})), "1000")

	t.Run("info shows linked", func(t *testing.T) {
		assert.Equal(t, []string{"ok", "linked"}, statuses(c.roundTrip(hostInfoXML("ns1.registrar1.net"))))
		assert.Equal(t, []string{"ok"}, statuses(c.roundTrip(hostInfoXML("ns2.registrar1.net"))))
	})
	t.Run("a linked host is not deleted", func(t *testing.T) {
		requireCode(t, c.roundTrip(hostDeleteXML("ns1.registrar1.net")), "2305")
		requireCode(t, c.roundTrip(hostInfoXML("ns1.registrar1.net")), "1000")
	})
	t.Run("a linked internal host keeps an address", func(t *testing.T) {
		requireCode(t, c.roundTrip(hostUpdateXML("ns1.mine.test", remXML(addrXML("v4", testV4)))), "2305")
		requireCode(t, c.roundTrip(hostUpdateXML("ns1.mine.test", addXML(addrXML("v6", testV6))+remXML(addrXML("v4", testV4)))), "1000")
	})
	t.Run("a renamed nameserver is renamed on its domains", func(t *testing.T) {
		requireCode(t, c.roundTrip(hostUpdateXML("ns1.mine.test", chgXML("ns9.mine.test"))), "1000")
		info := c.roundTrip(infoXML("linked.test", "del"))
		var ns []string
		for _, el := range info.FindElements("//infData/ns/hostObj") {
			ns = append(ns, el.Text())
		}
		assert.Contains(t, ns, "ns9.mine.test")
	})
	t.Run("clientDeleteProhibited refuses a delete", func(t *testing.T) {
		requireCode(t, c.roundTrip(hostUpdateXML("ns2.registrar1.net", addXML(statusXML("clientDeleteProhibited")))), "1000")
		requireCode(t, c.roundTrip(hostDeleteXML("ns2.registrar1.net")), "2304")
	})
}

// statuses are the s attributes of an info response's statuses.
func statuses(info *etree.Document) []string {
	var out []string
	for _, el := range info.FindElements("//infData/status") {
		out = append(out, el.SelectAttrValue("s", ""))
	}
	return out
}

// addresses maps an info response's addresses to their ip attributes.
func addresses(info *etree.Document) map[string]string {
	out := map[string]string{}
	for _, el := range info.FindElements("//infData/addr") {
		out[el.Text()] = el.SelectAttrValue("ip", "")
	}
	return out
}

func mustWrite(t *testing.T, doc *etree.Document) string {
	t.Helper()
	s, err := doc.WriteToString()
	require.NoError(t, err)
	return s
}

func TestObjectElementMustMatchCommand(t *testing.T) {
	_, _, c := hostServer(t)
	requireCode(t, c.roundTrip(hostCreateXML("ns1.example.net")), "1000")
	for _, tc := range []struct{ name, frame string }{
		{"<delete> carrying <host:info>", cmdXML(`<delete><host:info xmlns:host="urn:ietf:params:xml:ns:host-1.0"><host:name>ns1.example.net</host:name></host:info></delete>`, "MIX-1")},
		{"<update> carrying <host:delete>", cmdXML(`<update><host:delete xmlns:host="urn:ietf:params:xml:ns:host-1.0"><host:name>ns1.example.net</host:name></host:delete></update>`, "MIX-2")},
		{"<create> carrying <domain:info>", cmdXML(`<create><domain:info xmlns:domain="urn:ietf:params:xml:ns:domain-1.0"><domain:name>mix.test</domain:name></domain:info></create>`, "MIX-3")},
		{"<info> carrying <domain:check>", cmdXML(`<info><domain:check xmlns:domain="urn:ietf:params:xml:ns:domain-1.0"><domain:name>mine.test</domain:name></domain:check></info>`, "MIX-4")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			resp := c.roundTrip(tc.frame)
			requireCode(t, resp, "2001")
			assert.Contains(t, msg(resp), "not an object's")
		})
	}
	requireCode(t, c.roundTrip(hostInfoXML("ns1.example.net")), "1000")
	requireCode(t, c.roundTrip(infoXML("mix.test", "")), "2303")
}

func TestHostUpdateNeedsAChange(t *testing.T) {
	_, _, c := hostServer(t)
	requireCode(t, c.roundTrip(hostCreateXML("ns1.example.net")), "1000")
	resp := c.roundTrip(hostUpdateXML("ns1.example.net", ""))
	requireCode(t, resp, "2003")
	assert.Empty(t, text(c.roundTrip(hostInfoXML("ns1.example.net")), "//infData/upID"), "nothing was written")
}
