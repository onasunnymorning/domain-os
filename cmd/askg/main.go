// Ask G CLI — standalone entrypoint for the Ask G support assistant.
//
// Usage:
//
//	ANTHROPIC_API_KEY=sk-... go run ./cmd/askg "What is the status of example.best?"
//
// The CLI reads the question from the first argument (or stdin), queries the
// registry via in-process application services, and prints the structured
// result as JSON to stdout.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/onasunnymorning/domain-os/internal/application/interfaces"
	"github.com/onasunnymorning/domain-os/internal/application/services"
	"github.com/onasunnymorning/domain-os/internal/askg"
	anthropicprovider "github.com/onasunnymorning/domain-os/internal/askg/provider/anthropic"
	"github.com/onasunnymorning/domain-os/internal/infrastructure/db/postgres"
)

func main() {
	os.Exit(run())
}

func run() int {
	// Redirect ALL log output to stderr so stdout stays clean for JSON output.
	// This covers the standard "log" package used by infrastructure code (e.g.
	// GORM connection logging).
	log.SetOutput(os.Stderr)

	logger := slog.New(slog.NewJSONHandler(os.Stderr, nil))
	slog.SetDefault(logger)

	// Parse question from args
	question := strings.Join(os.Args[1:], " ")
	if question == "" {
		fmt.Fprintln(os.Stderr, "Usage: askg <question>")
		fmt.Fprintln(os.Stderr, "Example: askg 'What is the status of example.best?'")
		return 1
	}

	// Load config from environment
	cfg := askg.Config{
		Provider:        "anthropic",
		Model:           envOrDefault("LLM_MODEL", anthropicprovider.DefaultModel),
		ClassifierModel: envOrDefault("ASKG_CLASSIFIER_MODEL", anthropicprovider.DefaultClassifier),
		MaxIterations:   askg.DefaultMaxIterations,
		APIKey:          os.Getenv("ANTHROPIC_API_KEY"),
		BaseURL:         os.Getenv("ANTHROPIC_BASE_URL"),
	}

	if cfg.APIKey == "" {
		fmt.Fprintln(os.Stderr, "Error: ANTHROPIC_API_KEY environment variable is required")
		return 1
	}

	// Set up context with signal handling
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Set up database connection (same env vars as other servers)
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
		slog.Error("Failed to connect to database", "error", err)
		return 1
	}

	// Wire services — same pattern as cmd/mcp/main.go
	domainRepo := postgres.NewDomainRepository(gormDB)
	domainService := services.NewDomainService(
		domainRepo,
		nil,                    // hostRepo — not used by GetDomainByName
		services.RoidService{}, // roidService — not used by read-only tools
		nil, nil, nil, nil, nil, nil, nil,
	)

	tldRepo := postgres.NewGormTLDRepo(gormDB)
	tldService := services.NewTLDService(tldRepo, nil)

	// Create the tool executor with optional KnowledgeService
	var knowledgeSvc interfaces.KnowledgeService
	projectRoot := os.Getenv("KNOWLEDGE_BASE_DIR")
	if projectRoot == "" {
		projectRoot, _ = os.Getwd()
	}
	ks, ksErr := services.NewKnowledgeService(projectRoot)
	if ksErr != nil {
		slog.Warn("KnowledgeService not available — answer_system_question tool disabled",
			"error", ksErr,
			"project_root", projectRoot)
	} else {
		knowledgeSvc = ks
		slog.Info("KnowledgeService loaded",
			"docs", ks.DocCount(),
			"chunks", ks.ChunkCount())
	}

	executor := askg.NewInProcessToolExecutor(domainService, tldService, knowledgeSvc, logger)
	provider := anthropicprovider.NewAdapter(cfg)

	// Create the orchestrator
	orch := askg.NewOrchestrator(provider, executor, cfg, logger)

	// Run the query
	scope := askg.CallerScope{
		UserID: envOrDefault("ASKG_USER_ID", "cli-user"),
	}

	slog.Info("ask_g: processing question",
		slog.String("question", question),
		slog.String("model", cfg.Model),
		slog.String("provider", cfg.Provider),
	)

	result, err := orch.Ask(ctx, question, scope)
	if err != nil {
		slog.Error("Ask G failed", "error", err)
		return 1
	}

	// Output the result as formatted JSON to stdout
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(result); err != nil {
		slog.Error("Failed to encode result", "error", err)
		return 1
	}

	return 0
}

func envOrDefault(key, defaultValue string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultValue
}
