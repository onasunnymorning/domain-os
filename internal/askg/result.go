package askg

// ---------------------------------------------------------------------------
// Output contract — discriminated result with provenance
// ---------------------------------------------------------------------------

// Outcome is the discriminator for the Ask G result.
type Outcome string

const (
	// OutcomeAnswer means a grounded answer supported entirely by retrieved
	// registry data, with provenance.
	OutcomeAnswer Outcome = "answer"

	// OutcomeEscalate means insufficient, ambiguous, or out-of-scope data.
	// This is a first-class, expected outcome — not a failure path.
	OutcomeEscalate Outcome = "escalate"

	// OutcomeActionRequired means the question implies a mutation (RGP restore,
	// refund, transfer, unlock, price override, etc.). The agent never acts —
	// it returns a recommended action plus the evidence it gathered, routed to
	// a human.
	OutcomeActionRequired Outcome = "action_required"
)

// Evidence records a single tool invocation that informed the outcome.
// Every claim in Result.Answer must be supported by something in Evidence.
type Evidence struct {
	Tool   string `json:"tool"`   // tool name invoked
	Input  any    `json:"input"`  // input the model supplied
	Result any    `json:"result"` // scoped result returned (already authz-filtered)
}

// Result is the discriminated output of a single Ask G request.
type Result struct {
	// Outcome discriminates the three possible result types.
	Outcome Outcome `json:"outcome"`

	// Answer is populated for OutcomeAnswer — a grounded answer with provenance.
	Answer string `json:"answer,omitempty"`

	// Reason is populated for OutcomeEscalate and OutcomeActionRequired.
	Reason string `json:"reason,omitempty"`

	// Action is the recommended action for OutcomeActionRequired (never executed).
	Action string `json:"action,omitempty"`

	// Evidence captures every retrieval that informed the outcome.
	Evidence []Evidence `json:"evidence"`

	// Iterations records how many model turns were used.
	Iterations int `json:"iterations"`

	// TotalUsage aggregates token usage across all model turns.
	TotalUsage Usage `json:"total_usage"`
}

// CallerScope identifies the staff member making the request. This is
// threaded through every tool call for audit logging. For MVP, the
// underlying services return unscoped data (staff can see everything),
// but the scope field enables future registrar-scoped filtering.
type CallerScope struct {
	UserID string `json:"user_id"` // staff member identity
}
