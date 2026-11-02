// Command rdap runs the registry's public RDAP server. It is the composition
// root only: configuration is read here (INV-17) and everything else lives in
// internal/interface/rdap. See docs/rdap/ARCHITECTURE.md.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/onasunnymorning/domain-os/internal/application/services"
	"github.com/onasunnymorning/domain-os/internal/buildinfo"
	"github.com/onasunnymorning/domain-os/internal/config"
	"github.com/onasunnymorning/domain-os/internal/infrastructure/db/postgres"
	"github.com/onasunnymorning/domain-os/internal/infrastructure/ratelimit"
	"github.com/onasunnymorning/domain-os/internal/interface/rdap"
	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
)

func main() {
	cfg, err := config.LoadRDAP()
	if err != nil {
		slog.Error("RDAP server configuration is invalid", "error", err)
		os.Exit(1)
	}
	// Handles are ROIDs with the repository suffix (ADR 0017), so the
	// server does not start without the registry's repository ID.
	repositoryID, err := config.LoadRepositoryID()
	if err != nil {
		slog.Error("RDAP server configuration is invalid", "error", err)
		os.Exit(1)
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: cfg.LogLevel}))
	logger.Info("RDAP server starting", "version", buildinfo.Version, "git_sha", buildinfo.GitSHA, "base_url", cfg.BaseURL, "repository_id", repositoryID.String())

	if err := run(cfg, repositoryID, logger); err != nil {
		logger.Error("RDAP server stopped", "error", err)
		os.Exit(1)
	}
}

func run(cfg config.RDAP, repositoryID entities.RepositoryID, logger *slog.Logger) error {
	if cfg.TLS.SelfSigned {
		logger.Warn("RDAP server is presenting a self-signed certificate (RDAP_TLS_SELF_SIGNED=true); local development only")
	}

	// Read-only: an existing database, never created or migrated here.
	// A bounded pool, so a flood of lookups cannot crowd out the API, EPP
	// and the workers.
	db, err := postgres.OpenExisting(postgres.Config{
		User: cfg.DB.User, Pass: cfg.DB.Pass, Host: cfg.DB.Host, Port: cfg.DB.Port,
		DBName: cfg.DB.Name, SSLmode: cfg.DB.SSLMode, MaxOpenConns: cfg.DB.MaxOpenConns,
	})
	if err != nil {
		return fmt.Errorf("connect to the database at %s:%s: %w", cfg.DB.Host, cfg.DB.Port, err)
	}
	schemaLog := func(format string, args ...any) { logger.Info(fmt.Sprintf(format, args...)) }
	if err := postgres.EnforceSchemaGuard(db, buildinfo.Version, postgres.SchemaGuardModeFromEnv(), schemaLog); err != nil {
		return err
	}
	// Read-only (ADR 0020): verify the recorded repository ID, never
	// record it, so the server runs on SELECT-only credentials.
	if err := postgres.VerifyRepositoryID(context.Background(), db, repositoryID); err != nil {
		return err
	}
	logger.Info("Connected to the database", "host", cfg.DB.Host, "name", cfg.DB.Name)

	redisClient := redis.NewClient(&redis.Options{
		Addr:     cfg.Redis.Addr(),
		Password: cfg.Redis.Password,
		// The limiter bounds each call and falls back to the node when
		// Redis does not answer in time; without this the client ignores
		// that deadline on an open connection.
		ContextTimeoutEnabled: true,
	})
	// The limiter fails open without Redis (ADR 0022), so an unreachable
	// Redis is a warning; rdap_ratelimit_degraded shows it.
	if err := redisClient.Ping(context.Background()).Err(); err != nil {
		logger.Warn("Redis unreachable at start; clients are limited per node until it answers", "addr", cfg.Redis.Addr(), "error", err)
	} else {
		logger.Info("Connected to Redis", "addr", cfg.Redis.Addr())
	}

	tlds := postgres.NewGormTLDRepo(db)
	lookups := services.NewRDAPService(services.RDAPServiceDeps{
		Domains:    postgres.NewDomainRepository(db, repositoryID),
		Hosts:      postgres.NewGormHostRepository(db, repositoryID),
		Registrars: postgres.NewGormRegistrarRepository(db),
		TLDs:       tlds,
		Contacts:   postgres.NewContactRepository(db, repositoryID),
		// The TLDs' publication policies (ADR 0021), reloaded every
		// RDAP_POLICY_REFRESH_INTERVAL.
		Publication: services.NewRDAPPublicationPolicyService(postgres.NewRDAPPublicationPolicyRepository(db), tlds),
		// The networks' access policies (ADR 0022), reloaded with them.
		Access: services.NewRDAPAccessPolicyService(postgres.NewRDAPAccessPolicyRepository(db)),
	})
	// srv reports the limiter's health once it exists.
	var srv *rdap.Server
	limiter := ratelimit.NewFailOpen(ratelimit.NewRedisFor(redisClient, "rdap"), ratelimit.NewMemory(), func(degraded bool) {
		if srv != nil {
			srv.ReportRatesDegraded(degraded)
		}
	})
	srv, err = rdap.NewServer(rdap.Options{Config: cfg, Lookups: lookups, Logger: logger, Limiter: limiter})
	if err != nil {
		return err
	}

	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", cfg.Port))
	if err != nil {
		return fmt.Errorf("listen on port %d: %w", cfg.Port, err)
	}

	ops := &http.Server{
		Addr:              fmt.Sprintf(":%d", cfg.OpsPort),
		Handler:           srv.OpsHandler(),
		ReadHeaderTimeout: 5 * time.Second,
	}
	opsErr := make(chan error, 1)
	go func() {
		logger.Info("RDAP ops endpoint listening", "addr", ops.Addr)
		if err := ops.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
			opsErr <- fmt.Errorf("ops endpoint: %w", err)
		}
	}()

	served := make(chan error, 1)
	go func() { served <- srv.Serve(listener) }()

	stop, cancel := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer cancel()

	var runErr error
	select {
	case <-stop.Done():
		logger.Info("RDAP server received shutdown signal", "drain_timeout", cfg.Timeouts.Drain.String())
	case runErr = <-served:
		served = nil
	case runErr = <-opsErr:
	}

	drainCtx, cancelDrain := context.WithTimeout(context.Background(), cfg.Timeouts.Drain)
	defer cancelDrain()
	if err := srv.Shutdown(drainCtx); err != nil {
		logger.Warn("RDAP drain did not finish cleanly", "error", err)
	}
	if served != nil {
		if err := <-served; err != nil && runErr == nil {
			runErr = err
		}
	}

	opsCtx, cancelOps := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelOps()
	_ = ops.Shutdown(opsCtx)
	return runErr
}
