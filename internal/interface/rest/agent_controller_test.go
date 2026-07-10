package rest

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/onasunnymorning/domain-os/internal/askg"
	"github.com/onasunnymorning/domain-os/internal/askg/provider"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// Test helpers
// ---------------------------------------------------------------------------

// newTestOrchestrator builds an orchestrator with the given FakeProvider
// and a no-op tool executor for controller-level tests.
func newTestOrchestrator(fp *provider.FakeProvider) *askg.Orchestrator {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	cfg := askg.Config{
		Provider:      "fake",
		Model:         "fake-model",
		MaxIterations: 5,
	}
	return askg.NewOrchestrator(fp, &noopToolExecutor{}, cfg, logger)
}

// noopToolExecutor satisfies ToolExecutor with no tools — the FakeProvider
// returns a final message directly so tools are never invoked.
type noopToolExecutor struct{}

func (n *noopToolExecutor) Execute(_ context.Context, call askg.ToolCall, _ askg.CallerScope) askg.ToolResult {
	return askg.ToolResult{
		CallID:  call.ID,
		Result:  map[string]string{"error": "no tools configured in test"},
		IsError: true,
	}
}

func (n *noopToolExecutor) Tools() []askg.ToolDef {
	return nil
}

// setupAgentRouter creates a Gin engine with the agent controller wired in,
// using the given FakeProvider. Auth middleware sets the userid in the context.
func setupAgentRouter(fp *provider.FakeProvider, userID string) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	orch := newTestOrchestrator(fp)

	// Simulate auth middleware setting the userid
	authMiddleware := func(c *gin.Context) {
		c.Set("userid", userID)
		c.Next()
	}

	NewAgentController(r, orch, authMiddleware)

	return r
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

func TestAgentController_Ask_ValidQuestion_ReturnsSSEResult(t *testing.T) {
	fp := provider.NewFakeProvider(
		provider.FakeResponse{
			StopReason: askg.StopFinalMessage,
			Content:    `{"outcome":"answer","answer":"The domain example.best is active and in good standing."}`,
		},
	)

	r := setupAgentRouter(fp, "test-user@example.com")

	body := `{"question":"What is the status of example.best?"}`
	req := httptest.NewRequest(http.MethodPost, "/agent/ask", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "text/event-stream", w.Header().Get("Content-Type"))
	assert.Equal(t, "no-cache", w.Header().Get("Cache-Control"))
	assert.Equal(t, "keep-alive", w.Header().Get("Connection"))

	// Parse the SSE response
	responseBody := w.Body.String()
	require.Contains(t, responseBody, "event: result\n")
	require.Contains(t, responseBody, "data: ")

	// Extract the JSON data from the SSE event
	dataLine := extractSSEData(responseBody, "result")
	require.NotEmpty(t, dataLine, "expected an SSE data line for the result event")

	var result askg.Result
	err := json.Unmarshal([]byte(dataLine), &result)
	require.NoError(t, err, "failed to unmarshal SSE result data")

	assert.Equal(t, askg.OutcomeAnswer, result.Outcome)
	assert.Contains(t, result.Answer, "example.best")
	assert.Equal(t, 1, result.Iterations)
}

func TestAgentController_Ask_EmptyQuestion_Returns400(t *testing.T) {
	fp := provider.NewFakeProvider() // no responses needed — should fail before calling provider

	r := setupAgentRouter(fp, "test-user")

	body := `{"question":""}`
	req := httptest.NewRequest(http.MethodPost, "/agent/ask", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var errResp map[string]string
	err := json.Unmarshal(w.Body.Bytes(), &errResp)
	require.NoError(t, err)
	assert.Contains(t, errResp["error"], "question is required")
}

func TestAgentController_Ask_NoBody_Returns400(t *testing.T) {
	fp := provider.NewFakeProvider()

	r := setupAgentRouter(fp, "test-user")

	req := httptest.NewRequest(http.MethodPost, "/agent/ask", nil)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var errResp map[string]string
	err := json.Unmarshal(w.Body.Bytes(), &errResp)
	require.NoError(t, err)
	assert.Contains(t, errResp["error"], "failed to parse request body")
}

func TestAgentController_Ask_MissingQuestionField_Returns400(t *testing.T) {
	fp := provider.NewFakeProvider()

	r := setupAgentRouter(fp, "test-user")

	body := `{"query":"this is not the right field"}`
	req := httptest.NewRequest(http.MethodPost, "/agent/ask", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	// The JSON will parse fine (ShouldBindJSON doesn't require specific fields),
	// but the question will be empty, triggering the empty-question check.
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestAgentController_Ask_CallerScopeUserID(t *testing.T) {
	// Verify the userid from auth middleware flows into the orchestrator.
	// We use a FakeProvider that returns a direct answer — the userid
	// won't appear in the response, but we verify it doesn't crash
	// and that the SSE event is sent successfully.
	expectedUserID := "auth0|abc123"

	fp := provider.NewFakeProvider(
		provider.FakeResponse{
			StopReason: askg.StopFinalMessage,
			Content:    `{"outcome":"answer","answer":"Test answer for caller scope verification."}`,
		},
	)

	r := setupAgentRouter(fp, expectedUserID)

	body := `{"question":"Test question"}`
	req := httptest.NewRequest(http.MethodPost, "/agent/ask", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "text/event-stream", w.Header().Get("Content-Type"))

	// Verify a result event was sent (the orchestrator received the call
	// without error, meaning CallerScope was constructed properly)
	responseBody := w.Body.String()
	assert.Contains(t, responseBody, "event: result\n")

	dataLine := extractSSEData(responseBody, "result")
	var result askg.Result
	err := json.Unmarshal([]byte(dataLine), &result)
	require.NoError(t, err)
	assert.Equal(t, askg.OutcomeAnswer, result.Outcome)
}

func TestAgentController_Ask_EscalationOutcome(t *testing.T) {
	// Verify escalation outcomes are properly streamed via SSE.
	fp := provider.NewFakeProvider(
		provider.FakeResponse{
			StopReason: askg.StopFinalMessage,
			Content:    `{"outcome":"escalate","reason":"I cannot determine the answer from available data."}`,
		},
	)

	r := setupAgentRouter(fp, "test-user")

	body := `{"question":"What is the meaning of life?"}`
	req := httptest.NewRequest(http.MethodPost, "/agent/ask", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	dataLine := extractSSEData(w.Body.String(), "result")
	var result askg.Result
	err := json.Unmarshal([]byte(dataLine), &result)
	require.NoError(t, err)
	assert.Equal(t, askg.OutcomeEscalate, result.Outcome)
	assert.Contains(t, result.Reason, "cannot determine")
}

// ---------------------------------------------------------------------------
// SSE parsing helper
// ---------------------------------------------------------------------------

// extractSSEData parses a raw SSE response body and returns the data line
// associated with the given event name.
func extractSSEData(body, eventName string) string {
	lines := strings.Split(body, "\n")
	for i, line := range lines {
		if line == "event: "+eventName {
			// The next line should be the data line
			if i+1 < len(lines) {
				dataLine := lines[i+1]
				if strings.HasPrefix(dataLine, "data: ") {
					return strings.TrimPrefix(dataLine, "data: ")
				}
			}
		}
	}
	return ""
}
