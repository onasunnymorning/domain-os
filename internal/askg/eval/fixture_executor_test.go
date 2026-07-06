package eval

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onasunnymorning/domain-os/internal/askg"
)

func TestFixtureToolExecutor_MatchFound(t *testing.T) {
	fixtures := []ToolFixture{
		{
			Tool:  "get_domain",
			Input: map[string]any{"name": "example.best"},
			Result: map[string]any{
				"name":     "example.best",
				"statuses": []any{"ok"},
			},
		},
	}
	exec := NewFixtureToolExecutor(fixtures)

	result := exec.Execute(context.Background(), askg.ToolCall{
		ID:    "call-1",
		Name:  "get_domain",
		Input: map[string]any{"name": "example.best"},
	}, askg.CallerScope{UserID: "staff-1"})

	assert.False(t, result.IsError, "expected no error for matched fixture")
	assert.Equal(t, "call-1", result.CallID)

	m, ok := result.Result.(map[string]any)
	require.True(t, ok, "result should be a map")
	assert.Equal(t, "example.best", m["name"])
}

func TestFixtureToolExecutor_MatchNotFound(t *testing.T) {
	fixtures := []ToolFixture{
		{
			Tool:   "get_domain",
			Input:  map[string]any{"name": "example.best"},
			Result: map[string]any{"name": "example.best"},
		},
	}
	exec := NewFixtureToolExecutor(fixtures)

	result := exec.Execute(context.Background(), askg.ToolCall{
		ID:    "call-2",
		Name:  "get_domain",
		Input: map[string]any{"name": "no-such-domain.best"},
	}, askg.CallerScope{UserID: "staff-1"})

	assert.True(t, result.IsError, "expected error for unmatched fixture")
	assert.Equal(t, "call-2", result.CallID)

	m, ok := result.Result.(map[string]string)
	require.True(t, ok, "error result should be map[string]string")
	assert.Equal(t, "not found", m["error"])
}

func TestFixtureToolExecutor_ExecLogRecorded(t *testing.T) {
	exec := NewFixtureToolExecutor([]ToolFixture{
		{
			Tool:   "get_domain",
			Input:  map[string]any{"name": "a.best"},
			Result: "ok",
		},
	})

	scope := askg.CallerScope{UserID: "staff-1"}

	exec.Execute(context.Background(), askg.ToolCall{ID: "c1", Name: "get_domain", Input: map[string]any{"name": "a.best"}}, scope)
	exec.Execute(context.Background(), askg.ToolCall{ID: "c2", Name: "get_tld", Input: map[string]any{"name": "best"}}, scope)

	log := exec.ExecLog()
	require.Len(t, log, 2, "exec log should record all calls")
	assert.Equal(t, "c1", log[0].ID)
	assert.Equal(t, "get_domain", log[0].Name)
	assert.Equal(t, "c2", log[1].ID)
	assert.Equal(t, "get_tld", log[1].Name)
}

func TestFixtureToolExecutor_ScopeLogRecorded(t *testing.T) {
	exec := NewFixtureToolExecutor(nil)

	scope1 := askg.CallerScope{UserID: "alice"}
	scope2 := askg.CallerScope{UserID: "bob"}

	exec.Execute(context.Background(), askg.ToolCall{ID: "c1", Name: "get_domain", Input: map[string]any{"name": "x.best"}}, scope1)
	exec.Execute(context.Background(), askg.ToolCall{ID: "c2", Name: "get_domain", Input: map[string]any{"name": "y.best"}}, scope2)

	scopes := exec.ScopeLog()
	require.Len(t, scopes, 2, "scope log should record all scopes")
	assert.Equal(t, "alice", scopes[0].UserID)
	assert.Equal(t, "bob", scopes[1].UserID)
}

func TestFixtureToolExecutor_CaseInsensitiveMatch(t *testing.T) {
	fixtures := []ToolFixture{
		{
			Tool:   "get_domain",
			Input:  map[string]any{"name": "example.best"},
			Result: map[string]any{"name": "example.best", "statuses": []any{"ok"}},
		},
	}
	exec := NewFixtureToolExecutor(fixtures)

	// Input uses different casing — should still match.
	result := exec.Execute(context.Background(), askg.ToolCall{
		ID:    "call-ci",
		Name:  "get_domain",
		Input: map[string]any{"name": "Example.Best"},
	}, askg.CallerScope{UserID: "staff-1"})

	assert.False(t, result.IsError, "case-insensitive match should succeed")
	assert.Equal(t, "call-ci", result.CallID)

	m, ok := result.Result.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "example.best", m["name"])
}

func TestFixtureToolExecutor_Tools(t *testing.T) {
	exec := NewFixtureToolExecutor(nil)
	tools := exec.Tools()

	require.Len(t, tools, 3, "should return three tool definitions")

	names := make(map[string]bool, 3)
	for _, td := range tools {
		names[td.Name] = true
		assert.NotEmpty(t, td.Description, "tool %s should have a description", td.Name)
		assert.NotNil(t, td.InputSchema, "tool %s should have an input schema", td.Name)
	}

	assert.True(t, names["get_domain"], "should include get_domain")
	assert.True(t, names["get_tld"], "should include get_tld")
	assert.True(t, names["answer_system_question"], "should include answer_system_question")
}

func TestFixtureToolExecutor_ErrorFixture(t *testing.T) {
	fixtures := []ToolFixture{
		{
			Tool:    "get_domain",
			Input:   map[string]any{"name": "missing.best"},
			Result:  map[string]any{"error": "domain \"missing.best\" not found"},
			IsError: true,
		},
	}
	exec := NewFixtureToolExecutor(fixtures)

	result := exec.Execute(context.Background(), askg.ToolCall{
		ID:    "call-err",
		Name:  "get_domain",
		Input: map[string]any{"name": "missing.best"},
	}, askg.CallerScope{UserID: "staff-1"})

	assert.True(t, result.IsError, "fixture with IsError=true should return error result")
	assert.Equal(t, "call-err", result.CallID)

	m, ok := result.Result.(map[string]any)
	require.True(t, ok, "error result should be the fixture's result map")
	assert.Contains(t, m["error"], "not found")
}
