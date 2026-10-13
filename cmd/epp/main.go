// Command epp runs the registrar-facing EPP server. It is the composition
// root only: configuration is read here (INV-17) and everything else lives in
// internal/interface/epp. See docs/epp/ARCHITECTURE.md.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"os"

	"github.com/redis/go-redis/v9"

	"github.com/onasunnymorning/domain-os/internal/buildinfo"
	"github.com/onasunnymorning/domain-os/internal/config"
	"github.com/onasunnymorning/domain-os/internal/infrastructure/epp/middleware"
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
	redisClient := redis.NewClient(&redis.Options{
		Addr:     cfg.Redis.Addr(),
		Password: cfg.Redis.Password,
		DB:       0,
	})
	if err := redisClient.Ping(context.Background()).Err(); err != nil {
		return fmt.Errorf("connect to Redis at %s: %w", cfg.Redis.Addr(), err)
	}
	logger.Info("Connected to Redis", "addr", cfg.Redis.Addr())

	limiter := middleware.NewRateLimiter(redisClient, middleware.DefaultRateLimitConfig(), logger)

	cert, err := epp.SelfSignedCertificate()
	if err != nil {
		return fmt.Errorf("server certificate: %w", err)
	}

	listener, err := net.ListenTCP("tcp", &net.TCPAddr{Port: cfg.Port})
	if err != nil {
		return fmt.Errorf("listen on port %d: %w", cfg.Port, err)
	}

	return epp.NewServer(cert, limiter, logger).Serve(listener)
}
