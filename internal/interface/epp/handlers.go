package epp

import (
	"context"
	"log/slog"

	"github.com/beevik/etree"
	epplib "gitlab.com/internetstiftelsen-oss/epp-lib"

	"github.com/onasunnymorning/domain-os/internal/appcontext"
	"github.com/onasunnymorning/domain-os/internal/infrastructure/epp/middleware"
)

// handlers holds what the command handlers share. Every handler adheres to
// epp-lib's CommandFunc signature, and has no error path: when a write fails
// the connection is already gone.
type handlers struct {
	limiter *middleware.RateLimiter // nil disables limiting
	logger  *slog.Logger
}

// greeting sends the server greeting, both on connect and in reply to <hello>.
func (h *handlers) greeting(_ context.Context, rw epplib.Writer, _ *etree.Document) {
	_, _ = rw.Write([]byte(greetingXML()))
}

// login handles <login>.
//
// It does not authenticate: any request carrying a clID and a pw succeeds.
// Real authentication is Phase 2 of docs/epp/ARCHITECTURE.md.
func (h *handlers) login(ctx context.Context, rw epplib.Writer, doc *etree.Document) {
	clID := doc.FindElement("//clID")
	pw := doc.FindElement("//pw")

	var username string
	if clID != nil {
		username = clID.Text()
	}
	clientIP, _ := appcontext.ClientIP(ctx)
	h.logger.Info("EPP login attempt", "clid", username, "client_ip", clientIP)

	if h.limiter != nil && username != "" {
		locked, err := h.limiter.IsAccountLocked(ctx, username)
		if err != nil {
			h.logger.Error("EPP account lock status unavailable", "clid", username, "error", err)
		}
		if locked {
			h.logger.Warn("EPP login refused: account locked", "clid", username, "client_ip", clientIP)
			_, _ = rw.Write([]byte(authErrorXML("account locked")))
			return
		}
	}

	// TODO: Implement actual authentication logic here
	authSuccess := username != "" && pw != nil

	if !authSuccess {
		if h.limiter != nil && username != "" && clientIP != "" {
			if err := h.limiter.RecordFailedLogin(ctx, username, clientIP); err != nil {
				h.logger.Warn("EPP failed login recorded", "clid", username, "client_ip", clientIP, "error", err)
			}
		}
		_, _ = rw.Write([]byte(authErrorXML("invalid credentials")))
		return
	}

	if h.limiter != nil && username != "" && clientIP != "" {
		if err := h.limiter.ClearFailedLogins(ctx, username, clientIP); err != nil {
			h.logger.Error("EPP failed logins not cleared", "clid", username, "client_ip", clientIP, "error", err)
		}

		// TODO: carry the registrar ID for connection tracking. Enriching ctx
		// here is pointless while this handler neither propagates nor returns
		// it — the value was silently dropped. Wire it through the connection
		// once authentication above is real.
	}

	_, _ = rw.Write([]byte(loginResponseXML()))
}

// logout handles <logout> and closes the connection after replying.
func (h *handlers) logout(ctx context.Context, rw epplib.Writer, _ *etree.Document) {
	connectionID, _ := appcontext.ConnectionID(ctx)
	h.logger.Info("EPP logout", "connection_id", connectionID)
	_, _ = rw.Write([]byte(logoutResponseXML()))
	rw.CloseAfterWrite()
}

// domainCheck answers <domain:check> with a fixed placeholder response. It
// does not look at the names asked about; real handlers are Phase 4 of
// docs/epp/ARCHITECTURE.md.
func (h *handlers) domainCheck(_ context.Context, rw epplib.Writer, _ *etree.Document) {
	_, _ = rw.Write([]byte(placeholderDomainCheckXML()))
}
