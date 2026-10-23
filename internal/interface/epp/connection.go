package epp

import (
	"context"
	"crypto/tls"
	"errors"
	"net"
	"time"

	"github.com/onasunnymorning/domain-os/internal/appcontext"
	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
)

// errDraining refuses a session that completed its handshake after drain began.
var errDraining = errors.New("EPP server is draining")

// sharedStoreTimeout bounds each call to Redis on the connection path, so an
// unresponsive Redis delays a new session by this much at most before the
// node fails open.
const sharedStoreTimeout = 500 * time.Millisecond

// connContext is epp-lib's ConnContext hook. It runs once per connection,
// after the TLS handshake and before the greeting: it releases the handshake
// slot, takes a slot in the session registry under the source address's cap,
// registers the session locally and starts its login timer. Returning an
// error closes the connection without a greeting.
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

	sess := &session{
		id:          s.newID(),
		clientIP:    clientIP,
		source:      addrOf(conn.RemoteAddr()),
		conn:        conn,
		connectedAt: time.Now(),
	}
	if err := s.openInRegistry(ctx, sess); err != nil {
		s.metrics.connections.WithLabelValues(outcomeSourceCap).Inc()
		s.logger.Warn("EPP connection rejected", "client_ip", clientIP, "error", err)
		return ctx, err
	}

	sess.setState(stateAwaitingLogin)
	sess.loginTimer = time.AfterFunc(s.cfg.Timeouts.Login, func() { s.loginTimedOut(sess) })

	ctx = appcontext.WithConnectionID(ctx, sess.id)
	ctx = appcontext.WithClientIP(ctx, clientIP)

	s.sessions.add(sess)
	s.metrics.sessionsActive.Inc()
	s.metrics.connections.WithLabelValues(outcomeAccepted).Inc()
	s.logger.Info("EPP session opened", "connection_id", sess.id, "client_ip", clientIP)
	return ctx, nil
}

// openInRegistry takes the session's slot under its source address's cap.
// If the registry cannot answer, the node fails open (ADR 0013): it applies
// the same cap to its own sessions, so a Redis outage neither refuses every
// registrar nor removes the cap altogether.
func (s *Server) openInRegistry(ctx context.Context, sess *session) error {
	regCtx, cancel := context.WithTimeout(ctx, sharedStoreTimeout)
	defer cancel()
	err := s.tracker.Open(regCtx, sess.snapshot(s.cfg.Sessions.NodeID))
	switch {
	case err == nil:
		s.reportStoreDegraded(storeSessions, false)
		return nil
	case errors.Is(err, entities.ErrEPPSessionLimit):
		return err
	}
	s.reportStoreDegraded(storeSessions, true)
	s.logger.Warn("EPP session registry unavailable; applying the caps on this node only", "client_ip", sess.clientIP, "error", err)
	caps := s.tracker.Caps()
	if s.sessions.count(func(o *session) bool { return o != sess && o.source == sess.source }) >= caps.PerSource {
		return &entities.EPPSessionLimitError{Limit: entities.EPPSessionLimitSource, Cap: caps.PerSource}
	}
	return nil
}

// authenticateInRegistry records who the session logged in as, under limits
// (its registrar's EPP policy, its login's own cap, its family's), failing
// open to this node's own sessions like openInRegistry. The fallback counts
// leave sess out: it is already registered locally, and counting it would
// refuse a login under its cap.
func (s *Server) authenticateInRegistry(ctx context.Context, sess *session, auth *entities.EPPAuthenticatedLogin, limits entities.EPPSessionLimits) error {
	regCtx, cancel := context.WithTimeout(ctx, sharedStoreTimeout)
	defer cancel()
	err := s.tracker.Authenticate(regCtx, sess.snapshot(s.cfg.Sessions.NodeID), auth, limits)
	switch {
	case err == nil:
		s.reportStoreDegraded(storeSessions, false)
		return nil
	case errors.Is(err, entities.ErrEPPSessionLimit):
		return err
	}
	s.reportStoreDegraded(storeSessions, true)
	s.logger.Warn("EPP session registry unavailable; applying the caps on this node only", "connection_id", sess.id, "error", err)
	if n := s.sessions.count(func(o *session) bool { return o != sess && o.registrar() == string(auth.ClID) }); n >= limits.PerRegistrar {
		return &entities.EPPSessionLimitError{Limit: entities.EPPSessionLimitRegistrar, Cap: limits.PerRegistrar}
	}
	if s.sessions.count(func(o *session) bool { return o != sess && o.loggedInAs() == string(auth.LoginID) }) >= limits.PerLogin {
		return &entities.EPPSessionLimitError{Limit: entities.EPPSessionLimitLogin, Cap: limits.PerLogin}
	}
	if limits.Family != "" && s.sessions.count(func(o *session) bool { return o != sess && o.familyName() == limits.Family }) >= limits.PerFamily {
		return &entities.EPPSessionLimitError{Limit: entities.EPPSessionLimitFamily, Cap: limits.PerFamily}
	}
	return nil
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
	if auth := sess.auth.Load(); auth != nil {
		s.metrics.registrarSessions.WithLabelValues(string(auth.ClID), string(auth.LoginID)).Dec()
	}

	// The session's context is already cancelled; release the slot on a
	// fresh one, bounded so a slow Redis cannot hold up teardown. If this
	// fails, the lease expires on its own.
	releaseCtx, cancel := context.WithTimeout(context.Background(), sharedStoreTimeout)
	defer cancel()
	if err := s.tracker.Close(releaseCtx, sess.snapshot(s.cfg.Sessions.NodeID)); err != nil {
		s.logger.Warn("EPP session not removed from the registry; its lease will expire", "connection_id", sess.id, "error", err)
	}

	s.logger.Info("EPP session closed",
		"connection_id", sess.id,
		"client_ip", sess.clientIP,
		"clid", sess.registrar(),
		"login", sess.loggedInAs(),
		"commands", sess.commands.Load(),
		"duration_s", int64(time.Since(sess.connectedAt).Seconds()))
}
