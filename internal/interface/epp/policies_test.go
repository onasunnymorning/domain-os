package epp

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

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
