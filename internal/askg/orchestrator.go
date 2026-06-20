package askg

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"
)

// Orchestrator implements the Ask G agent loop. It classifies intent,
// plans retrieval, calls read-only tools via the ToolExecutor, and
// synthesizes a grounded outcome using the ModelProvider.
//
// The orchestrator is stateless per request — conversation/loop state
// is passed in and returned, not held server-side.
type Orchestrator struct {
	provider ModelProvider
	executor ToolExecutor
	config   Config
	logger   *slog.Logger
}

// NewOrchestrator creates a new Ask G orchestrator.
func NewOrchestrator(provider ModelProvider, executor ToolExecutor, cfg Config, logger *slog.Logger) *Orchestrator {
	return &Orchestrator{
		provider: provider,
		executor: executor,
		config:   cfg,
		logger:   logger,
	}
}

// Ask processes a support staff member's question and returns a structured
// Result. The CallerScope is threaded through every tool call for audit.
func (o *Orchestrator) Ask(ctx context.Context, question string, scope CallerScope) (*Result, error) {
	start := time.Now()

	o.logger.InfoContext(ctx, "ask_g: request received",
		slog.String("caller", scope.UserID),
		slog.String("question", question),
	)

	maxIter := o.config.EffectiveMaxIterations()
	tools := o.executor.Tools()

	// Build initial message history
	messages := []Message{
		{Role: RoleUser, Content: question},
	}

	var allEvidence []Evidence
	var totalUsage Usage
	var iterations int

	for iterations = 0; iterations < maxIter; iterations++ {
		// Build the model request
		req := ModelRequest{
			System:   SystemPrompt,
			Messages: messages,
			Tools:    tools,
			Model:    o.config.Model,
		}

		// Call the model
		resp, err := o.provider.Generate(ctx, req)
		if err != nil {
			o.logger.ErrorContext(ctx, "ask_g: model generation failed",
				slog.String("provider", o.provider.Name()),
				slog.Int("iteration", iterations),
				slog.String("error", err.Error()),
			)
			// On model error, escalate with what we have
			return o.escalateWithEvidence(allEvidence, iterations+1, totalUsage,
				fmt.Sprintf("Model error after %d iterations: %s", iterations+1, err.Error()),
			), nil
		}

		totalUsage.InputTokens += resp.Usage.InputTokens
		totalUsage.OutputTokens += resp.Usage.OutputTokens
		totalUsage.CacheCreationInputTokens += resp.Usage.CacheCreationInputTokens
		totalUsage.CacheReadInputTokens += resp.Usage.CacheReadInputTokens

		o.logger.InfoContext(ctx, "ask_g: model response",
			slog.Int("iteration", iterations),
			slog.Int("tool_calls", len(resp.ToolCalls)),
			slog.Int("stop", int(resp.Stop)),
			slog.Int("input_tokens", resp.Usage.InputTokens),
			slog.Int("output_tokens", resp.Usage.OutputTokens),
			slog.Int("cache_creation_input_tokens", resp.Usage.CacheCreationInputTokens),
			slog.Int("cache_read_input_tokens", resp.Usage.CacheReadInputTokens),
		)

		// If max tokens, escalate — check this before final-message
		// since a max-tokens response also has zero tool calls.
		if resp.Stop == StopMaxTokens {
			return o.escalateWithEvidence(allEvidence, iterations+1, totalUsage,
				"Model hit max tokens limit; response may be incomplete.",
			), nil
		}

		// If the model returned a final message (no tool calls), parse the result
		if resp.Stop == StopFinalMessage || len(resp.ToolCalls) == 0 {
			result := o.parseModelOutput(resp.Text, allEvidence, iterations+1, totalUsage)

			latency := time.Since(start)
			o.logger.InfoContext(ctx, "ask_g: request completed",
				slog.String("caller", scope.UserID),
				slog.String("outcome", string(result.Outcome)),
				slog.Int("iterations", result.Iterations),
				slog.Int("evidence_count", len(result.Evidence)),
				slog.Int("total_input_tokens", totalUsage.InputTokens),
				slog.Int("total_output_tokens", totalUsage.OutputTokens),
				slog.Int("total_cache_creation_input_tokens", totalUsage.CacheCreationInputTokens),
				slog.Int("total_cache_read_input_tokens", totalUsage.CacheReadInputTokens),
				slog.Duration("latency", latency),
			)

			return result, nil
		}


		// Execute tool calls
		assistantMsg := Message{
			Role:      RoleAssistant,
			Content:   resp.Text,
			ToolCalls: resp.ToolCalls,
		}
		messages = append(messages, assistantMsg)

		var toolResults []ToolResult
		for _, tc := range resp.ToolCalls {
			tr := o.executor.Execute(ctx, tc, scope)
			toolResults = append(toolResults, tr)

			// Record evidence for provenance
			allEvidence = append(allEvidence, Evidence{
				Tool:   tc.Name,
				Input:  tc.Input,
				Result: tr.Result,
			})
		}

		// Feed tool results back to the model
		toolMsg := Message{
			Role:        RoleTool,
			ToolResults: toolResults,
		}
		messages = append(messages, toolMsg)
	}

	// Iteration cap hit — escalate with what was gathered
	o.logger.WarnContext(ctx, "ask_g: iteration cap hit",
		slog.Int("max_iterations", maxIter),
		slog.Int("evidence_count", len(allEvidence)),
	)
	return o.escalateWithEvidence(allEvidence, iterations, totalUsage,
		fmt.Sprintf("Iteration cap (%d) reached. Partial data gathered, manual review required.", maxIter),
	), nil
}

// parseModelOutput attempts to parse the model's final text output as a
// structured JSON result. If parsing fails, it falls back to treating the
// raw text as an answer.
//
// When assistant prefilling is used (the orchestrator appends a partial
// assistant message containing '{'), the model's response text continues
// from that prefix and will NOT include the leading '{'. We prepend it
// back before parsing.
func (o *Orchestrator) parseModelOutput(text string, evidence []Evidence, iterations int, usage Usage) *Result {
	// Try to parse as-is first (covers the no-prefill / first-turn case).
	if result := o.tryParseJSON(text, evidence, iterations, usage); result != nil {
		return result
	}

	// Prepend '{' to handle the assistant-prefill case where the model
	// continues from '{' and returns e.g. `"outcome": "answer", ...}`.
	prefilled := "{" + text
	if result := o.tryParseJSON(prefilled, evidence, iterations, usage); result != nil {
		return result
	}

	// Model didn't return valid JSON — treat the raw text as an answer.
	o.logger.WarnContext(context.Background(), "ask_g: model output is not valid JSON, treating as raw answer",
		slog.String("text_prefix", truncate(text, 100)),
	)
	return &Result{
		Outcome:    OutcomeAnswer,
		Answer:     text,
		Evidence:   evidence,
		Iterations: iterations,
		TotalUsage: usage,
	}
}

// tryParseJSON attempts to unmarshal text as a JSON result.
// Returns nil if parsing fails (caller should try next strategy).
func (o *Orchestrator) tryParseJSON(text string, evidence []Evidence, iterations int, usage Usage) *Result {
	var parsed struct {
		Outcome string `json:"outcome"`
		Answer  string `json:"answer"`
		Reason  string `json:"reason"`
		Action  string `json:"action"`
	}

	if err := json.Unmarshal([]byte(text), &parsed); err != nil {
		return nil
	}

	result := &Result{
		Evidence:   evidence,
		Iterations: iterations,
		TotalUsage: usage,
	}

	switch Outcome(parsed.Outcome) {
	case OutcomeAnswer:
		result.Outcome = OutcomeAnswer
		result.Answer = parsed.Answer
	case OutcomeEscalate:
		result.Outcome = OutcomeEscalate
		result.Reason = parsed.Reason
	case OutcomeActionRequired:
		result.Outcome = OutcomeActionRequired
		result.Reason = parsed.Reason
		result.Action = parsed.Action
	default:
		// Unknown outcome — treat as answer with full text
		result.Outcome = OutcomeAnswer
		result.Answer = text
	}

	return result
}

// truncate returns s truncated to n characters with "..." appended if needed.
func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

// escalateWithEvidence constructs an escalation result with the evidence
// gathered so far. This is a first-class outcome, not a failure path.
func (o *Orchestrator) escalateWithEvidence(evidence []Evidence, iterations int, usage Usage, reason string) *Result {
	return &Result{
		Outcome:    OutcomeEscalate,
		Reason:     reason,
		Evidence:   evidence,
		Iterations: iterations,
		TotalUsage: usage,
	}
}
