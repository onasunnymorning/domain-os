package epp

import (
	"context"
	"strings"

	epplib "gitlab.com/internetstiftelsen-oss/epp-lib"
)

// Login results for epp_logins_total.
const (
	loginSuccess = "success"
	loginFailure = "failure"
	loginLocked  = "locked"
)

// login handles <login>.
//
// It does not authenticate: any request carrying a clID and a pw succeeds.
// Real authentication is Phase 2 of docs/epp/ARCHITECTURE.md.
func (s *Server) login(ctx context.Context, cmd *command) {
	var clID string
	if el := cmd.doc.FindElement("//login/clID"); el != nil {
		clID = strings.TrimSpace(el.Text())
	}
	pw := cmd.doc.FindElement("//login/pw")
	sess := cmd.session
	s.logger.Info("EPP login attempt", "connection_id", sess.id, "clid", clID, "client_ip", sess.clientIP)

	if s.limiter != nil && clID != "" {
		locked, err := s.limiter.IsAccountLocked(ctx, clID)
		if err != nil {
			s.logger.Error("EPP account lock status unavailable", "clid", clID, "error", err)
		}
		if locked {
			s.metrics.logins.WithLabelValues(loginLocked).Inc()
			s.logger.Warn("EPP login refused: account locked", "connection_id", sess.id, "clid", clID, "client_ip", sess.clientIP)
			cmd.respond(epplib.StatusAuthenticationError, "Authentication error; account locked", nil)
			return
		}
	}

	// TODO: Implement actual authentication logic here
	if clID == "" || pw == nil {
		if s.limiter != nil && clID != "" {
			if err := s.limiter.RecordFailedLogin(ctx, clID, sess.clientIP); err != nil {
				s.logger.Warn("EPP failed login recorded", "clid", clID, "client_ip", sess.clientIP, "error", err)
			}
		}
		s.metrics.logins.WithLabelValues(loginFailure).Inc()
		cmd.respond(epplib.StatusAuthenticationError, "Authentication error; invalid credentials", nil)
		return
	}

	if s.limiter != nil {
		if err := s.limiter.ClearFailedLogins(ctx, clID, sess.clientIP); err != nil {
			s.logger.Error("EPP failed logins not cleared", "clid", clID, "client_ip", sess.clientIP, "error", err)
		}
	}

	sess.loginTimer.Stop()
	if !sess.state.CompareAndSwap(int32(stateAwaitingLogin), int32(stateAuthenticated)) {
		// The login timeout fired while this command was being handled.
		cmd.respond(epplib.StatusCommandUseError, "Command use error; login timeout expired", nil)
		cmd.closeAfterResponse()
		return
	}
	sess.clID.Store(&clID)
	s.metrics.logins.WithLabelValues(loginSuccess).Inc()
	cmd.respond(epplib.StatusSuccess, "", nil)
}

// logout handles <logout> and closes the session after replying.
func (s *Server) logout(_ context.Context, cmd *command) {
	s.logger.Info("EPP logout", "connection_id", cmd.session.id, "clid", cmd.session.loggedInAs())
	cmd.respond(epplib.StatusEndingSession, "", nil)
	cmd.closeAfterResponse()
}

// domainCheck answers <domain:check> with a fixed placeholder response. It
// does not look at the names asked about; the real handler is Phase 4 of
// docs/epp/ARCHITECTURE.md.
func (s *Server) domainCheck(_ context.Context, cmd *command) {
	cmd.respond(epplib.StatusSuccess, "", placeholderCheckData())
}
