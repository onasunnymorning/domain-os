package epp

import (
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/pires/go-proxyproto"
)

// nodeListener enforces this node's own caps before epp-lib sees a
// connection: total open connections, and TLS handshakes in progress. Both are
// shed immediately rather than queued, so a flood costs an accept and a close,
// not a goroutine waiting on a slot.
type nodeListener struct {
	net.Listener
	conns            chan struct{}
	handshakes       chan struct{}
	handshakeTimeout time.Duration
	metrics          *metrics
}

func newNodeListener(l net.Listener, maxConns, maxHandshakes int, handshakeTimeout time.Duration, m *metrics) *nodeListener {
	return &nodeListener{
		Listener:         l,
		conns:            make(chan struct{}, maxConns),
		handshakes:       make(chan struct{}, maxHandshakes),
		handshakeTimeout: handshakeTimeout,
		metrics:          m,
	}
}

// Accept returns the next connection that fits under both caps. Connections
// over a cap are closed here and never reach the caller.
func (l *nodeListener) Accept() (net.Conn, error) {
	for {
		c, err := l.Listener.Accept()
		if err != nil {
			return nil, err
		}

		select {
		case l.conns <- struct{}{}:
		default:
			l.metrics.connections.WithLabelValues(outcomeNodeCap).Inc()
			_ = c.Close()
			continue
		}

		select {
		case l.handshakes <- struct{}{}:
		default:
			<-l.conns
			l.metrics.connections.WithLabelValues(outcomeHandshakeCap).Inc()
			_ = c.Close()
			continue
		}

		l.metrics.handshakesActive.Inc()
		nc := &nodeConn{Conn: c, listener: l, handshakeDeadline: time.Now().Add(l.handshakeTimeout)}
		nc.handshaking.Store(true)
		return nc, nil
	}
}

// nodeConn holds a connection's place under the node caps until it closes.
//
// While the TLS handshake is in progress it also clamps every deadline to the
// handshake deadline. epp-lib sets its read and write timeouts before the
// handshake; without the clamp a client could hold a handshake slot for the
// full read timeout.
type nodeConn struct {
	net.Conn
	listener          *nodeListener
	handshakeDeadline time.Time
	handshaking       atomic.Bool
	closeOnce         sync.Once
}

// handshakeComplete releases the handshake slot. It is called once the TLS
// handshake has succeeded; the connection slot is kept until Close.
func (c *nodeConn) handshakeComplete() {
	if c.handshaking.CompareAndSwap(true, false) {
		<-c.listener.handshakes
		c.listener.metrics.handshakesActive.Dec()
	}
}

// Close releases the connection's slots. A connection that closes before its
// handshake completed is counted as a failed handshake: that covers TLS
// errors, timeouts, bad PROXY headers and plain TCP probes alike.
func (c *nodeConn) Close() error {
	c.closeOnce.Do(func() {
		if c.handshaking.CompareAndSwap(true, false) {
			<-c.listener.handshakes
			c.listener.metrics.handshakesActive.Dec()
			c.listener.metrics.connections.WithLabelValues(outcomeHandshakeFailed).Inc()
		}
		<-c.listener.conns
	})
	return c.Conn.Close()
}

func (c *nodeConn) clamp(t time.Time) time.Time {
	if c.handshaking.Load() && (t.IsZero() || t.After(c.handshakeDeadline)) {
		return c.handshakeDeadline
	}
	return t
}

func (c *nodeConn) SetDeadline(t time.Time) error      { return c.Conn.SetDeadline(c.clamp(t)) }
func (c *nodeConn) SetReadDeadline(t time.Time) error  { return c.Conn.SetReadDeadline(c.clamp(t)) }
func (c *nodeConn) SetWriteDeadline(t time.Time) error { return c.Conn.SetWriteDeadline(c.clamp(t)) }

// SetKeepAlive and SetKeepAlivePeriod satisfy epp-lib, which refuses to serve
// a connection without them. They reach through a PROXY protocol wrapper to
// the TCP socket.
func (c *nodeConn) SetKeepAlive(on bool) error {
	if tcp := tcpConn(c.Conn); tcp != nil {
		return tcp.SetKeepAlive(on)
	}
	return nil
}

func (c *nodeConn) SetKeepAlivePeriod(d time.Duration) error {
	if tcp := tcpConn(c.Conn); tcp != nil {
		return tcp.SetKeepAlivePeriod(d)
	}
	return nil
}

func tcpConn(c net.Conn) *net.TCPConn {
	switch v := c.(type) {
	case *net.TCPConn:
		return v
	case *proxyproto.Conn:
		tcp, _ := v.TCPConn()
		return tcp
	default:
		return nil
	}
}

// proxyListener wraps l so the client address comes from a PROXY protocol
// header. Only peers in trusted may connect, and each must send a header;
// anything else is closed before it reaches the node caps.
func proxyListener(l net.Listener, trusted []string, headerTimeout time.Duration) (net.Listener, error) {
	policy, err := proxyproto.TrustProxyHeaderFromRanges(trusted)
	if err != nil {
		return nil, err
	}
	return &proxyproto.Listener{
		Listener:          l,
		ConnPolicy:        policy,
		ReadHeaderTimeout: headerTimeout,
	}, nil
}
