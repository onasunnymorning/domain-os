package epp

import (
	"context"
	"crypto/tls"
	"errors"
	"net"
	"time"

	"github.com/onasunnymorning/domain-os/internal/appcontext"
)

// errDraining refuses a session that completed its handshake after drain began.
var errDraining = errors.New("EPP server is draining")

// connContext is epp-lib's ConnContext hook. It runs once per connection,
// after the TLS handshake and before the greeting: it releases the handshake
// slot, applies the connection limit, registers the session and starts its
// login timer. Returning an error closes the connection without a greeting.
func (s *Server) connContext(ctx context.Context, conn *tls.Conn) (context.Context, error) {
	if nc, ok := conn.NetConn().(*nodeConn); ok {
		nc.handshakeComplete()
	}

	clientIP := conn.RemoteAddr().String()
	if tcpAddr, ok := conn.RemoteAddr().(*net.TCPAddr); ok {
		clientIP = tcpAddr.IP.String()
	}

	if s.draining.Load() {
		s.metrics.connections.WithLabelValues(outcomeDraining).Inc()
		return ctx, errDraining
	}

	if s.limiter != nil {
		if err := s.limiter.CheckConnectionLimit(ctx, clientIP, ""); err != nil {
			s.metrics.connections.WithLabelValues(outcomeLimiter).Inc()
			s.logger.Warn("EPP connection rejected", "client_ip", clientIP, "error", err)
			return ctx, err
		}
	}

	sess := &session{
		id:          s.newID(),
		clientIP:    clientIP,
		conn:        conn,
		connectedAt: time.Now(),
	}
	sess.setState(stateAwaitingLogin)
	sess.loginTimer = time.AfterFunc(s.cfg.Timeouts.Login, func() { s.loginTimedOut(sess) })

	ctx = appcontext.WithConnectionID(ctx, sess.id)
	ctx = appcontext.WithClientIP(ctx, clientIP)

	if s.limiter != nil {
		if err := s.limiter.IncrementConnection(ctx, clientIP, ""); err != nil {
			s.logger.Error("EPP connection not tracked", "connection_id", sess.id, "client_ip", clientIP, "error", err)
		}
	}

	s.sessions.add(sess)
	s.metrics.sessionsActive.Inc()
	s.metrics.connections.WithLabelValues(outcomeAccepted).Inc()
	s.logger.Info("EPP session opened", "connection_id", sess.id, "client_ip", clientIP)
	return ctx, nil
}

// loginTimedOut closes a session that has not logged in within the login
// timeout. A command already being handled finishes first.
func (s *Server) loginTimedOut(sess *session) {
	if !sess.state.CompareAndSwap(int32(stateAwaitingLogin), int32(stateClosing)) {
		return // logged in meanwhile, or already closing
	}
	s.metrics.connections.WithLabelValues(outcomeLoginTimeout).Inc()
	s.logger.Info("EPP session closed: no login within the login timeout",
		"connection_id", sess.id, "client_ip", sess.clientIP, "timeout", s.cfg.Timeouts.Login.String())
	if err := s.lib.CloseConnection(sess.conn); err != nil {
		s.logger.Error("EPP session not closed after login timeout", "connection_id", sess.id, "error", err)
	}
}

// closeConn is epp-lib's CloseConnHook. It runs after every connection
// closes, including ones refused in connContext, which never registered.
func (s *Server) closeConn(ctx context.Context, _ *tls.Conn) {
	id, ok := appcontext.ConnectionID(ctx)
	if !ok {
		return
	}
	sess := s.sessions.remove(id)
	if sess == nil {
		return
	}
	sess.loginTimer.Stop()
	s.metrics.sessionsActive.Dec()

	if s.limiter != nil {
		// The session's context is already cancelled; release the count on a
		// fresh one, bounded so a slow Redis cannot hold up connection teardown.
		releaseCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := s.limiter.DecrementConnection(releaseCtx, sess.clientIP, ""); err != nil {
			s.logger.Error("EPP connection not released", "connection_id", sess.id, "client_ip", sess.clientIP, "error", err)
		}
	}

	s.logger.Info("EPP session closed",
		"connection_id", sess.id,
		"client_ip", sess.clientIP,
		"clid", sess.loggedInAs(),
		"commands", sess.commands.Load(),
		"duration_s", int64(time.Since(sess.connectedAt).Seconds()))
}
