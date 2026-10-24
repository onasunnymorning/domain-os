package epp

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/onasunnymorning/domain-os/internal/application/epppolicy"
	"github.com/onasunnymorning/domain-os/internal/application/interfaces"
	"github.com/onasunnymorning/domain-os/internal/application/services"
	"github.com/onasunnymorning/domain-os/internal/config"
	"github.com/onasunnymorning/domain-os/internal/infrastructure/epp/lockout"
	"github.com/onasunnymorning/domain-os/internal/infrastructure/epp/ratelimit"
	"github.com/onasunnymorning/domain-os/internal/infrastructure/memstore"
	"github.com/onasunnymorning/domain-os/internal/infrastructure/passwordhash"
	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
	"github.com/onasunnymorning/domain-os/pkg/domain/repositories"
)

// The default test credentials, in RST's input shape (ADR 0015): a
// registrar whose login is its own ClID, like epp.clid01, a password like
// epp.pwd01, the test client certificate pinned, and both loopback addresses
// allowed, standing in for epp.clientACL.
const (
	testRegistrar = "registrar1"
	testLogin     = "registrar1"
	testPassword  = "secret-pw-01"
)

// fastHash keeps tests quick; production parameters are tested in
// passwordhash.
var fastHash = passwordhash.New(passwordhash.Params{MemoryKiB: 64, Iterations: 1, Parallelism: 1, SaltLength: 16, KeyLength: 32})

var (
	clientCertOnce sync.Once
	clientCert     tls.Certificate
	clientCertErr  error
)

// testClientCertificate is one client certificate for the whole test
// binary, so the default credentials can pin it.
func testClientCertificate(t testing.TB) tls.Certificate {
	t.Helper()
	clientCertOnce.Do(func() { clientCert, clientCertErr = SelfSignedCertificate() })
	require.NoError(t, clientCertErr)
	return clientCert
}

func certificatePEM(c tls.Certificate) []byte {
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: c.Certificate[0]})
}

// testCredentials is the credential store and session registry behind a
// test server, with the real application services over them. Servers built
// from the same testCredentials share both, as the nodes of a cluster share
// the database and Redis.
type testCredentials struct {
	t        testing.TB
	store    *memstore.EPPCredentials
	registry *memstore.EPPSessions
	// shared, when set, replaces registry: the Redis one, in the test that
	// runs against it.
	shared  repositories.EPPSessionRegistry
	lockout *lockout.Memory
	server  *Server
	// domains answers domain commands, and hosts host commands, over one
	// registry; audit is the audit log's store, behind the real audit
	// service auditLog.
	domains  *testDomains
	hosts    *services.EPPHostService
	contacts *services.EPPContactService
	audit    *memstore.EPPTransactions
	auditLog *services.EPPAuditService
	// policies are the EPP policies the servers load.
	policies *testPolicies
	// rates counts command rates and create quotas: on the node unless a
	// test shares one between servers or sets another.
	rates interfaces.EPPRateLimiter
	// failed counts failed creates; outcomes keeps the daily counts, in
	// outcomeStore.
	failed       *ratelimit.MemoryFailedCreates
	outcomes     *services.EPPCreateOutcomeRecorder
	outcomeStore *memstore.EPPCreateOutcomes
	// escalation, when set, replaces the failed-create escalation.
	escalation *epppolicy.Escalation
	// events records the outbox events the server writes.
	events *testEvents
}

// testEvents records published events; err, when set, fails them.
type testEvents struct {
	mu     sync.Mutex
	events []entities.DomainEvent
	err    error
}

func (e *testEvents) Publish(_ context.Context, events ...entities.DomainEvent) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.err != nil {
		return e.err
	}
	e.events = append(e.events, events...)
	return nil
}

func (e *testEvents) of(typ string) []entities.DomainEvent {
	e.mu.Lock()
	defer e.mu.Unlock()
	var out []entities.DomainEvent
	for _, ev := range e.events {
		if ev.Type == typ {
			out = append(out, ev)
		}
	}
	return out
}

// testPolicies is the EPP policies as a test sets them: built-in by default.
// A running server sees a change at its next refresh (refreshPolicies).
type testPolicies struct {
	mu          sync.Mutex
	policies    []*entities.EPPPolicy
	assignments map[entities.ClIDType]string
	families    []*entities.RegistrarFamily
	err         error
}

func (p *testPolicies) PolicySet(context.Context) (*entities.EPPPolicySet, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.err != nil {
		return nil, p.err
	}
	return entities.NewEPPPolicySet(p.policies, p.assignments, p.families, time.Now()), nil
}

// limitDefault changes the "default" policy's limits, which every
// registrar not assigned another has.
func (p *testPolicies) limitDefault(fn func(*entities.EPPPolicyLimits)) {
	p.mu.Lock()
	defer p.mu.Unlock()
	l := entities.DefaultEPPPolicyLimits()
	fn(&l)
	p.policies = append(p.policies, &entities.EPPPolicy{Name: entities.EPPPolicyDefault, Limits: l})
}

// assign gives clid a custom policy with limits.
func (p *testPolicies) assign(clid, name string, limits entities.EPPPolicyLimits) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.policies = append(p.policies, &entities.EPPPolicy{Name: name, Limits: limits})
	if p.assignments == nil {
		p.assignments = map[entities.ClIDType]string{}
	}
	p.assignments[entities.ClIDType(clid)] = name
}

// setFamilies replaces every family.
func (p *testPolicies) setFamilies(fs ...*entities.RegistrarFamily) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.families = fs
}

// family puts members in a family under policy (empty for "default").
func (p *testPolicies) family(name, policy string, members ...string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	f := &entities.RegistrarFamily{Name: name, Policy: policy}
	for _, m := range members {
		f.Members = append(f.Members, entities.ClIDType(m))
	}
	p.families = append(p.families, f)
}

// newTestCredentials seeds the default test registrar.
func newTestCredentials(t testing.TB) *testCredentials {
	t.Helper()
	tc := &testCredentials{
		t:        t,
		store:    memstore.NewEPPCredentials(testRegistrar, "registrar2"),
		registry: memstore.NewEPPSessions(),
		rates:    ratelimit.NewMemory(),
		failed:   ratelimit.NewMemoryFailedCreates(),
		events:   &testEvents{},
		lockout:  lockout.NewMemory(lockout.Policy{MaxFailures: 5, Window: time.Minute, BaseLock: time.Minute, MaxLock: time.Hour, StrikeMemory: time.Hour}),
		domains:  newTestDomains(),
		audit:    memstore.NewEPPTransactions(),
		policies: &testPolicies{},
	}
	tc.hosts = newTestHosts(tc.domains.registry)
	tc.contacts = services.NewEPPContactService(services.EPPContactServiceDeps{
		Contacts: tc.domains.registry.Contacts(), Roids: services.NewRoidService(tc.domains.registry, testRepositoryID),
	})
	tc.auditLog = services.NewEPPAuditService(tc.audit, 100)
	tc.outcomeStore = memstore.NewEPPCreateOutcomes()
	tc.outcomes = services.NewEPPCreateOutcomeRecorder(tc.outcomeStore)
	ctx, stop := context.WithCancel(context.Background())
	written := make(chan struct{})
	go func() { defer close(written); tc.auditLog.Run(ctx, time.Second, nil) }()
	t.Cleanup(func() { stop(); <-written })
	tc.update(testRegistrar, func(a *entities.EPPRegistrarAccess) error {
		if _, err := a.AddLogin(testLogin, entities.EPPLoginRoleTransform, "test", time.Now()); err != nil {
			return err
		}
		if _, err := a.AddPassword(testLogin, testPassword, fastHash, "test", time.Now()); err != nil {
			return err
		}
		if _, err := a.AddCertificatePin(certificatePEM(testClientCertificate(t)), nil, "test", time.Now()); err != nil {
			return err
		}
		for _, n := range []string{"127.0.0.1", "::1"} {
			if _, err := a.AddAllowedNetwork(n, "loopback", "test", time.Now()); err != nil {
				return err
			}
		}
		return nil
	})
	return tc
}

func (tc *testCredentials) authenticator() *services.EPPAuthenticationService {
	return services.NewEPPAuthenticationService(tc.store, fastHash, tc.lockout, nil, 2)
}

// options are the options for a server built from cfg over these
// credentials.
func (tc *testCredentials) options(cfg config.EPP, newID func() string) Options {
	return Options{
		Config: cfg, Authenticator: tc.authenticator(), Sessions: tc.tracker(cfg),
		Domains: tc.domains, Hosts: tc.hosts, Contacts: tc.contacts, Polls: services.NewEPPPollService(tc.domains.registry.Polls()), Policies: tc.policies, Rates: tc.rates, FailedCreates: tc.failed, CreateOutcomes: tc.outcomes, Escalation: tc.escalation, Events: tc.events, Audit: tc.auditLog, Logger: discardLogger(), NewID: newID,
	}
}

// tracker is the session service a server built from cfg uses.
func (tc *testCredentials) tracker(cfg config.EPP) *services.EPPSessionService {
	caps := entities.EPPSessionCaps{PerSource: cfg.Sessions.PerSource}
	var reg repositories.EPPSessionRegistry = tc.registry
	if tc.shared != nil {
		reg = tc.shared
	}
	return services.NewEPPSessionService(reg, caps, cfg.Sessions.Lease, nil)
}

// update changes a registrar's credentials, as the admin API would.
func (tc *testCredentials) update(clid string, fn func(*entities.EPPRegistrarAccess) error) {
	tc.t.Helper()
	_, err := tc.store.UpdateAccess(context.Background(), clid, func(a *entities.EPPRegistrarAccess) ([]entities.DomainEvent, error) {
		return nil, fn(a)
	})
	require.NoError(tc.t, err)
}

// access returns a registrar's credentials as stored now.
func (tc *testCredentials) access(clid string) *entities.EPPRegistrarAccess {
	tc.t.Helper()
	a, err := tc.store.GetAccess(context.Background(), clid)
	require.NoError(tc.t, err)
	return a
}

// refresh runs the server's credential refresh now, instead of waiting for
// the interval.
func (tc *testCredentials) refresh() {
	tc.t.Helper()
	tc.server.refreshCredentials(context.Background())
}

// pin pins cert for clid.
func (tc *testCredentials) pin(clid string, cert *x509.Certificate) {
	tc.t.Helper()
	tc.update(clid, func(a *entities.EPPRegistrarAccess) error {
		_, err := a.AddCertificatePin(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: cert.Raw}), nil, "test", time.Now())
		return err
	})
}
