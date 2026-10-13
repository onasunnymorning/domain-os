package epp

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

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
	// loginSessionLimit is a login with good credentials refused because its
	// registrar or login is at its session cap.
	loginSessionLimit = "session_limit"
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
	cmd.attemptedLogin = elementText(cmd.doc, "//login/clID")
	objects, extensions, ok := s.negotiate(cmd)
	if !ok {
		return
	}
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
	// The registry admits the login before the session records it: until
	// then the heartbeat must renew the session as not logged in, or it
	// would add it to the registrar's and login's indexes without a cap
	// check.
	if err := s.authenticateInRegistry(ctx, sess, auth); err != nil {
		s.metrics.logins.WithLabelValues(loginSessionLimit).Inc()
		s.logger.Warn("EPP login refused: session limit", "connection_id", sess.id, "clid", auth.ClID, "login", auth.LoginID,
			"client_ip", sess.clientIP, "error", err)
		cmd.respond(epplib.StatusSessionLimitExceededClosingConnection, "", nil)
		cmd.closeAfterResponse()
		return
	}
	if loginCmd.NewPassword != "" {
		// Only now, with the session admitted, is the password changed.
		replaced, err := s.auth.ReplacePassword(ctx, auth, loginCmd.NewPassword)
		if err != nil {
			s.metrics.logins.WithLabelValues(loginError).Inc()
			s.logger.Error("EPP login failed: <newPW> not applied", "connection_id", sess.id, "login", auth.LoginID, "error", err)
			cmd.respond(epplib.StatusCommandFailed, "Command failed; the password was not changed and the login did not complete", nil)
			cmd.closeAfterResponse()
			return
		}
		auth.ReplacementPasswordID = replaced
	}
	sess.objects, sess.extensions = objects, extensions
	sess.authAt.Store(time.Now().UnixNano())
	sess.auth.Store(auth)
	if !sess.state.CompareAndSwap(int32(stateAwaitingLogin), int32(stateAuthenticated)) {
		// The login timeout fired, or the session was ended, while this
		// command was being handled.
		sess.auth.Store(nil)
		cmd.respond(epplib.StatusCommandUseError, "Command use error; login timeout expired", nil)
		cmd.closeAfterResponse()
		return
	}
	s.metrics.registrarSessions.WithLabelValues(string(auth.ClID), string(auth.LoginID)).Inc()
	s.metrics.logins.WithLabelValues(loginSuccess).Inc()
	s.metrics.clientCertNotAfter.WithLabelValues(string(auth.ClID), string(auth.LoginID)).Set(float64(auth.CertificateNotAfter.Unix()))
	s.logger.Info("EPP login", "connection_id", sess.id, "clid", auth.ClID, "login", auth.LoginID, "role", auth.Role,
		"client_ip", sess.clientIP, "password_replaced", auth.PasswordReplaced())
	cmd.respond(epplib.StatusSuccess, "", nil)
}

// negotiate reads the services a <login> asks for. Every object service
// must be one the greeting offers, or the login is answered 2307
// (unimplemented object service); every extension likewise, or 2103
// (unimplemented extension). Neither counts as a failed login: no
// credential has been checked.
func (s *Server) negotiate(cmd *command) (objects, extensions []string, ok bool) {
	for _, el := range cmd.doc.FindElements("//login/svcs/objURI") {
		uri := strings.TrimSpace(el.Text())
		if !slices.Contains(supportedObjects, uri) {
			cmd.respond(epplib.StatusUnimplementedObjectService, "Unimplemented object service; "+uri+" is not offered", nil)
			return nil, nil, false
		}
		objects = append(objects, uri)
	}
	for _, el := range cmd.doc.FindElements("//login/svcs/svcExtension/extURI") {
		uri := strings.TrimSpace(el.Text())
		if !slices.Contains(supportedExtensions, uri) {
			cmd.respond(epplib.StatusUnimplementedExtension, "Unimplemented extension; "+uri+" is not offered", nil)
			return nil, nil, false
		}
		extensions = append(extensions, uri)
	}
	return objects, extensions, true
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

// domainCheck answers <domain:check>: whether each name is available, in
// the order asked. More names than EPP_MAX_CHECK_NAMES is 2306; a registry
// that cannot answer is 2400.
func (s *Server) domainCheck(ctx context.Context, cmd *command) {
	var names []string
	for _, el := range cmd.doc.FindElements("/epp/command/check/*/name") {
		names = append(names, strings.Join(strings.Fields(el.Text()), " "))
	}
	cmd.objects = names
	s.metrics.checkNames.Observe(float64(len(names)))
	if max := s.cfg.Commands.MaxCheckNames; len(names) > max {
		cmd.respond(epplib.StatusParameterPolicyError, fmt.Sprintf("Parameter value policy error; at most %d names per check", max), nil)
		return
	}
	ctx, cancel := context.WithTimeout(ctx, commandTimeout)
	defer cancel()
	results, err := s.domains.Check(ctx, names)
	if err != nil {
		s.logger.Error("EPP domain check failed", "connection_id", cmd.session.id, "svtrid", cmd.svTRID, "error", err)
		cmd.respond(epplib.StatusCommandFailed, "", nil)
		return
	}
	cmd.respond(epplib.StatusSuccess, "", checkData(results))
}
