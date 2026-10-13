package epp

import (
	"context"
	"errors"
	"strings"

	"github.com/beevik/etree"
	epplib "gitlab.com/internetstiftelsen-oss/epp-lib"

	"github.com/onasunnymorning/domain-os/internal/application/commands"
	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
)

// Login results for epp_logins_total.
const (
	loginSuccess = "success"
	loginFailure = "failure"
	loginLocked  = "locked"
	loginError   = "error"
)

// loginFailureReason names a refusal for epp_login_failures_total and the
// log. The client is told none of this: every refusal reads the same.
func loginFailureReason(err error) string {
	for reason, sentinel := range map[string]error{
		"unknown_login":           entities.ErrEPPUnknownLogin,
		"login_disabled":          entities.ErrEPPLoginDisabled,
		"source_not_allowed":      entities.ErrEPPSourceNotAllowed,
		"no_certificate":          entities.ErrEPPCertificateMissing,
		"certificate_not_pinned":  entities.ErrEPPCertificateNotPinned,
		"certificate_out_of_date": entities.ErrEPPCertificateOutOfDate,
		"wrong_password":          entities.ErrEPPPasswordMismatch,
		"locked":                  entities.ErrEPPLoginLocked,
	} {
		if errors.Is(err, sentinel) {
			return reason
		}
	}
	return "other"
}

// login handles <login>. It checks, in order, that the login exists and is
// enabled, that the session comes from one of the registrar's allowed
// networks, that its certificate is pinned and in date, that the (login,
// source) pair is not locked out, and that the password matches (ADR 0014).
//
// Every refusal is answered 2200 with the same text, so a client cannot tell
// which check failed. After MaxLoginAttempts refusals, or while the pair is
// locked out, the answer is 2501 and the session closes.
func (s *Server) login(ctx context.Context, cmd *command) {
	sess := cmd.session
	loginCmd := commands.EPPLoginCommand{
		LoginID:     elementText(cmd.doc, "//login/clID"),
		Password:    elementText(cmd.doc, "//login/pw"),
		NewPassword: elementText(cmd.doc, "//login/newPW"),
		Source:      sess.source,
		CheckSource: s.cfg.Auth.RequireAllowlist,
	}
	if peers := sess.conn.ConnectionState().PeerCertificates; len(peers) > 0 {
		loginCmd.Certificate = peers[0]
	}

	auth, err := s.auth.Login(ctx, loginCmd)
	switch {
	case err == nil:
	case errors.Is(err, entities.ErrEPPLoginLocked):
		s.metrics.logins.WithLabelValues(loginLocked).Inc()
		s.metrics.loginFailures.WithLabelValues(loginFailureReason(err)).Inc()
		s.logger.Warn("EPP login refused: locked out", "connection_id", sess.id, "login", loginCmd.LoginID, "client_ip", sess.clientIP, "error", err)
		cmd.respond(epplib.StatusAuthenticationErrorClosingConnection, "", nil)
		cmd.closeAfterResponse()
		return
	case errors.Is(err, entities.ErrEPPAuthentication):
		reason := loginFailureReason(err)
		s.metrics.logins.WithLabelValues(loginFailure).Inc()
		s.metrics.loginFailures.WithLabelValues(reason).Inc()
		sess.failedLogins++
		s.logger.Warn("EPP login refused", "connection_id", sess.id, "login", loginCmd.LoginID, "client_ip", sess.clientIP,
			"reason", reason, "attempt", sess.failedLogins)
		if sess.failedLogins >= s.cfg.Auth.MaxLoginAttempts {
			cmd.respond(epplib.StatusAuthenticationErrorClosingConnection, "", nil)
			cmd.closeAfterResponse()
			return
		}
		cmd.respond(epplib.StatusAuthenticationError, "", nil)
		return
	case errors.Is(err, entities.ErrInvalidEPPPassword):
		// The credentials were right but <newPW> breaks the password rules;
		// RFC 5730 leaves the login undone and the old password in place.
		s.metrics.logins.WithLabelValues(loginFailure).Inc()
		s.metrics.loginFailures.WithLabelValues("new_password_invalid").Inc()
		cmd.respond(epplib.StatusParameterPolicyError, "Parameter value policy error; newPW must be 6 to 16 printable ASCII characters with no leading, trailing or repeated spaces", nil)
		return
	default:
		s.metrics.logins.WithLabelValues(loginError).Inc()
		s.logger.Error("EPP login failed: credentials unavailable", "connection_id", sess.id, "login", loginCmd.LoginID, "error", err)
		cmd.respond(epplib.StatusCommandFailed, "", nil)
		return
	}

	sess.loginTimer.Stop()
	sess.auth.Store(auth)
	if !sess.state.CompareAndSwap(int32(stateAwaitingLogin), int32(stateAuthenticated)) {
		// The login timeout fired, or the session was ended, while this
		// command was being handled.
		sess.auth.Store(nil)
		cmd.respond(epplib.StatusCommandUseError, "Command use error; login timeout expired", nil)
		cmd.closeAfterResponse()
		return
	}
	s.metrics.logins.WithLabelValues(loginSuccess).Inc()
	s.metrics.clientCertNotAfter.WithLabelValues(string(auth.ClID), string(auth.LoginID)).Set(float64(auth.CertificateNotAfter.Unix()))
	s.logger.Info("EPP login", "connection_id", sess.id, "clid", auth.ClID, "login", auth.LoginID, "role", auth.Role,
		"client_ip", sess.clientIP, "password_replaced", auth.PasswordReplaced())
	cmd.respond(epplib.StatusSuccess, "", nil)
}

func elementText(doc *etree.Document, path string) string {
	if el := doc.FindElement(path); el != nil {
		return strings.TrimSpace(el.Text())
	}
	return ""
}

// logout handles <logout> and closes the session after replying.
func (s *Server) logout(_ context.Context, cmd *command) {
	s.logger.Info("EPP logout", "connection_id", cmd.session.id, "clid", cmd.session.registrar(), "login", cmd.session.loggedInAs())
	cmd.respond(epplib.StatusEndingSession, "", nil)
	cmd.closeAfterResponse()
}

// domainCheck answers <domain:check> with a fixed placeholder response. It
// does not look at the names asked about; the real handler is Phase 4 of
// docs/epp/ARCHITECTURE.md.
func (s *Server) domainCheck(_ context.Context, cmd *command) {
	cmd.respond(epplib.StatusSuccess, "", placeholderCheckData())
}
