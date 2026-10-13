package epp

import (
	"context"
	"net"
	"net/netip"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
)

// credentialWatch is the node's periodically refreshed view of registrars'
// credentials: the union of every allowed network, checked before the TLS
// handshake, and the credentials retired as compromised, whose sessions are
// ended. The authoritative checks happen at <login>, against the database;
// this view only sheds strangers early and reaches sessions already open.
type credentialWatch struct {
	networks atomic.Pointer[[]netip.Prefix]
}

// allows reports whether addr is in any registrar's allowed networks. Before
// the first refresh it allows nothing.
func (w *credentialWatch) allows(addr netip.Addr) bool {
	nets := w.networks.Load()
	if nets == nil {
		return false
	}
	addr = addr.Unmap()
	for _, p := range *nets {
		if p.Contains(addr) {
			return true
		}
	}
	return false
}

func (w *credentialWatch) loaded() bool { return w.networks.Load() != nil }

// addrOf returns the IP of a connection's remote address. Behind PROXY
// protocol that is the client's, not the load balancer's.
func addrOf(a net.Addr) netip.Addr {
	if tcp, ok := a.(*net.TCPAddr); ok {
		return tcp.AddrPort().Addr().Unmap()
	}
	if ap, err := netip.ParseAddrPort(a.String()); err == nil {
		return ap.Addr().Unmap()
	}
	return netip.Addr{}
}

// admitSource is the pre-handshake source check. With the allowlist
// disabled (local development) every source is admitted.
func (s *Server) admitSource(a net.Addr) bool {
	return !s.cfg.Auth.RequireAllowlist || s.watch.allows(addrOf(a))
}

// watchCredentials refreshes the credential view every RefreshInterval until
// ctx is done. It refreshes once at once, so the node becomes ready as soon
// as the database answers.
func (s *Server) watchCredentials(ctx context.Context) {
	ticker := time.NewTicker(s.cfg.Auth.RefreshInterval)
	defer ticker.Stop()
	for {
		s.refreshCredentials(ctx)
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

// refreshCredentials reloads the allowed networks and ends sessions that
// authenticated with a credential since retired as compromised. A failed
// load keeps the previous view: the networks only pre-filter, and every
// login is checked against the database itself.
func (s *Server) refreshCredentials(ctx context.Context) {
	ctx, cancel := context.WithTimeout(ctx, s.cfg.Auth.RefreshInterval)
	defer cancel()

	nets, err := s.auth.AllowedNetworks(ctx)
	if err != nil {
		s.metrics.credentialRefreshErrors.Inc()
		s.logger.Error("EPP allowed networks not refreshed; keeping the previous set", "error", err)
	} else {
		s.watch.networks.Store(&nets)
		s.metrics.allowedNetworks.Set(float64(len(nets)))
		s.metrics.credentialsRefreshed.SetToCurrentTime()
	}

	// A session lives at most MaxSession, so older compromises can have no
	// session left to end.
	ids, err := s.auth.CompromisedSince(ctx, time.Now().Add(-s.cfg.Timeouts.MaxSession))
	if err != nil {
		s.metrics.credentialRefreshErrors.Inc()
		s.logger.Error("EPP compromised credentials not checked", "error", err)
		return
	}
	s.endCompromisedSessions(ids)
}

// endCompromisedSessions closes every session on this node that
// authenticated with one of ids (ADR 0014 decision 6).
func (s *Server) endCompromisedSessions(ids []uuid.UUID) {
	if len(ids) == 0 {
		return
	}
	compromised := make(map[uuid.UUID]bool, len(ids))
	for _, id := range ids {
		compromised[id] = true
	}
	for _, sess := range s.sessions.all() {
		auth := sess.auth.Load()
		if auth == nil {
			continue
		}
		for _, id := range auth.CredentialIDs() {
			if compromised[id] {
				s.endSession(sess.id, endedCompromised)
				break
			}
		}
	}
}
