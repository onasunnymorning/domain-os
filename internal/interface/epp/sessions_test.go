package epp

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onasunnymorning/domain-os/internal/application/services"
	"github.com/onasunnymorning/domain-os/internal/config"
	"github.com/onasunnymorning/domain-os/internal/infrastructure/epp/sessionstore"
	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
	"github.com/onasunnymorning/domain-os/pkg/domain/queries"
)

// twoNodes starts two servers sharing one credential store and one session
// registry, as two nodes behind a load balancer share Postgres and Redis.
func twoNodes(t *testing.T, change func(*config.EPP)) (a, b *testServer, creds *testCredentials) {
	t.Helper()
	return twoNodesFor(t, newTestCredentials(t), change)
}

// twoNodesFor is twoNodes over creds, whose policies a test may have set.
func twoNodesFor(t *testing.T, creds *testCredentials, change func(*config.EPP)) (a, b *testServer, _ *testCredentials) {
	t.Helper()
	cfg := testConfig()
	if change != nil {
		change(&cfg)
	}
	cfgA, cfgB := cfg, cfg
	cfgA.Sessions.NodeID, cfgB.Sessions.NodeID = "node-a", "node-b"
	a = startServerWith(t, cfgA, listenLoopback(t), creds)
	b = startServerWith(t, cfgB, listenLoopback(t), creds)
	return a, b, creds
}

// requireRefusedAfterHandshake completes a TLS handshake and requires the
// server to close the connection without a greeting.
func requireRefusedAfterHandshake(t *testing.T, addr string) {
	t.Helper()
	c, err := dialTLS(t, addr, clientTLSConfig(t))
	require.NoError(t, err)
	require.NoError(t, c.conn.Handshake())
	require.NoError(t, c.conn.SetReadDeadline(time.Now().Add(3*time.Second)))
	n, err := c.conn.Read(make([]byte, 1))
	require.Error(t, err, "closed after the handshake")
	require.Zero(t, n, "without a greeting")
	require.NotContains(t, err.Error(), "timeout")
}

// admin is the session service as the admin API uses it.
func (tc *testCredentials) admin() *services.EPPSessionService {
	return services.NewEPPSessionService(tc.registry, entities.EPPSessionCaps{}, 0, nil)
}

func TestSourceCapSpansNodes(t *testing.T) {
	a, b, _ := twoNodes(t, func(c *config.EPP) { c.Sessions.PerSource = 2 })
	dial(t, a.addr)
	dial(t, b.addr)

	requireRefusedAfterHandshake(t, a.addr)
	assert.Equal(t, 1.0, a.counter(t, "connections", outcomeSourceCap))
}

func TestRegistrarCapSpansNodes(t *testing.T) {
	creds := newTestCredentials(t)
	creds.policies.limitDefault(func(l *entities.EPPPolicyLimits) { l.SessionsPerRegistrar = 2 })
	a, b, _ := twoNodesFor(t, creds, nil)
	dial(t, a.addr).login()
	dial(t, b.addr).login()

	third := dial(t, a.addr)
	requireCode(t, third.roundTrip(loginXML("CAP")), "2502")
	third.requireClosed()
	assert.Equal(t, 1.0, a.counter(t, "logins", loginSessionLimit))
}

func TestLoginOwnSessionCap(t *testing.T) {
	a, b, creds := twoNodes(t, nil)
	one := 1
	creds.update(testRegistrar, func(acc *entities.EPPRegistrarAccess) error {
		_, err := acc.UpdateLogin(testLogin, entities.EPPLoginChanges{SessionCap: &one}, time.Now())
		return err
	})
	first := dial(t, a.addr)
	first.login()
	second := dial(t, b.addr)
	requireCode(t, second.roundTrip(loginXML("CAP")), "2502")

	// Logging out frees the slot.
	requireCode(t, first.roundTrip(cmdXML("<logout/>", "OUT")), "1500")
	first.requireClosed()
	require.Eventually(t, func() bool {
		all, _, err := creds.registry.List(context.Background(), queries.ListItemsQuery{Filter: queries.ListEPPSessionsFilter{LoginID: testLogin}})
		return err == nil && len(all) == 0
	}, 2*time.Second, 10*time.Millisecond)
	dial(t, b.addr).login()
}

func TestRegistryRecordsWhoIsConnected(t *testing.T) {
	a, _, creds := twoNodes(t, nil)
	c := dial(t, a.addr)
	c.login()

	all, _, err := creds.admin().List(context.Background(), queries.ListItemsQuery{Filter: queries.ListEPPSessionsFilter{ClID: testRegistrar}})
	require.NoError(t, err)
	require.Len(t, all, 1)
	s := all[0]
	assert.Equal(t, "node-a", s.Node)
	assert.Equal(t, entities.ClIDType(testLogin), s.LoginID)
	assert.Equal(t, "127.0.0.1", s.ClientIP.String())
	assert.NotNil(t, s.AuthenticatedAt)
	pin := creds.access(testRegistrar).CertificatePins[0]
	assert.True(t, s.UsedCredential(pin.ID), "the registry knows which pin the session used")
	assert.Equal(t, 1.0, gauge(t, a.metrics.registrarSessions.WithLabelValues(testRegistrar, testLogin)))
}

func TestAdminKillReachesTheOwningNode(t *testing.T) {
	a, b, creds := twoNodes(t, nil)
	onB := dial(t, b.addr)
	onB.login()
	onA := dial(t, a.addr)
	onA.login()

	sessions, _, err := creds.admin().List(context.Background(), queries.ListItemsQuery{Filter: queries.ListEPPSessionsFilter{ClID: testRegistrar}})
	require.NoError(t, err)
	var target string
	for _, s := range sessions {
		if s.Node == "node-b" {
			target = s.ID
		}
	}
	require.NotEmpty(t, target)

	killed, err := creds.admin().Kill(context.Background(), target)
	require.NoError(t, err)
	assert.Equal(t, "node-b", killed.Node)
	onB.requireClosed()
	requireCode(t, onA.roundTrip(domainCheckXML("ALIVE")), "1000")
	assert.Equal(t, 1.0, counterValue(t, b.metrics.sessionsEnded.WithLabelValues(endedAdminKill)))

	_, err = creds.admin().Kill(context.Background(), "no-such-session")
	require.ErrorIs(t, err, entities.ErrEPPSessionNotFound)
}

func TestKillByCredential(t *testing.T) {
	a, b, creds := twoNodes(t, nil)
	x, y := dial(t, a.addr), dial(t, b.addr)
	x.login()
	y.login()
	pin := creds.access(testRegistrar).CertificatePins[0]

	killed, err := creds.admin().KillMatching(context.Background(), queries.ListEPPSessionsFilter{CredentialID: pin.ID})
	require.NoError(t, err)
	assert.Len(t, killed, 2)
	x.requireClosed()
	y.requireClosed()

	_, err = creds.admin().KillMatching(context.Background(), queries.ListEPPSessionsFilter{})
	require.ErrorIs(t, err, entities.ErrInvalidEPPSessionFilter, "an empty filter would end every session")
}

func TestRegistryOutageFailsOpenToNodeCaps(t *testing.T) {
	cfg := testConfig()
	cfg.Sessions.PerSource = 2
	creds := newTestCredentials(t)
	ts := startServerWith(t, cfg, listenLoopback(t), creds)
	creds.registry.Fail(errors.New("redis down"))

	c := dial(t, ts.addr)
	c.login() // admitted and logged in while the registry is down
	assert.Equal(t, 1.0, gauge(t, ts.metrics.storeDegraded.WithLabelValues(storeSessions)))

	dial(t, ts.addr)
	requireRefusedAfterHandshake(t, ts.addr) // the node still applies the cap to its own sessions

	// When the registry answers again, the heartbeat puts the sessions back.
	creds.registry.Fail(nil)
	require.Eventually(t, func() bool {
		all, _, err := creds.registry.List(context.Background(), queries.ListItemsQuery{})
		return err == nil && len(all) == 2
	}, 5*time.Second, 50*time.Millisecond)
	assert.Equal(t, 0.0, gauge(t, ts.metrics.storeDegraded.WithLabelValues(storeSessions)))
}

func TestHeartbeatRecordsActivity(t *testing.T) {
	creds := newTestCredentials(t)
	ts := startServerWith(t, testConfig(), listenLoopback(t), creds)
	c := dial(t, ts.addr)
	c.login()
	requireCode(t, c.roundTrip(domainCheckXML("CHK-1")), "1000")
	requireCode(t, c.roundTrip(domainCheckXML("CHK-2")), "1000")

	require.Eventually(t, func() bool {
		all, _, err := creds.registry.List(context.Background(), queries.ListItemsQuery{})
		return err == nil && len(all) == 1 && all[0].Commands == 3 && all[0].LastCommandAt != nil
	}, 5*time.Second, 50*time.Millisecond, "the heartbeat writes the command count and last command time")
}

// TestTwoNodesOverRedis runs two nodes against the production registry:
// the Lua scripts, the indexes and pub/sub, end to end. CI starts Redis and
// the test fails there without it; elsewhere it skips.
func TestTwoNodesOverRedis(t *testing.T) {
	client := redis.NewClient(&redis.Options{Addr: "localhost:6379", DB: 13})
	if err := client.Ping(context.Background()).Err(); err != nil {
		if os.Getenv("CI") != "" {
			t.Fatalf("Redis is required in CI: %v", err)
		}
		t.Skipf("no Redis on localhost:6379: %v", err)
	}
	t.Cleanup(func() { _ = client.Close() })
	require.NoError(t, client.FlushDB(context.Background()).Err())

	creds := newTestCredentials(t)
	creds.shared = sessionstore.NewRedis(client)
	creds.policies.limitDefault(func(l *entities.EPPPolicyLimits) { l.SessionsPerRegistrar = 2 })
	cfg := testConfig()
	cfgA, cfgB := cfg, cfg
	cfgA.Sessions.NodeID, cfgB.Sessions.NodeID = "node-a", "node-b"
	a := startServerWith(t, cfgA, listenLoopback(t), creds)
	b := startServerWith(t, cfgB, listenLoopback(t), creds)

	onA := dial(t, a.addr)
	onA.login()
	onB := dial(t, b.addr)
	onB.login()
	requireCode(t, dial(t, a.addr).roundTrip(loginXML("CAP")), "2502")

	admin := services.NewEPPSessionService(creds.shared, entities.EPPSessionCaps{}, 0, nil)
	all, _, err := admin.List(context.Background(), queries.ListItemsQuery{Filter: queries.ListEPPSessionsFilter{ClID: testRegistrar}})
	require.NoError(t, err)
	require.Len(t, all, 2)
	var onBID string
	for _, s := range all {
		if s.Node == "node-b" {
			onBID = s.ID
		}
	}
	_, err = admin.Kill(context.Background(), onBID)
	require.NoError(t, err)
	onB.requireClosed()
	requireCode(t, onA.roundTrip(domainCheckXML("ALIVE")), "1000")

	// The killed session's slot is free again.
	require.Eventually(t, func() bool {
		left, _, err := admin.List(context.Background(), queries.ListItemsQuery{Filter: queries.ListEPPSessionsFilter{ClID: testRegistrar}})
		return err == nil && len(left) == 1
	}, 2*time.Second, 20*time.Millisecond)
	dial(t, b.addr).login()
}

func TestRegistryOutageAdmitsLoginsUpToTheCap(t *testing.T) {
	cfg := testConfig()
	creds := newTestCredentials(t)
	creds.policies.limitDefault(func(l *entities.EPPPolicyLimits) { l.SessionsPerRegistrar, l.SessionsPerLogin = 1, 1 })
	ts := startServerWith(t, cfg, listenLoopback(t), creds)
	creds.registry.Fail(errors.New("redis down"))

	dial(t, ts.addr).login() // a cap of 1 admits one login, not none
	requireCode(t, dial(t, ts.addr).roundTrip(loginXML("CAP")), "2502")
}

// heartbeatDuringLogin runs a heartbeat while the registry is admitting a
// login, and records what the heartbeat renewed.
type heartbeatDuringLogin struct {
	*services.EPPSessionService
	srv     *Server
	renewed []*entities.EPPSession
}

func (h *heartbeatDuringLogin) Authenticate(ctx context.Context, s *entities.EPPSession, l *entities.EPPAuthenticatedLogin, limits entities.EPPSessionLimits) error {
	_ = h.srv.renewOnce(ctx)
	return h.EPPSessionService.Authenticate(ctx, s, l, limits)
}

func (h *heartbeatDuringLogin) Renew(ctx context.Context, sessions []*entities.EPPSession) error {
	h.renewed = append(h.renewed, sessions...)
	return h.EPPSessionService.Renew(ctx, sessions)
}

func TestHeartbeatNeverRenewsAPendingLogin(t *testing.T) {
	cfg := testConfig()
	creds := newTestCredentials(t)
	h := &heartbeatDuringLogin{EPPSessionService: creds.tracker(cfg)}
	opts := creds.options(cfg, func() string { return "PENDING-1" })
	opts.Sessions = h
	srv, err := NewServer(opts)
	require.NoError(t, err)
	h.srv, creds.server = srv, srv
	l := listenLoopback(t)
	done := make(chan error, 1)
	go func() { done <- srv.Serve(l) }()
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = srv.Shutdown(ctx)
		<-done
	})
	require.Eventually(t, srv.Ready, 2*time.Second, 5*time.Millisecond)

	dial(t, l.Addr().String()).login()
	require.NotEmpty(t, h.renewed, "a heartbeat ran during the login")
	for _, s := range h.renewed {
		assert.False(t, s.Authenticated(), "a login the registry has not admitted is renewed as not logged in")
	}
}

func TestNewPasswordIsNotAppliedToALoginOverItsCap(t *testing.T) {
	creds := newTestCredentials(t)
	creds.policies.limitDefault(func(l *entities.EPPPolicyLimits) { l.SessionsPerRegistrar = 1 })
	a, b, _ := twoNodesFor(t, creds, nil)
	dial(t, a.addr).login()

	refused := dial(t, b.addr)
	requireCode(t, refused.roundTrip(loginAsXML(testLogin, testPassword, "rotated-pw-02", "NEW")), "2502")
	refused.requireClosed()

	// The password did not change: the old one is still the one that works.
	c := dial(t, b.addr)
	requireCode(t, c.roundTrip(loginAsXML(testLogin, "rotated-pw-02", "", "NEWPW")), "2200")
}
