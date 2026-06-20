package askg

// DefaultMaxIterations is the hard cap on agent loop iterations.
// On cap, the orchestrator returns an escalation with what was gathered.
const DefaultMaxIterations = 10

// Config holds the configuration for the Ask G orchestrator.
// Values are loaded from environment variables at the entrypoint,
// never inside the orchestrator or adapters.
type Config struct {
	// Provider identifies the model provider to use (e.g. "anthropic").
	Provider string `json:"provider"`

	// Model is the provider-scoped model identifier for the main
	// tool-use model (e.g. "claude-sonnet-4-6").
	Model string `json:"model"`

	// ClassifierModel is an optional cheaper/faster model for the
	// first-pass intent classification. If empty, the main Model is used.
	ClassifierModel string `json:"classifier_model,omitempty"`

	// MaxIterations is the hard cap on agent loop iterations.
	// Defaults to DefaultMaxIterations if zero.
	MaxIterations int `json:"max_iterations"`

	// APIKey is the provider API key. Loaded from env, never hardcoded.
	APIKey string `json:"-"` // excluded from JSON serialization

	// BaseURL overrides the provider API base URL (for testing/proxies).
	BaseURL string `json:"base_url,omitempty"`
}

// EffectiveMaxIterations returns MaxIterations if set, otherwise the default.
func (c Config) EffectiveMaxIterations() int {
	if c.MaxIterations > 0 {
		return c.MaxIterations
	}
	return DefaultMaxIterations
}
