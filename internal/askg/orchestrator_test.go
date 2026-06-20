package askg

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// Stub ModelProvider for tests
// ---------------------------------------------------------------------------

// stubProvider is a fixture-backed ModelProvider for testing.
type stubProvider struct {
	name      string
	responses []ModelResponse
	errors    []error
	calls     []ModelRequest // records all calls for assertions
	callIndex int
}

func (s *stubProvider) Generate(_ context.Context, req ModelRequest) (ModelResponse, error) {
	s.calls = append(s.calls, req)
	idx := s.callIndex
	s.callIndex++

	if idx < len(s.errors) && s.errors[idx] != nil {
		return ModelResponse{}, s.errors[idx]
	}
	if idx < len(s.responses) {
		return s.responses[idx], nil
	}
	return ModelResponse{}, fmt.Errorf("stub: no more responses (call %d)", idx)
}

func (s *stubProvider) Name() string { return s.name }

// ---------------------------------------------------------------------------
// Stub ToolExecutor for tests
// ---------------------------------------------------------------------------

// stubToolExecutor is a fixture-backed ToolExecutor for testing.
type stubToolExecutor struct {
	tools    []ToolDef
	results  map[string]ToolResult // keyed by tool name
	execLog  []ToolCall            // records all executed calls
	scopeLog []CallerScope         // records caller scopes
}

func newStubToolExecutor() *stubToolExecutor {
	return &stubToolExecutor{
		tools: []ToolDef{
			{Name: "get_domain", Description: "Look up a domain", InputSchema: map[string]any{"type": "object"}},
			{Name: "get_tld", Description: "Look up a TLD", InputSchema: map[string]any{"type": "object"}},
		},
		results: make(map[string]ToolResult),
	}
}

func (s *stubToolExecutor) Tools() []ToolDef { return s.tools }

func (s *stubToolExecutor) Execute(_ context.Context, call ToolCall, scope CallerScope) ToolResult {
	s.execLog = append(s.execLog, call)
	s.scopeLog = append(s.scopeLog, scope)

	if r, ok := s.results[call.Name]; ok {
		r.CallID = call.ID
		return r
	}
	return ToolResult{CallID: call.ID, Result: map[string]string{"status": "ok"}, IsError: false}
}

// ---------------------------------------------------------------------------
// Helper to create a test orchestrator
// ---------------------------------------------------------------------------

func newTestOrchestrator(provider *stubProvider, executor *stubToolExecutor) *Orchestrator {
	return NewOrchestrator(provider, executor, Config{
		Model:         "test-model",
		MaxIterations: 5,
	}, newDiscardLogger())
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

func TestOrchestrator_CleanAnswerPath(t *testing.T) {
	answer := `{"outcome": "answer", "answer": "The domain example.best is active with status ok, expiring 2027-01-01."}`

	provider := &stubProvider{
		name: "test",
		responses: []ModelResponse{
			// First turn: model calls get_domain
			{
				ToolCalls: []ToolCall{{ID: "call-1", Name: "get_domain", Input: map[string]any{"name": "example.best"}}},
				Stop:      StopToolUse,
				Usage:     Usage{InputTokens: 100, OutputTokens: 50},
			},
			// Second turn: model returns final answer
			{
				Text:  answer,
				Stop:  StopFinalMessage,
				Usage: Usage{InputTokens: 200, OutputTokens: 100},
			},
		},
	}

	executor := newStubToolExecutor()
	executor.results["get_domain"] = ToolResult{
		Result: map[string]any{
			"name":     "example.best",
			"statuses": []string{"ok"},
		},
	}

	orch := newTestOrchestrator(provider, executor)
	result, err := orch.Ask(context.Background(), "What is the status of example.best?", CallerScope{UserID: "staff-1"})

	require.NoError(t, err)
	assert.Equal(t, OutcomeAnswer, result.Outcome)
	assert.Contains(t, result.Answer, "example.best")
	assert.Len(t, result.Evidence, 1)
	assert.Equal(t, "get_domain", result.Evidence[0].Tool)
	assert.Equal(t, 2, result.Iterations)
	assert.Equal(t, 300, result.TotalUsage.InputTokens)
	assert.Equal(t, 150, result.TotalUsage.OutputTokens)

	// Verify tool was called with correct scope
	require.Len(t, executor.scopeLog, 1)
	assert.Equal(t, "staff-1", executor.scopeLog[0].UserID)
}

func TestOrchestrator_EscalationOnMissingData(t *testing.T) {
	escalation := `{"outcome": "escalate", "reason": "Domain not found in the registry. Unable to determine status."}`

	provider := &stubProvider{
		name: "test",
		responses: []ModelResponse{
			{
				ToolCalls: []ToolCall{{ID: "call-1", Name: "get_domain", Input: map[string]any{"name": "missing.best"}}},
				Stop:      StopToolUse,
				Usage:     Usage{InputTokens: 100, OutputTokens: 50},
			},
			{
				Text:  escalation,
				Stop:  StopFinalMessage,
				Usage: Usage{InputTokens: 200, OutputTokens: 80},
			},
		},
	}

	executor := newStubToolExecutor()
	executor.results["get_domain"] = ToolResult{
		Result:  map[string]string{"error": "domain \"missing.best\" not found"},
		IsError: true,
	}

	orch := newTestOrchestrator(provider, executor)
	result, err := orch.Ask(context.Background(), "Status of missing.best?", CallerScope{UserID: "staff-2"})

	require.NoError(t, err)
	assert.Equal(t, OutcomeEscalate, result.Outcome)
	assert.Contains(t, result.Reason, "not found")
	assert.Len(t, result.Evidence, 1)
}

func TestOrchestrator_ActionRequiredDetection(t *testing.T) {
	actionRequired := `{"outcome": "action_required", "reason": "Domain is in redemptionPeriod. Current registrar: registrar1.", "action": "Restore domain example.best from redemption period."}`

	provider := &stubProvider{
		name: "test",
		responses: []ModelResponse{
			{
				ToolCalls: []ToolCall{{ID: "call-1", Name: "get_domain", Input: map[string]any{"name": "example.best"}}},
				Stop:      StopToolUse,
				Usage:     Usage{InputTokens: 100, OutputTokens: 50},
			},
			{
				Text:  actionRequired,
				Stop:  StopFinalMessage,
				Usage: Usage{InputTokens: 200, OutputTokens: 100},
			},
		},
	}

	executor := newStubToolExecutor()
	executor.results["get_domain"] = ToolResult{
		Result: map[string]any{
			"name":     "example.best",
			"rgpPhase": "redemptionPeriod",
		},
	}

	orch := newTestOrchestrator(provider, executor)
	result, err := orch.Ask(context.Background(), "Please restore example.best", CallerScope{UserID: "staff-3"})

	require.NoError(t, err)
	assert.Equal(t, OutcomeActionRequired, result.Outcome)
	assert.Contains(t, result.Reason, "redemptionPeriod")
	assert.Contains(t, result.Action, "Restore")
	assert.Len(t, result.Evidence, 1)
}

func TestOrchestrator_IterationCap(t *testing.T) {
	// Model always returns tool calls, never a final answer
	provider := &stubProvider{
		name: "test",
		responses: []ModelResponse{
			{ToolCalls: []ToolCall{{ID: "c1", Name: "get_domain", Input: map[string]any{"name": "a.best"}}}, Stop: StopToolUse, Usage: Usage{InputTokens: 10, OutputTokens: 5}},
			{ToolCalls: []ToolCall{{ID: "c2", Name: "get_tld", Input: map[string]any{"name": "best"}}}, Stop: StopToolUse, Usage: Usage{InputTokens: 10, OutputTokens: 5}},
			{ToolCalls: []ToolCall{{ID: "c3", Name: "get_domain", Input: map[string]any{"name": "b.best"}}}, Stop: StopToolUse, Usage: Usage{InputTokens: 10, OutputTokens: 5}},
			{ToolCalls: []ToolCall{{ID: "c4", Name: "get_tld", Input: map[string]any{"name": "best"}}}, Stop: StopToolUse, Usage: Usage{InputTokens: 10, OutputTokens: 5}},
			{ToolCalls: []ToolCall{{ID: "c5", Name: "get_domain", Input: map[string]any{"name": "c.best"}}}, Stop: StopToolUse, Usage: Usage{InputTokens: 10, OutputTokens: 5}},
		},
	}

	executor := newStubToolExecutor()
	orch := NewOrchestrator(provider, executor, Config{
		Model:         "test-model",
		MaxIterations: 3, // low cap for testing
	}, newDiscardLogger())

	result, err := orch.Ask(context.Background(), "Complex question", CallerScope{UserID: "staff-4"})

	require.NoError(t, err)
	assert.Equal(t, OutcomeEscalate, result.Outcome)
	assert.Contains(t, result.Reason, "cap")
	assert.Equal(t, 3, result.Iterations)
	assert.Len(t, result.Evidence, 3)
}

func TestOrchestrator_ToolErrorHandling(t *testing.T) {
	// Model calls a tool, it errors, model gets the error and escalates
	escalation := `{"outcome": "escalate", "reason": "Tool returned an error: failed to look up domain."}`

	provider := &stubProvider{
		name: "test",
		responses: []ModelResponse{
			{
				ToolCalls: []ToolCall{{ID: "call-1", Name: "get_domain", Input: map[string]any{"name": "error.best"}}},
				Stop:      StopToolUse,
				Usage:     Usage{InputTokens: 100, OutputTokens: 50},
			},
			{
				Text:  escalation,
				Stop:  StopFinalMessage,
				Usage: Usage{InputTokens: 200, OutputTokens: 80},
			},
		},
	}

	executor := newStubToolExecutor()
	executor.results["get_domain"] = ToolResult{
		Result:  map[string]string{"error": "failed to look up domain"},
		IsError: true,
	}

	orch := newTestOrchestrator(provider, executor)
	result, err := orch.Ask(context.Background(), "Status of error.best?", CallerScope{UserID: "staff-5"})

	require.NoError(t, err)
	assert.Equal(t, OutcomeEscalate, result.Outcome)
	assert.Len(t, result.Evidence, 1)
	// Evidence records the error result
	assert.True(t, strings.Contains(fmt.Sprintf("%v", result.Evidence[0].Result), "error"))
}

func TestOrchestrator_ModelError_GracefulEscalation(t *testing.T) {
	provider := &stubProvider{
		name:   "test",
		errors: []error{fmt.Errorf("API rate limit exceeded")},
	}

	executor := newStubToolExecutor()
	orch := newTestOrchestrator(provider, executor)

	result, err := orch.Ask(context.Background(), "Domain status?", CallerScope{UserID: "staff-6"})

	require.NoError(t, err) // model errors don't bubble up
	assert.Equal(t, OutcomeEscalate, result.Outcome)
	assert.Contains(t, result.Reason, "rate limit")
	assert.Equal(t, 1, result.Iterations)
}

func TestOrchestrator_TenantScopeThreading(t *testing.T) {
	answer := `{"outcome": "answer", "answer": "Domain is active."}`

	provider := &stubProvider{
		name: "test",
		responses: []ModelResponse{
			{
				ToolCalls: []ToolCall{
					{ID: "c1", Name: "get_domain", Input: map[string]any{"name": "a.best"}},
					{ID: "c2", Name: "get_tld", Input: map[string]any{"name": "best"}},
				},
				Stop:  StopToolUse,
				Usage: Usage{InputTokens: 100, OutputTokens: 50},
			},
			{
				Text:  answer,
				Stop:  StopFinalMessage,
				Usage: Usage{InputTokens: 200, OutputTokens: 100},
			},
		},
	}

	executor := newStubToolExecutor()
	scope := CallerScope{UserID: "staff-tenant-test"}
	orch := newTestOrchestrator(provider, executor)

	_, err := orch.Ask(context.Background(), "Domain and TLD info", scope)
	require.NoError(t, err)

	// Every tool call should have received the same caller scope
	require.Len(t, executor.scopeLog, 2)
	assert.Equal(t, "staff-tenant-test", executor.scopeLog[0].UserID)
	assert.Equal(t, "staff-tenant-test", executor.scopeLog[1].UserID)
}

func TestOrchestrator_MultipleToolCallsPerTurn(t *testing.T) {
	answer := `{"outcome": "answer", "answer": "Domain and TLD info retrieved."}`

	provider := &stubProvider{
		name: "test",
		responses: []ModelResponse{
			{
				ToolCalls: []ToolCall{
					{ID: "c1", Name: "get_domain", Input: map[string]any{"name": "multi.best"}},
					{ID: "c2", Name: "get_tld", Input: map[string]any{"name": "best"}},
				},
				Stop:  StopToolUse,
				Usage: Usage{InputTokens: 100, OutputTokens: 50},
			},
			{
				Text:  answer,
				Stop:  StopFinalMessage,
				Usage: Usage{InputTokens: 200, OutputTokens: 100},
			},
		},
	}

	executor := newStubToolExecutor()
	orch := newTestOrchestrator(provider, executor)

	result, err := orch.Ask(context.Background(), "Domain and TLD?", CallerScope{UserID: "staff-7"})

	require.NoError(t, err)
	assert.Equal(t, OutcomeAnswer, result.Outcome)
	assert.Len(t, result.Evidence, 2)
	assert.Equal(t, "get_domain", result.Evidence[0].Tool)
	assert.Equal(t, "get_tld", result.Evidence[1].Tool)
}

func TestOrchestrator_MaxTokensEscalation(t *testing.T) {
	provider := &stubProvider{
		name: "test",
		responses: []ModelResponse{
			{
				Text:  "partial response...",
				Stop:  StopMaxTokens,
				Usage: Usage{InputTokens: 100, OutputTokens: 4096},
			},
		},
	}

	executor := newStubToolExecutor()
	orch := newTestOrchestrator(provider, executor)

	result, err := orch.Ask(context.Background(), "Very complex question", CallerScope{UserID: "staff-8"})

	require.NoError(t, err)
	assert.Equal(t, OutcomeEscalate, result.Outcome)
	assert.Contains(t, result.Reason, "max tokens")
}

func TestOrchestrator_RawTextFallback(t *testing.T) {
	// Model returns plain text instead of JSON
	provider := &stubProvider{
		name: "test",
		responses: []ModelResponse{
			{
				Text:  "The domain example.best is active and not in any grace period.",
				Stop:  StopFinalMessage,
				Usage: Usage{InputTokens: 100, OutputTokens: 50},
			},
		},
	}

	executor := newStubToolExecutor()
	orch := newTestOrchestrator(provider, executor)

	result, err := orch.Ask(context.Background(), "Status?", CallerScope{UserID: "staff-9"})

	require.NoError(t, err)
	assert.Equal(t, OutcomeAnswer, result.Outcome)
	assert.Contains(t, result.Answer, "example.best")
}

func TestOrchestrator_SystemPromptPresent(t *testing.T) {
	provider := &stubProvider{
		name: "test",
		responses: []ModelResponse{
			{Text: `{"outcome": "answer", "answer": "test"}`, Stop: StopFinalMessage, Usage: Usage{}},
		},
	}

	executor := newStubToolExecutor()
	orch := newTestOrchestrator(provider, executor)

	_, err := orch.Ask(context.Background(), "test", CallerScope{UserID: "staff-10"})
	require.NoError(t, err)

	// Verify the system prompt was sent to the model
	require.Len(t, provider.calls, 1)
	assert.Equal(t, SystemPrompt, provider.calls[0].System)
	assert.Equal(t, "test-model", provider.calls[0].Model)
}

func TestOrchestrator_ToolDefsPassedToModel(t *testing.T) {
	provider := &stubProvider{
		name: "test",
		responses: []ModelResponse{
			{Text: `{"outcome": "answer", "answer": "test"}`, Stop: StopFinalMessage, Usage: Usage{}},
		},
	}

	executor := newStubToolExecutor()
	orch := newTestOrchestrator(provider, executor)

	_, err := orch.Ask(context.Background(), "test", CallerScope{UserID: "staff-11"})
	require.NoError(t, err)

	// Verify tool definitions were passed
	require.Len(t, provider.calls, 1)
	assert.Len(t, provider.calls[0].Tools, 2)
	assert.Equal(t, "get_domain", provider.calls[0].Tools[0].Name)
	assert.Equal(t, "get_tld", provider.calls[0].Tools[1].Name)
}

// ---------------------------------------------------------------------------
// Config tests
// ---------------------------------------------------------------------------

func TestConfig_EffectiveMaxIterations(t *testing.T) {
	assert.Equal(t, DefaultMaxIterations, Config{}.EffectiveMaxIterations())
	assert.Equal(t, 3, Config{MaxIterations: 3}.EffectiveMaxIterations())
	assert.Equal(t, DefaultMaxIterations, Config{MaxIterations: 0}.EffectiveMaxIterations())
}

// ---------------------------------------------------------------------------
// Result serialization tests
// ---------------------------------------------------------------------------

func TestResult_JSONSerialization(t *testing.T) {
	r := Result{
		Outcome: OutcomeAnswer,
		Answer:  "Domain is active.",
		Evidence: []Evidence{
			{Tool: "get_domain", Input: map[string]any{"name": "example.best"}, Result: map[string]any{"name": "example.best"}},
		},
		Iterations: 2,
		TotalUsage: Usage{InputTokens: 300, OutputTokens: 150},
	}

	data, err := json.Marshal(r)
	require.NoError(t, err)

	var decoded Result
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, OutcomeAnswer, decoded.Outcome)
	assert.Equal(t, "Domain is active.", decoded.Answer)
	assert.Len(t, decoded.Evidence, 1)
	assert.Equal(t, 2, decoded.Iterations)
}

// ---------------------------------------------------------------------------
// Tool executor helper tests
// ---------------------------------------------------------------------------

func TestExtractStringField(t *testing.T) {
	tests := []struct {
		name    string
		input   any
		field   string
		want    string
		wantErr bool
	}{
		{"map with field", map[string]any{"name": "test.best"}, "name", "test.best", false},
		{"missing field", map[string]any{"other": "val"}, "name", "", true},
		{"non-string field", map[string]any{"name": 123}, "name", "", true},
		{"json string", `{"name": "test.best"}`, "name", "test.best", false},
		{"invalid json", "not json", "name", "", true},
		{"nil input", nil, "name", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := extractStringField(tt.input, tt.field)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}
