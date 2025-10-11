package service

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/onasunnymorning/domain-os/internal/agent/client"
	"github.com/onasunnymorning/domain-os/internal/agent/functions"
	"github.com/sashabaranov/go-openai"
	"go.uber.org/zap"
)

// AgentService handles AI agent operations
type AgentService struct {
	llmClient   *openai.Client
	adminClient *client.AdminAPIClient
	functions   *functions.Functions
	logger      *zap.Logger
}

// NewAgentService creates a new agent service
func NewAgentService(openaiAPIKey, adminAPIURL, adminAPIToken string, logger *zap.Logger) *AgentService {
	adminClient := client.NewAdminAPIClient(adminAPIURL, adminAPIToken)
	funcs := functions.NewFunctions(adminClient)

	return &AgentService{
		llmClient:   openai.NewClient(openaiAPIKey),
		adminClient: adminClient,
		functions:   funcs,
		logger:      logger,
	}
}

// ChatRequest represents a chat request from the user
type ChatRequest struct {
	Message             string    `json:"message" binding:"required"`
	ConversationID      string    `json:"conversation_id"`
	ConversationHistory []Message `json:"history"`
}

// Message represents a single message in the conversation
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// NavigationAction represents a navigation action the UI can perform
type NavigationAction struct {
	Type         string `json:"type"`         // Always "navigate"
	Label        string `json:"label"`        // Button label
	Path         string `json:"path"`         // Navigation path
	Variant      string `json:"variant"`      // Button variant: default, outline, secondary
	AutoNavigate bool   `json:"autoNavigate"` // Auto-navigate without user click
}

// ChatResponse represents the response from the agent
type ChatResponse struct {
	Message        string             `json:"message"`
	ConversationID string             `json:"conversation_id"`
	Actions        []NavigationAction `json:"actions,omitempty"`
}

// StreamWriter is a callback for streaming responses
type StreamWriter func(chunk string) error

// Chat processes a chat request and returns a response
func (s *AgentService) Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	s.logger.Info("Processing chat request",
		zap.String("message", req.Message),
		zap.String("conversation_id", req.ConversationID),
	)

	// Build conversation history
	messages := s.buildMessages(req)

	// Create chat completion request
	chatReq := openai.ChatCompletionRequest{
		Model:    openai.GPT4TurboPreview,
		Messages: messages,
		Tools:    s.functions.GetFunctionDefinitions(),
	}

	// Call OpenAI API
	resp, err := s.llmClient.CreateChatCompletion(ctx, chatReq)
	if err != nil {
		s.logger.Error("OpenAI API error", zap.Error(err))
		return nil, fmt.Errorf("failed to get LLM response: %w", err)
	}

	// Check if function calling is needed
	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("no response from LLM")
	}

	choice := resp.Choices[0]

	// Handle function calls
	if len(choice.Message.ToolCalls) > 0 {
		response, err := s.handleFunctionCalls(ctx, messages, choice.Message.ToolCalls)
		if err != nil {
			return nil, err
		}

		// Add navigation actions based on function results
		s.addNavigationActions(response, req.Message, response.Message)

		return response, nil
	}

	// Return direct response
	response := &ChatResponse{
		Message:        choice.Message.Content,
		ConversationID: req.ConversationID,
	}

	// Add navigation actions based on context
	s.addNavigationActions(response, req.Message, choice.Message.Content)

	return response, nil
}

// ChatStream processes a chat request and streams the response
func (s *AgentService) ChatStream(ctx context.Context, req ChatRequest, writer StreamWriter) error {
	s.logger.Info("Processing streaming chat request",
		zap.String("message", req.Message),
		zap.String("conversation_id", req.ConversationID),
	)

	// Build conversation history
	messages := s.buildMessages(req)

	// Create streaming chat completion request
	chatReq := openai.ChatCompletionRequest{
		Model:    openai.GPT4TurboPreview,
		Messages: messages,
		Tools:    s.functions.GetFunctionDefinitions(),
		Stream:   true,
	}

	// Call OpenAI API with streaming
	stream, err := s.llmClient.CreateChatCompletionStream(ctx, chatReq)
	if err != nil {
		s.logger.Error("OpenAI streaming API error", zap.Error(err))
		return fmt.Errorf("failed to create stream: %w", err)
	}
	defer stream.Close()

	// Track function calls for potential execution
	var toolCalls []openai.ToolCall
	var assistantMessage string

	// Process stream
	for {
		response, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			s.logger.Error("Stream error", zap.Error(err))
			return fmt.Errorf("stream error: %w", err)
		}

		if len(response.Choices) == 0 {
			continue
		}

		delta := response.Choices[0].Delta

		// Handle content streaming
		if delta.Content != "" {
			assistantMessage += delta.Content
			if err := writer(delta.Content); err != nil {
				return fmt.Errorf("writer error: %w", err)
			}
		}

		// Handle tool calls
		if len(delta.ToolCalls) > 0 {
			// Accumulate tool calls
			for _, tc := range delta.ToolCalls {
				if tc.Index != nil {
					// New tool call or update existing
					idx := *tc.Index
					if idx >= len(toolCalls) {
						toolCalls = append(toolCalls, tc)
					} else {
						// Update existing tool call
						if tc.Function.Name != "" {
							toolCalls[idx].Function.Name = tc.Function.Name
						}
						if tc.Function.Arguments != "" {
							toolCalls[idx].Function.Arguments += tc.Function.Arguments
						}
					}
				}
			}
		}
	}

	// If function calls were made, execute them
	if len(toolCalls) > 0 {
		s.logger.Info("Executing function calls", zap.Int("count", len(toolCalls)))

		// Notify user that functions are being executed
		if err := writer("\n\n_Executing functions..._\n\n"); err != nil {
			return err
		}

		// Execute functions and get final response
		finalResp, err := s.handleFunctionCalls(ctx, messages, toolCalls)
		if err != nil {
			return err
		}

		// Stream final response
		return writer(finalResp.Message)
	}

	return nil
}

// buildMessages constructs the message array for the LLM
func (s *AgentService) buildMessages(req ChatRequest) []openai.ChatCompletionMessage {
	messages := []openai.ChatCompletionMessage{
		{
			Role: openai.ChatMessageRoleSystem,
			Content: `You are Alpaca Agent, an expert AI assistant for Domain-OS, a domain registry management system. 

Your role is to help users manage registry operations including:
- Registry Operators (ROs)
- Top-level domains (TLDs)
- Phases (Sunrise, Landrush, General Availability)
- Domain registrations
- Pricing and fees

When helping users:
1. Be concise and professional
2. Ask clarifying questions if needed
3. Use function calls to perform operations
4. Confirm success or explain errors clearly
5. Format responses with markdown when helpful
6. When users ask to "show", "open", or "view" pages, I will automatically navigate them there

Available operations:
- Create and list registry operators
- Create and list TLDs
- Create and list phases
- Search domains
- Get TLD information

Navigation hints:
- When listing items, offer to take users to the relevant page
- Respond naturally to navigation requests like "show me all TLDs" or "open the domains page"

Always ensure you have required information before calling functions.
Present yourself as "Alpaca Agent" when introducing yourself.`,
		},
	}

	// Add conversation history
	for _, msg := range req.ConversationHistory {
		messages = append(messages, openai.ChatCompletionMessage{
			Role:    msg.Role,
			Content: msg.Content,
		})
	}

	// Add current message
	messages = append(messages, openai.ChatCompletionMessage{
		Role:    openai.ChatMessageRoleUser,
		Content: req.Message,
	})

	return messages
}

// handleFunctionCalls executes function calls and gets the final response
func (s *AgentService) handleFunctionCalls(
	ctx context.Context,
	messages []openai.ChatCompletionMessage,
	toolCalls []openai.ToolCall,
) (*ChatResponse, error) {
	s.logger.Info("Handling function calls", zap.Int("count", len(toolCalls)))

	// Add assistant message with tool calls to history
	assistantMsg := openai.ChatCompletionMessage{
		Role:      openai.ChatMessageRoleAssistant,
		ToolCalls: toolCalls,
	}
	messages = append(messages, assistantMsg)

	// Execute each function call
	for _, toolCall := range toolCalls {
		s.logger.Info("Executing function",
			zap.String("function", toolCall.Function.Name),
			zap.String("arguments", toolCall.Function.Arguments),
		)

		result, err := s.functions.ExecuteFunction(
			toolCall.Function.Name,
			toolCall.Function.Arguments,
		)
		if err != nil {
			s.logger.Error("Function execution error",
				zap.String("function", toolCall.Function.Name),
				zap.Error(err),
			)
			result = fmt.Sprintf("Error executing function: %s", err.Error())
		}

		// Add function result to messages
		messages = append(messages, openai.ChatCompletionMessage{
			Role:       openai.ChatMessageRoleTool,
			Content:    result,
			ToolCallID: toolCall.ID,
		})
	}

	// Get final response from LLM with function results
	finalReq := openai.ChatCompletionRequest{
		Model:    openai.GPT4TurboPreview,
		Messages: messages,
	}

	finalResp, err := s.llmClient.CreateChatCompletion(ctx, finalReq)
	if err != nil {
		return nil, fmt.Errorf("failed to get final response: %w", err)
	}

	if len(finalResp.Choices) == 0 {
		return nil, fmt.Errorf("no final response from LLM")
	}

	return &ChatResponse{
		Message: finalResp.Choices[0].Message.Content,
	}, nil
}

// addNavigationActions analyzes the conversation and adds relevant navigation actions
func (s *AgentService) addNavigationActions(response *ChatResponse, userMessage, assistantMessage string) {
	userLower := strings.ToLower(userMessage)
	assistantLower := strings.ToLower(assistantMessage)

	// Check for auto-navigation triggers (show me, open, go to, navigate to)
	autoNavigate := strings.Contains(userLower, "show me") ||
		strings.Contains(userLower, "open") ||
		strings.Contains(userLower, "go to") ||
		strings.Contains(userLower, "navigate to") ||
		strings.Contains(userLower, "take me to")

	// TLDs page navigation
	if strings.Contains(userLower, "tld") || strings.Contains(assistantLower, "tld") {
		if strings.Contains(userLower, "all tlds") || strings.Contains(userLower, "list tlds") ||
			strings.Contains(userLower, "show tlds") || strings.Contains(assistantLower, "here are") {
			response.Actions = append(response.Actions, NavigationAction{
				Type:         "navigate",
				Label:        "View All TLDs",
				Path:         "/tlds",
				Variant:      "default",
				AutoNavigate: autoNavigate,
			})
		}
	}

	// Registry Operators page navigation
	if strings.Contains(userLower, "registry operator") || strings.Contains(userLower, "operators") ||
		strings.Contains(assistantLower, "registry operator") || strings.Contains(assistantLower, "operators") {
		if strings.Contains(userLower, "all") || strings.Contains(userLower, "list") ||
			strings.Contains(userLower, "show") || strings.Contains(assistantLower, "here are") {
			response.Actions = append(response.Actions, NavigationAction{
				Type:         "navigate",
				Label:        "View All Registry Operators",
				Path:         "/registry-operators",
				Variant:      "default",
				AutoNavigate: autoNavigate,
			})
		}
	}

	// Domains page navigation
	if strings.Contains(userLower, "domain") && !strings.Contains(userLower, "tld") {
		if strings.Contains(userLower, "all domains") || strings.Contains(userLower, "list domains") ||
			strings.Contains(userLower, "show domains") {
			response.Actions = append(response.Actions, NavigationAction{
				Type:         "navigate",
				Label:        "View All Domains",
				Path:         "/domains",
				Variant:      "default",
				AutoNavigate: autoNavigate,
			})
		}
	}

	// Dashboard navigation
	if strings.Contains(userLower, "dashboard") || strings.Contains(userLower, "home") ||
		strings.Contains(userLower, "overview") {
		response.Actions = append(response.Actions, NavigationAction{
			Type:         "navigate",
			Label:        "Go to Dashboard",
			Path:         "/",
			Variant:      "default",
			AutoNavigate: autoNavigate,
		})
	}
}
