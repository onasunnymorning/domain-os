// Package anthropic provides a ModelProvider adapter for the Anthropic Claude
// API. It translates the normalized askg types to the Anthropic SDK wire
// format and back. This is the reference adapter; additional providers
// (OpenAI, etc.) can be added by implementing the same interface with a new
// adapter + config, with zero changes to the loop, tool layer, or output
// contract.
package anthropic

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/onasunnymorning/domain-os/internal/askg"
)

// Default model identifiers — pinned from Anthropic's current docs (June 2026).
const (
	DefaultModel      = "claude-sonnet-4-6"
	DefaultClassifier = "claude-haiku-4-5"
)

// Adapter implements askg.ModelProvider for Anthropic Claude.
type Adapter struct {
	client anthropic.Client
}

// NewAdapter creates a new Anthropic adapter. The API key and optional base
// URL are loaded from the askg.Config (which loads from env at the entrypoint).
// The adapter never reads env directly.
func NewAdapter(cfg askg.Config) *Adapter {
	var opts []option.RequestOption
	if cfg.APIKey != "" {
		opts = append(opts, option.WithAPIKey(cfg.APIKey))
	}
	if cfg.BaseURL != "" {
		opts = append(opts, option.WithBaseURL(cfg.BaseURL))
	}

	return &Adapter{
		client: anthropic.NewClient(opts...),
	}
}

// Name identifies the provider for logging/metrics.
func (a *Adapter) Name() string { return "anthropic" }

// Generate runs one turn against the Claude API. It translates the normalized
// ModelRequest to the Anthropic wire format, calls the API, and translates
// the response back.
func (a *Adapter) Generate(ctx context.Context, req askg.ModelRequest) (askg.ModelResponse, error) {
	// Build Anthropic messages from normalized messages
	messages := make([]anthropic.MessageParam, 0, len(req.Messages))
	for _, msg := range req.Messages {
		switch msg.Role {
		case askg.RoleUser:
			messages = append(messages, anthropic.NewUserMessage(
				anthropic.NewTextBlock(msg.Content),
			))
		case askg.RoleAssistant:
			blocks := buildAssistantBlocks(msg)
			messages = append(messages, anthropic.NewAssistantMessage(blocks...))
		case askg.RoleTool:
			blocks := buildToolResultBlocks(msg)
			messages = append(messages, anthropic.NewUserMessage(blocks...))
		}
	}

	// Build tool definitions
	tools := make([]anthropic.ToolUnionParam, 0, len(req.Tools))
	for _, t := range req.Tools {
		schema := buildInputSchema(t.InputSchema)
		tp := anthropic.ToolUnionParamOfTool(schema, t.Name)
		if tp.OfTool != nil {
			tp.OfTool.Description = anthropic.String(t.Description)
		}
		tools = append(tools, tp)
	}

	// Build the API request.
	// Prompt caching: the explicit breakpoint on the system prompt caches
	// tools + system (the static prefix). The top-level CacheControl enables
	// automatic caching, which places a second breakpoint on the last
	// cacheable message block — so on iterations 2+ of the tool loop the
	// growing conversation prefix is also served from cache.
	params := anthropic.MessageNewParams{
		Model:     req.Model,
		MaxTokens: 4096,
		CacheControl: anthropic.NewCacheControlEphemeralParam(),
		System: []anthropic.TextBlockParam{
			{
				Text:         req.System,
				CacheControl: anthropic.NewCacheControlEphemeralParam(),
			},
		},
		Messages: messages,
		Tools:    tools,
	}

	// Call the API
	resp, err := a.client.Messages.New(ctx, params)
	if err != nil {
		return askg.ModelResponse{}, fmt.Errorf("anthropic: API call failed: %w", err)
	}

	// Translate response
	return translateResponse(resp), nil
}

// ---------------------------------------------------------------------------
// Translation helpers
// ---------------------------------------------------------------------------

// buildAssistantBlocks converts a normalized assistant message (which may
// contain tool calls) to Anthropic content blocks.
func buildAssistantBlocks(msg askg.Message) []anthropic.ContentBlockParamUnion {
	var blocks []anthropic.ContentBlockParamUnion

	if msg.Content != "" {
		blocks = append(blocks, anthropic.NewTextBlock(msg.Content))
	}

	for _, tc := range msg.ToolCalls {
		blocks = append(blocks, anthropic.NewToolUseBlock(tc.ID, tc.Input, tc.Name))
	}

	return blocks
}

// buildToolResultBlocks converts normalized tool results to Anthropic
// tool_result content blocks (sent as user messages per the API spec).
func buildToolResultBlocks(msg askg.Message) []anthropic.ContentBlockParamUnion {
	var blocks []anthropic.ContentBlockParamUnion

	for _, tr := range msg.ToolResults {
		resultJSON, _ := json.Marshal(tr.Result)
		block := anthropic.NewToolResultBlock(tr.CallID, string(resultJSON), tr.IsError)
		blocks = append(blocks, block)
	}

	return blocks
}

// buildInputSchema converts a generic JSON Schema (any) to the
// Anthropic SDK's ToolInputSchemaParam.
func buildInputSchema(schema any) anthropic.ToolInputSchemaParam {
	m, ok := schema.(map[string]any)
	if !ok {
		return anthropic.ToolInputSchemaParam{}
	}

	param := anthropic.ToolInputSchemaParam{}
	if props, ok := m["properties"]; ok {
		param.Properties = props
	}
	if req, ok := m["required"].([]string); ok {
		param.Required = req
	}

	return param
}

// translateResponse converts an Anthropic API response to the normalized
// ModelResponse format.
func translateResponse(resp *anthropic.Message) askg.ModelResponse {
	var text string
	var toolCalls []askg.ToolCall

	for _, block := range resp.Content {
		switch b := block.AsAny().(type) {
		case anthropic.TextBlock:
			text += b.Text
		case anthropic.ToolUseBlock:
			var input any
			_ = json.Unmarshal(b.Input, &input)
			toolCalls = append(toolCalls, askg.ToolCall{
				ID:    b.ID,
				Name:  b.Name,
				Input: input,
			})
		}
	}

	stop := askg.StopFinalMessage
	switch resp.StopReason {
	case "tool_use":
		stop = askg.StopToolUse
	case "max_tokens":
		stop = askg.StopMaxTokens
	case "end_turn":
		stop = askg.StopFinalMessage
	}

	return askg.ModelResponse{
		Text:      text,
		ToolCalls: toolCalls,
		Stop:      stop,
		Usage: askg.Usage{
			InputTokens:              int(resp.Usage.InputTokens),
			OutputTokens:             int(resp.Usage.OutputTokens),
			CacheCreationInputTokens: int(resp.Usage.CacheCreationInputTokens),
			CacheReadInputTokens:     int(resp.Usage.CacheReadInputTokens),
		},
	}
}
