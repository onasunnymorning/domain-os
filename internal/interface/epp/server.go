// Package epp is the EPP (RFC 5730) inbound adapter: it terminates registrar
// sessions over TLS (RFC 5734) and translates EPP frames into application
// calls.
//
// It is a protocol stub today: login is not authenticated and only greeting,
// login, logout and a placeholder domain:check are bound. The target
// architecture, and the phased plan for reaching it, is
// docs/epp/ARCHITECTURE.md.
package epp

import (
	"crypto/tls"
	"log/slog"
	"net"
	"time"

	epplib "gitlab.com/internetstiftelsen-oss/epp-lib"

	"github.com/onasunnymorning/domain-os/internal/infrastructure/epp/middleware"
)

// Transport settings. They are fixed in code until Phase 1 of
// docs/epp/ARCHITECTURE.md makes them configurable; until then
// docs/epp/POLICY_REFERENCE.md lists them as fixed.
const (
	sessionTimeout = time.Hour
	idleTimeout    = 350 * time.Second
	writeTimeout   = 2 * time.Minute
	readTimeout    = 10 * time.Second
	maxMessageSize = 1000 // bytes
)

// Server is an EPP server. Build it with NewServer and start it with Serve.
type Server struct {
	srv    *epplib.Server
	logger *slog.Logger
}

// NewServer wires the command handlers into an EPP server presenting cert.
// limiter may be nil, which disables connection and login limiting.
func NewServer(cert tls.Certificate, limiter *middleware.RateLimiter, logger *slog.Logger) *Server {
	h := &handlers{limiter: limiter, logger: logger}

	mux := &epplib.CommandMux{}
	mux.BindGreeting(h.greeting)
	mux.Bind(epplib.NewXMLPathBuilder().AddOrphan("//hello", epplib.NamespaceIETFEPP10.String()).String(), h.greeting)
	// Login and logout are direct children of <command>, not in an object namespace.
	mux.Bind(epplib.NewXMLPathBuilder().
		AddOrphan("//command", epplib.NamespaceIETFEPP10.String()).
		Add("login", epplib.NamespaceIETFEPP10.String()).String(), h.login)
	mux.Bind(epplib.NewXMLPathBuilder().
		AddOrphan("//command", epplib.NamespaceIETFEPP10.String()).
		Add("logout", epplib.NamespaceIETFEPP10.String()).String(), h.logout)
	mux.BindCommand("check", epplib.NamespaceIETFDomain10.String(), h.domainCheck)
	// TODO: contact info is not bound — there is no handler for it yet.

	return &Server{
		logger: logger,
		srv: &epplib.Server{
			HandleCommand: mux.Handle,
			Greeting:      mux.GetGreeting,
			TLSConfig: tls.Config{
				Certificates: []tls.Certificate{cert},
				ClientAuth:   tls.RequireAnyClientCert,
				MinVersion:   tls.VersionTLS12,
			},
			ConnContext:    h.connContext,
			Timeout:        sessionTimeout,
			IdleTimeout:    idleTimeout,
			WriteTimeout:   writeTimeout,
			ReadTimeout:    readTimeout,
			Logger:         logger,
			MaxMessageSize: maxMessageSize,
		},
	}
}

// Serve accepts EPP connections on l until l is closed.
func (s *Server) Serve(l net.Listener) error {
	s.logger.Info("EPP server listening", "addr", l.Addr().String())
	return s.srv.Serve(l)
}
