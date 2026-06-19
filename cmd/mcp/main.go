package main

import (
	"context"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/onasunnymorning/domain-os/internal/application/services"
	"github.com/onasunnymorning/domain-os/internal/infrastructure/db/postgres"
	mcpserver "github.com/onasunnymorning/domain-os/internal/interface/mcp"
)

func main() {
	os.Exit(run())
}

func run() int {
	// Redirect ALL log output to stderr so stdout stays clean for the MCP
	// stdio JSON-RPC transport. This covers the standard "log" package used
	// by infrastructure code (e.g. GORM connection logging).
	log.SetOutput(os.Stderr)

	// Structured logging also goes to stderr.
	logger := slog.New(slog.NewJSONHandler(os.Stderr, nil))
	slog.SetDefault(logger)

	// Set up context with signal handling for graceful shutdown.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Set up the database connection using the same env vars as other servers.
	gormDB, err := postgres.NewConnection(
		postgres.Config{
			User:    os.Getenv("DB_USER"),
			Pass:    os.Getenv("DB_PASS"),
			Host:    os.Getenv("DB_HOST"),
			Port:    os.Getenv("DB_PORT"),
			DBName:  os.Getenv("DB_NAME"),
			SSLmode: os.Getenv("DB_SSLMODE"),
		},
	)
	if err != nil {
		slog.Error("Failed to connect to the database", "error", err)
		return 1
	}

	// Set up repositories and services.
	// Each service is wired with only the repositories it needs for read-only
	// tools. Unused dependencies are passed as nil with comments explaining why.
	domainRepo := postgres.NewDomainRepository(gormDB)
	domainService := services.NewDomainService(
		domainRepo,
		nil,                    // hostRepo — not used by GetDomainByName (hosts are preloaded via domainRepo)
		services.RoidService{}, // roidService — not used by read-only tools
		nil,                    // nndnRepo
		nil,                    // tldRepo
		nil,                    // phaseRepo
		nil,                    // premiumLabelRepo
		nil,                    // fxRepo
		nil,                    // registrarRepo
		nil,                    // eventPublisher
	)

	tldRepo := postgres.NewGormTLDRepo(gormDB)
	tldService := services.NewTLDService(
		tldRepo,
		nil, // dnsRecRepo — not used by GetTLDByName
	)

	// Create and run the MCP server over stdio.
	server := mcpserver.NewServer(domainService, tldService)
	slog.Info("Starting domain-os MCP server (stdio)")
	if err := server.Run(ctx); err != nil {
		slog.Error("MCP server exited with error", "error", err)
		return 1
	}

	return 0
}
