// Package askg implements the "Ask G" orchestrator — a staff-facing support
// agent that answers registrar/registrant escalations by retrieving registry
// state through existing read-only application services and reasoning over it.
//
// Ask G is a tool consumer that sits above the interface layer. It calls
// application services directly (in-process), never through MCP transport.
// It depends on two injectable interfaces — ModelProvider and ToolExecutor —
// so the entire agent loop can be tested without a live model or database.
//
// See the package-level README.md for usage and configuration.
package askg

import "context"

// ---------------------------------------------------------------------------
// Normalized, provider-agnostic types the agent loop speaks in.
// ---------------------------------------------------------------------------

// ToolDef describes a tool the model may call. The InputSchema is a JSON
// Schema object derived from the MCP tool definitions (the single source of
// truth) so that MCP, REST, and Ask G never drift.
type ToolDef struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	InputSchema any    `json:"input_schema"` // JSON Schema object
}

// ToolCall represents a tool invocation requested by the model.
type ToolCall struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Input any    `json:"input"` // JSON-decoded input matching the tool's schema
}

// ToolResult carries the outcome of executing a single tool call.
type ToolResult struct {
	CallID  string `json:"call_id"`
	Result  any    `json:"result"`
	IsError bool   `json:"is_error"`
}

// Message is a single turn in the normalized conversation history.
type Message struct {
	Role        MessageRole  `json:"role"`
	Content     string       `json:"content,omitempty"`
	ToolCalls   []ToolCall   `json:"tool_calls,omitempty"`
	ToolResults []ToolResult `json:"tool_results,omitempty"`
}

// MessageRole identifies the sender of a message.
type MessageRole string

const (
	RoleUser      MessageRole = "user"
	RoleAssistant MessageRole = "assistant"
	RoleSystem    MessageRole = "system"
	RoleTool      MessageRole = "tool"
)

// ModelRequest is the normalized input to a single model turn.
type ModelRequest struct {
	System   string    `json:"system"`
	Messages []Message `json:"messages"`
	Tools    []ToolDef `json:"tools"`
	Model    string    `json:"model"` // provider-scoped model identifier
}

// StopReason indicates why the model stopped generating.
type StopReason int

const (
	StopFinalMessage StopReason = iota
	StopToolUse
	StopMaxTokens
	StopError
)

// Usage holds normalized token accounting for observability.
type Usage struct {
	InputTokens              int `json:"input_tokens"`
	OutputTokens             int `json:"output_tokens"`
	CacheCreationInputTokens int `json:"cache_creation_input_tokens"`
	CacheReadInputTokens     int `json:"cache_read_input_tokens"`
}

// ModelResponse is the normalized output of a single model turn.
type ModelResponse struct {
	Text      string     `json:"text"`
	ToolCalls []ToolCall `json:"tool_calls,omitempty"`
	Stop      StopReason `json:"stop"`
	Usage     Usage      `json:"usage"`
}

// ---------------------------------------------------------------------------
// ModelProvider interface
// ---------------------------------------------------------------------------

// ModelProvider abstracts access to an LLM. The agent loop depends only on
// this interface; vendor SDKs are confined to adapters behind it.
//
// Generate runs one turn. The adapter owns translation of ModelRequest
// (system + messages + tools) to the vendor wire format, and the vendor
// response back to ModelResponse — including tool-call and tool-result
// shapes, which differ across providers.
//
// The interface is non-streaming for MVP but does not preclude a streaming
// variant later.
type ModelProvider interface {
	Generate(ctx context.Context, req ModelRequest) (ModelResponse, error)
	Name() string
}
