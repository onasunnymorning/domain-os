package rdap

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"sync"
	"sync/atomic"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/onasunnymorning/domain-os/internal/application/interfaces"
	"github.com/onasunnymorning/domain-os/internal/config"
	"github.com/onasunnymorning/domain-os/internal/infrastructure/tlsconfig"
)

// Options are what the server is built from.
type Options struct {
	Config config.RDAP
	// Lookups reads the registry. Required.
	Lookups interfaces.RDAPLookups
	Logger  *slog.Logger
}

// Server is the RDAP server: HTTPS on the node (ADR 0020), with health,
// readiness and metrics on a separate ops handler.
type Server struct {
	cfg     config.RDAP
	lookups interfaces.RDAPLookups
	logger  *slog.Logger
	metrics *metrics
	certs   *tlsconfig.CertSource
	http    *http.Server

	// baseURL is the public base, ending in "/". basePath is its path,
	// decoded, which requests are routed by; escapedBasePath is the same
	// path as it appears in a URL, which links are built from.
	baseURL, basePath, escapedBasePath string
	termsOfService                     string

	startOnce sync.Once
	serving   atomic.Bool
	draining  atomic.Bool
}

// NewServer builds a server. It loads the certificate, so a missing or
// unusable one stops the process at start.
func NewServer(opts Options) (*Server, error) {
	if opts.Lookups == nil {
		return nil, errors.New("rdap: Lookups is required")
	}
	if opts.Logger == nil {
		opts.Logger = slog.Default()
	}
	base, err := url.Parse(opts.Config.BaseURL)
	if err != nil {
		return nil, fmt.Errorf("rdap base URL: %w", err)
	}
	s := &Server{
		cfg:             opts.Config,
		lookups:         opts.Lookups,
		logger:          opts.Logger,
		metrics:         newMetrics(),
		baseURL:         opts.Config.BaseURL,
		basePath:        base.Path,
		escapedBasePath: base.EscapedPath(),
		termsOfService:  opts.Config.Notices.TermsOfServiceURL,
	}
	if s.basePath == "" {
		s.basePath, s.escapedBasePath = "/", "/"
	}

	hooks := tlsconfig.Hooks{
		Loaded:       func(notAfter time.Time) { s.metrics.certNotAfter.Set(float64(notAfter.Unix())) },
		ReloadFailed: s.metrics.certReloadErrors.Inc,
	}
	if s.cfg.TLS.SelfSigned {
		var cert tls.Certificate
		name := s.cfg.TLS.ServerName
		if name == "" {
			name = base.Hostname()
		}
		if cert, err = tlsconfig.SelfSigned(name); err == nil {
			s.certs, err = tlsconfig.NewStaticCertSource(cert, hooks)
		}
	} else {
		s.certs, err = tlsconfig.NewFileCertSource(tlsconfig.FileOptions{
			CertPath: s.cfg.TLS.CertPath, KeyPath: s.cfg.TLS.KeyPath,
			ServerName: s.cfg.TLS.ServerName, ServerNameSetting: "RDAP_SERVER_NAME",
			Interval: s.cfg.TLS.ReloadInterval, Service: "RDAP", Logger: s.logger, Hooks: hooks,
		})
	}
	if err != nil {
		return nil, fmt.Errorf("rdap server certificate: %w", err)
	}

	s.http = &http.Server{
		Handler:           s.handler(),
		TLSConfig:         tlsconfig.Server(s.certs),
		ReadHeaderTimeout: s.cfg.Timeouts.Read,
		ReadTimeout:       s.cfg.Timeouts.Read,
		WriteTimeout:      s.cfg.Timeouts.Write,
		IdleTimeout:       s.cfg.Timeouts.Idle,
		MaxHeaderBytes:    16 << 10,
		// Failed handshakes and malformed requests from the internet are
		// routine; they are not errors of this server.
		ErrorLog: slog.NewLogLogger(s.logger.Handler(), slog.LevelDebug),
	}
	return s, nil
}

// Serve answers RDAP over HTTPS on l until Shutdown. With PROXY protocol
// enabled, l must be the listener the load balancer connects to.
func (s *Server) Serve(l net.Listener) error {
	started := false
	s.startOnce.Do(func() { started = true })
	if !started {
		return errors.New("rdap: Serve called more than once")
	}
	if s.cfg.Proxy.Enabled {
		pl, err := tlsconfig.ProxyListener(l, s.cfg.Proxy.TrustedCIDRs, s.cfg.Timeouts.Read)
		if err != nil {
			return fmt.Errorf("rdap PROXY protocol listener: %w", err)
		}
		l = pl
	}

	// The certificate watch lives as long as Serve: shutting the HTTP
	// server down returns ServeTLS, which stops it.
	watchCtx, stop := context.WithCancel(context.Background())
	defer stop()
	go s.certs.Watch(watchCtx)

	s.logger.Info("RDAP server listening",
		"addr", l.Addr().String(),
		"base_url", s.baseURL,
		"proxy_protocol", s.cfg.Proxy.Enabled,
		"cert_not_after", s.certs.NotAfter().UTC().Format(time.RFC3339))
	if s.termsOfService == "" {
		s.logger.Warn("RDAP_TERMS_OF_SERVICE_URL is unset; responses carry no Terms of Use notice, which the gTLD RDAP Profile requires")
	}
	s.serving.Store(true)
	// ServeTLS with GetCertificate set needs no files, and enables HTTP/2.
	err := s.http.ServeTLS(l, "", "")
	s.serving.Store(false)
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

// Shutdown drains the server: readiness turns 503 so the load balancer stops
// sending, new connections are refused, and requests in flight finish until
// ctx is done.
func (s *Server) Shutdown(ctx context.Context) error {
	s.draining.Store(true)
	s.metrics.draining.Set(1)
	return s.http.Shutdown(ctx)
}

// Ready reports whether the node should receive traffic.
func (s *Server) Ready() bool {
	return s.serving.Load() && !s.draining.Load()
}

// OpsHandler serves /healthz, /readyz and /metrics. It belongs on a port
// that is never exposed publicly.
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
