package epp

import (
	"errors"
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
	// admit, when set, drops connections from sources it refuses before a
	// TLS byte is read. See Accept for when it runs.
	admit            func(net.Addr) bool
	conns            chan struct{}
	handshakes       chan struct{}
	handshakeTimeout time.Duration
	metrics          *metrics
}

func newNodeListener(l net.Listener, admit func(net.Addr) bool, maxConns, maxHandshakes int, handshakeTimeout time.Duration, m *metrics) *nodeListener {
	return &nodeListener{
		Listener:         l,
		admit:            admit,
		conns:            make(chan struct{}, maxConns),
		handshakes:       make(chan struct{}, maxHandshakes),
		handshakeTimeout: handshakeTimeout,
		metrics:          m,
	}
}

// Accept returns the next connection that fits under both caps. Connections
// over a cap are closed here and never reach the caller.
//
// The source check runs here for a direct connection. Behind PROXY protocol
// the source is only known once the header has been read, and reading it here
// would let one slow client stall every accept; the check runs instead on the
// connection's first read, on its own goroutine and within the handshake
// deadline, before any TLS byte is read.
func (l *nodeListener) Accept() (net.Conn, error) {
	for {
		c, err := l.Listener.Accept()
		if err != nil {
			return nil, err
		}

		_, behindProxy := c.(*proxyproto.Conn)
		if l.admit != nil && !behindProxy && !l.admit(c.RemoteAddr()) {
			l.metrics.connections.WithLabelValues(outcomeSourceIP).Inc()
			_ = c.Close()
			continue
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
		if l.admit != nil && behindProxy {
			nc.admit = l.admit
		}
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

	// admit, when set, is checked once, on the first read (PROXY protocol).
	admit     func(net.Addr) bool
	admitOnce sync.Once
	admitErr  error
	refused   atomic.Bool
}

// errSourceRefused closes a connection from outside every allowed network.
var errSourceRefused = errors.New("source address not in any registrar's allowed networks")

// Read checks the source on the first read, then reads.
func (c *nodeConn) Read(b []byte) (int, error) {
	if c.admit != nil {
		c.admitOnce.Do(func() {
			if !c.admit(c.Conn.RemoteAddr()) {
				c.refused.Store(true)
				c.listener.metrics.connections.WithLabelValues(outcomeSourceIP).Inc()
				c.admitErr = errSourceRefused
			}
		})
		if c.admitErr != nil {
			return 0, c.admitErr
		}
	}
	return c.Conn.Read(b)
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
			if !c.refused.Load() { // already counted as rejected_source_ip
				c.listener.metrics.connections.WithLabelValues(outcomeHandshakeFailed).Inc()
			}
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
