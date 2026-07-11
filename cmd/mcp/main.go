package main

import (
	"context"
	"fmt"
	"log"

	"github.com/onasunnymorning/domain-os/internal/buildinfo"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/modelcontextprotocol/go-sdk/mcp"
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

	logger.Info("MCP server build info", "version", buildinfo.Version, "git_sha", buildinfo.GitSHA)

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

	// Create the MCP server adapter.
	server := mcpserver.NewServer(domainService, tldService)

	// Select transport based on MCP_TRANSPORT env var.
	// - "stdio" (default): JSON-RPC over stdin/stdout, for local IDE use
	// - "http": Streamable HTTP on MCP_PORT (default 3001), for container deployment
	transport := strings.ToLower(os.Getenv("MCP_TRANSPORT"))
	if transport == "" {
		transport = "stdio"
	}

	switch transport {
	case "stdio":
		slog.Info("Starting domain-os MCP server", "transport", "stdio", "version", mcpserver.Version)
		if err := server.Run(ctx); err != nil {
			slog.Error("MCP server exited with error", "error", err)
			return 1
		}

	case "http":
		port := os.Getenv("MCP_PORT")
		if port == "" {
			port = "3001"
		}
		addr := fmt.Sprintf(":%s", port)

		// Get the configured MCP protocol server with all tools registered.
		mcpSrv := server.MCPServer()

		// Create the Streamable HTTP handler. Stateless mode is ideal for a
		// read-only tool server — each request creates a temporary session,
		// no session management overhead.
		handler := mcp.NewStreamableHTTPHandler(
			func(r *http.Request) *mcp.Server { return mcpSrv },
			&mcp.StreamableHTTPOptions{
				Stateless: true,
				Logger:    logger,
			},
		)

		mux := http.NewServeMux()
		mux.Handle("/mcp", handler)
		mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			fmt.Fprintf(w, `{"status":"ok","version":"%s"}`, mcpserver.Version)
		})

		httpServer := &http.Server{
			Addr:    addr,
			Handler: mux,
		}

		// Graceful shutdown: when context is cancelled, shut down the HTTP server.
		go func() {
			<-ctx.Done()
			slog.Info("Shutting down MCP HTTP server")
			if err := httpServer.Shutdown(context.Background()); err != nil {
				slog.Error("HTTP server shutdown error", "error", err)
			}
		}()

		slog.Info("Starting domain-os MCP server",
			"transport", "http",
			"addr", addr,
			"version", mcpserver.Version,
		)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("MCP HTTP server exited with error", "error", err)
			return 1
		}

	default:
		slog.Error("Unknown MCP_TRANSPORT value — must be 'stdio' or 'http'",
			"transport", transport,
		)
		return 1
	}

	return 0
}
