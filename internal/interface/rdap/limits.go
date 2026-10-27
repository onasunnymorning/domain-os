package rdap

import (
	"context"
	"fmt"
	"math"
	"net"
	"net/http"
	"net/netip"
	"strconv"
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

// notFoundDebtWait is how long a not-found's extra tokens may be owed. The
// lookup has already been answered, so they are always taken: they only
// push the client's next requests back.
const notFoundDebtWait = time.Hour

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
func (s *Server) admit(w http.ResponseWriter, r *http.Request) (charge func(extra int), ok bool) {
	none := func(int) {}
	if s.limiter == nil {
		return none, true
	}
	addr, known := clientAddr(r)
	if !known {
		return none, true
	}
	p := s.lookups.AccessPolicy(addr)
	buckets, scopes := clientBuckets(addr, p)
	res, err := s.limiter.Take(r.Context(), buckets, 1, 0)
	if err != nil || res.Taken {
		s.metrics.policyDecisions.WithLabelValues(p.Name, "", actionAllowed).Inc()
		return func(extra int) {
			if extra > 0 {
				_, _ = s.limiter.Take(context.WithoutCancel(r.Context()), buckets, extra, notFoundDebtWait)
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
	closed  bool
}

func (c *limitedConn) Close() error {
	if !c.closed {
		c.closed = true
		c.release()
	}
	return c.Conn.Close()
}
