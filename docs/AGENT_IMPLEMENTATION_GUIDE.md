# Agent Implementation Quick Start Guide

## 🚀 Getting Started in 4 Weeks

This guide provides a practical, step-by-step implementation plan for integrating an AI agent into Domain-OS.

---

## Week 1: Foundation & Prototype

### Day 1-2: Infrastructure Setup

**1. Create Agent API Service**

```bash
# Create new service structure
mkdir -p cmd/api/agent
mkdir -p internal/agent/{prompts,functions,state}
```

```go
// cmd/api/agent/main.go
package main

import (
    "github.com/gin-gonic/gin"
    "github.com/onasunnymorning/domain-os/internal/agent"
)

func main() {
    r := gin.Default()
    
    // Initialize agent service
    agentSvc := agent.NewService(
        os.Getenv("OPENAI_API_KEY"),
        os.Getenv("ADMIN_API_URL"),
    )
    
    // Routes
    r.POST("/chat", agentSvc.HandleChat)
    r.POST("/stream", agentSvc.HandleStream)
    r.GET("/conversations/:id", agentSvc.GetConversation)
    
    r.Run(":8081")
}
```

**2. Add Frontend Chat UI**

```bash
cd frontend
mkdir -p components/agent
mkdir -p lib/hooks/useAgent
```

```tsx
// frontend/components/agent/AgentDrawer.tsx
'use client';

import { useState } from 'react';
import { Sheet, SheetContent, SheetHeader, SheetTitle } from '@/components/ui/sheet';
import { Button } from '@/components/ui/button';
import { MessageSquare } from 'lucide-react';
import { AgentChat } from './AgentChat';

export function AgentDrawer() {
  const [open, setOpen] = useState(false);
  
  return (
    <>
      <Button
        onClick={() => setOpen(true)}
        className="fixed bottom-6 right-6 rounded-full w-14 h-14 shadow-lg"
      >
        <MessageSquare className="h-6 w-6" />
      </Button>
      
      <Sheet open={open} onOpenChange={setOpen}>
        <SheetContent side="right" className="w-[500px] sm:w-[600px]">
          <SheetHeader>
            <SheetTitle>Registry Assistant</SheetTitle>
          </SheetHeader>
          <AgentChat />
        </SheetContent>
      </Sheet>
    </>
  );
}
```

### Day 3-4: LLM Integration

**1. Create LLM Client**

```go
// internal/agent/llm/client.go
package llm

import (
    "context"
    openai "github.com/sashabaranov/go-openai"
)

type Client struct {
    api *openai.Client
}

func NewClient(apiKey string) *Client {
    return &Client{
        api: openai.NewClient(apiKey),
    }
}

func (c *Client) Chat(ctx context.Context, messages []openai.ChatCompletionMessage, functions []openai.FunctionDefinition) (string, error) {
    resp, err := c.api.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
        Model:     openai.GPT4,
        Messages:  messages,
        Functions: functions,
    })
    
    if err != nil {
        return "", err
    }
    
    return resp.Choices[0].Message.Content, nil
}

func (c *Client) StreamChat(ctx context.Context, messages []openai.ChatCompletionMessage) (<-chan string, error) {
    stream, err := c.api.CreateChatCompletionStream(ctx, openai.ChatCompletionRequest{
        Model:    openai.GPT4,
        Messages: messages,
        Stream:   true,
    })
    
    if err != nil {
        return nil, err
    }
    
    ch := make(chan string)
    go func() {
        defer close(ch)
        for {
            response, err := stream.Recv()
            if err != nil {
                return
            }
            ch <- response.Choices[0].Delta.Content
        }
    }()
    
    return ch, nil
}
```

**2. System Prompt**

```go
// internal/agent/prompts/system.go
package prompts

const SystemPrompt = `You are a Domain Registry Operations Assistant with expert knowledge of the Domain-OS system.

YOUR CAPABILITIES:
You can help users with:
- Creating and managing Registry Operators (ROs)
- Setting up TLDs with proper configuration
- Managing phases (GA, Launch, Sunrise)
- Configuring pricing and fees
- Accrediting registrars
- Domain operations and troubleshooting

BACKEND KNOWLEDGE:
- All prices are in CENTS (multiply dollars by 100)
- Dates must be in RFC3339 format
- TLD names are lowercase, phase names are case-sensitive
- Grace periods are in HOURS
- Label lengths: min/max between 1-63 characters

INTERACTION STYLE:
1. Understand user intent first
2. Ask for missing required information
3. Explain what will happen before executing
4. Confirm destructive operations
5. Show results clearly
6. Suggest logical next steps

IMPORTANT:
- Never make up data
- Always use function calls to interact with backend
- If unsure, ask clarifying questions
- Provide specific error messages when operations fail`
```

### Day 5: First Function Implementation

**1. Define Core Functions**

```go
// internal/agent/functions/registry_operator.go
package functions

import (
    "github.com/sashabaranov/go-openai/jsonschema"
)

var CreateRegistryOperatorFunction = openai.FunctionDefinition{
    Name:        "create_registry_operator",
    Description: "Create a new registry operator organization",
    Parameters: jsonschema.Definition{
        Type: jsonschema.Object,
        Properties: map[string]jsonschema.Definition{
            "name": {
                Type:        jsonschema.String,
                Description: "Registry operator name",
            },
            "email": {
                Type:        jsonschema.String,
                Description: "Contact email address",
            },
            "url": {
                Type:        jsonschema.String,
                Description: "Website URL (optional)",
            },
        },
        Required: []string{"name", "email"},
    },
}

var CreateTLDFunction = openai.FunctionDefinition{
    Name:        "create_tld",
    Description: "Create a new top-level domain",
    Parameters: jsonschema.Definition{
        Type: jsonschema.Object,
        Properties: map[string]jsonschema.Definition{
            "name": {
                Type:        jsonschema.String,
                Description: "TLD name without leading dot (e.g., 'com', 'shop')",
            },
            "type": {
                Type:        jsonschema.String,
                Enum:        []string{"generic", "country-code", "second-level"},
                Description: "TLD type",
            },
            "ryid": {
                Type:        jsonschema.String,
                Description: "Registry operator ID (RyID)",
            },
        },
        Required: []string{"name", "type", "ryid"},
    },
}

// Add 5-10 more core functions...
```

**2. Function Executor**

```go
// internal/agent/executor/executor.go
package executor

type Executor struct {
    adminClient *AdminAPIClient
}

func (e *Executor) ExecuteFunction(name string, args map[string]interface{}) (interface{}, error) {
    switch name {
    case "create_registry_operator":
        return e.createRegistryOperator(args)
    case "create_tld":
        return e.createTLD(args)
    // ... other functions
    default:
        return nil, fmt.Errorf("unknown function: %s", name)
    }
}

func (e *Executor) createRegistryOperator(args map[string]interface{}) (interface{}, error) {
    ro := &RegistryOperator{
        Name:  args["name"].(string),
        Email: args["email"].(string),
        URL:   getStringOrEmpty(args, "url"),
    }
    
    return e.adminClient.CreateRegistryOperator(ro)
}
```

---

## Week 2: Core Functions & Integration

### Day 6-7: Implement 10 Essential Functions

```go
// internal/agent/functions/all.go
package functions

var AllFunctions = []openai.FunctionDefinition{
    // Registry Operators
    CreateRegistryOperatorFunction,
    GetRegistryOperatorFunction,
    ListRegistryOperatorsFunction,
    
    // TLDs
    CreateTLDFunction,
    GetTLDFunction,
    UpdateTLDFunction,
    
    // Phases
    CreatePhaseFunction,
    UpdatePhasePolicyFunction,
    
    // Prices & Fees
    AddPriceFunction,
    AddFeeFunction,
    
    // Queries
    CheckDomainAvailabilityFunction,
    CalculatePriceFunction,
    
    // Accreditation
    AccreditRegistrarFunction,
    ListAccreditationsFunction,
}
```

### Day 8-9: Admin API Client

```go
// internal/agent/client/admin.go
package client

import (
    "bytes"
    "encoding/json"
    "fmt"
    "net/http"
)

type AdminAPIClient struct {
    baseURL string
    token   string
    client  *http.Client
}

func NewAdminAPIClient(baseURL, token string) *AdminAPIClient {
    return &AdminAPIClient{
        baseURL: baseURL,
        token:   token,
        client:  &http.Client{Timeout: 30 * time.Second},
    }
}

func (c *AdminAPIClient) CreateRegistryOperator(ro *RegistryOperator) (*RegistryOperator, error) {
    data, _ := json.Marshal(ro)
    req, _ := http.NewRequest("POST", c.baseURL+"/registry-operators", bytes.NewBuffer(data))
    req.Header.Set("Authorization", "Bearer "+c.token)
    req.Header.Set("Content-Type", "application/json")
    
    resp, err := c.client.Do(req)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()
    
    if resp.StatusCode != 200 && resp.StatusCode != 201 {
        var errResp ErrorResponse
        json.NewDecoder(resp.Body).Decode(&errResp)
        return nil, fmt.Errorf("API error: %s", errResp.Error)
    }
    
    var result RegistryOperator
    json.NewDecoder(resp.Body).Decode(&result)
    return &result, nil
}

// Implement all other API calls...
```

### Day 10: State Management

```go
// internal/agent/state/conversation.go
package state

import (
    "time"
    "github.com/google/uuid"
)

type ConversationStore struct {
    conversations map[string]*Conversation
    // In production: use Redis or PostgreSQL
}

type Conversation struct {
    ID        string
    UserID    string
    Messages  []Message
    Context   map[string]interface{}
    CreatedAt time.Time
    UpdatedAt time.Time
}

type Message struct {
    Role      string    // "user", "assistant", "function"
    Content   string
    Function  *FunctionCall
    Timestamp time.Time
}

func NewConversationStore() *ConversationStore {
    return &ConversationStore{
        conversations: make(map[string]*Conversation),
    }
}

func (s *ConversationStore) Create(userID string) *Conversation {
    conv := &Conversation{
        ID:        uuid.New().String(),
        UserID:    userID,
        Messages:  []Message{},
        Context:   make(map[string]interface{}),
        CreatedAt: time.Now(),
        UpdatedAt: time.Now(),
    }
    s.conversations[conv.ID] = conv
    return conv
}

func (s *ConversationStore) AddMessage(convID string, msg Message) error {
    conv, exists := s.conversations[convID]
    if !exists {
        return fmt.Errorf("conversation not found")
    }
    
    conv.Messages = append(conv.Messages, msg)
    conv.UpdatedAt = time.Now()
    return nil
}
```

---

## Week 3: Workflows & UI Polish

### Day 11-12: Workflow Templates

```go
// internal/agent/workflows/new_tld.go
package workflows

type NewTLDWorkflow struct {
    executor *executor.Executor
}

func (w *NewTLDWorkflow) Execute(ctx context.Context, params map[string]interface{}) (*WorkflowResult, error) {
    result := &WorkflowResult{Steps: []StepResult{}}
    
    // Step 1: Create or get Registry Operator
    roName := params["roName"].(string)
    ro, err := w.executor.GetOrCreateRO(roName, params)
    if err != nil {
        return nil, err
    }
    result.Steps = append(result.Steps, StepResult{
        Name:    "Create Registry Operator",
        Status:  "success",
        Data:    ro,
    })
    
    // Step 2: Create TLD
    tld, err := w.executor.CreateTLD(params["tldName"].(string), params["tldType"].(string), ro.RyID)
    if err != nil {
        return nil, err
    }
    result.Steps = append(result.Steps, StepResult{
        Name:   "Create TLD",
        Status: "success",
        Data:   tld,
    })
    
    // Step 3: Create GA Phase
    phase, err := w.executor.CreatePhase(tld.Name, "ga", params["startDate"].(string))
    if err != nil {
        return nil, err
    }
    result.Steps = append(result.Steps, StepResult{
        Name:   "Create GA Phase",
        Status: "success",
        Data:   phase,
    })
    
    // Step 4: Set default policy
    policy, err := w.executor.SetDefaultPolicy(tld.Name, "ga")
    if err != nil {
        return nil, err
    }
    result.Steps = append(result.Steps, StepResult{
        Name:   "Configure Phase Policy",
        Status: "success",
        Data:   policy,
    })
    
    return result, nil
}
```

### Day 13-14: Enhanced UI Components

```tsx
// frontend/components/agent/MessageBubble.tsx
import { Check, X, Loader2 } from 'lucide-react';

interface MessageBubbleProps {
  message: Message;
}

export function MessageBubble({ message }: MessageBubbleProps) {
  if (message.role === 'user') {
    return (
      <div className="flex justify-end">
        <div className="bg-primary text-primary-foreground rounded-lg px-4 py-2 max-w-[80%]">
          {message.content}
        </div>
      </div>
    );
  }
  
  if (message.role === 'assistant') {
    return (
      <div className="flex justify-start">
        <div className="bg-muted rounded-lg px-4 py-2 max-w-[80%]">
          <MessageContent content={message.content} />
          
          {message.actions && (
            <div className="mt-3 flex gap-2">
              {message.actions.map((action) => (
                <ActionButton key={action.id} action={action} />
              ))}
            </div>
          )}
        </div>
      </div>
    );
  }
  
  if (message.role === 'function') {
    return (
      <div className="flex justify-center">
        <div className="bg-accent rounded-lg px-4 py-2 text-sm">
          {message.loading ? (
            <div className="flex items-center gap-2">
              <Loader2 className="h-4 w-4 animate-spin" />
              <span>Executing: {message.functionName}</span>
            </div>
          ) : message.success ? (
            <div className="flex items-center gap-2 text-green-600">
              <Check className="h-4 w-4" />
              <span>Completed: {message.functionName}</span>
            </div>
          ) : (
            <div className="flex items-center gap-2 text-destructive">
              <X className="h-4 w-4" />
              <span>Failed: {message.functionName}</span>
            </div>
          )}
        </div>
      </div>
    );
  }
}
```

```tsx
// frontend/components/agent/ActionButton.tsx
import { Button } from '@/components/ui/button';

export function ActionButton({ action }: { action: Action }) {
  const [loading, setLoading] = useState(false);
  
  const handleClick = async () => {
    setLoading(true);
    try {
      await executeAction(action.id, action.params);
    } finally {
      setLoading(false);
    }
  };
  
  return (
    <Button
      size="sm"
      variant={action.variant || 'default'}
      onClick={handleClick}
      disabled={loading}
    >
      {loading ? <Loader2 className="h-3 w-3 animate-spin" /> : action.label}
    </Button>
  );
}
```

### Day 15: Agent Service Integration

```go
// internal/agent/service.go
package agent

type Service struct {
    llm       *llm.Client
    executor  *executor.Executor
    store     *state.ConversationStore
    functions []openai.FunctionDefinition
}

func NewService(openaiKey, adminAPIURL string) *Service {
    return &Service{
        llm:       llm.NewClient(openaiKey),
        executor:  executor.NewExecutor(client.NewAdminAPIClient(adminAPIURL, os.Getenv("ADMIN_TOKEN"))),
        store:     state.NewConversationStore(),
        functions: functions.AllFunctions,
    }
}

func (s *Service) HandleChat(c *gin.Context) {
    var req ChatRequest
    if err := c.BindJSON(&req); err != nil {
        c.JSON(400, gin.H{"error": err.Error()})
        return
    }
    
    // Get or create conversation
    var conv *state.Conversation
    if req.ConversationID == "" {
        conv = s.store.Create(req.UserID)
    } else {
        conv = s.store.Get(req.ConversationID)
    }
    
    // Add user message
    s.store.AddMessage(conv.ID, state.Message{
        Role:    "user",
        Content: req.Message,
    })
    
    // Build messages for LLM
    messages := s.buildMessages(conv)
    
    // Call LLM
    response, err := s.llm.Chat(c.Request.Context(), messages, s.functions)
    if err != nil {
        c.JSON(500, gin.H{"error": err.Error()})
        return
    }
    
    // Check if function call
    if response.FunctionCall != nil {
        // Execute function
        result, err := s.executor.ExecuteFunction(
            response.FunctionCall.Name,
            response.FunctionCall.Arguments,
        )
        
        // Add function message
        s.store.AddMessage(conv.ID, state.Message{
            Role:     "function",
            Content:  fmt.Sprintf("%v", result),
            Function: response.FunctionCall,
        })
        
        // Get final response from LLM
        messages = s.buildMessages(conv)
        finalResponse, _ := s.llm.Chat(c.Request.Context(), messages, nil)
        
        s.store.AddMessage(conv.ID, state.Message{
            Role:    "assistant",
            Content: finalResponse,
        })
        
        c.JSON(200, ChatResponse{
            ConversationID: conv.ID,
            Message:        finalResponse,
        })
        return
    }
    
    // Regular response
    s.store.AddMessage(conv.ID, state.Message{
        Role:    "assistant",
        Content: response.Content,
    })
    
    c.JSON(200, ChatResponse{
        ConversationID: conv.ID,
        Message:        response.Content,
    })
}
```

---

## Week 4: Testing, Polish & Documentation

### Day 16-17: Testing

```go
// internal/agent/service_test.go
package agent_test

func TestCreateTLDWorkflow(t *testing.T) {
    // Mock admin API
    mockAPI := &MockAdminAPI{}
    
    // Create service
    svc := agent.NewServiceWithMocks(mockAPI)
    
    // Test conversation
    resp, err := svc.HandleChat(context.Background(), agent.ChatRequest{
        UserID:  "test-user",
        Message: "Create a new .shop TLD for Shopify starting next week",
    })
    
    assert.NoError(t, err)
    assert.Contains(t, resp.Message, "I'll help you set up")
    
    // Verify API calls
    assert.Equal(t, 1, mockAPI.CreateTLDCalls)
    assert.Equal(t, 1, mockAPI.CreatePhaseCalls)
}
```

### Day 18: Integration & Documentation

```markdown
# Agent User Guide

## Quick Start

1. Click the chat icon in the bottom right corner
2. Type your request in natural language
3. Follow the agent's guidance
4. Confirm actions when prompted

## Example Conversations

### Creating a New TLD
```
You: Create a .store TLD for my company starting January 1st
Agent: I'll help you set up the .store TLD. First, what's your company name?
You: Store Inc
Agent: Great! I'll create the registry operator "Store Inc" and set up .store as a generic TLD...
```

### Checking Domain Availability
```
You: Is premium-domain.shop available?
Agent: Let me check... Yes, premium-domain.shop is available!
      
      Registration price: $10.00/year
      
      Would you like to register it?
```
```

### Day 19-20: Deployment

```yaml
# docker-compose.yml
version: '3.8'

services:
  agent-api:
    build:
      context: .
      dockerfile: Dockerfile.agent
    ports:
      - "8081:8081"
    environment:
      - OPENAI_API_KEY=${OPENAI_API_KEY}
      - ADMIN_API_URL=http://admin-api:8080
      - ADMIN_TOKEN=${ADMIN_TOKEN}
      - GIN_MODE=release
    depends_on:
      - admin-api
    networks:
      - domain-os
```

```dockerfile
# Dockerfile.agent
FROM golang:1.21-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o agent-api ./cmd/api/agent

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=builder /app/agent-api .
EXPOSE 8081
CMD ["./agent-api"]
```

---

## Production Checklist

### Security
- [ ] Implement proper authentication
- [ ] Add rate limiting
- [ ] Audit log all agent actions
- [ ] Sanitize LLM responses
- [ ] Validate function arguments
- [ ] Add RBAC for sensitive operations

### Monitoring
- [ ] Track conversation metrics
- [ ] Monitor LLM token usage
- [ ] Alert on high error rates
- [ ] Dashboard for agent performance
- [ ] User satisfaction tracking

### Performance
- [ ] Cache common responses
- [ ] Implement streaming for long responses
- [ ] Optimize function execution
- [ ] Add response timeout handling
- [ ] Load test with concurrent users

### User Experience
- [ ] Add typing indicators
- [ ] Show function execution progress
- [ ] Provide undo/rollback options
- [ ] Add conversation history
- [ ] Enable conversation export

---

## Cost Optimization

### Reduce LLM Costs
1. **Cache responses** for common queries
2. **Use GPT-3.5** for simple tasks, GPT-4 for complex
3. **Implement prompt optimization** to reduce tokens
4. **Add response streaming** to show progress faster
5. **Consider local LLM** for high-volume scenarios

### Example Caching Strategy

```go
type ResponseCache struct {
    cache *redis.Client
}

func (rc *ResponseCache) Get(query string) (string, bool) {
    // Hash query for cache key
    key := fmt.Sprintf("agent:cache:%s", hashQuery(query))
    
    val, err := rc.cache.Get(context.Background(), key).Result()
    if err != nil {
        return "", false
    }
    
    return val, true
}

func (rc *ResponseCache) Set(query, response string) {
    key := fmt.Sprintf("agent:cache:%s", hashQuery(query))
    rc.cache.Set(context.Background(), key, response, 24*time.Hour)
}
```

---

## Troubleshooting

### Common Issues

**LLM not calling functions:**
- Check function definitions match OpenAI schema
- Verify function descriptions are clear
- Add examples in system prompt

**Slow responses:**
- Enable streaming
- Reduce context window
- Cache common queries
- Optimize function execution

**Incorrect results:**
- Improve system prompt clarity
- Add validation to function arguments
- Implement multi-step confirmation
- Add error recovery prompts

---

## Next Features (Beyond MVP)

1. **Multi-turn workflows** with state persistence
2. **Voice interface** for hands-free operation
3. **Scheduled tasks** ("Create phase next Monday")
4. **Learning from corrections** (RLHF)
5. **Multi-agent collaboration** (separate agents for different domains)
6. **What-if scenarios** ("What happens if I delete this phase?")
7. **Bulk operations** with progress tracking
8. **Integration with external tools** (Slack, Teams)

---

## Resources

- [OpenAI Function Calling Guide](https://platform.openai.com/docs/guides/function-calling)
- [Building Conversational AI](https://www.deeplearning.ai/short-courses/building-conversational-ai/)
- [LangChain Documentation](https://python.langchain.com/docs/get_started/introduction)
- [Prompt Engineering Guide](https://www.promptingguide.ai/)

---

## Support

For questions or issues:
- GitHub Issues: `domain-os/issues`
- Slack: `#agent-development`
- Email: `support@domain-os.dev`
