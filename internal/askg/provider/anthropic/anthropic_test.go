package anthropic

import (
	"testing"

	"github.com/onasunnymorning/domain-os/internal/askg"
	"github.com/stretchr/testify/assert"
)

func TestAdapter_Name(t *testing.T) {
	a := NewAdapter(askg.Config{APIKey: "test-key"})
	assert.Equal(t, "anthropic", a.Name())
}

func TestAdapter_NewAdapter_WithConfig(t *testing.T) {
	cfg := askg.Config{
		APIKey:  "sk-test-key",
		BaseURL: "https://test.example.com",
	}
	a := NewAdapter(cfg)
	assert.NotNil(t, a)
}

func TestBuildAssistantBlocks_TextOnly(t *testing.T) {
	msg := askg.Message{
		Role:    askg.RoleAssistant,
		Content: "Hello, I can help you.",
	}
	blocks := buildAssistantBlocks(msg)
	assert.Len(t, blocks, 1)
}

func TestBuildAssistantBlocks_WithToolCalls(t *testing.T) {
	msg := askg.Message{
		Role:    askg.RoleAssistant,
		Content: "Let me look that up.",
		ToolCalls: []askg.ToolCall{
			{ID: "call-1", Name: "get_domain", Input: map[string]any{"name": "example.best"}},
		},
	}
	blocks := buildAssistantBlocks(msg)
	assert.Len(t, blocks, 2) // text + tool_use
}

func TestBuildToolResultBlocks(t *testing.T) {
	msg := askg.Message{
		Role: askg.RoleTool,
		ToolResults: []askg.ToolResult{
			{CallID: "call-1", Result: map[string]any{"name": "example.best"}, IsError: false},
			{CallID: "call-2", Result: map[string]string{"error": "not found"}, IsError: true},
		},
	}
	blocks := buildToolResultBlocks(msg)
	assert.Len(t, blocks, 2)
}

func TestBuildInputSchema(t *testing.T) {
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"name": map[string]any{
				"type":        "string",
				"description": "domain name",
			},
		},
		"required": []string{"name"},
	}

	param := buildInputSchema(schema)
	assert.NotNil(t, param.Properties)
	assert.Equal(t, []string{"name"}, param.Required)
}

func TestBuildInputSchema_NilInput(t *testing.T) {
	param := buildInputSchema(nil)
	assert.Nil(t, param.Properties)
}

func TestDefaultModelConstants(t *testing.T) {
	assert.Equal(t, "claude-sonnet-5", DefaultModel)
	assert.Equal(t, "claude-haiku-4-5", DefaultClassifier)
}
