package provider

import (
	"context"
	"testing"

	"github.com/onasunnymorning/domain-os/internal/askg"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFakeProvider_ReturnsResponsesInSequence(t *testing.T) {
	fp := NewFakeProvider(
		FakeResponse{
			StopReason: askg.StopToolUse,
			Content:    "thinking...",
			ToolCalls: []askg.ToolCall{
				{ID: "tc-1", Name: "get_domain", Input: map[string]any{"name": "example.best"}},
			},
		},
		FakeResponse{
			StopReason: askg.StopFinalMessage,
			Content:    `{"outcome":"answer","answer":"The domain is active."}`,
		},
	)

	ctx := context.Background()
	req := askg.ModelRequest{Model: "fake-model"}

	// First call — tool use
	resp1, err := fp.Generate(ctx, req)
	require.NoError(t, err)
	assert.Equal(t, askg.StopToolUse, resp1.Stop)
	assert.Equal(t, "thinking...", resp1.Text)
	assert.Len(t, resp1.ToolCalls, 1)
	assert.Equal(t, "get_domain", resp1.ToolCalls[0].Name)
	assert.Equal(t, 1, fp.CallCount())

	// Second call — final message
	resp2, err := fp.Generate(ctx, req)
	require.NoError(t, err)
	assert.Equal(t, askg.StopFinalMessage, resp2.Stop)
	assert.Contains(t, resp2.Text, "answer")
	assert.Empty(t, resp2.ToolCalls)
	assert.Equal(t, 2, fp.CallCount())
}

func TestFakeProvider_ErrorOnExhaustedResponses(t *testing.T) {
	fp := NewFakeProvider(
		FakeResponse{
			StopReason: askg.StopFinalMessage,
			Content:    `{"outcome":"answer","answer":"done"}`,
		},
	)

	ctx := context.Background()
	req := askg.ModelRequest{Model: "fake-model"}

	// First call succeeds
	_, err := fp.Generate(ctx, req)
	require.NoError(t, err)

	// Second call should error
	_, err = fp.Generate(ctx, req)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "exhausted")
	assert.Contains(t, err.Error(), "1 pre-configured responses")
}

func TestFakeProvider_Name(t *testing.T) {
	fp := NewFakeProvider()
	assert.Equal(t, "fake", fp.Name())
}

func TestFakeProvider_UsageIsPopulated(t *testing.T) {
	fp := NewFakeProvider(
		FakeResponse{
			StopReason: askg.StopFinalMessage,
			Content:    `{"outcome":"answer","answer":"test"}`,
		},
	)

	resp, err := fp.Generate(context.Background(), askg.ModelRequest{})
	require.NoError(t, err)
	assert.Greater(t, resp.Usage.InputTokens, 0)
	assert.Greater(t, resp.Usage.OutputTokens, 0)
}
