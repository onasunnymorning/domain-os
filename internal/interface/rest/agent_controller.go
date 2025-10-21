package rest

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/onasunnymorning/domain-os/internal/agent/service"
	"go.uber.org/zap"
)

// AgentController handles agent API endpoints
type AgentController struct {
	agentService *service.AgentService
	logger       *zap.Logger
}

// NewAgentController creates a new agent controller
func NewAgentController(agentService *service.AgentService, logger *zap.Logger) *AgentController {
	return &AgentController{
		agentService: agentService,
		logger:       logger,
	}
}

// RegisterRoutes registers agent routes
func (c *AgentController) RegisterRoutes(router *gin.RouterGroup) {
	agent := router.Group("/agent")
	{
		agent.POST("/chat", c.Chat)
		agent.POST("/chat/stream", c.ChatStream)
	}
}

// Chat godoc
// @Summary Chat with AI agent
// @Description Send a message to the AI agent and receive a response
// @Tags Agent
// @Accept json
// @Produce json
// @Param request body service.ChatRequest true "Chat request"
// @Success 200 {object} service.ChatResponse
// @Failure 400 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /agent/chat [post]
// @Security Bearer
func (c *AgentController) Chat(ctx *gin.Context) {
	var req service.ChatRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		c.logger.Error("Invalid request", zap.Error(err))
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	resp, err := c.agentService.Chat(ctx.Request.Context(), req)
	if err != nil {
		c.logger.Error("Chat error", zap.Error(err))
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, resp)
}

// ChatStream godoc
// @Summary Chat with AI agent (streaming)
// @Description Send a message to the AI agent and receive a streaming response
// @Tags Agent
// @Accept json
// @Produce text/event-stream
// @Param request body service.ChatRequest true "Chat request"
// @Success 200 {string} string "SSE stream"
// @Failure 400 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /agent/chat/stream [post]
// @Security Bearer
func (c *AgentController) ChatStream(ctx *gin.Context) {
	var req service.ChatRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		c.logger.Error("Invalid request", zap.Error(err))
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Set headers for SSE
	ctx.Header("Content-Type", "text/event-stream")
	ctx.Header("Cache-Control", "no-cache")
	ctx.Header("Connection", "keep-alive")
	ctx.Header("Transfer-Encoding", "chunked")

	// Get flusher for streaming
	flusher, ok := ctx.Writer.(http.Flusher)
	if !ok {
		c.logger.Error("Streaming not supported")
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "streaming not supported"})
		return
	}

	// Stream writer function
	writer := func(chunk string) error {
		// Write SSE format: data: <content>\n\n
		_, err := fmt.Fprintf(ctx.Writer, "data: %s\n\n", chunk)
		if err != nil {
			return err
		}
		flusher.Flush()
		return nil
	}

	// Process stream
	err := c.agentService.ChatStream(ctx.Request.Context(), req, writer)
	if err != nil {
		c.logger.Error("Stream error", zap.Error(err))
		// Send error as SSE event
		fmt.Fprintf(ctx.Writer, "data: {\"error\": \"%s\"}\n\n", err.Error())
		flusher.Flush()
		return
	}

	// Send done event
	fmt.Fprintf(ctx.Writer, "data: [DONE]\n\n")
	flusher.Flush()
}

// HealthCheck godoc
// @Summary Agent health check
// @Description Check if agent service is running
// @Tags Agent
// @Produce json
// @Success 200 {object} map[string]string
// @Router /agent/health [get]
func (c *AgentController) HealthCheck(ctx *gin.Context) {
	ctx.JSON(http.StatusOK, gin.H{
		"status":  "ok",
		"service": "agent",
	})
}
