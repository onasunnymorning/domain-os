package eval

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/onasunnymorning/domain-os/internal/askg"
)

// RunSuite runs all eval cases against a real model provider. Each case is
// executed N times (from config) to account for stochastic model behaviour.
// The tool layer is fixture-backed — deterministic by construction.
func RunSuite(ctx context.Context, cases []EvalCase, provider askg.ModelProvider, judgeProvider askg.ModelProvider, cfg EvalConfig, logger *slog.Logger) *SuiteReport {
	results := make([]CaseResult, 0, len(cases))

	for i, ec := range cases {
		logger.InfoContext(ctx, "eval: running case",
			slog.Int("index", i+1),
			slog.Int("total", len(cases)),
			slog.String("case_id", ec.ID),
			slog.String("category", string(ec.Category)),
		)

		cr := RunCase(ctx, ec, provider, judgeProvider, cfg, logger)
		results = append(results, cr)

		logger.InfoContext(ctx, "eval: case completed",
			slog.String("case_id", ec.ID),
			slog.Float64("pass_rate", cr.PassRate),
			slog.Int("runs", len(cr.Runs)),
		)
	}

	return BuildSuiteReport(results)
}

// RunCase runs a single eval case N times and aggregates results.
func RunCase(ctx context.Context, ec EvalCase, provider askg.ModelProvider, judgeProvider askg.ModelProvider, cfg EvalConfig, logger *slog.Logger) CaseResult {
	n := cfg.N
	if n <= 0 {
		n = 3
	}

	runs := make([]RunResult, 0, n)
	gateAccum := make(map[string]int) // gate name → pass count
	gateTotal := make(map[string]int)

	for i := 0; i < n; i++ {
		logger.InfoContext(ctx, "eval: run",
			slog.String("case_id", ec.ID),
			slog.Int("run", i+1),
			slog.Int("of", n),
		)

		rr := runOnce(ctx, ec, provider, judgeProvider, cfg, logger)
		runs = append(runs, rr)

		for _, g := range rr.Gates {
			gateTotal[g.Name]++
			if g.Pass {
				gateAccum[g.Name]++
			}
		}
	}

	// Compute pass rate
	passCount := 0
	for _, rr := range runs {
		if rr.Pass {
			passCount++
		}
	}

	gatePassRates := make(map[string]float64, len(gateAccum))
	for name, total := range gateTotal {
		if total > 0 {
			gatePassRates[name] = float64(gateAccum[name]) / float64(total)
		}
	}

	return CaseResult{
		CaseID:             ec.ID,
		Category:           ec.Category,
		Runs:               runs,
		PassRate:            float64(passCount) / float64(n),
		GatePassRates:      gatePassRates,
		expectedOutcomeStr: string(ec.Expect.Outcome),
	}
}

// runOnce executes a single eval run: builds the orchestrator with a
// fixture-backed tool executor, runs the agent, and scores the result.
func runOnce(ctx context.Context, ec EvalCase, provider askg.ModelProvider, judgeProvider askg.ModelProvider, cfg EvalConfig, logger *slog.Logger) RunResult {
	// Build fixture-backed executor
	executor := NewFixtureToolExecutor(ec.Fixtures)

	// Build orchestrator with real provider + fixture executor
	orchCfg := askg.Config{
		Model:         cfg.Model,
		MaxIterations: cfg.MaxIterations,
	}
	if orchCfg.MaxIterations <= 0 {
		orchCfg.MaxIterations = 10
	}

	orch := askg.NewOrchestrator(provider, executor, orchCfg, logger)

	// Run the agent
	result, err := orch.Ask(ctx, ec.Question, ec.CallerScope)
	if err != nil {
		// Orchestrator should never return errors (it escalates instead),
		// but handle it defensively.
		logger.ErrorContext(ctx, "eval: orchestrator returned error",
			slog.String("case_id", ec.ID),
			slog.String("error", err.Error()),
		)
		return RunResult{
			Result: &askg.Result{
				Outcome: askg.OutcomeEscalate,
				Reason:  fmt.Sprintf("orchestrator error: %s", err.Error()),
			},
			Gates: []GateResult{{
				Name:   "orchestrator_error",
				Pass:   false,
				Detail: err.Error(),
			}},
			Pass: false,
		}
	}

	// Score deterministic gates
	gates := ScoreAllGates(result, ec, executor.ExecLog())

	// Run LLM judge for fuzzy axes (only for answer outcomes)
	var judgments []JudgeVerdict
	if judgeProvider != nil {
		var judgeErr error
		judgments, judgeErr = Judge(ctx, judgeProvider, cfg.JudgeModel, ec, result)
		if judgeErr != nil {
			logger.WarnContext(ctx, "eval: judge failed",
				slog.String("case_id", ec.ID),
				slog.String("error", judgeErr.Error()),
			)
			// Judge failure is not a hard gate failure — log and continue
		}
	}

	return RunResult{
		Result:    result,
		Gates:     gates,
		Judgments: judgments,
		ToolTrace: executor.ExecLog(),
		Pass:      AllGatesPass(gates),
	}
}
