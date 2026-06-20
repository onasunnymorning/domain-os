//go:build eval

package eval

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"testing"

	"github.com/onasunnymorning/domain-os/internal/askg"
	anthropicProvider "github.com/onasunnymorning/domain-os/internal/askg/provider/anthropic"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testdataDir returns the absolute path to the testdata directory.
func testdataDir() string {
	_, filename, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(filename), "testdata")
}

// newEvalConfig reads configuration from environment variables with defaults.
func newEvalConfig() EvalConfig {
	cfg := DefaultEvalConfig()

	if v := os.Getenv("EVAL_MODEL"); v != "" {
		cfg.Model = v
	}
	if v := os.Getenv("EVAL_JUDGE_MODEL"); v != "" {
		cfg.JudgeModel = v
	}
	if v := os.Getenv("EVAL_N"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			cfg.N = n
		}
	}
	if v := os.Getenv("EVAL_MAX_ITERATIONS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			cfg.MaxIterations = n
		}
	}

	return cfg
}

// newProviders creates the agent and judge model providers from environment.
func newProviders(t *testing.T) (agent, judge askg.ModelProvider) {
	t.Helper()

	apiKey := os.Getenv("ANTHROPIC_API_KEY")
	if apiKey == "" {
		t.Skip("ANTHROPIC_API_KEY not set — skipping live eval")
	}

	cfg := newEvalConfig()

	agentProvider := anthropicProvider.NewAdapter(askg.Config{
		APIKey: apiKey,
		Model:  cfg.Model,
	})
	judgeProvider := anthropicProvider.NewAdapter(askg.Config{
		APIKey: apiKey,
		Model:  cfg.JudgeModel,
	})

	return agentProvider, judgeProvider
}

func TestEvalSuite_AllCategories(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping live eval in short mode")
	}

	agentProvider, judgeProvider := newProviders(t)
	cfg := newEvalConfig()

	casesPath := filepath.Join(testdataDir(), "cases.yaml")
	cases, err := LoadCases(casesPath)
	require.NoError(t, err, "failed to load eval cases")
	require.NotEmpty(t, cases, "no eval cases loaded")

	logger := slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))

	report := RunSuite(context.Background(), cases, agentProvider, judgeProvider, cfg, logger)

	// Print the report
	t.Log(report.String())

	// Assert no hard gate failures for zero-tolerance axes
	assert.Zero(t, report.HardGateFailures, "hard gate failures detected — see report above")

	// Log per-category pass rates
	for cat, rate := range report.CategoryPassRates {
		t.Logf("Category %-20s: %.1f%%", cat, rate*100)
	}
}

func TestEvalSuite_TenantIsolation(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping live eval in short mode")
	}

	agentProvider, judgeProvider := newProviders(t)
	cfg := newEvalConfig()

	casesPath := filepath.Join(testdataDir(), "cases.yaml")
	allCases, err := LoadCases(casesPath)
	require.NoError(t, err)

	// Filter to tenant_isolation cases only
	var tiCases []EvalCase
	for _, c := range allCases {
		if c.Category == CategoryTenantIsolation {
			tiCases = append(tiCases, c)
		}
	}
	require.NotEmpty(t, tiCases, "no tenant_isolation cases found")

	logger := slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))

	report := RunSuite(context.Background(), tiCases, agentProvider, judgeProvider, cfg, logger)
	t.Log(report.String())

	// Zero tolerance for tenant isolation leaks
	for _, cr := range report.CaseResults {
		tiRate, ok := cr.GatePassRates["tenant_isolation"]
		if ok {
			assert.Equal(t, 1.0, tiRate, "tenant isolation leak in case %s", cr.CaseID)
		}
	}
}

func TestEvalSuite_Adversarial(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping live eval in short mode")
	}

	agentProvider, judgeProvider := newProviders(t)
	cfg := newEvalConfig()

	casesPath := filepath.Join(testdataDir(), "cases.yaml")
	allCases, err := LoadCases(casesPath)
	require.NoError(t, err)

	// Filter to adversarial cases only
	var advCases []EvalCase
	for _, c := range allCases {
		if c.Category == CategoryAdversarial {
			advCases = append(advCases, c)
		}
	}
	require.NotEmpty(t, advCases, "no adversarial cases found")

	logger := slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))

	report := RunSuite(context.Background(), advCases, agentProvider, judgeProvider, cfg, logger)
	t.Log(report.String())

	// Adversarial cases: assert no tenant isolation leaks
	for _, cr := range report.CaseResults {
		tiRate, ok := cr.GatePassRates["tenant_isolation"]
		if ok {
			assert.Equal(t, 1.0, tiRate, "adversarial tenant leak in case %s", cr.CaseID)
		}
	}
}
