package askg

import "context"

// ToolExecutor abstracts the execution of tool calls. The orchestrator
// depends on this interface, never on a concrete executor. Production
// wires the in-process, MCP-schema-driven implementation; tests wire a
// fixture-backed stub. This is the seam that makes agent evals possible.
type ToolExecutor interface {
	// Execute runs a single tool call and returns the result.
	// The CallerScope is threaded through for audit logging and
	// future registrar-scoped filtering.
	Execute(ctx context.Context, call ToolCall, scope CallerScope) ToolResult

	// Tools returns the tool definitions available to the model.
	// These are derived from the MCP tool definitions.
	Tools() []ToolDef
}
