package rdap

import (
	"context"
	"fmt"
	"math"
	"net"
	"net/http"
	"net/netip"
	"strconv"
	"sync"
	"time"

	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
)

// RateLimiter takes tokens from several buckets at once, or from none. The
// shared limiter (internal/infrastructure/ratelimit) is one, failing open to
// the node when Redis cannot answer (ADR 0022).
type RateLimiter interface {
	Take(ctx context.Context, buckets []entities.EPPRateBucket, cost int, maxWait time.Duration) (entities.EPPRateTake, error)
}

// Scopes, for the scope label and the refusal's description.
const (
	scopeAddress   = "address"
	scopeAggregate = "aggregate"
)

// Actions, for rdap_policy_decisions_total.
const (
	actionAllowed = "allowed"
	actionRefused = "refused"
)

// notFoundDebtWait is how long a not-found's extra tokens may be owed:
// without limit. The lookup has already been answered, so they are always
// taken, however far they push the client's next requests back; a bounded
// wait would drop the surcharge for a policy with a slow rate and a large
// cost.
const notFoundDebtWait = time.Duration(math.MaxInt64)

// clientBuckets are the buckets a request from addr takes from under p. The
// keys carry the policy: a monitoring /128 inside an anonymous /64 has its
// own allowance, not a share of its neighbours'.
func clientBuckets(addr netip.Addr, p *entities.RDAPAccessPolicy) ([]entities.EPPRateBucket, []string) {
	address, aggregate := entities.RDAPAddressBuckets(addr)
	buckets := []entities.EPPRateBucket{{Key: "addr:" + p.Name + ":" + address.String(), Rate: p.Limits.PerAddress}}
	scopes := []string{scopeAddress}
	if p.Limits.PerAggregate.PerSecond > 0 {
		buckets = append(buckets, entities.EPPRateBucket{Key: "agg:" + p.Name + ":" + aggregate.String(), Rate: p.Limits.PerAggregate})
		scopes = append(scopes, scopeAggregate)
	}
	return buckets, scopes
}

// clientAddr is the request's client address: the peer, or, behind a load
// balancer with PROXY protocol, the client it names.
func clientAddr(r *http.Request) (netip.Addr, bool) {
	ap, err := netip.ParseAddrPort(r.RemoteAddr)
	if err != nil {
		return netip.Addr{}, false
	}
	return ap.Addr().Unmap(), true
}

// admit takes the request's tokens. It answers 429 and returns false when
// the client is over a limit; the request then costs nothing more.
// notFound charges what a lookup that found nothing owes on top, under the
// policy and buckets the request was admitted with, not those of a reload
// that happened during the lookup.
func (s *Server) admit(w http.ResponseWriter, r *http.Request) (notFound func(), ok bool) {
	none := func() {}
	if s.limiter == nil {
		return none, true
	}
	addr, known := clientAddr(r)
	if !known {
		return none, true
	}
	p := s.lookups.AccessPolicy(addr)
	buckets, scopes := clientBuckets(addr, p)
	// A client that disconnects must not look like a Redis outage to the
	// fail-open breaker, so the call does not inherit the request's
	// cancellation; the shared limiter bounds it with its own timeout.
	ctx := context.WithoutCancel(r.Context())
	res, err := s.limiter.Take(ctx, buckets, 1, 0)
	if err != nil || res.Taken {
		s.metrics.policyDecisions.WithLabelValues(p.Name, "", actionAllowed).Inc()
		extra := p.Limits.NotFoundCost - 1
		return func() {
			if extra > 0 {
				_, _ = s.limiter.Take(ctx, buckets, extra, notFoundDebtWait)
			}
		}, true
	}
	scope := scopes[min(res.Limiting, len(scopes)-1)]
	s.metrics.policyDecisions.WithLabelValues(p.Name, scope, actionRefused).Inc()
	retry := int(math.Ceil(res.Wait.Seconds()))
	w.Header().Set("Retry-After", strconv.Itoa(max(retry, 1)))
	s.fail(w, r, http.StatusTooManyRequests, fmt.Sprintf("rate limit exceeded; scope=%s; retry after %ds", scope, max(retry, 1)))
	return none, false
}

// ReportRatesDegraded records whether the limiter has fallen back to the
// node's own buckets because Redis cannot answer.
func (s *Server) ReportRatesDegraded(degraded bool) {
	v := 0.0
	if degraded {
		v = 1
	}
	s.metrics.ratesDegraded.Set(v)
}

// connLimitListener closes connections over max at accept, so a flood costs
// an accept and a close, not a goroutine.
type connLimitListener struct {
	net.Listener
	slots   chan struct{}
	refused func()
}

func newConnLimitListener(l net.Listener, max int, refused func()) net.Listener {
	return &connLimitListener{Listener: l, slots: make(chan struct{}, max), refused: refused}
}

func (l *connLimitListener) Accept() (net.Conn, error) {
	for {
		c, err := l.Listener.Accept()
		if err != nil {
			return nil, err
		}
		select {
		case l.slots <- struct{}{}:
			return &limitedConn{Conn: c, release: func() { <-l.slots }}, nil
		default:
			l.refused()
			_ = c.Close()
		}
	}
}

type limitedConn struct {
	net.Conn
	release func()
	once    sync.Once
}

// Close gives the connection's slot back once, however many times and from
// however many goroutines it is called.
func (c *limitedConn) Close() error {
	c.once.Do(c.release)
	return c.Conn.Close()
}
