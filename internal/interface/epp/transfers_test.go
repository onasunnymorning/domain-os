package epp

import (
	"testing"
	"time"

	"github.com/beevik/etree"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
)

func transferXML(op, name, inner string) string {
	return cmdXML(`<transfer op="`+op+`"><domain:transfer xmlns:domain="urn:ietf:params:xml:ns:domain-1.0"><domain:name>`+name+`</domain:name>`+inner+`</domain:transfer></transfer>`, "TRN-"+op)
}

func pwXML(code string) string {
	return `<domain:authInfo><domain:pw>` + code + `</domain:pw></domain:authInfo>`
}

func pollReqXML() string { return cmdXML(`<poll op="req"/>`, "POLL-REQ") }

func pollAckXML(id string) string {
	return cmdXML(`<poll op="ack" msgID="`+id+`"/>`, "POLL-ACK")
}

// registrar2Full logs registrar2 in with every object service and RGP.
func registrar2Full(t *testing.T, ts *testServer) *client {
	t.Helper()
	c := dial(t, ts.addr)
	requireCode(t, c.roundTrip(cmdXML(`<login><clID>registrar2</clID><pw>secret-pw-02</pw>
<options><version>1.0</version><lang>en</lang></options>
<svcs><objURI>`+nsDomain+`</objURI><objURI>`+nsHost+`</objURI><objURI>`+nsContact+`</objURI>
<svcExtension><extURI>`+nsRGP+`</extURI></svcExtension></svcs></login>`, "R2-LOGIN")), "1000")
	return c
}

// transferServer is domainServer with .test's transfer lock at 0, as for
// an RST or OT&E TLD, and registrar2 logged in with RGP.
func transferServer(t *testing.T) (*testCredentials, *client, *client) {
	t.Helper()
	creds, ts, c := domainServer(t)
	zero := 0
	creds.domains.registry.SetPolicy("test", func(p *entities.PhasePolicy) { p.TransferLockPeriod = &zero })
	return creds, c, registrar2Full(t, ts)
}

// drain acknowledges every message in c's queue.
func drain(t *testing.T, c *client) {
	t.Helper()
	for {
		resp := c.roundTrip(pollReqXML())
		if code(resp) == "1300" {
			return
		}
		requireCode(t, resp, "1301")
		requireCode(t, c.roundTrip(pollAckXML(resp.FindElement("//response/msgQ").SelectAttrValue("id", ""))), "1000")
	}
}

func requireValidResponse(t *testing.T, doc *etree.Document) {
	t.Helper()
	requireSchemaValid(t, []byte(mustWrite(t, doc)))
}

func TestDomainTransferRequest(t *testing.T) {
	creds, losing, gaining := transferServer(t)
	requireCode(t, losing.roundTrip(createXML("xfer.test", createOpts{pw: testTransferCode})), "1000")

	t.Run("refusals", func(t *testing.T) {
		requireCode(t, gaining.roundTrip(transferXML("request", "xfer.test", "")), "2202")
		requireCode(t, gaining.roundTrip(transferXML("request", "xfer.test", pwXML(testTransferCode+"x"))), "2202")
		requireCode(t, gaining.roundTrip(transferXML("request", "xfer.test", `<domain:authInfo><domain:ext><host:delete xmlns:host="urn:ietf:params:xml:ns:host-1.0"><host:name>ns.example.net</host:name></host:delete></domain:ext></domain:authInfo>`)), "2102")
		requireCode(t, losing.roundTrip(transferXML("request", "xfer.test", pwXML(testTransferCode))), "2106")
		requireCode(t, gaining.roundTrip(transferXML("request", "nothing.test", pwXML(testTransferCode))), "2303")
		requireCode(t, gaining.roundTrip(transferXML("request", "xfer.test", `<domain:period unit="y">10</domain:period>`+pwXML(testTransferCode))), "2306")
		requireCode(t, gaining.roundTrip(transferXML("request", "xfer.test", `<domain:period unit="m">18</domain:period>`+pwXML(testTransferCode))), "2306")
		creds.domains.registry.SetDomainStatus("xfer.test", func(s *entities.DomainStatus) { s.ClientTransferProhibited = true })
		requireCode(t, gaining.roundTrip(transferXML("request", "xfer.test", pwXML(testTransferCode))), "2304")
		creds.domains.registry.SetDomainStatus("xfer.test", func(s *entities.DomainStatus) { s.ClientTransferProhibited = false })
		assert.NotContains(t, statusesOf(losing.roundTrip(infoXML("xfer.test", ""))), "pendingTransfer", "no refusal changed the domain")
	})

	t.Run("in the transfer lock: 2106", func(t *testing.T) {
		lock := 60
		creds.domains.registry.SetPolicy("test", func(p *entities.PhasePolicy) { p.TransferLockPeriod = &lock })
		requireCode(t, losing.roundTrip(createXML("locked.test", createOpts{pw: testTransferCode})), "1000")
		requireCode(t, gaining.roundTrip(transferXML("request", "locked.test", pwXML(testTransferCode))), "2106")
		zero := 0
		creds.domains.registry.SetPolicy("test", func(p *entities.PhasePolicy) { p.TransferLockPeriod = &zero })
	})

	var exDate string
	t.Run("a valid request: 1001, the transfer pending", func(t *testing.T) {
		before := text(losing.roundTrip(infoXML("xfer.test", "")), "//infData/exDate")
		resp := gaining.roundTrip(transferXML("request", "xfer.test", `<domain:period unit="y">1</domain:period>`+pwXML(testTransferCode)))
		requireCode(t, resp, "1001")
		requireValidResponse(t, resp)
		assert.Equal(t, "xfer.test", text(resp, "//trnData/name"))
		assert.Equal(t, "pending", text(resp, "//trnData/trStatus"))
		assert.Equal(t, "registrar2", text(resp, "//trnData/reID"))
		assert.Equal(t, testRegistrar, text(resp, "//trnData/acID"))
		exp, err := time.Parse(time.RFC3339, before)
		require.NoError(t, err)
		exDate = text(resp, "//trnData/exDate")
		assert.Equal(t, eppTime(exp.AddDate(1, 0, 0)), exDate, "exDate: a year added")
		assert.Contains(t, statusesOf(losing.roundTrip(infoXML("xfer.test", ""))), "pendingTransfer")
	})
	t.Run("a second request while pending: 2300", func(t *testing.T) {
		requireCode(t, gaining.roundTrip(transferXML("request", "xfer.test", pwXML(testTransferCode))), "2300")
	})
	t.Run("the losing registrar is told by poll", func(t *testing.T) {
		resp := losing.roundTrip(pollReqXML())
		requireCode(t, resp, "1301")
		requireValidResponse(t, resp)
		assert.Equal(t, "1", resp.FindElement("//response/msgQ").SelectAttrValue("count", ""))
		assert.Equal(t, entities.PollMsgTransferRequested, text(resp, "//response/msgQ/msg"))
		assert.Equal(t, "pending", text(resp, "//resData/trnData/trStatus"))
		requireCode(t, gaining.roundTrip(pollReqXML()), "1300")
	})
	t.Run("query: by either registrar, or with the code; anyone else 2201", func(t *testing.T) {
		for _, c := range []*client{losing, gaining} {
			resp := c.roundTrip(transferXML("query", "xfer.test", ""))
			requireCode(t, resp, "1000")
			requireValidResponse(t, resp)
			assert.Equal(t, "pending", text(resp, "//trnData/trStatus"))
			assert.Equal(t, exDate, text(resp, "//trnData/exDate"))
		}
		// registered.test is registrar2's, never transferred.
		requireCode(t, losing.roundTrip(transferXML("query", "registered.test", "")), "2201")
		requireCode(t, losing.roundTrip(transferXML("query", "registered.test", pwXML(testTransferCode))), "2202")
		requireCode(t, gaining.roundTrip(transferXML("query", "registered.test", "")), "2301")
	})
	t.Run("approve, reject and cancel only by their registrars: 2201", func(t *testing.T) {
		requireCode(t, gaining.roundTrip(transferXML("approve", "xfer.test", "")), "2201")
		requireCode(t, gaining.roundTrip(transferXML("reject", "xfer.test", "")), "2201")
		requireCode(t, losing.roundTrip(transferXML("cancel", "xfer.test", "")), "2201")
	})
	t.Run("a transform of a domain pending transfer: 2304", func(t *testing.T) {
		requireCode(t, losing.roundTrip(domainUpdateXML("xfer.test", `<domain:add><domain:status s="clientHold"/></domain:add>`, "")), "2304")
		requireCode(t, losing.roundTrip(domainDeleteXML("xfer.test")), "2304")
	})
}

func TestDomainTransferApprove(t *testing.T) {
	creds, losing, gaining := transferServer(t)
	reg := creds.domains.registry
	// A nameserver under the domain (it goes with it), an external one
	// (the gaining registrar gets its own), and one under another of the
	// losing registrar's domains (it keeps its link).
	requireCode(t, losing.roundTrip(createXML("other.test", createOpts{})), "1000")
	requireCode(t, losing.roundTrip(hostCreateXML("ns.other.test", addrXML("v4", testV4))), "1000")
	requireCode(t, losing.roundTrip(createXML("app.test", createOpts{pw: testTransferCode,
		ns: `<domain:ns><domain:hostObj>ns1.registrar1.net</domain:hostObj><domain:hostObj>ns.other.test</domain:hostObj></domain:ns>`})), "1000")
	requireCode(t, losing.roundTrip(hostCreateXML("ns1.app.test", addrXML("v4", testV4))), "1000")
	requireCode(t, losing.roundTrip(domainUpdateXML("app.test", `<domain:add><domain:ns><domain:hostObj>ns1.app.test</domain:hostObj></domain:ns><domain:status s="clientUpdateProhibited"/></domain:add>`, "")), "1000")
	drain(t, losing)
	before, err := time.Parse(time.RFC3339, text(losing.roundTrip(infoXML("app.test", "")), "//infData/exDate"))
	require.NoError(t, err)

	requireCode(t, gaining.roundTrip(transferXML("request", "app.test", `<domain:period unit="y">2</domain:period>`+pwXML(testTransferCode))), "1001")
	resp := losing.roundTrip(transferXML("approve", "app.test", ""))
	requireCode(t, resp, "1000")
	requireValidResponse(t, resp)
	assert.Equal(t, "clientApproved", text(resp, "//trnData/trStatus"))
	assert.Equal(t, testRegistrar, text(resp, "//trnData/acID"))

	info := gaining.roundTrip(infoXML("app.test", ""))
	requireCode(t, info, "1000")
	requireValidResponse(t, info)
	t.Run("the domain is the gaining registrar's", func(t *testing.T) {
		assert.Equal(t, "registrar2", text(info, "//infData/clID"))
		assert.NotContains(t, statusesOf(info), "pendingTransfer")
		assert.Contains(t, statusesOf(info), "clientUpdateProhibited", "client statuses stay")
		assert.Equal(t, eppTime(before.AddDate(2, 0, 0)), text(info, "//infData/exDate"), "the period is added")
		assert.NotEmpty(t, text(info, "//infData/trDate"))
		requireCode(t, losing.roundTrip(infoXML("app.test", "")), "2201")
	})
	t.Run("the transfer code is cleared (RFC 9154)", func(t *testing.T) {
		assert.Nil(t, info.FindElement("//infData/authInfo"))
		requireCode(t, losing.roundTrip(transferXML("request", "app.test", pwXML(testTransferCode))), "2202")
	})
	t.Run("the transfer grace period, and no other", func(t *testing.T) {
		var rgp []string
		for _, el := range info.FindElements("//extension/infData/rgpStatus") {
			rgp = append(rgp, el.SelectAttrValue("s", ""))
		}
		assert.Equal(t, []string{"transferPeriod"}, rgp, "the add grace period of the losing registrar's registration ends")
	})
	t.Run("hosts: the subordinate one moves, the external one is the gaining registrar's own, the other keeps its link", func(t *testing.T) {
		assert.ElementsMatch(t, []string{"ns1.app.test", "ns1.registrar1.net", "ns.other.test"}, nameservers(info))
		assert.Equal(t, []string{"ns1.app.test"}, texts(info.FindElements("//infData/host")), "the subordinate host is the gaining registrar's")
		requireCode(t, gaining.roundTrip(hostInfoXML("ns1.app.test")), "1000")
		requireCode(t, losing.roundTrip(hostInfoXML("ns1.app.test")), "2201") // another registrar's host (Phase 4e)
		require.NotNil(t, reg.Host("ns1.registrar1.net", "registrar2"), "created for the gaining registrar")
		require.NotNil(t, reg.Host("ns1.registrar1.net", testRegistrar), "the losing registrar's stays")
		assert.Nil(t, reg.Host("ns.other.test", "registrar2"), "no copy of a host under another registrar's domain")
		assert.Equal(t, entities.ClIDType(testRegistrar), linkedHost(t, reg, "app.test", "ns.other.test").ClID)
		assert.Equal(t, entities.ClIDType("registrar2"), linkedHost(t, reg, "app.test", "ns1.registrar1.net").ClID)
	})
	t.Run("the gaining registrar is told to replace the contacts; the losing registrar is not told what it did", func(t *testing.T) {
		resp := gaining.roundTrip(pollReqXML())
		requireCode(t, resp, "1301")
		assert.Contains(t, text(resp, "//response/msgQ/msg"), "replace them with your own")
		assert.Equal(t, "clientApproved", text(resp, "//resData/trnData/trStatus"))
		mine := losing.roundTrip(pollReqXML())
		assert.Equal(t, "1", mine.FindElement("//response/msgQ").SelectAttrValue("count", ""))
		assert.Equal(t, entities.PollMsgTransferRequested, text(mine, "//response/msgQ/msg"), "only the request")
	})
	t.Run("query after: the completed transfer, to both registrars", func(t *testing.T) {
		for _, c := range []*client{losing, gaining} {
			resp := c.roundTrip(transferXML("query", "app.test", ""))
			requireCode(t, resp, "1000")
			assert.Equal(t, "clientApproved", text(resp, "//trnData/trStatus"))
		}
		requireCode(t, losing.roundTrip(transferXML("approve", "app.test", "")), "2201") // no longer the sponsor
		requireCode(t, gaining.roundTrip(transferXML("approve", "app.test", "")), "2301")
	})
}

// linkedHost is the host object domain has as nameserver name.
func linkedHost(t *testing.T, reg interface {
	Domain(string) *entities.Domain
}, domain, name string) *entities.Host {
	t.Helper()
	for _, h := range reg.Domain(domain).Hosts {
		if h.Name.String() == name {
			return h
		}
	}
	t.Fatalf("%s has no nameserver %s", domain, name)
	return nil
}

func TestDomainTransferRejectAndCancel(t *testing.T) {
	_, losing, gaining := transferServer(t)
	requireCode(t, losing.roundTrip(createXML("rej.test", createOpts{pw: testTransferCode})), "1000")
	drain(t, losing)

	t.Run("rejected: the domain stays as it was", func(t *testing.T) {
		requireCode(t, gaining.roundTrip(transferXML("request", "rej.test", pwXML(testTransferCode))), "1001")
		resp := losing.roundTrip(transferXML("reject", "rej.test", ""))
		requireCode(t, resp, "1000")
		requireValidResponse(t, resp)
		assert.Equal(t, "clientRejected", text(resp, "//trnData/trStatus"))
		assert.Nil(t, resp.FindElement("//trnData/exDate"), "no expiry change")
		info := losing.roundTrip(infoXML("rej.test", ""))
		assert.Equal(t, testRegistrar, text(info, "//infData/clID"))
		assert.NotContains(t, statusesOf(info), "pendingTransfer")
		assert.NotNil(t, info.FindElement("//infData/authInfo/pw"), "the code stays")
		msg := gaining.roundTrip(pollReqXML())
		requireCode(t, msg, "1301")
		assert.Equal(t, entities.PollMsgTransferRejected, text(msg, "//response/msgQ/msg"))
		requireCode(t, losing.roundTrip(transferXML("reject", "rej.test", "")), "2301")
		drain(t, gaining)
		drain(t, losing)
	})
	t.Run("cancelled by the gaining registrar", func(t *testing.T) {
		requireCode(t, gaining.roundTrip(transferXML("request", "rej.test", pwXML(testTransferCode))), "1001")
		drain(t, losing)
		resp := gaining.roundTrip(transferXML("cancel", "rej.test", ""))
		requireCode(t, resp, "1000")
		assert.Equal(t, "clientCancelled", text(resp, "//trnData/trStatus"))
		assert.Equal(t, "registrar2", text(resp, "//trnData/acID"), "acID: who cancelled")
		assert.NotContains(t, statusesOf(losing.roundTrip(infoXML("rej.test", ""))), "pendingTransfer")
		msg := losing.roundTrip(pollReqXML())
		assert.Equal(t, entities.PollMsgTransferCancelled, text(msg, "//response/msgQ/msg"))
		requireCode(t, gaining.roundTrip(transferXML("cancel", "rej.test", "")), "2301")
	})
}

func TestPoll(t *testing.T) {
	creds, losing, gaining := transferServer(t)
	for _, name := range []string{"p1.test", "p2.test"} {
		requireCode(t, losing.roundTrip(createXML(name, createOpts{pw: testTransferCode})), "1000")
		requireCode(t, gaining.roundTrip(transferXML("request", name, pwXML(testTransferCode))), "1001")
	}
	msgs := creds.domains.registry.Messages(testRegistrar)
	require.Len(t, msgs, 2)

	t.Run("req shows the oldest, and leaves it", func(t *testing.T) {
		for range 2 {
			resp := losing.roundTrip(pollReqXML())
			requireCode(t, resp, "1301")
			q := resp.FindElement("//response/msgQ")
			assert.Equal(t, "2", q.SelectAttrValue("count", ""))
			assert.Equal(t, "p1.test", text(resp, "//resData/trnData/name"))
			assert.NotEmpty(t, text(resp, "//response/msgQ/qDate"))
		}
	})
	t.Run("another registrar's message cannot be acknowledged: 2303", func(t *testing.T) {
		id := losing.roundTrip(pollReqXML()).FindElement("//response/msgQ").SelectAttrValue("id", "")
		requireCode(t, gaining.roundTrip(pollAckXML(id)), "2303")
		requireCode(t, losing.roundTrip(pollAckXML("999999")), "2303")
		requireCode(t, losing.roundTrip(pollAckXML("not-a-number")), "2303")
		requireCode(t, losing.roundTrip(cmdXML(`<poll op="ack"/>`, "ACK-NO-ID")), "2003")
	})
	t.Run("ack removes it and says how many remain", func(t *testing.T) {
		id := losing.roundTrip(pollReqXML()).FindElement("//response/msgQ").SelectAttrValue("id", "")
		resp := losing.roundTrip(pollAckXML(id))
		requireCode(t, resp, "1000")
		requireValidResponse(t, resp)
		assert.Equal(t, "1", resp.FindElement("//response/msgQ").SelectAttrValue("count", ""))
		assert.Equal(t, id, resp.FindElement("//response/msgQ").SelectAttrValue("id", ""))
		assert.Equal(t, "p2.test", text(losing.roundTrip(pollReqXML()), "//resData/trnData/name"))
		requireCode(t, losing.roundTrip(pollAckXML(id)), "2303") // acknowledged once
	})
	t.Run("an empty queue: 1300", func(t *testing.T) {
		drain(t, losing)
		resp := losing.roundTrip(pollReqXML())
		requireCode(t, resp, "1300")
		requireValidResponse(t, resp)
		assert.Nil(t, resp.FindElement("//response/msgQ"))
	})
}
