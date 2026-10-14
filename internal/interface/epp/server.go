// Package epp is the EPP (RFC 5730) inbound adapter: it terminates registrar
// sessions over TLS (RFC 5734) and translates EPP frames into application
// calls.
//
// Logins are authenticated against registrars' EPP credentials (ADR 0014).
// Every command is validated against Go models of the EPP schemas (ADR 0016)
// and recorded in the audit log. Greeting, login, logout and domain check,
// info and create are bound. The target architecture, and the phased plan for reaching it, is
// docs/epp/ARCHITECTURE.md; every setting is listed in
// docs/epp/POLICY_REFERENCE.md.
package epp

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	epplib "gitlab.com/internetstiftelsen-oss/epp-lib"

	"github.com/onasunnymorning/domain-os/internal/application/interfaces"
	"github.com/onasunnymorning/domain-os/internal/application/services"
	"github.com/onasunnymorning/domain-os/internal/config"
)

// forceCloseGrace is how long Shutdown waits for sessions to unwind after it
// has closed their connections at the end of the drain timeout.
const forceCloseGrace = 5 * time.Second

// rfc9325CipherSuites are the TLS 1.2 cipher suites RFC 9325 §4.2
// recommends, and the only ones offered (RST epp-01 #10). Go's defaults also
// include CBC-mode suites. TLS 1.3 suites are not configurable in Go and are
// all AEAD.
var rfc9325CipherSuites = []uint16{
	tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
	tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
	tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
	tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
}

// Options are what the server is built from.
type Options struct {
	Config config.EPP
	// Authenticator checks logins against registrars' credentials. Required.
	Authenticator interfaces.EPPAuthenticator
	// Sessions is the cluster-wide session registry, with its caps. Required.
	Sessions interfaces.EPPSessionTracker
	// Domains answers domain commands. Required.
	Domains interfaces.EPPDomainService
	// Audit records every command and response. Required.
	Audit  interfaces.EPPAuditLog
	Logger *slog.Logger
	// NewID returns a unique identifier, used for session IDs and svTRIDs.
	// It must be unique across every node, not just this one.
	NewID func() string
}

// Server is an EPP server. Build it with NewServer, run it with Serve, stop
// it with Shutdown. OpsHandler serves health, readiness and metrics.
type Server struct {
	cfg      config.EPP
	lib      *epplib.Server
	certs    *certSource
	sessions *sessionRegistry
	metrics  *metrics
	tracker  interfaces.EPPSessionTracker
	auth     interfaces.EPPAuthenticator
	domains  interfaces.EPPDomainService
	auditLog interfaces.EPPAuditLog
	policy   commandPolicy
	watch    credentialWatch
	logger   *slog.Logger
	newID    func() string
	router   map[routeKey]route

	serving   atomic.Bool
	draining  atomic.Bool
	served    chan struct{}
	stopWatch context.CancelFunc
	startOnce sync.Once
}

// NewServer builds a server from opts. It loads the server certificate, so
// a missing or unreadable certificate is reported here, before anything
// listens.
func NewServer(opts Options) (*Server, error) {
	if opts.NewID == nil {
		return nil, errors.New("epp: Options.NewID is required")
	}
	if opts.Authenticator == nil {
		return nil, errors.New("epp: Options.Authenticator is required")
	}
	if opts.Sessions == nil {
		return nil, errors.New("epp: Options.Sessions is required")
	}
	if opts.Domains == nil {
		return nil, errors.New("epp: Options.Domains is required")
	}
	if opts.Audit == nil {
		return nil, errors.New("epp: Options.Audit is required")
	}
	s := &Server{
		cfg:      opts.Config,
		sessions: newSessionRegistry(),
		metrics:  newMetrics(),
		tracker:  opts.Sessions,
		auth:     opts.Authenticator,
		domains:  opts.Domains,
		auditLog: opts.Audit,
		policy:   allowAll{},
		logger:   opts.Logger,
		newID:    opts.NewID,
		served:   make(chan struct{}),
	}
	s.router = s.routes()

	var err error
	if s.cfg.TLS.SelfSigned {
		var cert tls.Certificate
		if cert, err = SelfSignedCertificate(); err == nil {
			s.certs, err = newStaticCertSource(cert, s.metrics)
		}
	} else {
		s.certs, err = newFileCertSource(s.cfg.TLS.CertPath, s.cfg.TLS.KeyPath, s.cfg.TLS.ServerName, s.cfg.TLS.ReloadInterval, s.metrics, s.logger)
	}
	if err != nil {
		return nil, fmt.Errorf("epp server certificate: %w", err)
	}

	s.lib = &epplib.Server{
		HandleCommand: s.handleCommand,
		Greeting:      s.greeting,
		ConnContext:   s.connContext,
		CloseConnHook: s.closeConn,
		TLSConfig: tls.Config{
			GetCertificate: s.certs.getCertificate,
			// A certificate is required here and checked at <login> against
			// the registrar's pins (ADR 0014): registrars may use any CA or a
			// self-signed certificate, so there is no chain to verify.
			ClientAuth:   tls.RequireAnyClientCert,
			MinVersion:   tls.VersionTLS12,
			CipherSuites: rfc9325CipherSuites,
		},
		Timeout:        s.cfg.Timeouts.MaxSession,
		IdleTimeout:    s.cfg.Timeouts.Idle,
		WriteTimeout:   s.cfg.Timeouts.Write,
		ReadTimeout:    s.cfg.Timeouts.Read,
		MaxMessageSize: s.cfg.Limits.MaxFrameBytes,
		Logger:         s.logger,
	}
	return s, nil
}

// Serve accepts EPP sessions on l until Shutdown. With PROXY protocol
// enabled, l must be the listener the load balancer connects to.
func (s *Server) Serve(l net.Listener) error {
	started := false
	s.startOnce.Do(func() { started = true })
	if !started {
		return errors.New("epp: Serve called more than once")
	}
	defer close(s.served)

	if s.cfg.Proxy.Enabled {
		pl, err := proxyListener(l, s.cfg.Proxy.TrustedCIDRs, s.cfg.Timeouts.Handshake)
		if err != nil {
			return fmt.Errorf("epp PROXY protocol listener: %w", err)
		}
		l = pl
	}
	nl := newNodeListener(l, s.admitSource, s.cfg.Limits.MaxConnections, s.cfg.Limits.MaxHandshakes, s.cfg.Timeouts.Handshake, s.metrics)

	watchCtx, stop := context.WithCancel(context.Background())
	s.stopWatch = stop
	go s.certs.watch(watchCtx)
	go s.watchCredentials(watchCtx)
	go s.renewSessions(watchCtx)
	go s.followKillRequests(watchCtx)

	s.logger.Info("EPP server listening",
		"addr", l.Addr().String(),
		"proxy_protocol", s.cfg.Proxy.Enabled,
		"cert_not_after", s.certs.notAfter().UTC().Format(time.RFC3339))
	s.serving.Store(true)
	err := s.lib.Serve(nl)
	s.serving.Store(false)
	stop()
	return err
}

// Shutdown drains the server. It stops accepting connections and closes
// every session that is waiting for a command; a session in the middle of a
// command finishes it first. If sessions are still open when ctx is done,
// their connections are closed outright.
func (s *Server) Shutdown(ctx context.Context) error {
	s.draining.Store(true)
	s.metrics.draining.Set(1)
	if !s.serving.Load() {
		return nil
	}
	s.logger.Info("EPP server draining", "sessions", s.sessions.len())
	if err := s.lib.Close(); err != nil {
		return fmt.Errorf("epp: close listener: %w", err)
	}

	select {
	case <-s.served:
		s.logger.Info("EPP server drained")
		return nil
	case <-ctx.Done():
	}

	remaining := s.sessions.all()
	s.logger.Warn("EPP drain timeout reached; closing remaining sessions", "sessions", len(remaining))
	for _, sess := range remaining {
		_ = sess.conn.Close()
	}
	select {
	case <-s.served:
	case <-time.After(forceCloseGrace):
	}
	return ctx.Err()
}

// Ready reports whether the server is accepting sessions: it is serving, not
// draining, and has loaded the allowed networks it filters sources by.
func (s *Server) Ready() bool {
	return s.serving.Load() && !s.draining.Load() && (!s.cfg.Auth.RequireAllowlist || s.watch.loaded())
}

// ReportLockoutDegraded records whether the shared lockout store is
// unreachable, for epp_policy_store_degraded{store="lockout"}.
func (s *Server) ReportLockoutDegraded(degraded bool) {
	s.reportStoreDegraded(storeLockout, degraded)
}

// ReportAuditError records a failure of the audit log's batch writer: a
// batch it retries, or records it lost at shutdown.
func (s *Server) ReportAuditError(err error) {
	var lost *services.EPPAuditLostError
	if errors.As(err, &lost) {
		s.metrics.auditErrors.WithLabelValues(auditStageLost).Add(float64(lost.Records))
		s.logger.Error("EPP audit records lost at shutdown", "records", lost.Records, "error", err)
		return
	}
	s.metrics.auditErrors.WithLabelValues(auditStageBatch).Inc()
	s.logger.Error("EPP audit batch not written; retrying", "error", err)
}

func (s *Server) reportStoreDegraded(store string, degraded bool) {
	v := 0.0
	if degraded {
		v = 1
	}
	s.metrics.storeDegraded.WithLabelValues(store).Set(v)
}

// OpsHandler serves /healthz (the process is up), /readyz (accepting
// sessions; 503 while starting or draining) and /metrics. It belongs on the
// ops port, never on the EPP port.
func (s *Server) OpsHandler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("ok\n"))
	})
	mux.HandleFunc("/readyz", func(w http.ResponseWriter, _ *http.Request) {
		if !s.Ready() {
			http.Error(w, "not ready", http.StatusServiceUnavailable)
			return
		}
		_, _ = w.Write([]byte("ready\n"))
	})
	mux.Handle("/metrics", promhttp.HandlerFor(s.metrics.registry, promhttp.HandlerOpts{}))
	return mux
}
