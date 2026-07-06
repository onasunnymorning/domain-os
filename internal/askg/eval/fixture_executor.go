package eval

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/onasunnymorning/domain-os/internal/askg"
)

// FixtureToolExecutor implements askg.ToolExecutor backed by canned fixture
// data. It matches incoming tool calls against fixtures by tool name and
// normalized input, returning the pre-recorded result. Every call is logged
// for provenance assertions in tests.
type FixtureToolExecutor struct {
	fixtures []ToolFixture
	execLog  []askg.ToolCall
	scopeLog []askg.CallerScope
}

// compile-time interface check
var _ askg.ToolExecutor = (*FixtureToolExecutor)(nil)

// NewFixtureToolExecutor creates a fixture-backed executor with the given
// canned tool responses.
func NewFixtureToolExecutor(fixtures []ToolFixture) *FixtureToolExecutor {
	return &FixtureToolExecutor{
		fixtures: fixtures,
	}
}

// Execute runs a tool call against the fixture set. It records every call
// and scope for later assertions, then matches on tool name + normalized
// input (case-insensitive on string values).
func (e *FixtureToolExecutor) Execute(_ context.Context, call askg.ToolCall, scope askg.CallerScope) askg.ToolResult {
	e.execLog = append(e.execLog, call)
	e.scopeLog = append(e.scopeLog, scope)

	if f, ok := e.matchFixture(call); ok {
		return askg.ToolResult{
			CallID:  call.ID,
			Result:  f.Result,
			IsError: f.IsError,
		}
	}

	return askg.ToolResult{
		CallID:  call.ID,
		Result:  map[string]string{"error": "not found"},
		IsError: true,
	}
}

// Tools returns the tool definitions available in the eval harness:
// get_domain, get_tld, and answer_system_question with identical
// descriptions and input schemas to InProcessToolExecutor.
func (e *FixtureToolExecutor) Tools() []askg.ToolDef {
	return []askg.ToolDef{
		{
			Name:        "get_domain",
			Description: "Look up the current registry state of a domain name, including EPP status codes, expiry, redemption/RGP state, nameservers, and sponsoring registrar.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"name": map[string]any{
						"type":        "string",
						"description": "Fully-qualified domain name to look up, e.g. example.best. Must include the TLD.",
					},
				},
				"required": []string{"name"},
			},
		},
		{
			Name:        "get_tld",
			Description: "Look up the configuration and lifecycle state of a top-level domain (TLD), including type, registry operator, DNS status, and currently active phases with pricing and policy.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"name": map[string]any{
						"type":        "string",
						"description": "TLD name to look up, e.g. best, radio, or xn--e1a4c (IDN). Do not include a leading dot.",
					},
				},
				"required": []string{"name"},
			},
		},
		{
			Name:        "answer_system_question",
			Description: "Search the internal knowledge base for information about system architecture, workflows, deployment, domain lifecycle, and registry operations. Use this when the question is about HOW the system works, not about specific domain or TLD data.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"question": map[string]any{
						"type":        "string",
						"description": "The question to search the knowledge base for",
					},
				},
				"required": []string{"question"},
			},
		},
	}
}

// ExecLog returns all tool calls that were executed, in order.
func (e *FixtureToolExecutor) ExecLog() []askg.ToolCall {
	return e.execLog
}

// ScopeLog returns all caller scopes that were recorded, in order.
func (e *FixtureToolExecutor) ScopeLog() []askg.CallerScope {
	return e.scopeLog
}

// matchFixture searches for a fixture matching the tool call by name and
// normalized input. String values in input maps are compared case-insensitively
// to handle domain name casing differences.
func (e *FixtureToolExecutor) matchFixture(call askg.ToolCall) (ToolFixture, bool) {
	for _, f := range e.fixtures {
		if f.Tool != call.Name {
			continue
		}
		if normalizedEqual(f.Input, call.Input) {
			return f, true
		}
	}
	return ToolFixture{}, false
}

// normalizedEqual compares a fixture input (map[string]any) with the call
// input (any) by normalizing both to lowercase JSON and comparing bytes.
func normalizedEqual(fixtureInput map[string]any, callInput any) bool {
	fixtureBytes, err := marshalNormalized(fixtureInput)
	if err != nil {
		return false
	}

	callBytes, err := marshalNormalized(callInput)
	if err != nil {
		return false
	}

	return string(fixtureBytes) == string(callBytes)
}

// marshalNormalized JSON-encodes a value with all string values lowercased
// for case-insensitive comparison.
func marshalNormalized(v any) ([]byte, error) {
	lowered := lowerStrings(v)
	return json.Marshal(lowered)
}

// lowerStrings recursively walks a value and lowercases all string values.
func lowerStrings(v any) any {
	switch val := v.(type) {
	case string:
		return strings.ToLower(val)
	case map[string]any:
		out := make(map[string]any, len(val))
		for k, v := range val {
			out[k] = lowerStrings(v)
		}
		return out
	case []any:
		out := make([]any, len(val))
		for i, v := range val {
			out[i] = lowerStrings(v)
		}
		return out
	default:
		return v
	}
}
