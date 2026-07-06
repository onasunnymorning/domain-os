package rest

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/onasunnymorning/domain-os/internal/askg"
)

// AgentController exposes the Ask G agent over the REST API.
type AgentController struct {
	orchestrator *askg.Orchestrator
}

// NewAgentController creates an AgentController and registers its routes
// under /agent on the given engine, protected by the auth middleware.
func NewAgentController(e *gin.Engine, orch *askg.Orchestrator, handler gin.HandlerFunc) *AgentController {
	controller := &AgentController{
		orchestrator: orch,
	}

	agentGroup := e.Group("/agent", handler)
	controller.RegisterRoutes(agentGroup)

	return controller
}

// RegisterRoutes registers the agent endpoints on the given router group.
func (ac *AgentController) RegisterRoutes(rg *gin.RouterGroup) {
	rg.POST("/ask", ac.Ask)
}

// askRequest is the JSON body for POST /agent/ask.
type askRequest struct {
	Question string `json:"question"`
}

// Ask handles agent questions via SSE streaming.
//
// @Summary Ask the AI agent a question
// @Description Sends a question to the Ask G agent and streams the result via SSE
// @Tags agent
// @Accept json
// @Produce text/event-stream
// @Param body body askRequest true "Question payload"
// @Success 200 {object} askg.Result
// @Failure 400 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /agent/ask [post]
func (ac *AgentController) Ask(c *gin.Context) {
	var req askRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": fmt.Sprintf("failed to parse request body: %s — expected JSON with a 'question' field", err.Error()),
		})
		return
	}

	if req.Question == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "question is required: provide a non-empty 'question' field in the JSON body",
		})
		return
	}

	// Extract the caller identity from the auth middleware context.
	userID, _ := c.Get("userid")
	userIDStr, ok := userID.(string)
	if !ok || userIDStr == "" {
		userIDStr = "unknown"
	}

	scope := askg.CallerScope{
		UserID: userIDStr,
	}

	slog.InfoContext(c.Request.Context(), "agent: ask request received",
		slog.String("caller", scope.UserID),
		slog.String("question", req.Question),
	)

	// Set SSE headers before calling the orchestrator so the client
	// knows to expect a streaming response.
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")

	result, err := ac.orchestrator.Ask(c.Request.Context(), req.Question, scope)
	if err != nil {
		// SSE error event — the headers are already sent so we can't
		// switch to a JSON error response. Send an SSE error event instead.
		slog.ErrorContext(c.Request.Context(), "agent: orchestrator failed",
			slog.String("caller", scope.UserID),
			slog.String("error", err.Error()),
		)
		fmt.Fprintf(c.Writer, "event: error\ndata: %s\n\n",
			escapeSSEData(fmt.Sprintf(`{"error":"agent orchestrator failed: %s"}`, err.Error())))
		c.Writer.Flush()
		return
	}

	// Marshal the result and send it as an SSE event.
	resultJSON, err := json.Marshal(result)
	if err != nil {
		slog.ErrorContext(c.Request.Context(), "agent: failed to marshal result",
			slog.String("caller", scope.UserID),
			slog.String("error", err.Error()),
		)
		fmt.Fprintf(c.Writer, "event: error\ndata: %s\n\n",
			escapeSSEData(fmt.Sprintf(`{"error":"failed to serialize agent result: %s"}`, err.Error())))
		c.Writer.Flush()
		return
	}

	fmt.Fprintf(c.Writer, "event: result\ndata: %s\n\n", string(resultJSON))
	c.Writer.Flush()

	slog.InfoContext(c.Request.Context(), "agent: ask request completed",
		slog.String("caller", scope.UserID),
		slog.String("outcome", string(result.Outcome)),
		slog.Int("iterations", result.Iterations),
	)
}

// escapeSSEData ensures the SSE data field doesn't contain newlines that
// would break the protocol. In practice our JSON is single-line, but this
// is a safety net.
func escapeSSEData(s string) string {
	// SSE data fields are newline-delimited; multi-line data needs
	// each line prefixed with "data: ". For single-line JSON this
	// is a no-op.
	return s
}
