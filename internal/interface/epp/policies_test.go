package epp

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/beevik/etree"
	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onasunnymorning/domain-os/internal/application/epppolicy"
	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
)

// The policy engine (docs/epp/ARCHITECTURE.md §5, ADR 0019) end to end:
// each registrar's commands and sessions are held to its policy.

// checkOf is a <domain:check> of n names.
func checkOf(n int) string {
	var b strings.Builder
	for i := range n {
		b.WriteString("<domain:name>n" + strings.Repeat("x", i%5) + string(rune('a'+i%26)) + ".test</domain:name>")
	}
	return cmdXML(`<check><domain:check xmlns:domain="urn:ietf:params:xml:ns:domain-1.0">`+b.String()+`</domain:check></check>`, "CHK")
}

func TestReadonlyLoginCannotTransform(t *testing.T) {
	creds := newTestCredentials(t)
	creds.update(testRegistrar, func(a *entities.EPPRegistrarAccess) error {
		if _, err := a.AddLogin("ro-login", entities.EPPLoginRoleReadonly, "test", time.Now()); err != nil {
			return err
		}
		_, err := a.AddPassword("ro-login", "readonly-pw1", fastHash, "test", time.Now())
		return err
	})
	ts := startServerWith(t, testConfig(), listenLoopback(t), creds)
	c := dial(t, ts.addr)
	requireCode(t, c.roundTrip(objectsLoginXML("ro-login", "readonly-pw1")), "1000")

	requireCode(t, c.roundTrip(domainCheckXML("RO-CHECK")), "1000")
	requireCode(t, c.roundTrip(infoXML("registered.test", "")), "2201") // another registrar's: sponsorship, not the role
	resp := c.roundTrip(createXML("ro.test", createOpts{}))
	requireCode(t, resp, "2201")
	assert.Contains(t, msg(resp), "read-only")
	assert.Nil(t, creds.domains.registry.Domain("ro.test"), "nothing was created")
	requireCode(t, c.roundTrip(pollReqXML()), "1300")
	assert.Equal(t, 1.0, counterOf(t, ts.metrics.policyDecisions, "role", "reject"))

	rw := dial(t, ts.addr)
	requireCode(t, rw.roundTrip(objectsLoginXML(testLogin, testPassword)), "1000")
	requireCode(t, rw.roundTrip(createXML("rw.test", createOpts{})), "1000")
}

func TestCheckNamesFollowTheRegistrarsPolicy(t *testing.T) {
	creds := newTestCredentials(t)
	small := entities.DefaultEPPPolicyLimits()
	small.CheckNames = 3
	creds.policies.assign(testRegistrar, "small", small)
	ts := startServerWith(t, testConfig(), listenLoopback(t), creds)
	c := dial(t, ts.addr)
	c.login()

	requireCode(t, c.roundTrip(checkOf(3)), "1000")
	resp := c.roundTrip(checkOf(4))
	requireCode(t, resp, "2306")
	assert.Contains(t, msg(resp), "at most 3 names")
	assert.Equal(t, 1.0, counterOf(t, ts.metrics.policyDecisions, "check_names", "reject"))
}

func TestFamilySessionCap(t *testing.T) {
	creds := newTestCredentials(t)
	rstRegistrar2(t, creds)
	tight := entities.DefaultEPPPolicyLimits()
	tight.SessionsPerFamily = 1
	creds.policies.assign("unused", "tight", tight) // stores the policy
	creds.policies.family("acme", "tight", testRegistrar, "registrar2")
	ts := startServerWith(t, testConfig(), listenLoopback(t), creds)

	dial(t, ts.addr).login()
	other := dial(t, ts.addr)
	requireCode(t, other.roundTrip(objectsLoginXML("registrar2", "secret-pw-02")), "2502")
	other.requireClosed()
	snap := ts.sessions.all()
	require.Len(t, snap, 1)
	assert.Equal(t, "acme", snap[0].familyName())
}

func TestPoliciesReloadWithoutRestart(t *testing.T) {
	creds := newTestCredentials(t)
	ts := startServerWith(t, testConfig(), listenLoopback(t), creds)
	c := dial(t, ts.addr)
	c.login()
	requireCode(t, c.roundTrip(checkOf(5)), "1000")

	creds.policies.limitDefault(func(l *entities.EPPPolicyLimits) { l.CheckNames = 4 })
	ts.refreshPolicies(context.Background())
	requireCode(t, c.roundTrip(checkOf(5)), "2306")

	t.Run("a failed reload keeps the set last loaded", func(t *testing.T) {
		creds.policies.mu.Lock()
		creds.policies.err = errors.New("database down")
		creds.policies.mu.Unlock()
		ts.refreshPolicies(context.Background())
		requireCode(t, c.roundTrip(checkOf(5)), "2306")
		var m dto.Metric
		require.NoError(t, ts.metrics.policyRefreshErrors.Write(&m))
		assert.Equal(t, 1.0, m.GetCounter().GetValue())
	})
}

// A family change reaches the sessions already open at the next reload:
// a registrar that leaves no longer counts against the family, and one
// that joins counts from then on. Open sessions stay open either way.
func TestFamilyMembershipFollowsReload(t *testing.T) {
	creds := newTestCredentials(t)
	rstRegistrar2(t, creds)
	one := entities.DefaultEPPPolicyLimits()
	one.SessionsPerFamily = 1
	creds.policies.assign("unused", "one", one)
	both := &entities.RegistrarFamily{Name: "acme", Policy: "one", Members: []entities.ClIDType{testRegistrar, "registrar2"}}
	creds.policies.setFamilies(both)
	ts := startServerWith(t, testConfig(), listenLoopback(t), creds)
	ctx := context.Background()

	held := dial(t, ts.addr)
	held.login()
	requireCode(t, dial(t, ts.addr).roundTrip(objectsLoginXML("registrar2", "secret-pw-02")), "2502")

	// registrar1 leaves: its session no longer holds acme's slot.
	creds.policies.setFamilies(&entities.RegistrarFamily{Name: "acme", Policy: "one", Members: []entities.ClIDType{"registrar2"}})
	ts.refreshPolicies(ctx)
	r2 := dial(t, ts.addr)
	requireCode(t, r2.roundTrip(objectsLoginXML("registrar2", "secret-pw-02")), "1000")
	requireCode(t, held.roundTrip(domainCheckXML("STILL-OPEN")), "1000")

	// registrar1 joins again: both open sessions stay, and count.
	creds.policies.setFamilies(both)
	ts.refreshPolicies(ctx)
	requireCode(t, held.roundTrip(domainCheckXML("STILL-OPEN-2")), "1000")
	requireCode(t, dial(t, ts.addr).roundTrip(objectsLoginXML("registrar2", "secret-pw-02")), "2502")
	for _, sess := range ts.sessions.all() {
		if sess.auth.Load() != nil {
			assert.Equal(t, "acme", sess.familyName())
		}
	}
}

// A session over its rate is slowed, then refused; checks are metered per
// name (docs/epp/POLICY_REFERENCE.md § Command rates).
func TestCommandRates(t *testing.T) {
	creds := newTestCredentials(t)
	slow := entities.DefaultEPPPolicyLimits()
	slow.Rates.Info = entities.EPPRate{PerSecond: 20, Burst: 1}  // a token every 50ms
	slow.Rates.Check = entities.EPPRate{PerSecond: 1, Burst: 50} // a full check, then a name a second
	creds.policies.assign(testRegistrar, "slow", slow)
	ts := startServerWith(t, testConfig(), listenLoopback(t), creds)
	c := dial(t, ts.addr)
	c.login()

	requireCode(t, c.roundTrip(infoXML("registered.test", "")), "2201")
	start := time.Now()
	requireCode(t, c.roundTrip(infoXML("registered.test", "")), "2201")
	assert.GreaterOrEqual(t, time.Since(start), 30*time.Millisecond, "the second info waited for its token")
	assert.Equal(t, 1.0, counterOf(t, ts.metrics.policyDecisions, "rate", "delay"))

	requireCode(t, c.roundTrip(checkOf(50)), "1000")
	resp := c.roundTrip(checkOf(3))
	requireCode(t, resp, "2306")
	assert.Contains(t, msg(resp), "the registrar's check rate is exceeded, try again in")
	assert.Equal(t, "undef", resp.FindElement("/epp/response/result/extValue/value/*").Tag)
	assert.Regexp(t, `^rate-limit; class=check; scope=registrar; retry-after-ms=\d+$`, resp.FindElement("/epp/response/result/extValue/reason").Text())
	assert.Equal(t, 1.0, counterOf(t, ts.metrics.policyDecisions, "rate", "reject"))
	// The session goes on, and other classes have their own buckets.
	requireCode(t, c.roundTrip(pollReqXML()), "1300")

	t.Run("a session that keeps being refused is closed with 2502", func(t *testing.T) {
		for range epppolicy.RefusalsBeforeClose - 2 {
			requireCode(t, c.roundTrip(checkOf(3)), "2306")
		}
		resp := c.roundTrip(checkOf(3))
		requireCode(t, resp, "2502")
		assert.Contains(t, msg(resp), "command rate refusals")
		c.requireClosed()
		assert.Equal(t, 1.0, counterOf(t, ts.metrics.policyDecisions, "rate", "close"))
		closed := creds.events.of(entities.EventEPPSessionClosedByPolicy)
		require.Len(t, closed, 1)
		assert.Equal(t, "rate", closed[0].Data.(entities.EPPSessionClosedData).Rule)
	})
}

func TestFamilyCommandRates(t *testing.T) {
	creds := newTestCredentials(t)
	rstRegistrar2(t, creds)
	shared := entities.DefaultEPPPolicyLimits()
	shared.FamilyRates.Check = entities.EPPRate{PerSecond: 1, Burst: 60}
	creds.policies.assign("unused", "shared", shared)
	creds.policies.family("acme", "shared", testRegistrar, "registrar2")
	ts := startServerWith(t, testConfig(), listenLoopback(t), creds)

	one := dial(t, ts.addr)
	one.login()
	requireCode(t, one.roundTrip(checkOf(50)), "1000")
	two := dial(t, ts.addr)
	requireCode(t, two.roundTrip(objectsLoginXML("registrar2", "secret-pw-02")), "1000")
	resp := two.roundTrip(checkOf(20))
	requireCode(t, resp, "2306")
	assert.Contains(t, msg(resp), "the registrar family's check rate", "the family's bucket is shared by its members")
	assert.Equal(t, 1.0, counterOf(t, ts.metrics.policyDecisions, "family_rate", "reject"))
	requireCode(t, two.roundTrip(checkOf(10)), "1000") // what is left fits
}

func TestCreateQuota(t *testing.T) {
	creds := newTestCredentials(t)
	one := entities.DefaultEPPPolicyLimits()
	one.CreatesPerDay = 1
	creds.policies.assign(testRegistrar, "one-a-day", one)
	ts := startServerWith(t, testConfig(), listenLoopback(t), creds)
	c := dial(t, ts.addr)
	requireCode(t, c.roundTrip(objectsLoginXML(testLogin, testPassword)), "1000")

	requireCode(t, c.roundTrip(createXML("registered.test", createOpts{})), "2302")
	requireCode(t, c.roundTrip(createXML("first.test", createOpts{})), "1000") // the create that failed did not count
	resp := c.roundTrip(createXML("second.test", createOpts{}))
	requireCode(t, resp, "2308")
	assert.Contains(t, msg(resp), "may create 1 domains a day")
	assert.Nil(t, creds.domains.registry.Domain("second.test"))
	assert.Equal(t, 1.0, counterOf(t, ts.metrics.policyDecisions, "create_quota", "reject"))
	requireCode(t, c.roundTrip(createXML("third.test", createOpts{})), "2308")
	reached := creds.events.of(entities.EventEPPQuotaReached)
	require.Len(t, reached, 1, "recorded once a day, at the first refusal")
	qd := reached[0].Data.(entities.EPPQuotaReachedData)
	assert.Equal(t, 1, qd.CreatesPerDay)
	assert.Equal(t, time.Now().UTC().Format(time.DateOnly), qd.Day)
	requireCode(t, c.roundTrip(hostCreateXML("ns1.first.test", addrXML("v4", "192.0.2.1"))), "1000") // host creates do not count
}

// Creating registered names past the failed-create budget is held, then
// closed, then suspends the login (docs/epp/POLICY_REFERENCE.md § Failed
// creates).
func TestFailedCreateBudget(t *testing.T) {
	creds := newTestCredentials(t)
	tight := entities.DefaultEPPPolicyLimits()
	tight.FailedCreates = entities.EPPFailedCreateBudget{Failures: 2, RatioPercent: 100}
	creds.policies.assign(testRegistrar, "tight", tight)
	creds.escalation = &epppolicy.Escalation{TarpitStrikes: 2, TarpitStep: 50 * time.Millisecond, SuspendAfter: 3, SuspendFor: time.Minute}
	ts := startServerWith(t, testConfig(), listenLoopback(t), creds)
	login := func() *client {
		c := dial(t, ts.addr)
		requireCode(t, c.roundTrip(objectsLoginXML(testLogin, testPassword)), "1000")
		return c
	}
	taken := func(c *client) *etree.Document { return c.roundTrip(createXML("registered.test", createOpts{})) }

	c := login()
	requireCode(t, taken(c), "2302")
	requireCode(t, taken(c), "2302") // two failures: the budget is reached
	start := time.Now()
	requireCode(t, taken(c), "2302")
	requireCode(t, taken(c), "2302")
	assert.GreaterOrEqual(t, time.Since(start), 150*time.Millisecond, "strikes 1 and 2 were held 50ms and 100ms")
	assert.Equal(t, 2.0, counterOf(t, ts.metrics.policyDecisions, "failed_creates", "delay"))

	resp := taken(c)
	requireCode(t, resp, "2502")
	assert.Contains(t, msg(resp), "the registrar's creates of registered names are over their budget")
	assert.Equal(t, "failed-creates; scope=registrar; suspended-for-ms=0", resp.FindElement("/epp/response/result/extValue/reason").Text())
	c.requireClosed()
	closed := creds.events.of(entities.EventEPPSessionClosedByPolicy)
	require.Len(t, closed, 1)
	cd := closed[0].Data.(entities.EPPSessionClosedData)
	assert.Equal(t, "failed_creates", cd.Rule)
	assert.Equal(t, 2502, cd.Code)
	assert.Equal(t, "system", closed[0].Actor)

	other := login()
	c = login()
	resp = taken(c)
	requireCode(t, resp, "2502")
	assert.Contains(t, msg(resp), "the login is suspended for 1m0s")
	assert.Equal(t, "failed-creates; scope=registrar; suspended-for-ms=60000", resp.FindElement("/epp/response/result/extValue/reason").Text())
	c.requireClosed()
	other.requireClosed() // the login's other sessions end too
	assert.Equal(t, 1.0, counterOf(t, ts.metrics.loginsSuspended, testRegistrar))
	suspended := creds.events.of(entities.EventEPPLoginSuspended)
	require.Len(t, suspended, 1)
	assert.Equal(t, "system", suspended[0].Actor)
	data, ok := suspended[0].Data.(entities.EPPLoginSuspendedData)
	require.True(t, ok)
	assert.Equal(t, entities.ClIDType(testRegistrar), data.ClID)
	assert.Equal(t, testLogin, data.Login)
	assert.Equal(t, "registrar", data.Scope)
	assert.Equal(t, 4, data.Strikes, "past SuspendAfter (3)")
	assert.WithinDuration(t, time.Now().Add(time.Minute), data.EndsAt, 2*time.Second)
	assert.NotEmpty(t, data.ClientIP)

	resp = dial(t, ts.addr).roundTrip(objectsLoginXML(testLogin, testPassword))
	requireCode(t, resp, "2501")
	assert.Contains(t, msg(resp), "suspended")

	require.NoError(t, creds.outcomes.Flush(context.Background()))
	got, err := creds.outcomeStore.ListCreateOutcomes(context.Background(), testRegistrar, time.Now(), time.Now())
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, int64(4), got[0].Failed, "the creates that ran and failed; the closed ones did not run")
}

// A <domain:create> waits for one of the node's create slots, and is
// refused with 2400 if none comes within the queue wait.
func TestCreatesWaitForASlot(t *testing.T) {
	creds := newTestCredentials(t)
	cfg := testConfig()
	cfg.Commands.CreatesInFlight, cfg.Commands.CreateQueueWait = 1, 50*time.Millisecond
	ts := startServerWith(t, cfg, listenLoopback(t), creds)
	c := dial(t, ts.addr)
	c.login()

	hold, _, err := ts.creates.acquire(context.Background(), "someone-else", time.Second)
	require.NoError(t, err)
	start := time.Now()
	resp := c.roundTrip(createXML("queued.test", createOpts{}))
	requireCode(t, resp, "2400")
	assert.GreaterOrEqual(t, time.Since(start), 50*time.Millisecond)
	assert.Contains(t, msg(resp), "busy with creates")
	assert.Regexp(t, `^create-queue; waited-ms=\d+$`, resp.FindElement("/epp/response/result/extValue/reason").Text())
	assert.Nil(t, creds.domains.registry.Domain("queued.test"))
	var m dto.Metric
	require.NoError(t, ts.metrics.createQueueTimeouts.Write(&m))
	assert.Equal(t, 1.0, m.GetCounter().GetValue())

	hold() // a slot freed while a create waits goes to it: TestCreateGateTakesTurns
	requireCode(t, c.roundTrip(createXML("queued.test", createOpts{})), "1000")
	running, waiting := ts.creates.inFlight()
	assert.Zero(t, running)
	assert.Zero(t, waiting)
}

// A suspension holds even when its event cannot be written.
func TestSuspensionWithoutItsEvent(t *testing.T) {
	creds := newTestCredentials(t)
	tight := entities.DefaultEPPPolicyLimits()
	tight.FailedCreates = entities.EPPFailedCreateBudget{Failures: 1, RatioPercent: 100}
	creds.policies.assign(testRegistrar, "tight", tight)
	creds.escalation = &epppolicy.Escalation{TarpitStrikes: 0, TarpitStep: time.Millisecond, SuspendAfter: 0, SuspendFor: time.Minute}
	creds.events.err = errors.New("database down")
	ts := startServerWith(t, testConfig(), listenLoopback(t), creds)
	c := dial(t, ts.addr)
	requireCode(t, c.roundTrip(objectsLoginXML(testLogin, testPassword)), "1000")
	requireCode(t, c.roundTrip(createXML("registered.test", createOpts{})), "2302")
	resp := c.roundTrip(createXML("registered.test", createOpts{}))
	requireCode(t, resp, "2502")
	assert.Contains(t, msg(resp), "suspended")
	assert.Equal(t, 1.0, counterOf(t, ts.metrics.eventsLost, entities.EventEPPLoginSuspended))
	requireCode(t, dial(t, ts.addr).roundTrip(objectsLoginXML(testLogin, testPassword)), "2501")
}

// The failed login that starts a lockout is recorded, once.
func TestLockoutIsRecorded(t *testing.T) {
	creds := newTestCredentials(t)
	ts := startServerWith(t, testConfig(), listenLoopback(t), creds)
	bad := loginAsXML(testLogin, "not-the-pw-1", "", "BAD")
	for range 2 { // testConfig allows 3 attempts a session; the test lockout locks at 5
		c := dial(t, ts.addr)
		for range 3 {
			c.roundTrip(bad)
		}
	}
	locked := creds.events.of(entities.EventEPPLoginLocked)
	require.Len(t, locked, 1, "the attempt that locked, not those refused while locked")
	l := locked[0].Data.(entities.EPPLoginLockout)
	assert.Equal(t, entities.ClIDType(testRegistrar), l.ClID)
	assert.Equal(t, testLogin, l.Login)
	assert.Equal(t, "127.0.0.1", l.ClientIP)
	assert.True(t, l.EndsAt.After(time.Now()))
}
