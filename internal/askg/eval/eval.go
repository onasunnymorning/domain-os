// Package eval provides the evaluation harness for the Ask G agent
// orchestrator. It defines fixture-backed test cases, deterministic
// gates, and LLM-judge axes that together measure correctness,
// grounding, and safety of agent responses.
//
// The types here carry no build tag — they are used by both
// deterministic unit tests and live model-backed evaluation runs.
package eval

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"

	"github.com/onasunnymorning/domain-os/internal/askg"
)

// ---------------------------------------------------------------------------
// Category — what the test case is probing
// ---------------------------------------------------------------------------

// Category classifies what an eval case is designed to test.
type Category string

const (
	// CategoryAnswer tests that the agent produces a grounded answer.
	CategoryAnswer Category = "answer"

	// CategoryMustEscalate tests that the agent correctly escalates.
	CategoryMustEscalate Category = "must_escalate"

	// CategoryActionRequired tests that the agent detects a mutation.
	CategoryActionRequired Category = "action_required"

	// CategoryTenantIsolation tests cross-registrar data leak prevention.
	CategoryTenantIsolation Category = "tenant_isolation"

	// CategoryAdversarial tests prompt-injection and jailbreak resilience.
	CategoryAdversarial Category = "adversarial"
)

// ---------------------------------------------------------------------------
// Fixture types — canned tool inputs/outputs
// ---------------------------------------------------------------------------

// ToolFixture pairs a tool name + input with a canned result.
// The fixture executor matches on tool name + normalized input.
type ToolFixture struct {
	Tool    string         `yaml:"tool"                   json:"tool"`
	Input   map[string]any `yaml:"input"                  json:"input"`
	Result  any            `yaml:"result"                 json:"result"`
	IsError bool           `yaml:"is_error,omitempty"     json:"is_error,omitempty"`
}

// ---------------------------------------------------------------------------
// Expectation types — what the eval asserts
// ---------------------------------------------------------------------------

// ExpectedToolCall describes a tool invocation the agent should make.
type ExpectedToolCall struct {
	Tool  string         `yaml:"tool"            json:"tool"`
	Input map[string]any `yaml:"input,omitempty" json:"input,omitempty"`
}

// AnswerRubric defines substring checks against the agent's answer text.
type AnswerRubric struct {
	MustInclude    []string `yaml:"must_include,omitempty"     json:"must_include,omitempty"`
	MustNotInclude []string `yaml:"must_not_include,omitempty" json:"must_not_include,omitempty"`
}

// Expectation captures the deterministic and fuzzy assertions for a case.
type Expectation struct {
	Outcome        askg.Outcome       `yaml:"outcome"                   json:"outcome"`
	ToolCalls      []ExpectedToolCall `yaml:"tool_calls,omitempty"      json:"tool_calls,omitempty"`
	AnswerRubric   AnswerRubric       `yaml:"answer_rubric,omitempty"   json:"answer_rubric,omitempty"`
	DetectedAction string             `yaml:"detected_action,omitempty" json:"detected_action,omitempty"`
}

// ---------------------------------------------------------------------------
// EvalCase — a single test scenario
// ---------------------------------------------------------------------------

// EvalCase is a self-contained evaluation scenario: question, fixtures,
// and expectations. Cases are loaded from YAML files.
type EvalCase struct {
	ID          string           `yaml:"id"                          json:"id"`
	Category    Category         `yaml:"category"                    json:"category"`
	Question    string           `yaml:"question"                    json:"question"`
	CallerScope askg.CallerScope `yaml:"caller_scope"                json:"caller_scope"`
	Fixtures    []ToolFixture    `yaml:"fixtures"                    json:"fixtures"`
	OutOfScope  []string         `yaml:"out_of_scope_data,omitempty" json:"out_of_scope_data,omitempty"`
	Expect      Expectation      `yaml:"expect"                      json:"expect"`
}

// ---------------------------------------------------------------------------
// Result types — what the harness records
// ---------------------------------------------------------------------------

// GateResult is the outcome of a single deterministic gate.
type GateResult struct {
	Name   string `json:"name"`
	Pass   bool   `json:"pass"`
	Detail string `json:"detail"`
}

// JudgeVerdict is the outcome of a single LLM-judge axis.
type JudgeVerdict struct {
	Axis      string  `json:"axis"`      // e.g. "correctness", "grounding"
	Score     float64 `json:"score"`     // 0.0–1.0
	Reasoning string  `json:"reasoning"`
}

// RunResult captures the outcome of a single run of a single case.
type RunResult struct {
	Result    *askg.Result    `json:"result"`
	Gates     []GateResult    `json:"gates"`
	Judgments []JudgeVerdict  `json:"judgments,omitempty"`
	ToolTrace []askg.ToolCall `json:"tool_trace"`
	Pass      bool            `json:"pass"` // true only if ALL hard gates pass
}

// CaseResult aggregates results across N runs of the same case.
type CaseResult struct {
	CaseID             string             `json:"case_id"`
	Category           Category           `json:"category"`
	Runs               []RunResult        `json:"runs"`
	PassRate           float64            `json:"pass_rate"`       // fraction of runs where Pass==true
	GatePassRates      map[string]float64 `json:"gate_pass_rates"` // per-gate pass rates across runs
	expectedOutcomeStr string             // internal, set during RunCase for reporting
}

// ---------------------------------------------------------------------------
// EvalConfig — harness configuration
// ---------------------------------------------------------------------------

// EvalConfig controls how the eval harness runs.
type EvalConfig struct {
	N             int    `json:"n"`              // runs per case (default 3)
	Model         string `json:"model"`          // model to use for the agent
	JudgeModel    string `json:"judge_model"`    // model to use for LLM judge
	MaxIterations int    `json:"max_iterations"` // orchestrator iteration cap
}

// DefaultEvalConfig returns sensible defaults for the eval harness.
func DefaultEvalConfig() EvalConfig {
	return EvalConfig{
		N:             3,
		Model:         "claude-sonnet-5",
		JudgeModel:    "claude-haiku-4-5",
		MaxIterations: 10,
	}
}

// ---------------------------------------------------------------------------
// YAML loading
// ---------------------------------------------------------------------------

// caseFile wraps the top-level structure of an eval YAML file.
type caseFile struct {
	Cases []EvalCase `yaml:"cases"`
}

// LoadCases reads an eval YAML file and returns the cases it contains.
// The YAML must have a top-level `cases:` key wrapping the list.
func LoadCases(path string) ([]EvalCase, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("eval: read cases file %s: %w", path, err)
	}

	var f caseFile
	if err := yaml.Unmarshal(data, &f); err != nil {
		return nil, fmt.Errorf("eval: unmarshal cases file %s: %w", path, err)
	}

	if len(f.Cases) == 0 {
		return nil, fmt.Errorf("eval: no cases found in %s", path)
	}

	return f.Cases, nil
}
