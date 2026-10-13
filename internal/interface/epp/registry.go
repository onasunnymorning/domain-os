package epp

import (
	"context"
	"time"

	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
)

// renewSessions renews this node's sessions in the cluster-wide registry
// every heartbeat until ctx is done. A renewal also puts back any session the
// registry lost, so after a Redis outage the registry heals within one
// heartbeat.
func (s *Server) renewSessions(ctx context.Context) {
	ticker := time.NewTicker(s.cfg.Sessions.Heartbeat)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
		err := s.renewOnce(ctx)
		if err != nil {
			s.metrics.registryRenewals.WithLabelValues("error").Inc()
			s.reportStoreDegraded(storeSessions, true)
			s.logger.Warn("EPP sessions not renewed in the registry; retrying next heartbeat", "error", err)
			continue
		}
		s.metrics.registryRenewals.WithLabelValues("ok").Inc()
		s.reportStoreDegraded(storeSessions, false)
	}
}

// renewOnce renews every session this node holds. A session that closes
// after its snapshot is taken is not brought back: the registry refuses to
// renew a closed session.
func (s *Server) renewOnce(ctx context.Context) error {
	local := s.sessions.all()
	if len(local) == 0 {
		return nil
	}
	snaps := make([]*entities.EPPSession, len(local))
	for i, sess := range local {
		snaps[i] = sess.snapshot(s.cfg.Sessions.NodeID)
	}
	renewCtx, cancel := context.WithTimeout(ctx, s.cfg.Sessions.Heartbeat/2)
	defer cancel()
	return s.tracker.Renew(renewCtx, snaps)
}

// killRetry is how long to wait before subscribing again after the kill
// subscription fails.
const killRetry = 5 * time.Second

// followKillRequests ends the sessions operators ask to end, until ctx is
// done. Every node receives every request and acts on the sessions it holds.
func (s *Server) followKillRequests(ctx context.Context) {
	for {
		ch, err := s.tracker.KillRequests(ctx)
		if err != nil {
			s.logger.Warn("EPP kill requests not subscribed; retrying", "error", err, "retry_in", killRetry.String())
			select {
			case <-ctx.Done():
				return
			case <-time.After(killRetry):
				continue
			}
		}
		for {
			select {
			case <-ctx.Done():
				return
			case id, ok := <-ch:
				if !ok {
					break
				}
				s.endSession(id, endedAdminKill)
				continue
			}
			break // the subscription ended; subscribe again
		}
	}
}

// endSession closes a session held by this node, if it holds it.
func (s *Server) endSession(id, reason string) {
	sess := s.sessions.get(id)
	if sess == nil {
		return
	}
	if sess.state.Swap(int32(stateClosing)) == int32(stateClosing) {
		return
	}
	s.metrics.sessionsEnded.WithLabelValues(reason).Inc()
	s.logger.Warn("EPP session ended", "reason", reason, "connection_id", sess.id,
		"clid", sess.registrar(), "login", sess.loggedInAs(), "client_ip", sess.clientIP)
	if err := s.lib.CloseConnection(sess.conn); err != nil {
		_ = sess.conn.Close()
	}
}
