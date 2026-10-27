package rdap

import (
	"net"
	"net/http"
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
