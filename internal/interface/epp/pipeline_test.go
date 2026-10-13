package epp

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/beevik/etree"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onasunnymorning/domain-os/internal/application/services"
	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
)

// loginWithServices is a <login> asking for the given object and extension
// services.
func loginWithServices(objects, extensions []string) string {
	var svcs strings.Builder
	for _, o := range objects {
		svcs.WriteString("<objURI>" + o + "</objURI>")
	}
	if len(extensions) > 0 {
		svcs.WriteString("<svcExtension>")
		for _, e := range extensions {
			svcs.WriteString("<extURI>" + e + "</extURI>")
		}
		svcs.WriteString("</svcExtension>")
	}
	return cmdXML(`<login><clID>`+testLogin+`</clID><pw>`+testPassword+`</pw>
<options><version>1.0</version><lang>en</lang></options><svcs>`+svcs.String()+`</svcs></login>`, "SVC-1")
}

func checkNamesXML(names ...string) string {
	var b strings.Builder
	for _, n := range names {
		b.WriteString("<domain:name>" + n + "</domain:name>")
	}
	return cmdXML(`<check><domain:check xmlns:domain="urn:ietf:params:xml:ns:domain-1.0">`+b.String()+`</domain:check></check>`, "CHK-N")
}

func TestSchemaInvalidCommandsAreSyntaxErrors(t *testing.T) {
	ts := startServer(t, testConfig())
	c := dial(t, ts.addr)

	long := "a-password-too-long-for-epp"
	resp := c.roundTrip(loginAsXML(testLogin, long, "", "BAD-1"))
	requireCode(t, resp, "2001")
	msg := text(resp, "//result/msg")
	assert.Contains(t, msg, "/epp/command/login/pw")
	assert.Contains(t, msg, "6 to 16 characters")
	assert.NotContains(t, msg, long, "a rejected password is never echoed")
	assert.Equal(t, 1.0, ts.counter(t, "framesRejected", rejectSchema))
	assert.Equal(t, 0.0, ts.counter(t, "logins", loginFailure), "a malformed login is not a failed login")

	c.login()
	requireCode(t, c.roundTrip(checkNamesXML()), "2001") // at least one name
	requireCode(t, c.roundTrip(cmdXML(`<check><domain:check xmlns:domain="urn:ietf:params:xml:ns:domain-1.0">
<domain:name>a.test</domain:name><domain:period unit="y">1</domain:period></domain:check></check>`, "BAD-2")), "2001")
	requireCode(t, c.roundTrip(checkNamesXML("ok.test")), "1000")
}

func TestLoginNegotiatesServices(t *testing.T) {
	ts := startServer(t, testConfig())

	t.Run("an object service the server does not offer is 2307", func(t *testing.T) {
		c := dial(t, ts.addr)
		resp := c.roundTrip(loginWithServices([]string{nsDomain, "urn:example:widget-1.0"}, nil))
		requireCode(t, resp, "2307")
		assert.Contains(t, text(resp, "//result/msg"), "urn:example:widget-1.0")
		c.login() // not a failed login: the session can go on
	})

	t.Run("an extension the server does not offer is 2103", func(t *testing.T) {
		c := dial(t, ts.addr)
		requireCode(t, c.roundTrip(loginWithServices([]string{nsDomain}, []string{"urn:ietf:params:xml:ns:secDNS-1.1"})), "2103")
		c.login()
	})

	t.Run("an object not asked for at login is 2307", func(t *testing.T) {
		c := dial(t, ts.addr)
		requireCode(t, c.roundTrip(loginWithServices([]string{nsHost}, nil)), "1000")
		requireCode(t, c.roundTrip(checkNamesXML("ok.test")), "2307")
	})

	t.Run("an extension not asked for at login is 2103", func(t *testing.T) {
		c := dial(t, ts.addr)
		c.login()
		resp := c.roundTrip(cmdXML(`<check><domain:check xmlns:domain="urn:ietf:params:xml:ns:domain-1.0">
<domain:name>ok.test</domain:name></domain:check></check>
<extension><fee:check xmlns:fee="urn:ietf:params:xml:ns:epp:fee-1.0"><fee:currency>USD</fee:currency></fee:check></extension>`, "EXT-1"))
		requireCode(t, resp, "2103")
		assert.Contains(t, text(resp, "//result/msg"), "urn:ietf:params:xml:ns:epp:fee-1.0")
	})
}

func TestDomainCheck(t *testing.T) {
	creds := newTestCredentials(t)
	ts := startServerWith(t, testConfig(), listenLoopback(t), creds)
	c := dial(t, ts.addr)
	c.login()

	resp := c.roundTrip(checkNamesXML("free.test", "registered.test", "reserved.test", "-bad-.test", "elsewhere.example", " spaced.test "))
	requireCode(t, resp, "1000")
	cds := resp.FindElements("//resData/chkData/cd")
	require.Len(t, cds, 6, "one answer per name, in order")
	want := []struct{ name, avail, reason string }{
		{"free.test", "1", ""},
		{"registered.test", "0", "In use"},
		{"reserved.test", "0", "Reserved"},
		{"-bad-.test", "0", "Invalid domain name"},
		{"elsewhere.example", "0", "TLD not served by this registry"},
		{"spaced.test", "1", ""},
	}
	for i, w := range want {
		name := cds[i].FindElement("name")
		assert.Equal(t, w.name, name.Text())
		assert.Equal(t, w.avail, name.SelectAttrValue("avail", ""))
		reason := ""
		if r := cds[i].FindElement("reason"); r != nil {
			reason = r.Text()
		}
		assert.Equal(t, w.reason, reason)
	}
	assert.Equal(t, []string{"free.test", "registered.test", "reserved.test", "-bad-.test", "elsewhere.example", "spaced.test"}, creds.domains.call(0),
		"names reach the registry whitespace-collapsed, as the schema's token type defines them")

	t.Run("more names than the cap is 2306, without asking the registry", func(t *testing.T) {
		names := make([]string, testConfig().Commands.MaxCheckNames+1)
		for i := range names {
			names[i] = fmt.Sprintf("n%d.test", i)
		}
		calls := creds.domains.calls()
		resp := c.roundTrip(checkNamesXML(names...))
		requireCode(t, resp, "2306")
		assert.Contains(t, text(resp, "//result/msg"), "at most 50 names")
		assert.Equal(t, calls, creds.domains.calls())
	})

	t.Run("a registry that cannot answer is 2400, and the session goes on", func(t *testing.T) {
		creds.domains.fail(errors.New("database down"))
		resp := c.roundTrip(checkNamesXML("free.test"))
		requireCode(t, resp, "2400")
		assert.NotContains(t, text(resp, "//result/msg"), "database down")
		creds.domains.fail(nil)
		requireCode(t, c.roundTrip(checkNamesXML("free.test")), "1000")
	})
}

// scriptedPolicy decides by command label.
type scriptedPolicy struct {
	mu        sync.Mutex
	decisions map[string]policyDecision
}

func (p *scriptedPolicy) set(label string, d policyDecision) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.decisions = map[string]policyDecision{label: d}
}

func (p *scriptedPolicy) decide(_ context.Context, cmd *command) policyDecision {
	p.mu.Lock()
	defer p.mu.Unlock()
	if d, ok := p.decisions[cmd.frame.label()]; ok {
		return d
	}
	return policyDecision{action: policyAllow, rule: "none"}
}

func TestPolicyDecisions(t *testing.T) {
	ts := startServer(t, testConfig())
	policy := &scriptedPolicy{}
	ts.policy = policy // before any session, so no goroutine reads it yet
	c := dial(t, ts.addr)
	c.login()

	policy.set("domain:check", policyDecision{action: policyReject, rule: "test_reject", code: 2306, msg: "Parameter value policy error; rejected by test"})
	resp := c.roundTrip(checkNamesXML("free.test"))
	requireCode(t, resp, "2306")
	assert.Equal(t, "Parameter value policy error; rejected by test", text(resp, "//result/msg"))

	policy.set("domain:check", policyDecision{action: policyDelay, rule: "test_delay", wait: 200 * time.Millisecond})
	start := time.Now()
	requireCode(t, c.roundTrip(checkNamesXML("free.test")), "1000")
	assert.GreaterOrEqual(t, time.Since(start), 200*time.Millisecond, "a delayed command is held, then handled")

	policy.set("domain:check", policyDecision{action: policyClose, rule: "test_close", code: 2502, msg: "Session limit exceeded; server closing connection"})
	requireCode(t, c.roundTrip(checkNamesXML("free.test")), "2502")
	c.requireClosed()

	for rule, action := range map[string]string{"test_reject": "reject", "test_delay": "delay", "test_close": "close"} {
		assert.Equal(t, 1.0, counterOf(t, ts.metrics.policyDecisions, rule, action), rule)
	}
}

func TestAuditLog(t *testing.T) {
	creds := newTestCredentials(t)
	ts := startServerWith(t, testConfig(), listenLoopback(t), creds)

	bad := dial(t, ts.addr)
	requireCode(t, bad.roundTrip(loginAsXML(testLogin, "wrong-password", "", "AUD-0")), "2200")

	c := dial(t, ts.addr)
	login := c.roundTrip(loginXML("AUD-1"))
	requireCode(t, login, "1000")
	check := c.roundTrip(checkNamesXML("free.test", "registered.test"))
	requireCode(t, check, "1000")
	requireCode(t, c.roundTrip(cmdXML("<logout/>", "AUD-3")), "1500")

	var txs []entities.EPPTransaction
	require.Eventually(t, func() bool {
		txs = creds.audit.All()
		return len(txs) == 4
	}, 3*time.Second, 20*time.Millisecond, "every command is recorded; queries within a batch interval")
	byCmd := map[string][]entities.EPPTransaction{}
	for _, tx := range txs {
		byCmd[tx.Command] = append(byCmd[tx.Command], tx)
	}

	logins := byCmd["login"]
	require.Len(t, logins, 2)
	for _, tx := range logins {
		assert.NotContains(t, tx.Request, testPassword, "passwords never reach the audit log")
		assert.NotContains(t, tx.Request, "wrong-password")
		assert.Contains(t, tx.Request, "<pw>[redacted]</pw>")
		assert.Equal(t, entities.ClIDType(testLogin), tx.LoginID, "a refused login records the login it presented")
		assert.NotEmpty(t, tx.Response, "a transform keeps its response")
		assert.Equal(t, "127.0.0.1", tx.ClientIP.String())
		assert.Equal(t, "node-a", tx.Node)
	}

	chk := byCmd["domain:check"]
	require.Len(t, chk, 1)
	assert.Equal(t, entities.ClIDType(testRegistrar), chk[0].ClID)
	assert.Equal(t, []string{"free.test", "registered.test"}, chk[0].Objects)
	assert.Equal(t, 1000, chk[0].ResultCode)
	assert.Equal(t, "CHK-N", chk[0].ClTRID)
	assert.Equal(t, text(check, "//trID/svTRID"), chk[0].SvTRID)
	assert.Contains(t, chk[0].Request, "free.test")
	assert.Empty(t, chk[0].Response, "query responses are not kept unless EPP_AUDIT_QUERY_RESPONSES is set")

	require.Len(t, byCmd["logout"], 1)
	assert.Equal(t, 1500, byCmd["logout"][0].ResultCode)
}

func TestAuditKeepsQueryResponsesWhenConfigured(t *testing.T) {
	cfg := testConfig()
	cfg.Audit.QueryResponses = true
	creds := newTestCredentials(t)
	ts := startServerWith(t, cfg, listenLoopback(t), creds)
	c := dial(t, ts.addr)
	c.login()
	requireCode(t, c.roundTrip(checkNamesXML("free.test")), "1000")
	require.Eventually(t, func() bool {
		for _, tx := range creds.audit.All() {
			if tx.Command == "domain:check" {
				return strings.Contains(tx.Response, `avail="1"`)
			}
		}
		return false
	}, 3*time.Second, 20*time.Millisecond)
}

func TestAuditOutageDoesNotWithholdResponses(t *testing.T) {
	creds := newTestCredentials(t)
	ts := startServerWith(t, testConfig(), listenLoopback(t), creds)
	creds.audit.Fail(errors.New("database down"))

	c := dial(t, ts.addr)
	c.login() // the transform's record cannot be written; it is queued and the login still answers
	requireCode(t, c.roundTrip(checkNamesXML("free.test")), "1000")
	assert.Equal(t, 1.0, counterOf(t, ts.metrics.auditErrors, auditStageSync))
	assert.Equal(t, 0.0, counterOf(t, ts.metrics.auditErrors, auditStageLost), "queued, not lost")

	creds.audit.Fail(nil)
	require.Eventually(t, func() bool { return len(creds.audit.All()) == 2 }, 3*time.Second, 20*time.Millisecond,
		"queued records are written once the database answers")
}

func TestRedactedFrame(t *testing.T) {
	doc := etree.NewDocument()
	require.NoError(t, doc.ReadFromString(`<epp xmlns="urn:ietf:params:xml:ns:epp-1.0"><command><create>
<domain:create xmlns:domain="urn:ietf:params:xml:ns:domain-1.0"><domain:name>a.test</domain:name>
<domain:authInfo token="s3cret-attr"><domain:pw roid="X-1">s3cret-auth</domain:pw></domain:authInfo></domain:create></create>
<extension><x:thing xmlns:x="urn:example"><x:newPW>s3cret-new</x:newPW></x:thing></extension></command></epp>`))
	out := redactedFrame(doc)
	assert.NotContains(t, out, "s3cret")
	assert.Contains(t, out, `<domain:authInfo token="[redacted]"><domain:pw roid="[redacted]">[redacted]</domain:pw></domain:authInfo>`,
		"inside authInfo, attribute values are redacted too")
	assert.Contains(t, out, "<domain:name>a.test</domain:name>", "everything else is kept")
	assert.Contains(t, doc.Root().FindElement("//pw").Text(), "s3cret-auth", "the frame itself is not changed")
}

func TestReportAuditErrorCountsLostRecords(t *testing.T) {
	srv, err := NewServer(testOptions(t, testConfig()))
	require.NoError(t, err)
	srv.ReportAuditError(errors.New("batch failed"))
	srv.ReportAuditError(&services.EPPAuditLostError{Records: 7, Err: errors.New("database down")})
	assert.Equal(t, 1.0, counterOf(t, srv.metrics.auditErrors, auditStageBatch))
	assert.Equal(t, 7.0, counterOf(t, srv.metrics.auditErrors, auditStageLost), "each lost record counts")
}

func TestPolicyRefusalsAreAuditedWithWhatTheyAskedFor(t *testing.T) {
	creds := newTestCredentials(t)
	ts := startServerWith(t, testConfig(), listenLoopback(t), creds)
	policy := &scriptedPolicy{}
	ts.policy = policy

	c := dial(t, ts.addr)
	policy.set("login", policyDecision{action: policyReject, rule: "test", code: 2306, msg: "Parameter value policy error; refused"})
	requireCode(t, c.roundTrip(loginXML("POL-1")), "2306")
	policy.set("", policyDecision{})
	c.login()
	policy.set("domain:check", policyDecision{action: policyReject, rule: "test", code: 2306, msg: "Parameter value policy error; refused"})
	requireCode(t, c.roundTrip(checkNamesXML("asked.test", " spaced.test ")), "2306")

	require.Eventually(t, func() bool { return len(creds.audit.All()) == 3 }, 3*time.Second, 20*time.Millisecond)
	for _, tx := range creds.audit.All() {
		switch {
		case tx.Command == "login" && tx.ResultCode == 2306:
			assert.Equal(t, entities.ClIDType(testLogin), tx.LoginID, "a refused login keeps the login it presented")
		case tx.Command == "domain:check":
			assert.Equal(t, 2306, tx.ResultCode)
			assert.Equal(t, []string{"asked.test", "spaced.test"}, tx.Objects, "a refused check keeps the names it asked about")
		}
	}
}
