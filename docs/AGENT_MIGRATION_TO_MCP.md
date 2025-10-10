# Migration Path: Embedded Assistant → MCP Server

## 📋 Overview

This guide provides a detailed migration path for evolving your AI agent from an **Embedded Assistant** architecture to a **Model Context Protocol (MCP) Server** architecture. This migration enables multi-client support, standards-based integration, and enhanced flexibility.

---

## 🎯 Why Migrate to MCP?

### Triggers for Migration

**You should consider migrating when:**
- ✅ Need to support multiple client applications (web, mobile, CLI, IDE)
- ✅ Want standards-based integration with various LLM clients
- ✅ Team wants to use Claude Desktop, VS Code, or other MCP-compatible tools
- ✅ Need better separation of concerns between agent and clients
- ✅ Want to decouple from specific LLM providers
- ✅ Ready to invest in a more flexible, future-proof architecture

**Don't migrate if:**
- ❌ Only have one client application
- ❌ Embedded solution working well
- ❌ No need for IDE/desktop integration
- ❌ Team bandwidth limited
- ❌ ROI not justified yet

---

## 📊 Before vs After Comparison

### Embedded Assistant (Current)

```
┌─────────────────────────────────────┐
│      Next.js Frontend (Single)     │
│                                     │
│  ┌──────────┐    ┌──────────────┐  │
│  │ TLD Page │    │ Agent Chat   │  │
│  └────┬─────┘    └──────┬───────┘  │
│       └─────────┬────────┘          │
└─────────────────┼─────────────────┘
                  │
        ┌─────────▼──────────┐
        │   Agent API (Go)   │
        │                    │
        │  - LLM Client      │
        │  - Functions       │
        │  - Chat Logic      │
        └─────────┬──────────┘
                  │
        ┌─────────▼──────────┐
        │   Admin API        │
        └────────────────────┘
```

**Characteristics:**
- Tightly coupled to Next.js frontend
- Single client support
- LLM provider locked (OpenAI)
- Chat-focused interaction

### MCP Server (After Migration)

```
┌──────────────┐  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐
│  Next.js     │  │ Claude       │  │  VS Code     │  │   CLI Tool   │
│  Web App     │  │ Desktop      │  │  Extension   │  │              │
└──────┬───────┘  └──────┬───────┘  └──────┬───────┘  └──────┬───────┘
       │                 │                 │                 │
       └─────────────────┴─────────────────┴─────────────────┘
                                │
                    MCP Protocol (JSON-RPC)
                                │
                    ┌───────────▼────────────┐
                    │  Domain-OS MCP Server  │
                    │  (Go Implementation)   │
                    │                        │
                    │  ┌──────────────────┐  │
                    │  │  MCP Handler     │  │
                    │  └────────┬─────────┘  │
                    │           │            │
                    │  ┌────────▼─────────┐  │
                    │  │  Tool Registry   │  │
                    │  │  - create_ro     │  │
                    │  │  - create_tld    │  │
                    │  │  - setup_phase   │  │
                    │  └────────┬─────────┘  │
                    │           │            │
                    │  ┌────────▼─────────┐  │
                    │  │  Resources       │  │
                    │  │  - API Docs      │  │
                    │  │  - Workflows     │  │
                    │  └────────┬─────────┘  │
                    └───────────┼────────────┘
                                │
                    ┌───────────▼────────────┐
                    │   Admin API (Existing) │
                    └────────────────────────┘
```

**Characteristics:**
- Decoupled from specific clients
- Multi-client support (web, desktop, IDE, CLI)
- LLM-agnostic (client chooses LLM)
- Tool + resource based interaction
- Standards-compliant

---

## 🗺️ Migration Path: 4 Phases

### Phase 1: Preparation (Week 1-2) - No Breaking Changes

**Goal:** Add MCP compatibility layer alongside existing architecture

#### Step 1.1: Study MCP Protocol
```bash
# Review MCP specification
https://modelcontextprotocol.io/docs

# Install MCP SDK (if using)
go get github.com/modelcontextprotocol/go-mcp
```

#### Step 1.2: Extract Function Definitions
```go
// internal/agent/mcp/tools.go
package mcp

import (
    "github.com/modelcontextprotocol/go-mcp/protocol"
)

// Convert existing functions to MCP tool format
func GetMCPTools() []protocol.Tool {
    return []protocol.Tool{
        {
            Name:        "create_registry_operator",
            Description: "Create a new registry operator organization",
            InputSchema: protocol.JSONSchema{
                Type: "object",
                Properties: map[string]interface{}{
                    "name": map[string]interface{}{
                        "type":        "string",
                        "description": "Registry operator name",
                    },
                    "email": map[string]interface{}{
                        "type":        "string",
                        "description": "Contact email",
                    },
                    "url": map[string]interface{}{
                        "type":        "string",
                        "description": "Website URL (optional)",
                    },
                },
                Required: []string{"name", "email"},
            },
        },
        // ... convert all existing functions
    }
}
```

#### Step 1.3: Create Resource Definitions
```go
// internal/agent/mcp/resources.go
package mcp

func GetMCPResources() []protocol.Resource {
    return []protocol.Resource{
        {
            URI:         "domain-os://api/docs",
            Name:        "API Documentation",
            Description: "Complete REST API reference",
            MimeType:    "text/markdown",
        },
        {
            URI:         "domain-os://workflows/new-tld",
            Name:        "New TLD Setup Workflow",
            Description: "Step-by-step guide for TLD creation",
            MimeType:    "application/json",
        },
        {
            URI:         "domain-os://schemas/phase-policy",
            Name:        "Phase Policy Schema",
            Description: "JSON schema for phase policy configuration",
            MimeType:    "application/json",
        },
    }
}
```

#### Step 1.4: Implement Resource Handlers
```go
// internal/agent/mcp/resource_handler.go
package mcp

type ResourceHandler struct {
    adminClient *client.AdminAPIClient
}

func (h *ResourceHandler) GetResource(uri string) (string, error) {
    switch uri {
    case "domain-os://api/docs":
        return h.getAPIDocs()
    case "domain-os://workflows/new-tld":
        return h.getNewTLDWorkflow()
    case "domain-os://schemas/phase-policy":
        return h.getPhasePolicySchema()
    default:
        return "", fmt.Errorf("resource not found: %s", uri)
    }
}

func (h *ResourceHandler) getAPIDocs() (string, error) {
    // Return API documentation in markdown
    return `# Domain-OS Admin API
    
## Registry Operators
- POST /registry-operators - Create new RO
- GET /registry-operators - List ROs
...
`, nil
}
```

**Deliverable:** MCP tools and resources defined, no changes to existing code

---

### Phase 2: Parallel Implementation (Week 3-4) - Add MCP Server

**Goal:** Run MCP server alongside existing Agent API

#### Step 2.1: Implement MCP Server
```go
// cmd/mcp/server/main.go
package main

import (
    "os"
    "github.com/modelcontextprotocol/go-mcp/server"
    "github.com/onasunnymorning/domain-os/internal/agent/mcp"
)

func main() {
    // Create MCP server
    mcpServer := server.NewServer(
        "domain-os",
        "1.0.0",
    )
    
    // Register tools
    tools := mcp.GetMCPTools()
    for _, tool := range tools {
        mcpServer.AddTool(tool)
    }
    
    // Register resources
    resources := mcp.GetMCPResources()
    for _, resource := range resources {
        mcpServer.AddResource(resource)
    }
    
    // Register tool execution handler
    executor := mcp.NewToolExecutor(
        os.Getenv("ADMIN_API_URL"),
        os.Getenv("ADMIN_TOKEN"),
    )
    
    mcpServer.SetToolHandler(executor.Execute)
    
    // Register resource handler
    resourceHandler := mcp.NewResourceHandler()
    mcpServer.SetResourceHandler(resourceHandler.GetResource)
    
    // Start server (stdio or HTTP transport)
    transport := os.Getenv("MCP_TRANSPORT") // "stdio" or "http"
    if transport == "stdio" {
        mcpServer.ServeStdio()
    } else {
        mcpServer.ServeHTTP(":9000")
    }
}
```

#### Step 2.2: Implement Tool Executor
```go
// internal/agent/mcp/executor.go
package mcp

type ToolExecutor struct {
    adminClient *client.AdminAPIClient
}

func NewToolExecutor(apiURL, token string) *ToolExecutor {
    return &ToolExecutor{
        adminClient: client.NewAdminAPIClient(apiURL, token),
    }
}

func (e *ToolExecutor) Execute(toolName string, arguments map[string]interface{}) (interface{}, error) {
    switch toolName {
    case "create_registry_operator":
        return e.createRegistryOperator(arguments)
    case "create_tld":
        return e.createTLD(arguments)
    case "create_phase":
        return e.createPhase(arguments)
    // ... all other tools
    default:
        return nil, fmt.Errorf("unknown tool: %s", toolName)
    }
}

func (e *ToolExecutor) createRegistryOperator(args map[string]interface{}) (interface{}, error) {
    ro := &entities.RegistryOperator{
        Name:  args["name"].(string),
        Email: args["email"].(string),
        URL:   getStringOrEmpty(args, "url"),
    }
    
    return e.adminClient.CreateRegistryOperator(ro)
}
```

#### Step 2.3: Create MCP Configuration
```json
// .mcp/config.json
{
  "mcpServers": {
    "domain-os": {
      "command": "/path/to/domain-os-mcp",
      "args": [],
      "env": {
        "ADMIN_API_URL": "http://localhost:8080",
        "ADMIN_TOKEN": "${ADMIN_TOKEN}",
        "MCP_TRANSPORT": "stdio"
      }
    }
  }
}
```

#### Step 2.4: Test with Claude Desktop
```bash
# Build MCP server
cd cmd/mcp/server
go build -o ~/bin/domain-os-mcp

# Configure Claude Desktop
# Add to ~/Library/Application Support/Claude/claude_desktop_config.json
{
  "mcpServers": {
    "domain-os": {
      "command": "/Users/you/bin/domain-os-mcp",
      "env": {
        "ADMIN_API_URL": "http://localhost:8080",
        "ADMIN_TOKEN": "your-token"
      }
    }
  }
}

# Restart Claude Desktop
# Test: "Use domain-os to create a new registry operator"
```

**Deliverable:** Working MCP server that can be used with Claude Desktop

---

### Phase 3: Migration (Week 5-6) - Gradual Transition

**Goal:** Migrate clients from embedded API to MCP

#### Step 3.1: Create MCP Client for Next.js Frontend

**Option A: Use Claude API with MCP**
```typescript
// frontend/lib/mcp/client.ts
import Anthropic from '@anthropic-ai/sdk';

const anthropic = new Anthropic({
  apiKey: process.env.ANTHROPIC_API_KEY,
});

export async function chatWithMCP(
  message: string,
  conversationHistory: Message[] = []
) {
  const response = await anthropic.messages.create({
    model: 'claude-3-5-sonnet-20241022',
    max_tokens: 1024,
    tools: [
      {
        type: 'mcp',
        mcp_server: {
          name: 'domain-os',
          transport: {
            type: 'http',
            url: 'http://localhost:9000/mcp'
          }
        }
      }
    ],
    messages: [
      ...conversationHistory,
      { role: 'user', content: message }
    ],
  });
  
  return response;
}
```

**Option B: Keep Using OpenAI, Proxy Through MCP Tools**
```typescript
// frontend/lib/mcp/openai-mcp-bridge.ts
import OpenAI from 'openai';

const openai = new OpenAI({
  apiKey: process.env.OPENAI_API_KEY,
});

// Fetch MCP tools and convert to OpenAI format
async function getMCPToolsAsOpenAI() {
  const mcpTools = await fetch('http://localhost:9000/tools').then(r => r.json());
  
  return mcpTools.map(tool => ({
    type: 'function',
    function: {
      name: tool.name,
      description: tool.description,
      parameters: tool.inputSchema,
    }
  }));
}

export async function chatWithMCPBridge(message: string) {
  const tools = await getMCPToolsAsOpenAI();
  
  const response = await openai.chat.completions.create({
    model: 'gpt-4',
    messages: [{ role: 'user', content: message }],
    tools: tools,
  });
  
  // If function call, execute via MCP
  if (response.choices[0].message.tool_calls) {
    const toolCall = response.choices[0].message.tool_calls[0];
    
    const result = await fetch('http://localhost:9000/execute', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        tool: toolCall.function.name,
        arguments: JSON.parse(toolCall.function.arguments),
      }),
    }).then(r => r.json());
    
    // Continue conversation with result
    // ...
  }
  
  return response;
}
```

#### Step 3.2: Update Frontend to Use MCP
```typescript
// frontend/lib/hooks/useAgent.ts
import { useState } from 'react';
import { chatWithMCPBridge } from '@/lib/mcp/openai-mcp-bridge';

export function useAgent() {
  const [isLoading, setIsLoading] = useState(false);
  
  const sendMessage = async (message: string) => {
    setIsLoading(true);
    try {
      const response = await chatWithMCPBridge(message);
      return response;
    } finally {
      setIsLoading(false);
    }
  };
  
  return { sendMessage, isLoading };
}
```

#### Step 3.3: Gradual Rollout
```typescript
// Feature flag for migration
const USE_MCP = process.env.NEXT_PUBLIC_USE_MCP === 'true';

export function useAgent() {
  if (USE_MCP) {
    return useMCPAgent(); // New implementation
  } else {
    return useEmbeddedAgent(); // Old implementation
  }
}
```

**Rollout Plan:**
1. **Week 5:** 
   - Enable MCP for internal users only
   - Monitor performance and errors
   - Gather feedback

2. **Week 6:**
   - Enable for 25% of users (canary)
   - Compare metrics with embedded version
   - Fix any issues

**Deliverable:** Frontend successfully using MCP server

---

### Phase 4: Cleanup (Week 7-8) - Remove Old Code

**Goal:** Remove embedded agent API, complete migration

#### Step 4.1: Verify All Clients Migrated
```bash
# Check usage metrics
# Ensure old Agent API has zero traffic

# Verify new MCP server handling 100% of requests
curl http://localhost:9000/metrics | grep request_count
```

#### Step 4.2: Remove Embedded Agent API
```bash
# Remove old agent API service
rm -rf cmd/api/agent

# Remove old agent-specific code
rm -rf internal/agent/llm     # LLM client (now in clients)
rm -rf internal/agent/service # Old service layer
```

#### Step 4.3: Consolidate Code
```bash
# Keep shared code
internal/agent/
├── mcp/              # MCP server implementation
│   ├── server.go
│   ├── tools.go
│   ├── resources.go
│   └── executor.go
├── client/           # Admin API client (shared)
└── functions/        # Function implementations (shared)
```

#### Step 4.4: Update Documentation
```markdown
# Update all docs to reference MCP
- README.md
- AGENT_ARCHITECTURE.md
- AGENT_IMPLEMENTATION_GUIDE.md
- API documentation
```

#### Step 4.5: Update Deployment
```yaml
# docker-compose.yml
services:
  # Remove old agent-api service
  # agent-api:
  #   ...
  
  # Keep MCP server
  mcp-server:
    build:
      context: .
      dockerfile: Dockerfile.mcp
    ports:
      - "9000:9000"
    environment:
      - ADMIN_API_URL=http://admin-api:8080
      - ADMIN_TOKEN=${ADMIN_TOKEN}
      - MCP_TRANSPORT=http
```

**Deliverable:** Clean codebase with only MCP server

---

## 🔄 Side-by-Side Comparison

### Code Volume Reduction

**Before (Embedded):**
```
cmd/api/agent/           # 500 lines
internal/agent/
  ├── llm/              # 300 lines (OpenAI client)
  ├── service/          # 800 lines (chat logic)
  ├── state/            # 200 lines (conversation state)
  ├── functions/        # 1000 lines (function defs)
  └── client/           # 400 lines (Admin API client)
Total: ~3,200 lines
```

**After (MCP):**
```
cmd/mcp/server/         # 200 lines
internal/agent/mcp/
  ├── server.go         # 150 lines
  ├── tools.go          # 400 lines (tool defs)
  ├── resources.go      # 200 lines (resource defs)
  └── executor.go       # 600 lines (execution logic)
internal/agent/client/  # 400 lines (shared)
Total: ~1,950 lines (40% reduction)
```

### Feature Comparison

| Feature | Embedded | MCP | Winner |
|---------|----------|-----|--------|
| Multi-client support | ❌ | ✅ | MCP |
| LLM flexibility | ❌ (OpenAI only) | ✅ (Any) | MCP |
| Standards-based | ❌ | ✅ | MCP |
| Conversation state | ✅ (Built-in) | ⚠️ (Client-managed) | Embedded |
| Streaming | ✅ | ⚠️ (Client-dependent) | Embedded |
| Setup complexity | ✅ Simple | ⚠️ Complex | Embedded |
| IDE integration | ❌ | ✅ | MCP |
| Desktop app support | ❌ | ✅ (Claude Desktop) | MCP |
| Code maintenance | ⚠️ More code | ✅ Less code | MCP |
| Future-proof | ❌ | ✅ | MCP |

---

## 💰 Migration Cost Analysis

### Development Effort

| Phase | Tasks | Time | Cost |
|-------|-------|------|------|
| **Phase 1: Prep** | Study MCP, extract functions, define tools | 2 weeks | $12k |
| **Phase 2: Parallel** | Build MCP server, test with Claude | 2 weeks | $12k |
| **Phase 3: Migration** | Update clients, gradual rollout | 2 weeks | $12k |
| **Phase 4: Cleanup** | Remove old code, documentation | 2 weeks | $12k |
| **Total** | Complete migration | **8 weeks** | **$48k** |

### Operational Cost Change

**Before (Embedded):**
- Agent API hosting: $50/month
- OpenAI API: $200/month
- Total: $250/month

**After (MCP):**
- MCP server hosting: $30/month (smaller footprint)
- LLM API: $0/month (clients pay for their LLM)
- Total: $30/month

**Savings: $220/month = $2,640/year**

---

## 🎯 Migration Checklist

### Pre-Migration
- [ ] Review MCP specification thoroughly
- [ ] Identify all current agent functions
- [ ] Document current API usage patterns
- [ ] Set up test Claude Desktop environment
- [ ] Plan rollback strategy

### Phase 1: Preparation
- [ ] Convert all functions to MCP tool format
- [ ] Define resources (docs, workflows, schemas)
- [ ] Implement resource handlers
- [ ] Create MCP configuration files
- [ ] Document MCP tools and resources

### Phase 2: Parallel Implementation
- [ ] Build MCP server binary
- [ ] Implement tool executor
- [ ] Test with Claude Desktop
- [ ] Test with HTTP transport
- [ ] Monitor performance metrics

### Phase 3: Migration
- [ ] Create MCP client for Next.js
- [ ] Implement feature flag
- [ ] Deploy to staging
- [ ] Internal testing (1 week)
- [ ] Canary rollout (25% users)
- [ ] Full rollout (100% users)

### Phase 4: Cleanup
- [ ] Verify zero traffic to old API
- [ ] Remove old agent API code
- [ ] Update all documentation
- [ ] Update deployment configs
- [ ] Archive old code (don't delete)
- [ ] Celebrate! 🎉

---

## 🚨 Risks & Mitigations

### Risk 1: MCP Specification Changes
**Impact:** Medium - Protocol updates may break compatibility  
**Mitigation:**
- Version lock MCP dependencies
- Monitor MCP changelog
- Build abstraction layer for easy updates

### Risk 2: Client Compatibility Issues
**Impact:** High - Clients may not support all MCP features  
**Mitigation:**
- Test with multiple clients early
- Document client-specific limitations
- Provide fallback mechanisms

### Risk 3: Loss of Conversation State
**Impact:** Medium - MCP is stateless, clients manage state  
**Mitigation:**
- Provide guidance on state management
- Offer optional state service
- Document best practices

### Risk 4: Performance Degradation
**Impact:** Low - Additional protocol overhead  
**Mitigation:**
- Benchmark before/after
- Optimize tool execution
- Use HTTP transport for lower latency

### Risk 5: User Confusion During Transition
**Impact:** Medium - Multiple interfaces during migration  
**Mitigation:**
- Clear communication plan
- Gradual rollout with feature flags
- Support documentation

---

## 📈 Success Metrics

### Technical Metrics
- ✅ MCP server response time: <100ms
- ✅ Tool execution success rate: >95%
- ✅ Zero regression in functionality
- ✅ Code reduction: >30%

### User Metrics
- ✅ No degradation in user satisfaction
- ✅ Successful client connections: 100%
- ✅ Error rate: <1%
- ✅ Feature parity maintained

### Business Metrics
- ✅ Migration completed on time (8 weeks)
- ✅ Cost reduction: >$200/month
- ✅ Multi-client capability unlocked
- ✅ Standards compliance achieved

---

## 🔮 Post-Migration Opportunities

### New Capabilities Unlocked

1. **IDE Integration**
   - VS Code extension with MCP
   - IntelliJ plugin
   - Cursor AI integration

2. **Desktop Applications**
   - Claude Desktop (already working)
   - Custom Electron apps
   - Raycast extensions

3. **CLI Tools**
   ```bash
   # Direct MCP usage
   domain-os-cli create tld --name shop --type generic
   ```

4. **Third-Party Integration**
   - Allow partners to build their own clients
   - Marketplace of MCP-compatible tools
   - White-label solutions

5. **Advanced Features**
   - Multi-agent collaboration (multiple MCP servers)
   - Specialized tools per domain (TLD tools, Domain tools)
   - Custom resource providers

---

## 📚 Resources

### MCP Documentation
- [Model Context Protocol Spec](https://modelcontextprotocol.io/)
- [MCP Quickstart](https://modelcontextprotocol.io/quickstart)
- [Go MCP SDK](https://github.com/modelcontextprotocol/go-mcp)

### Example Implementations
- [MCP Servers Collection](https://github.com/modelcontextprotocol/servers)
- [Filesystem MCP Server](https://github.com/modelcontextprotocol/servers/tree/main/src/filesystem)
- [Git MCP Server](https://github.com/modelcontextprotocol/servers/tree/main/src/git)

### Testing Tools
- [MCP Inspector](https://github.com/modelcontextprotocol/inspector)
- [Claude Desktop](https://claude.ai/download)
- [MCP Test Client](https://github.com/modelcontextprotocol/test-client)

---

## 🎓 Lessons Learned (From Others)

### Best Practices

1. **Start with stdio transport**
   - Simpler to implement
   - Easier to debug
   - Claude Desktop ready

2. **Version your tools**
   - Include version in tool names
   - Deprecate gradually
   - Maintain backwards compatibility

3. **Resource URIs should be stable**
   - Don't change URIs
   - Use versioned URIs if needed
   - Document URI schema

4. **Error handling is critical**
   - Return clear error messages
   - Include error codes
   - Log errors server-side

5. **Documentation is essential**
   - Document every tool
   - Provide examples
   - Keep README updated

---

## 🚀 Quick Start: Begin Migration Today

### Day 1: Study & Plan
```bash
# 1. Read MCP spec (2 hours)
open https://modelcontextprotocol.io/docs

# 2. Install Claude Desktop
open https://claude.ai/download

# 3. Review current functions
cd internal/agent/functions
ls -la
```

### Day 2-3: Build Proof of Concept
```bash
# 1. Create MCP server directory
mkdir -p cmd/mcp/server
mkdir -p internal/agent/mcp

# 2. Implement basic MCP server (copy from examples)
# 3. Convert 3-5 functions to MCP tools
# 4. Test with Claude Desktop
```

### Week 1: Complete Phase 1
- Extract all functions
- Define resources
- Test thoroughly

### Week 2+: Follow migration phases
- Build parallel MCP server
- Update clients
- Gradual rollout
- Cleanup

---

## 📞 Getting Help

### During Migration
- **Technical issues:** Check MCP Discord/GitHub
- **Architecture questions:** Review this guide
- **Implementation blockers:** Pair programming sessions
- **Testing problems:** Use MCP Inspector

### Post-Migration
- **Performance tuning:** Profile tool execution
- **Client integration:** Refer to client docs
- **New features:** MCP community examples
- **Bug fixes:** Standard debugging process

---

## ✅ Final Recommendation

**Proceed with migration when:**
1. ✅ Embedded assistant proven valuable (6+ months in production)
2. ✅ Need for multi-client support identified
3. ✅ Team comfortable with Go development
4. ✅ 8-week timeline acceptable
5. ✅ Budget approved ($48k dev + testing)

**Migration will provide:**
- 🎯 Standards-based architecture
- 🚀 Multi-client capability
- 💰 Lower operational costs
- 🔧 Easier maintenance
- 🌟 Future-proof foundation

**Start with Phase 1 (Preparation) - no risk, all learning!**

---

*Migration guide version: 1.0*  
*Last updated: October 10, 2025*  
*Estimated total migration time: 8 weeks*  
*Recommended team: 2 engineers*
