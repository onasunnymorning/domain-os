# AI Agent Integration Architecture for Domain-OS

## Executive Summary

This document outlines a comprehensive architecture for integrating a chat-driven AI agent into the Domain-OS registry system. The agent acts as an intelligent interface layer between users and the backend business logic, translating complex registry workflows into natural language interactions.

## 📋 Table of Contents

1. [Vision & Philosophy](#vision--philosophy)
2. [Current Architecture Analysis](#current-architecture-analysis)
3. [Agent Architecture Design](#agent-architecture-design)
4. [Implementation Strategies](#implementation-strategies)
5. [Use Case Examples](#use-case-examples)
6. [Technical Specifications](#technical-specifications)
7. [Security & Compliance](#security--compliance)
8. [Deployment Options](#deployment-options)
9. [Roadmap & Milestones](#roadmap--milestones)

---

## Vision & Philosophy

### Core Principles

**Backend = Source of Truth**
- All business logic remains in Go backend
- Agent never implements business rules
- Agent orchestrates existing APIs, doesn't replace them

**Frontend = Human-Friendly Workflows**
- Traditional UI for structured, repetitive tasks
- Agent UI for exploratory, complex, or multi-step workflows
- Both interfaces consume the same backend APIs

**Agent = Intelligent Orchestrator**
- Understands backend capabilities deeply
- Translates user intent to API sequences
- Provides guided workflows with validation
- Learns from common patterns and user behavior

### Problem Statement

**Business Logic ≠ Human Logic**

Domain registry operations involve:
- Complex RFC-compliant rules (EPP, DNS, DNSSEC)
- Multi-step workflows (create RO → create TLD → setup phases → accredit registrars)
- Temporal dependencies (phases, grace periods)
- Pricing calculations (base + premium + fees + FX)
- Validation constraints (domain labels, status transitions)

**Current Pain Points:**
- Users must understand internal data models
- Multi-step workflows require multiple screens
- Error recovery requires technical knowledge
- No guided assistance for complex scenarios

---

## Current Architecture Analysis

### Backend Structure (Clean Architecture)

```
domain-os/
├── internal/
│   ├── domain/          # Entities & Business Rules
│   │   └── entities/    # Phase, TLD, Domain, Registrar, etc.
│   ├── application/     # Use Cases & Orchestration
│   │   ├── services/    # Business logic services
│   │   ├── commands/    # Write operations
│   │   ├── queries/     # Read operations
│   │   └── workflows/   # Temporal workflows
│   ├── infrastructure/  # External Concerns
│   │   ├── db/postgres/ # GORM repositories
│   │   ├── broker/      # RabbitMQ event streaming
│   │   └── web/         # External API clients
│   └── interface/       # API Layer
│       └── rest/        # Gin controllers
├── cmd/
│   ├── api/ry-admin/    # Admin API (main entry)
│   ├── epp/             # EPP server
│   ├── whois/           # WHOIS server
│   └── workers/         # Background workers
└── frontend/            # Next.js 15 app
```

### Key Entities & Workflows

**Core Entities:**
- Registry Operator (RO) - Organization managing TLDs
- TLD - Top-level domain (.com, .org, etc.)
- Phase - Lifecycle stage (GA, Launch, Sunrise)
- Registrar - Accredited entity selling domains
- Domain - Registered domain name
- Contact - Contact information
- Host - Nameserver

**Common Workflows:**
1. **New TLD Setup**: RO → TLD → Phase → Prices/Fees → Accredit Registrars
2. **Domain Registration**: Validate → Check Availability → Calculate Price → Reserve → Register
3. **Phase Management**: Create → Set Policy → Configure Pricing → Activate
4. **Registrar Onboarding**: Create → Validate → Accredit to TLDs

### Existing APIs (30+ endpoints)

**REST API Structure:**
```
/registry-operators/*     # RO management
/tlds/*                   # TLD operations
/tlds/:tld/phases/*       # Phase lifecycle
/tlds/:tld/phases/:phase/prices/*   # Pricing
/tlds/:tld/phases/:phase/fees/*     # Fees
/registrars/*             # Registrar CRUD
/accreditations/*         # TLD-Registrar linking
/domains/*                # Domain operations
/contacts/*               # Contact management
/hosts/*                  # Nameserver management
/whois/*                  # WHOIS queries
```

---

## Agent Architecture Design

### Option 1: Embedded Assistant (Recommended for MVP)

**Architecture:**

```
┌─────────────────────────────────────────────────────────┐
│                    Frontend (Next.js)                   │
│  ┌──────────────────┐         ┌──────────────────────┐ │
│  │  Traditional UI  │         │   Agent Chat UI      │ │
│  │  (Tables/Forms)  │         │  (Chat Interface)    │ │
│  └────────┬─────────┘         └──────────┬───────────┘ │
│           │                               │             │
│           └───────────────┬───────────────┘             │
│                           │                             │
│                    ┌──────▼────────┐                    │
│                    │  API Client   │                    │
│                    │  (React Query)│                    │
│                    └──────┬────────┘                    │
└───────────────────────────┼─────────────────────────────┘
                            │
                    ┌───────▼────────┐
                    │   Agent API    │
                    │  (Go Service)  │
                    └───────┬────────┘
                            │
        ┌───────────────────┴────────────────────┐
        │                                        │
┌───────▼────────┐                      ┌────────▼─────────┐
│   LLM Service  │                      │  Admin API       │
│  (OpenAI/Local)│                      │  (Existing)      │
│                │                      │                  │
│ - Intent       │                      │ - TLD Service    │
│   Recognition  │                      │ - Phase Service  │
│ - Context      │                      │ - Domain Service │
│   Management   │                      │ - etc.           │
│ - Response     │                      │                  │
│   Generation   │                      └──────────────────┘
└────────────────┘
```

**Components:**

1. **Agent Chat UI** (`frontend/components/agent/`)
   - Chat interface component
   - Message history
   - Streaming responses
   - Action buttons (confirm, retry, cancel)
   - Rich media (tables, forms, diagrams)

2. **Agent API Service** (`cmd/api/agent/`)
   - New Go service (separate from admin API)
   - Intent classification
   - Context management
   - API orchestration
   - Response formatting

3. **LLM Integration** (`internal/infrastructure/llm/`)
   - Provider abstraction (OpenAI, Anthropic, local)
   - Prompt templates
   - Function calling for API operations
   - Conversation memory

4. **Knowledge Base** (`internal/agent/knowledge/`)
   - API documentation embeddings
   - Workflow templates
   - Error resolution guides
   - Best practices

### Option 2: MCP Server Integration

**Using Model Context Protocol (MCP):**

```
┌─────────────────────────────────────────┐
│         Claude Desktop / VS Code        │
│  ┌───────────────────────────────────┐  │
│  │       Chat Interface              │  │
│  └───────────────┬───────────────────┘  │
└──────────────────┼──────────────────────┘
                   │ MCP Protocol
         ┌─────────▼──────────┐
         │  Domain-OS MCP     │
         │     Server         │
         │  (Go Implementation)│
         └─────────┬──────────┘
                   │
    ┌──────────────┴─────────────┐
    │                            │
┌───▼─────────┐         ┌────────▼────────┐
│  Tools      │         │   Resources     │
│             │         │                 │
│ - create_ro │         │ - API Docs      │
│ - create_tld│         │ - Workflows     │
│ - setup_    │         │ - Schemas       │
│   phase     │         │ - Error Codes   │
│ - etc.      │         │                 │
└─────────────┘         └─────────────────┘
```

**Benefits:**
- Standards-based integration
- Works with multiple LLM clients
- Clear tool/resource separation
- Built-in security model

**MCP Tools:**

```go
// internal/agent/mcp/tools.go

type DomainOSTools struct {
    // Registry Operator Tools
    CreateRegistryOperator(name, email, url string) (*entities.RegistryOperator, error)
    ListRegistryOperators(filters map[string]string) ([]*entities.RegistryOperator, error)
    
    // TLD Tools
    CreateTLD(name, type, ryid string) (*entities.TLD, error)
    SetupTLDPhase(tld, phaseName string, policy PhasePolicy) (*entities.Phase, error)
    
    // Domain Tools
    CheckAvailability(domain string) (bool, error)
    CalculatePrice(domain, operation string) (*Quote, error)
    RegisterDomain(domain, registrar string, contacts ContactSet) (*entities.Domain, error)
    
    // Workflow Tools
    ExecuteWorkflow(workflowName string, params map[string]interface{}) (WorkflowResult, error)
}
```

### Option 3: Standalone Agent Service

**Microservice Architecture:**

```
┌──────────────┐    ┌──────────────┐    ┌──────────────┐
│   Frontend   │◄──►│ Agent Service│◄──►│  Admin API   │
│  (Chat UI)   │    │              │    │              │
└──────────────┘    │ - NLP        │    │ - Business   │
                    │ - Orchestr.  │    │   Logic      │
                    │ - State Mgmt │    │              │
                    │              │    └──────────────┘
                    │              │
                    └──────┬───────┘
                           │
                    ┌──────▼───────┐
                    │  Agent DB    │
                    │ - Contexts   │
                    │ - History    │
                    │ - Preferences│
                    └──────────────┘
```

---

## Implementation Strategies

### Strategy 1: Function Calling with OpenAI/Anthropic

**Approach:**
Use LLM's native function calling to invoke backend APIs

```typescript
// frontend/lib/agent/functions.ts

export const agentFunctions = [
  {
    name: "create_registry_operator",
    description: "Create a new registry operator organization",
    parameters: {
      type: "object",
      properties: {
        name: { type: "string", description: "Operator name" },
        email: { type: "string", description: "Contact email" },
        url: { type: "string", description: "Website URL" }
      },
      required: ["name", "email"]
    }
  },
  {
    name: "create_tld",
    description: "Create a new top-level domain",
    parameters: {
      type: "object",
      properties: {
        name: { type: "string", description: "TLD name (e.g., 'com')" },
        type: { type: "string", enum: ["generic", "country-code", "second-level"] },
        ryid: { type: "string", description: "Registry operator ID" }
      },
      required: ["name", "type", "ryid"]
    }
  },
  {
    name: "setup_ga_phase",
    description: "Set up a General Availability phase for a TLD with pricing",
    parameters: {
      type: "object",
      properties: {
        tldName: { type: "string" },
        phaseName: { type: "string", default: "ga" },
        startDate: { type: "string", format: "date-time" },
        policy: { type: "object", /* PhasePolicy schema */ },
        prices: { type: "array", /* Price schema */ }
      },
      required: ["tldName", "startDate"]
    }
  },
  // ... 20+ more functions
];
```

**System Prompt:**

```typescript
const systemPrompt = `You are a Domain Registry Operations Assistant with expert knowledge of:

BACKEND CAPABILITIES:
- Registry Operator (RO) management
- Registrar management
- TLD lifecycle and configuration
- Phase management (GA, Launch, Sunrise)
- Pricing and fee structures
- Registrar accreditation
- Domain registration workflows
- EPP protocol operations
- DNS/DNSSEC configuration

WORKFLOW KNOWLEDGE:
1. New TLD Setup:
   - Create Registry Operator (if new)
   - Create TLD with type (generic/country-code/second-level)
   - Create GA Phase with start date
   - Configure phase policy (label lengths, grace periods)
   - Set base prices (registration, renewal, transfer, restore)
   - Add optional fees (application, restore, etc.)
   - Accredit registrars to the TLD

2. Domain Registration:
   - Validate domain format
   - Check availability
   - Calculate total price (base + premium + fees)
   - Verify registrar accreditation
   - Create/link contacts
   - Create/link nameservers
   - Register domain

CONSTRAINTS:
- All dates must be in RFC3339 format
- TLD names are case-insensitive, stored lowercase
- Phase names are case-sensitive
- Prices are in CENTS (multiply dollars by 100)
- Grace periods are in DAYS, except max horizon which is in YEARS
- Label lengths: min 1-63 chars, max 1-63 chars

RESPONSE STYLE:
- Ask clarifying questions when needed
- Confirm before executing destructive operations
- Explain business logic impacts
- Provide next steps after operations
- Show relevant data in tables/lists
- Suggest best practices

When users ask to perform tasks:
1. Understand the intent
2. Gather required information
3. Explain what will happen
4. Execute the operations
5. Confirm results
6. Suggest next steps`;
```

### Strategy 2: Workflow Templates + RAG

**Combine predefined workflows with retrieval:**

```go
// internal/agent/workflows/templates.go

type WorkflowTemplate struct {
    ID          string
    Name        string
    Description string
    Steps       []WorkflowStep
    Params      []Parameter
}

var NewTLDWorkflow = WorkflowTemplate{
    ID:          "new-tld-setup",
    Name:        "New TLD Setup",
    Description: "Complete workflow to set up a new TLD from scratch",
    Params: []Parameter{
        {Name: "tldName", Type: "string", Required: true},
        {Name: "tldType", Type: "enum", Options: []string{"generic", "country-code"}},
        {Name: "ryName", Type: "string", Description: "New or existing RO"},
        {Name: "startDate", Type: "datetime"},
    },
    Steps: []WorkflowStep{
        {
            Order: 1,
            Name:  "Create or Select Registry Operator",
            API:   "POST /registry-operators OR GET /registry-operators?name={ryName}",
            OnSuccess: "Store RO.RyID for next step",
        },
        {
            Order: 2,
            Name:  "Create TLD",
            API:   "POST /tlds",
            Params: map[string]string{
                "name": "{tldName}",
                "type": "{tldType}",
                "ryid": "{RO.RyID}",
            },
        },
        {
            Order: 3,
            Name:  "Create GA Phase",
            API:   "POST /tlds/{tldName}/phases",
            Params: map[string]string{
                "name":   "ga",
                "type":   "GA",
                "starts": "{startDate}",
            },
        },
        {
            Order: 4,
            Name:  "Configure Phase Policy",
            API:   "PUT /tlds/{tldName}/phases/ga/policy",
            Params: map[string]interface{}{
                "minLabelLength": 2,
                "maxLabelLength": 63,
                // ... default policy values
            },
        },
        {
            Order: 5,
            Name:  "Set Base Prices",
            API:   "POST /tlds/{tldName}/phases/ga/prices",
            Interactive: true, // Ask user for prices
        },
    },
}
```

**RAG Knowledge Base:**

```go
// internal/agent/knowledge/embeddings.go

type KnowledgeBase struct {
    Documents []Document
    Embedder  EmbeddingService
    VectorDB  VectorStore
}

type Document struct {
    ID       string
    Type     string // "api", "workflow", "entity", "error"
    Content  string
    Metadata map[string]interface{}
}

// Example documents to embed:
var knowledgeDocuments = []Document{
    {
        ID:   "phase-policy-explanation",
        Type: "entity",
        Content: `Phase Policy Configuration:

A Phase Policy defines the rules and constraints for domain operations during a specific phase.

Key Fields:
- minLabelLength (1-63): Minimum characters in a domain label
- maxLabelLength (1-63): Maximum characters in a domain label
- addGracePeriod: Hours after creation where domain can be deleted with refund
- autoRenewGracePeriod: Hours after auto-renew where it can be reversed
- redemptionGracePeriod: Hours after deletion where domain can be restored
- transferGracePeriod: Hours after transfer completes
- renewGracePeriod: Hours after manual renewal
- pendingDeletePeriod: Hours between deletion and purge
- enableIDN: Allow international domain names
- enableDNSSEC: Enable DNSSEC for this phase

Default values align with ICANN recommendations.`,
        Metadata: map[string]interface{}{
            "entity": "PhasePolicy",
            "apis":   []string{"/tlds/:tld/phases/:phase/policy"},
        },
    },
    // ... hundreds more documents
}
```

### Strategy 3: State Machine for Complex Workflows

**Track multi-turn conversations:**

```go
// internal/agent/state/conversation.go

type ConversationState struct {
    ID          string
    UserID      string
    WorkflowID  string
    CurrentStep int
    Context     map[string]interface{}
    History     []Message
    CreatedAt   time.Time
    UpdatedAt   time.Time
}

type WorkflowExecutor struct {
    state   *ConversationState
    backend BackendClient
    llm     LLMClient
}

func (we *WorkflowExecutor) Execute(userInput string) (*Response, error) {
    // 1. Update context with user input
    we.state.History = append(we.state.History, Message{
        Role:    "user",
        Content: userInput,
    })
    
    // 2. Get current workflow step
    workflow := getWorkflow(we.state.WorkflowID)
    step := workflow.Steps[we.state.CurrentStep]
    
    // 3. Check if we have all required params for this step
    missingParams := step.GetMissingParams(we.state.Context)
    if len(missingParams) > 0 {
        return we.askForParams(missingParams), nil
    }
    
    // 4. Execute the API call
    result, err := we.backend.Call(step.API, we.state.Context)
    if err != nil {
        return we.handleError(err, step), nil
    }
    
    // 5. Store results in context
    we.state.Context[step.OutputKey] = result
    
    // 6. Move to next step or complete
    we.state.CurrentStep++
    if we.state.CurrentStep >= len(workflow.Steps) {
        return we.complete(), nil
    }
    
    // 7. Generate response for next step
    return we.nextStep(), nil
}
```

---

## Use Case Examples

### Use Case 1: Complete TLD Setup

**User:** "I need to set up a new .shop TLD for Shopify"

**Agent Flow:**

```
Agent: I'll help you set up the .shop TLD. First, let me check if Shopify 
       is already registered as a registry operator.
       
       [Calls: GET /registry-operators?name=Shopify]
       
       ✓ Found existing operator: Shopify (RYID: shopify-001)
       
       Now I'll create the TLD. What type should it be?
       - generic (gTLD) - recommended for .shop
       - country-code (ccTLD)
       - second-level (SLD)

User: generic

Agent: Perfect. When should the General Availability phase start?

User: Next Monday

Agent: [Calculates] That's 2024-10-14T00:00:00Z. 
       
       I'll now:
       1. ✓ Create TLD .shop (generic, operator: shopify-001)
       2. ✓ Create GA phase starting 2024-10-14
       3. ⏳ Configure phase policy with defaults
       4. ⏳ Set up pricing
       
       For pricing, what should the base fees be?
       - Registration (per year): ?
       - Renewal (per year): ?
       - Transfer: ?
       - Restore: ?

User: $10 for registration and renewal, $5 for transfer, $100 for restore

Agent: Got it. I'll set:
       - Registration: $10.00/year
       - Renewal: $10.00/year  
       - Transfer: $5.00
       - Restore: $100.00
       
       [Executes: POST /tlds/.shop/phases/ga/prices with converted cents]
       
       ✓ All done! Your .shop TLD is ready.
       
       Next steps:
       1. Accredit registrars to start selling domains
       2. Configure premium pricing if needed
       3. Set up DNS infrastructure
       
       Would you like me to help with any of these?
```

### Use Case 2: Troubleshooting Failed Domain Registration

**User:** "Domain registration failed with error 2306"

**Agent Flow:**

```
Agent: Error 2306 indicates "Parameter value policy error". Let me investigate.
       
       [Retrieves from knowledge base: EPP error codes]
       
       This usually means the domain doesn't meet the current phase policy.
       Which domain failed?

User: super-long-domain-name-that-exceeds-limits.shop

Agent: [Analyzes] I found the issue. Let me check the .shop phase policy.
       
       [Calls: GET /tlds/.shop/phases/ga]
       
       Current policy:
       - Min label length: 2 characters ✓
       - Max label length: 20 characters ✗
       
       Your domain label is 44 characters, but the max allowed is 20.
       
       Options:
       1. Choose a shorter domain (≤20 chars)
       2. Update the phase policy to allow longer names (requires admin)
       
       Would you like me to find available shorter alternatives?
```

### Use Case 3: Bulk Operations

**User:** "Accredit all active registrars to the new .shop TLD"

**Agent Flow:**

```
Agent: I'll accredit all active registrars to .shop. Let me check:
       
       [Calls: GET /registrars?status=active]
       
       Found 147 active registrars. This will create 147 accreditations.
       
       ⚠️  Warning: This is a bulk operation. Are you sure?
       
       [Shows preview table of first 10 registrars]

User: Yes, proceed

Agent: [Executes in batches]
       ████████████████░░░░ 80% (118/147)
       
       ✓ Complete: 147 accreditations created
       
       Summary:
       - Success: 145
       - Already accredited: 2
       - Failed: 0
       
       [Shows detailed results table]
```

---

## Technical Specifications

### Agent API Specification

```go
// cmd/api/agent/main.go

type AgentAPI struct {
    router      *gin.Engine
    llmClient   LLMClient
    adminClient *AdminAPIClient
    stateStore  StateStore
    knowledge   *KnowledgeBase
}

// Endpoints

// POST /agent/chat
// Start or continue a conversation
type ChatRequest struct {
    ConversationID string                 `json:"conversationId,omitempty"`
    Message        string                 `json:"message"`
    Context        map[string]interface{} `json:"context,omitempty"`
    Stream         bool                   `json:"stream"`
}

type ChatResponse struct {
    ConversationID string      `json:"conversationId"`
    Message        string      `json:"message"`
    Actions        []Action    `json:"actions,omitempty"`
    Suggestions    []string    `json:"suggestions,omitempty"`
    Metadata       interface{} `json:"metadata,omitempty"`
}

// POST /agent/execute
// Execute a specific action
type ExecuteRequest struct {
    ConversationID string                 `json:"conversationId"`
    ActionID       string                 `json:"actionId"`
    Params         map[string]interface{} `json:"params"`
}

// GET /agent/workflows
// List available workflow templates
type WorkflowsResponse struct {
    Workflows []WorkflowTemplate `json:"workflows"`
}

// POST /agent/workflows/{id}/start
// Start a guided workflow
type StartWorkflowRequest struct {
    Params map[string]interface{} `json:"params"`
}
```

### Frontend Chat Component

```typescript
// frontend/components/agent/AgentChat.tsx

'use client';

import { useState, useRef, useEffect } from 'react';
import { useAgent } from '@/lib/hooks/useAgent';
import { Message, Action } from '@/lib/types/agent';

export function AgentChat() {
  const [messages, setMessages] = useState<Message[]>([]);
  const [input, setInput] = useState('');
  const { sendMessage, isLoading } = useAgent();
  
  const handleSend = async () => {
    const userMessage = { role: 'user', content: input };
    setMessages(prev => [...prev, userMessage]);
    setInput('');
    
    const response = await sendMessage(input, {
      stream: true,
      onToken: (token) => {
        // Update last message with streaming tokens
      },
      onAction: (action) => {
        // Show action button
      },
      onComplete: (finalMessage) => {
        setMessages(prev => [...prev, {
          role: 'assistant',
          content: finalMessage,
        }]);
      }
    });
  };
  
  return (
    <div className="flex flex-col h-full">
      {/* Message history */}
      <div className="flex-1 overflow-y-auto p-4 space-y-4">
        {messages.map((msg, i) => (
          <MessageBubble key={i} message={msg} />
        ))}
      </div>
      
      {/* Input area */}
      <div className="border-t p-4">
        <div className="flex gap-2">
          <input
            value={input}
            onChange={(e) => setInput(e.target.value)}
            onKeyPress={(e) => e.key === 'Enter' && handleSend()}
            placeholder="Ask me anything about domain registry operations..."
            className="flex-1 px-4 py-2 border rounded-lg"
          />
          <button
            onClick={handleSend}
            disabled={isLoading}
            className="px-6 py-2 bg-primary text-white rounded-lg"
          >
            Send
          </button>
        </div>
        
        {/* Quick actions */}
        <div className="mt-3 flex gap-2">
          <button className="text-sm text-muted-foreground hover:text-foreground">
            Create TLD
          </button>
          <button className="text-sm text-muted-foreground hover:text-foreground">
            Check Availability
          </button>
          <button className="text-sm text-muted-foreground hover:text-foreground">
            Setup Phase
          </button>
        </div>
      </div>
    </div>
  );
}
```

### LLM Client Abstraction

```go
// internal/infrastructure/llm/client.go

type LLMClient interface {
    Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error)
    StreamChat(ctx context.Context, req ChatRequest) (<-chan string, error)
    Embed(ctx context.Context, text string) ([]float64, error)
}

type ChatRequest struct {
    Messages   []Message
    Functions  []Function
    MaxTokens  int
    Temp       float64
}

// OpenAI Implementation
type OpenAIClient struct {
    apiKey string
    model  string
}

func (c *OpenAIClient) Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
    // Call OpenAI API with function calling
    // Parse response
    // Return structured response
}

// Anthropic Implementation
type AnthropicClient struct {
    apiKey string
    model  string
}

// Local LLM Implementation (Ollama)
type OllamaClient struct {
    endpoint string
    model    string
}
```

---

## Security & Compliance

### Authentication & Authorization

```go
// Agent inherits existing security model

type AgentAuthMiddleware struct {
    adminAPIToken string
    userService   UserService
}

func (m *AgentAuthMiddleware) Authenticate() gin.HandlerFunc {
    return func(c *gin.Context) {
        // 1. Validate user session
        // 2. Check permissions for requested operations
        // 3. Pass user context to agent
        
        userID := c.GetHeader("X-User-ID")
        user, err := m.userService.GetUser(userID)
        if err != nil {
            c.JSON(401, gin.H{"error": "unauthorized"})
            return
        }
        
        // Attach user to context
        c.Set("user", user)
        c.Next()
    }
}
```

### Audit Logging

```go
// Log all agent actions

type AgentAuditLog struct {
    ID             string
    ConversationID string
    UserID         string
    Action         string
    APICall        string
    Request        interface{}
    Response       interface{}
    Success        bool
    Error          string
    Timestamp      time.Time
}

func (agent *AgentAPI) logAction(ctx context.Context, action AgentAuditLog) {
    // Store in database
    // Send to event stream
    // Alert on sensitive operations
}
```

### Rate Limiting

```go
// Prevent abuse

type RateLimiter struct {
    store redis.Client
}

func (rl *RateLimiter) CheckLimit(userID string) error {
    key := fmt.Sprintf("agent:ratelimit:%s", userID)
    count, _ := rl.store.Incr(key)
    if count == 1 {
        rl.store.Expire(key, time.Minute)
    }
    if count > 60 { // 60 requests per minute
        return ErrRateLimitExceeded
    }
    return nil
}
```

---

## Deployment Options

### Option A: Integrated Deployment

```yaml
# docker-compose.yml (extended)

services:
  admin-api:
    # ... existing config
  
  agent-api:
    build:
      context: .
      dockerfile: Dockerfile.agent
    environment:
      - ADMIN_API_URL=http://admin-api:8080
      - OPENAI_API_KEY=${OPENAI_API_KEY}
      - REDIS_URL=redis://redis:6379
    depends_on:
      - admin-api
      - redis
    ports:
      - "8081:8081"
  
  frontend:
    # ... existing config
    environment:
      - NEXT_PUBLIC_AGENT_API_URL=http://localhost:8081
```

### Option B: Serverless Deployment

```yaml
# deploy/serverless/agent-function.yml

service: domain-os-agent

provider:
  name: aws
  runtime: go1.x
  environment:
    ADMIN_API_URL: ${ssm:/domain-os/admin-api-url}
    OPENAI_API_KEY: ${ssm:/domain-os/openai-key}

functions:
  chat:
    handler: bin/agent
    events:
      - http:
          path: /agent/chat
          method: post
          cors: true
    timeout: 30
```

### Option C: Kubernetes Deployment

```yaml
# deploy/k8s/agent-deployment.yml

apiVersion: apps/v1
kind: Deployment
metadata:
  name: agent-api
spec:
  replicas: 3
  selector:
    matchLabels:
      app: agent-api
  template:
    metadata:
      labels:
        app: agent-api
    spec:
      containers:
      - name: agent
        image: domain-os/agent:latest
        env:
        - name: ADMIN_API_URL
          value: "http://admin-api:8080"
        - name: OPENAI_API_KEY
          valueFrom:
            secretKeyRef:
              name: agent-secrets
              key: openai-key
        resources:
          requests:
            memory: "256Mi"
            cpu: "250m"
          limits:
            memory: "512Mi"
            cpu: "500m"
```

---

## Roadmap & Milestones

### Phase 1: Foundation (Weeks 1-4)

**Milestone 1.1: Basic Agent Infrastructure**
- [ ] Create agent API service structure
- [ ] Implement LLM client abstraction
- [ ] Set up conversation state management
- [ ] Create basic frontend chat UI

**Milestone 1.2: Core Functions**
- [ ] Implement 5-10 essential functions (create RO, TLD, phase, etc.)
- [ ] Create function calling integration
- [ ] Build response formatting
- [ ] Add error handling

**Milestone 1.3: Knowledge Base**
- [ ] Extract API documentation
- [ ] Create workflow templates
- [ ] Build embedding pipeline
- [ ] Set up vector database

### Phase 2: Workflows (Weeks 5-8)

**Milestone 2.1: Workflow Engine**
- [ ] Implement workflow executor
- [ ] Create template system
- [ ] Add state persistence
- [ ] Build rollback mechanism

**Milestone 2.2: Common Workflows**
- [ ] New TLD setup workflow
- [ ] Domain registration workflow
- [ ] Registrar onboarding workflow
- [ ] Phase management workflow

**Milestone 2.3: UI Enhancements**
- [ ] Rich message formatting
- [ ] Action confirmation UI
- [ ] Progress indicators
- [ ] Error recovery flows

### Phase 3: Intelligence (Weeks 9-12)

**Milestone 3.1: RAG Implementation**
- [ ] Semantic search over docs
- [ ] Context injection
- [ ] Relevance ranking
- [ ] Cache optimization

**Milestone 3.2: Learning & Optimization**
- [ ] Track successful patterns
- [ ] Build user preference model
- [ ] Implement suggestions
- [ ] A/B test prompts

**Milestone 3.3: Advanced Features**
- [ ] Multi-step planning
- [ ] Conflict resolution
- [ ] Bulk operations
- [ ] What-if scenarios

### Phase 4: Production (Weeks 13-16)

**Milestone 4.1: Security & Compliance**
- [ ] Audit logging
- [ ] Permission checks
- [ ] Data privacy controls
- [ ] Compliance reporting

**Milestone 4.2: Monitoring & Observability**
- [ ] Metrics dashboard
- [ ] Conversation analytics
- [ ] Error tracking
- [ ] Performance optimization

**Milestone 4.3: Documentation & Training**
- [ ] User guides
- [ ] Video tutorials
- [ ] Admin documentation
- [ ] API reference

---

## Cost Analysis

### Infrastructure Costs

**LLM API Costs (OpenAI GPT-4):**
- Input: $10/1M tokens
- Output: $30/1M tokens
- Estimated: ~2,000 tokens per conversation
- 1,000 conversations/month: **~$100/month**

**Alternative: Self-hosted (Ollama):**
- GPU server: 1x NVIDIA A100
- Cloud cost: **~$1,000/month**
- Break-even: ~10,000 conversations/month

**Agent API Hosting:**
- Container: 512MB RAM, 0.5 CPU
- AWS ECS: **~$50/month**
- Or serverless: **$0.20 per 1M requests**

**Vector Database:**
- Pinecone: **$70/month** (1M vectors)
- Or Qdrant self-hosted: **~$30/month**

**Total Estimated (Cloud):** $200-300/month

### Development Costs

**Phase 1-2 (Foundation + Workflows):** 4-6 weeks
- 1 Backend Engineer (Go)
- 1 Frontend Engineer (React/Next.js)
- 0.5 ML/AI Engineer

**Phase 3-4 (Intelligence + Production):** 4-6 weeks
- Same team

**Total:** 8-12 weeks of development

---

## Conclusion

### Recommended Approach

**Start with Option 1 (Embedded Assistant) + Function Calling**

**Why:**
1. ✅ Fastest to MVP (4-6 weeks)
2. ✅ Leverages existing backend (no duplication)
3. ✅ Clear separation of concerns
4. ✅ Incremental complexity
5. ✅ Easy to iterate based on feedback

**Success Criteria:**
- 80%+ of common workflows completable via agent
- <5 second response time for simple queries
- 95%+ accuracy on API orchestration
- Positive user feedback (NPS >50)

**Evolution Path:**
1. Start: Function calling + simple prompts
2. Add: Workflow templates for common tasks
3. Enhance: RAG for deep knowledge
4. Optimize: Learn from usage patterns
5. Scale: Multi-agent collaboration

### Next Steps

1. **Week 1**: Prototype chat UI + OpenAI integration
2. **Week 2**: Implement 5 core functions + test
3. **Week 3**: Add workflow engine + 2 workflows
4. **Week 4**: User testing + iterate
5. **Week 5+**: Expand based on feedback

---

## Appendix A: Example Prompts

```
"Create a new .store TLD for Company X starting next month"
"What's the status of the .shop TLD?"
"Show me all domains expiring in the next 30 days"
"Accredit Registrar ABC to all generic TLDs"
"Calculate the price to register premium-domain.shop"
"Why did domain registration fail for test.example?"
"Set up a Sunrise phase for .brand with trademark validation"
"Show me registrars that aren't accredited to .shop yet"
"What are the current phase policies for .com?"
"Help me configure DNS for my new TLD"
```

## Appendix B: API Function Reference

See `/docs/AGENT_FUNCTIONS.md` for complete list of:
- 50+ callable functions
- Parameter schemas
- Response formats
- Error codes
- Usage examples

## Appendix C: Workflow Templates

See `/internal/agent/workflows/templates/` for:
- New TLD Setup
- Domain Registration
- Registrar Onboarding
- Phase Transition
- Bulk Operations
- DNS Configuration
- Premium Pricing Setup
- Multi-TLD Management
