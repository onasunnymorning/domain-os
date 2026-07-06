// Package provider contains ModelProvider adapters for the askg orchestrator.
package provider

import (
	"context"
	"fmt"

	"github.com/onasunnymorning/domain-os/internal/askg"
)

// FakeResponse is a single pre-configured model response for testing.
type FakeResponse struct {
	StopReason askg.StopReason
	Content    string
	ToolCalls  []askg.ToolCall
}

// FakeProvider is a deterministic ModelProvider that returns pre-configured
// responses in sequence. It is used in tests to exercise the orchestrator
// and agent controller without hitting a real LLM API.
type FakeProvider struct {
	responses []FakeResponse
	callIndex int
}

// NewFakeProvider creates a FakeProvider that returns the given responses
// in order. If Generate is called more times than there are responses,
// it returns an error.
func NewFakeProvider(responses ...FakeResponse) *FakeProvider {
	return &FakeProvider{
		responses: responses,
	}
}

// Generate returns the next pre-configured response in sequence.
// Returns an error if all responses have been exhausted.
func (f *FakeProvider) Generate(_ context.Context, _ askg.ModelRequest) (askg.ModelResponse, error) {
	if f.callIndex >= len(f.responses) {
		return askg.ModelResponse{}, fmt.Errorf(
			"fake provider: exhausted all %d pre-configured responses (call index %d) — add more FakeResponse entries to the provider",
			len(f.responses), f.callIndex,
		)
	}

	resp := f.responses[f.callIndex]
	f.callIndex++

	return askg.ModelResponse{
		Text:      resp.Content,
		ToolCalls: resp.ToolCalls,
		Stop:      resp.StopReason,
		Usage: askg.Usage{
			InputTokens:  10,
			OutputTokens: 5,
		},
	}, nil
}

// Name identifies this provider for logging/metrics.
func (f *FakeProvider) Name() string { return "fake" }

// CallCount returns the number of Generate calls made so far.
func (f *FakeProvider) CallCount() int {
	return f.callIndex
}
