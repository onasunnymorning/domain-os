// Command epp runs the registrar-facing EPP server. It is the composition
// root only: configuration is read here (INV-17) and everything else lives in
// internal/interface/epp. See docs/epp/ARCHITECTURE.md.
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
	"strconv"
	"syscall"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/onasunnymorning/domain-os/internal/buildinfo"
	"github.com/onasunnymorning/domain-os/internal/config"
	"github.com/onasunnymorning/domain-os/internal/infrastructure/epp/middleware"
	"github.com/onasunnymorning/domain-os/internal/infrastructure/snowflakeidgenerator"
	"github.com/onasunnymorning/domain-os/internal/interface/epp"
)

func main() {
	cfg, err := config.LoadEPP()
	if err != nil {
		slog.Error("EPP server configuration is invalid", "error", err)
		os.Exit(1)
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: cfg.LogLevel}))
	logger.Info("EPP server starting", "version", buildinfo.Version, "git_sha", buildinfo.GitSHA)

	if err := run(cfg, logger); err != nil {
		logger.Error("EPP server stopped", "error", err)
		os.Exit(1)
	}
}

func run(cfg config.EPP, logger *slog.Logger) error {
	if cfg.TLS.SelfSigned {
		logger.Warn("EPP server is presenting a self-signed certificate (EPP_TLS_SELF_SIGNED=true); local development only")
	}

	redisClient := redis.NewClient(&redis.Options{
		Addr:     cfg.Redis.Addr(),
		Password: cfg.Redis.Password,
		DB:       0,
	})
	if err := redisClient.Ping(context.Background()).Err(); err != nil {
		return fmt.Errorf("connect to Redis at %s: %w", cfg.Redis.Addr(), err)
	}
	logger.Info("Connected to Redis", "addr", cfg.Redis.Addr())

	ids, err := snowflakeidgenerator.NewIDGenerator()
	if err != nil {
		return fmt.Errorf("id generator: %w", err)
	}

	srv, err := epp.NewServer(epp.Options{
		Config:  cfg,
		Limiter: middleware.NewRateLimiter(redisClient, middleware.DefaultRateLimitConfig(), logger),
		Logger:  logger,
		NewID:   func() string { return strconv.FormatInt(ids.GenerateID(), 10) },
	})
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
		logger.Info("EPP ops endpoint listening", "addr", ops.Addr)
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
		logger.Info("EPP server received shutdown signal", "drain_timeout", cfg.Timeouts.Drain.String())
	case runErr = <-served:
		served = nil
	case runErr = <-opsErr:
	}

	drainCtx, cancelDrain := context.WithTimeout(context.Background(), cfg.Timeouts.Drain)
	defer cancelDrain()
	if err := srv.Shutdown(drainCtx); err != nil {
		logger.Warn("EPP drain did not finish cleanly", "error", err)
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
