package rdap

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onasunnymorning/domain-os/internal/infrastructure/ratelimit"
	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
)

// limitedServer is a server whose clients (127.0.0.1) are limited by the
// node-local GCRA buckets, under access.
func limitedServer(t *testing.T, access *entities.RDAPAccessSet) *testServer {
	t.Helper()
	lookups := registry()
	lookups.access = access
	return startServerWith(t, Options{Config: testConfig("https://rdap.example.test/"), Lookups: lookups, Limiter: ratelimit.NewMemory()})
}

func TestAnonymousClientsAreLimited(t *testing.T) {
	ts := limitedServer(t, nil)
	burst := entities.AnonymousRDAPAccessLimits().PerAddress.Burst

	for i := 0; i < burst; i++ {
		resp, _ := ts.do(t, http.MethodGet, "help")
		require.Equal(t, http.StatusOK, resp.StatusCode, "request %d is within the burst", i+1)
	}
	resp, body := ts.do(t, http.MethodGet, "help")
	require.Equal(t, http.StatusTooManyRequests, resp.StatusCode, "one past the burst is refused")
	e := decodeError(t, body)
	assert.Equal(t, 429, e.ErrorCode)
	assert.Contains(t, e.Description[0], "scope=address")
	assert.Equal(t, "*", resp.Header.Get("Access-Control-Allow-Origin"))
	retry, err := strconv.Atoi(resp.Header.Get("Retry-After"))
	require.NoError(t, err)
	assert.GreaterOrEqual(t, retry, 1)
	assert.Equal(t, "no-store", resp.Header.Get("Cache-Control"))

	metrics := opsGet(ts.Server, "/metrics").Body.String()
	assert.Contains(t, metrics, `rdap_policy_decisions_total{action="refused",policy="anonymous",scope="address"} 1`)
	assert.Contains(t, metrics, `rdap_requests_total{code="429",route="invalid"} 1`, "a refused request is counted before it is parsed")
}

func TestNotFoundCostsMore(t *testing.T) {
	ts := limitedServer(t, nil)
	l := entities.AnonymousRDAPAccessLimits()
	// Each lookup that finds nothing takes NotFoundCost tokens: one to be
	// admitted, the rest owed after it is answered. Enough misses to reach
	// the burst leave the client in debt.
	misses := (l.PerAddress.Burst + l.NotFoundCost - 1) / l.NotFoundCost
	for i := 0; i < misses; i++ {
		resp, _ := ts.do(t, http.MethodGet, "domain/missing"+strconv.Itoa(i)+".tld")
		require.Equal(t, http.StatusNotFound, resp.StatusCode)
	}
	resp, _ := ts.do(t, http.MethodGet, "help")
	assert.Equal(t, http.StatusTooManyRequests, resp.StatusCode, "%d misses spent a burst of %d", misses, l.PerAddress.Burst)
}

func TestMonitoringNetworksAreNotThrottledAtAnonymousLimits(t *testing.T) {
	access := entities.NewRDAPAccessSet(nil, []entities.RDAPAccessNetwork{
		{Network: netip.MustParsePrefix("127.0.0.0/8"), Policy: entities.RDAPAccessMonitoring},
	}, time.Now())
	ts := limitedServer(t, access)
	for i := 0; i < 3*entities.AnonymousRDAPAccessLimits().PerAddress.Burst; i++ {
		resp, _ := ts.do(t, http.MethodGet, "domain/missing.tld")
		require.Equal(t, http.StatusNotFound, resp.StatusCode, "request %d", i+1)
	}
}

func TestAggregateLimit(t *testing.T) {
	tight := &entities.RDAPAccessPolicy{Name: "tight-aggregate", Limits: entities.RDAPAccessLimits{
		PerAddress: entities.EPPRate{PerSecond: 100, Burst: 100}, PerAggregate: entities.EPPRate{PerSecond: 1, Burst: 2}, NotFoundCost: 1,
	}}
	access := entities.NewRDAPAccessSet([]*entities.RDAPAccessPolicy{tight}, []entities.RDAPAccessNetwork{
		{Network: netip.MustParsePrefix("127.0.0.1/32"), Policy: "tight-aggregate"},
	}, time.Now())
	ts := limitedServer(t, access)
	for i := 0; i < 2; i++ {
		resp, _ := ts.do(t, http.MethodGet, "help")
		require.Equal(t, http.StatusOK, resp.StatusCode)
	}
	resp, body := ts.do(t, http.MethodGet, "help")
	require.Equal(t, http.StatusTooManyRequests, resp.StatusCode)
	assert.Contains(t, decodeError(t, body).Description[0], "scope=aggregate")
}

func TestRequestsOverInFlightAreShed(t *testing.T) {
	lookups := registry()
	lookups.block = make(chan struct{})
	cfg := testConfig("https://rdap.example.test/")
	cfg.Limits.MaxInFlight = 1
	ts := startServer(t, cfg, lookups)

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		resp, _ := ts.do(t, http.MethodGet, "domain/example.tld")
		assert.Equal(t, http.StatusOK, resp.StatusCode)
	}()
	require.Eventually(t, func() bool { return len(ts.inFlight) == 1 }, 2*time.Second, 5*time.Millisecond)

	resp, body := ts.do(t, http.MethodGet, "help")
	assert.Equal(t, http.StatusServiceUnavailable, resp.StatusCode)
	assert.Equal(t, 503, decodeError(t, body).ErrorCode)
	assert.Equal(t, "1", resp.Header.Get("Retry-After"))
	close(lookups.block)
	wg.Wait()
	assert.Contains(t, opsGet(ts.Server, "/metrics").Body.String(), `rdap_shed_total{reason="in_flight"} 1`)
}

func TestConnectionsOverTheCapAreClosed(t *testing.T) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	refused := 0
	ll := newConnLimitListener(l, 1, func() { refused++ })
	defer func() { _ = ll.Close() }()

	accepted := make(chan net.Conn, 2)
	go func() {
		for {
			c, err := ll.Accept()
			if err != nil {
				return
			}
			accepted <- c
		}
	}()
	first, err := net.Dial("tcp", l.Addr().String())
	require.NoError(t, err)
	defer func() { _ = first.Close() }()
	held := <-accepted

	second, err := net.Dial("tcp", l.Addr().String())
	require.NoError(t, err)
	defer func() { _ = second.Close() }()
	require.NoError(t, second.SetReadDeadline(time.Now().Add(2*time.Second)))
	_, err = second.Read(make([]byte, 1))
	assert.Error(t, err, "the second connection is closed at accept")

	require.NoError(t, held.Close())
	third, err := net.Dial("tcp", l.Addr().String())
	require.NoError(t, err)
	defer func() { _ = third.Close() }()
	select {
	case <-accepted:
	case <-time.After(2 * time.Second):
		t.Fatal("a slot freed by a close is used again")
	}
}

func TestBucketsAreKeptApartByPolicy(t *testing.T) {
	// A monitoring /128 and an anonymous neighbour share an IPv6 /64 and
	// /48, but not their allowance.
	set := entities.NewRDAPAccessSet(nil, []entities.RDAPAccessNetwork{
		{Network: netip.MustParsePrefix("2001:db8:1:2::7/128"), Policy: entities.RDAPAccessMonitoring},
	}, time.Now())
	probe, neighbour := netip.MustParseAddr("2001:db8:1:2::7"), netip.MustParseAddr("2001:db8:1:2::8")
	pb, _ := clientBuckets(probe, set.ForAddress(probe))
	nb, _ := clientBuckets(neighbour, set.ForAddress(neighbour))
	require.Equal(t, entities.RDAPAccessMonitoring, set.ForAddress(probe).Name)
	for _, b := range nb {
		for _, m := range pb {
			assert.NotEqual(t, m.Key, b.Key, "an anonymous neighbour must not spend the probe's tokens")
		}
	}
	assert.Contains(t, pb[0].Key, entities.RDAPAccessMonitoring)
}

// recordingLimiter allows everything and records each call.
type recordingLimiter struct {
	mu    sync.Mutex
	calls []recordedTake
}

type recordedTake struct {
	keys      []string
	cost      int
	maxWait   time.Duration
	cancelled bool
}

func (l *recordingLimiter) Take(ctx context.Context, buckets []entities.EPPRateBucket, cost int, maxWait time.Duration) (entities.EPPRateTake, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	var keys []string
	for _, b := range buckets {
		keys = append(keys, b.Key)
	}
	l.calls = append(l.calls, recordedTake{keys: keys, cost: cost, maxWait: maxWait, cancelled: ctx.Err() != nil})
	return entities.EPPRateTake{Taken: true}, nil
}

func TestAdmissionAndNotFoundCharge(t *testing.T) {
	lim := &recordingLimiter{}
	lookups := registry()
	srv, err := NewServer(Options{Config: testConfig("https://rdap.example.test/"), Lookups: lookups, Limiter: lim})
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	r := httptest.NewRequest(http.MethodGet, "/domain/missing.tld", nil).WithContext(ctx)
	r.RemoteAddr = "192.0.2.7:4242"
	notFound, ok := srv.admit(httptest.NewRecorder(), r)
	require.True(t, ok)

	// The client disconnects, and a reload moves its network to a policy
	// with no surcharge, before the 404 is charged.
	cancel()
	lookups.access = entities.NewRDAPAccessSet(nil, []entities.RDAPAccessNetwork{
		{Network: netip.MustParsePrefix("192.0.2.0/24"), Policy: entities.RDAPAccessMonitoring},
	}, time.Now())
	notFound()

	admitted := lim.calls[0]
	require.Len(t, lim.calls, 1+len(admitted.keys), "admission, then one debit per bucket")
	assert.False(t, admitted.cancelled, "a disconnect must not reach the store as a failure")
	for i, owed := range lim.calls[1:] {
		assert.False(t, owed.cancelled)
		assert.Equal(t, entities.AnonymousRDAPAccessLimits().NotFoundCost-1, owed.cost, "the policy the request was admitted under")
		assert.Equal(t, []string{admitted.keys[i]}, owed.keys, "charged to the buckets it was admitted from, each on its own")
		assert.Equal(t, notFoundDebtWait, owed.maxWait)
		assert.Greater(t, owed.maxWait, 100*24*time.Hour, "a surcharge is never dropped for being far off")
	}
}

func TestConcurrentCloseReleasesOneSlot(t *testing.T) {
	inner, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	l := newConnLimitListener(inner, 2, func() {}).(*connLimitListener)
	defer func() { _ = l.Close() }()

	dial := func() {
		c, err := net.Dial("tcp", inner.Addr().String())
		require.NoError(t, err)
		t.Cleanup(func() { _ = c.Close() })
	}
	dial()
	dial()
	a, err := l.Accept()
	require.NoError(t, err)
	b, err := l.Accept()
	require.NoError(t, err)
	defer func() { _ = b.Close() }()
	require.Len(t, l.slots, 2)

	done := make(chan struct{})
	go func() {
		var wg sync.WaitGroup
		for i := 0; i < 8; i++ {
			wg.Add(1)
			go func() { defer wg.Done(); _ = a.Close() }()
		}
		wg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("closing one connection many times blocked")
	}
	assert.Len(t, l.slots, 1, "one slot back, and the other connection keeps its own")
}

func TestNotFoundDebtSparesNeighbours(t *testing.T) {
	// Ten misses admitted together from one address spend 30 of its
	// aggregate's 60 tokens. A neighbour in the same /24 must still get in:
	// the address's debt is its own, not the aggregate's.
	srv, err := NewServer(Options{Config: testConfig("https://rdap.example.test/"), Lookups: registry(), Limiter: ratelimit.NewMemory()})
	require.NoError(t, err)
	from := func(addr string) *http.Request {
		r := httptest.NewRequest(http.MethodGet, "/domain/missing.tld", nil)
		r.RemoteAddr = addr + ":4242"
		return r
	}
	var owed []func()
	for i := 0; i < entities.AnonymousRDAPAccessLimits().PerAddress.Burst; i++ {
		notFound, ok := srv.admit(httptest.NewRecorder(), from("192.0.2.7"))
		require.True(t, ok, "miss %d is within the address's burst", i+1)
		owed = append(owed, notFound)
	}
	for _, notFound := range owed {
		notFound()
	}
	_, ok := srv.admit(httptest.NewRecorder(), from("192.0.2.8"))
	assert.True(t, ok, "the neighbour pays nothing for the address's misses")
	_, ok = srv.admit(httptest.NewRecorder(), from("192.0.2.7"))
	assert.False(t, ok, "the address does")
}

// stallingLimiter admits, then holds every later take until release closes.
type stallingLimiter struct {
	release chan struct{}
	mu      sync.Mutex
	takes   int
}

func (l *stallingLimiter) Take(context.Context, []entities.EPPRateBucket, int, time.Duration) (entities.EPPRateTake, error) {
	l.mu.Lock()
	l.takes++
	first := l.takes == 1
	l.mu.Unlock()
	if !first {
		<-l.release
	}
	return entities.EPPRateTake{Taken: true}, nil
}

func TestNotFoundIsDeliveredBeforeTheCharge(t *testing.T) {
	lim := &stallingLimiter{release: make(chan struct{})}
	ts := startServerWith(t, Options{Config: testConfig("https://rdap.example.test/"), Lookups: registry(), Limiter: lim})
	defer close(lim.release)

	got := make(chan int, 1)
	go func() {
		resp, _ := ts.do(t, http.MethodGet, "domain/missing.tld")
		got <- resp.StatusCode
	}()
	select {
	case code := <-got:
		assert.Equal(t, http.StatusNotFound, code)
	case <-time.After(2 * time.Second):
		t.Fatal("the 404 waited on the surcharge")
	}
}
