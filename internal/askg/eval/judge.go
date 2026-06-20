package eval

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/onasunnymorning/domain-os/internal/askg"
)

// CALIBRATION NOTE: Spot-check against human labels before trusting.
// Not used for safety-critical axes.

// judgeSystemPrompt instructs the judge model to evaluate an agent's answer
// for correctness and grounding against the tool evidence.
const judgeSystemPrompt = `You are an evaluation judge for a support agent that answers domain-registry questions.

You will receive:
1. The original question the agent was asked
2. The tool evidence (JSON) the agent retrieved before answering
3. The agent's final answer

Evaluate the answer on two axes:

**Correctness**: Is the answer factually correct given the evidence? Score 0.0-1.0.
- 1.0 = every factual claim is correct and supported by the evidence
- 0.5 = partially correct, some claims wrong or missing key facts
- 0.0 = fundamentally wrong or contradicts the evidence

**Grounding**: Does each claim in the answer follow from the evidence provided? Score 0.0-1.0.
- 1.0 = every claim can be traced to a specific piece of evidence
- 0.5 = some claims grounded, others asserted without evidence
- 0.0 = answer is entirely ungrounded / fabricated

Return ONLY a JSON object with this exact structure, no other text:
{"correctness": {"score": 0.0, "reasoning": "..."}, "grounding": {"score": 0.0, "reasoning": "..."}}`

// judgeAxisScore is the inner structure for a single axis in the judge response.
type judgeAxisScore struct {
	Score     float64 `json:"score"`
	Reasoning string  `json:"reasoning"`
}

// judgeResponse is the expected JSON structure from the judge model.
type judgeResponse struct {
	Correctness judgeAxisScore `json:"correctness"`
	Grounding   judgeAxisScore `json:"grounding"`
}

// Judge uses an LLM-as-judge to evaluate the agent's answer for correctness
// and grounding. Only meaningful for OutcomeAnswer results — for escalate or
// action_required outcomes, returns empty verdicts since there is no answer
// to evaluate.
func Judge(ctx context.Context, provider askg.ModelProvider, model string, evalCase EvalCase, result *askg.Result) ([]JudgeVerdict, error) {
	// Only judge answer outcomes — escalation and action_required have no
	// answer text to evaluate.
	if result.Outcome != askg.OutcomeAnswer {
		return nil, nil
	}

	// Serialize evidence for the judge prompt.
	evidenceJSON, err := json.MarshalIndent(result.Evidence, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to serialize evidence: %w", err)
	}

	userPrompt := fmt.Sprintf(`**Question**: %s

**Tool Evidence**:
%s

**Agent Answer**:
%s`,
		evalCase.Question,
		string(evidenceJSON),
		result.Answer,
	)

	resp, err := provider.Generate(ctx, askg.ModelRequest{
		System: judgeSystemPrompt,
		Messages: []askg.Message{
			{
				Role:    askg.RoleUser,
				Content: userPrompt,
			},
		},
		Model: model,
	})
	if err != nil {
		return nil, fmt.Errorf("judge model call failed: %w", err)
	}

	// Parse the judge response — extract JSON from the response text.
	responseText := strings.TrimSpace(resp.Text)
	var jr judgeResponse
	if err := json.Unmarshal([]byte(responseText), &jr); err != nil {
		// Try to find JSON within the response (model may add surrounding text).
		start := strings.Index(responseText, "{")
		end := strings.LastIndex(responseText, "}")
		if start >= 0 && end > start {
			if err2 := json.Unmarshal([]byte(responseText[start:end+1]), &jr); err2 != nil {
				return nil, fmt.Errorf("failed to parse judge response as JSON: %w (raw: %s)", err2, responseText)
			}
		} else {
			return nil, fmt.Errorf("failed to parse judge response as JSON: %w (raw: %s)", err, responseText)
		}
	}

	return []JudgeVerdict{
		{
			Axis:      "correctness",
			Score:     jr.Correctness.Score,
			Reasoning: jr.Correctness.Reasoning,
		},
		{
			Axis:      "grounding",
			Score:     jr.Grounding.Score,
			Reasoning: jr.Grounding.Reasoning,
		},
	}, nil
}
