export const AGENT_ALPACA_DOC_MARKDOWN = `
# Agent Alpaca & MCP Server

## Overview

domain-os ships two complementary AI interfaces that share the same application services and type contracts:

- **Agent Alpaca (Ask Alpaca)** — A staff-facing AI support agent embedded in the admin UI sidebar. It answers domain/TLD lookups and system-behaviour questions via an in-process tool loop backed by Claude.
- **MCP Server** — A standalone Model Context Protocol server that exposes the same read-only tools over stdio or Streamable HTTP, allowing external AI clients (Claude Desktop, Cursor, etc.) to query registry state.

Both are **read-only** — they never perform mutations. If a user request implies a write operation, Agent Alpaca returns \`action_required\` with a description of what a human should do.

---

## Architecture

\`\`\`mermaid
graph TB
    subgraph "External MCP Clients"
        IDE["AI IDE / MCP Client"]
    end

    subgraph "Frontend"
        Panel["AgentPanel — sidebar chat"]
        Cards["DomainCard / TLDPricingCard / EscalateCard"]
        Panel --> Cards
    end

    subgraph "Backend API"
        AgentCtrl["POST /agent/ask — SSE endpoint"]
    end

    subgraph "Agent Alpaca Core"
        Executor["InProcessToolExecutor"]
        Prompt["System prompt + guardrails"]
        Provider["ModelProvider — Claude"]
    end

    subgraph "MCP Server"
        MCPServer["domain-os-mcp v0.3.0"]
        MCPTransport["stdio / Streamable HTTP"]
    end

    subgraph "Shared Types"
        Types["mcp.GetDomainOutput\\nmcp.GetTLDOutput\\nmcp.PhaseOutput"]
    end

    subgraph "Application Services"
        DomainSvc["DomainService"]
        TLDSvc["TLDService"]
        KBSvc["KnowledgeService — BM25"]
    end

    subgraph "Database"
        PG["PostgreSQL"]
    end

    IDE -->|"JSON-RPC"| MCPTransport --> MCPServer
    Panel -->|"SSE stream"| AgentCtrl --> Provider --> Executor
    MCPServer --> Types
    Executor --> Types
    MCPServer --> DomainSvc
    MCPServer --> TLDSvc
    Executor --> DomainSvc
    Executor --> TLDSvc
    Executor --> KBSvc
    DomainSvc --> PG
    TLDSvc --> PG
\`\`\`

---

## Shared Tool Contract

The MCP package defines the canonical output types. Agent Alpaca imports and reuses these directly — schemas never drift.

| Tool | Description | MCP Server | Agent Alpaca |
|------|-------------|:----------:|:------------:|
| \`get_domain\` | Domain registry state (EPP statuses, expiry, RGP, nameservers, registrar) | ✅ | ✅ |
| \`get_tld\` | TLD config & lifecycle (type, registry operator, DNS, phases with pricing) | ✅ | ✅ |
| \`answer_system_question\` | BM25 knowledge base search (~30 curated docs) | — | ✅ |

All tools are annotated as \`ReadOnlyHint: true\`, \`DestructiveHint: false\`, \`IdempotentHint: true\`.

---

## Agent Alpaca

### How It Works

1. User types a question in the sidebar chat panel
2. \`POST /agent/ask\` sends the question to the backend via SSE
3. The orchestrator loops (up to 10 iterations):
   - Claude generates a response or tool calls
   - \`InProcessToolExecutor\` executes tools against live DB
   - Results are fed back to Claude
4. Final JSON result streams back with outcome, answer, and evidence

### Three Outcomes

| Outcome | When | UI Treatment |
|---------|------|-------------|
| \`answer\` | Grounded in tool results with provenance | Markdown response + rich cards |
| \`escalate\` | Insufficient data, out of scope, low confidence | Amber escalation card |
| \`action_required\` | Mutation detected — routed to human | Description of required action |

### Safety Behaviors

- **Escalates** when asked about capabilities it doesn't have
- **Refuses** to reveal system prompt, tools, or API keys
- **Low-confidence gate**: KB search results below score 0.5 trigger escalation
- **Never performs mutations** — returns \`action_required\` instead

### Configuration

| Environment Variable | Default | Description |
|---------------------|---------|-------------|
| \`ANTHROPIC_API_KEY\` | — | Required to enable the feature |
| \`ANTHROPIC_BASE_URL\` | Anthropic default | Optional API base URL override |
| \`ASKG_MODEL\` | \`claude-sonnet-5\` | Model for tool-use reasoning |
| \`ASKG_CLASSIFIER_MODEL\` | \`claude-haiku-4-5\` | *(Future)* Cheaper model for intent routing |
| \`ASKG_MAX_ITERATIONS\` | \`10\` | Max tool-call loop iterations |

Agent Alpaca is **conditionally enabled** — if \`ANTHROPIC_API_KEY\` is not set, the feature is disabled and the sidebar button is hidden.

---

## MCP Server

### Transports

The MCP server supports two transports, selected via the \`MCP_TRANSPORT\` environment variable:

| Transport | Use Case | Default Port | How to Run |
|-----------|----------|:------------:|------------|
| \`stdio\` (default) | Local IDE integration (Claude Desktop, Cursor) | N/A | \`doppler run -- go run ./cmd/mcp\` |
| \`http\` | Container deployment, network-accessible | 3001 | \`MCP_TRANSPORT=http go run ./cmd/mcp\` |

### HTTP Mode Endpoints

| Endpoint | Method | Description |
|----------|--------|-------------|
| \`/mcp\` | POST | Streamable HTTP MCP protocol endpoint |
| \`/healthz\` | GET | Health check — returns \`{"status":"ok","version":"0.3.0"}\` |

### Configuration

| Environment Variable | Default | Description |
|---------------------|---------|-------------|
| \`MCP_TRANSPORT\` | \`stdio\` | Transport mode: \`stdio\` or \`http\` |
| \`MCP_PORT\` | \`3001\` | HTTP listen port (only used when \`MCP_TRANSPORT=http\`) |
| \`DB_HOST\` | — | PostgreSQL host |
| \`DB_PORT\` | — | PostgreSQL port |
| \`DB_USER\` | — | PostgreSQL user |
| \`DB_PASS\` | — | PostgreSQL password |
| \`DB_NAME\` | — | PostgreSQL database name |
| \`DB_SSLMODE\` | — | PostgreSQL SSL mode |

### Docker / Tilt

The MCP server is available in both Tilt modes:

- **Docker mode**: \`mcp-server\` service in the \`full\` profile, auto_init=False
- **Native mode**: \`mcp-server\` local_resource with HTTP transport on port 3001, auto_init=False

Start it manually from the Tilt UI when needed.

### Connecting an MCP Client

For **stdio** (e.g., Claude Desktop \`claude_desktop_config.json\`):

\`\`\`json
{
  "mcpServers": {
    "domain-os": {
      "command": "doppler",
      "args": ["run", "--", "go", "run", "./cmd/mcp"],
      "cwd": "/path/to/domain-os"
    }
  }
}
\`\`\`

For **HTTP** (network MCP clients):

\`\`\`
Endpoint: http://localhost:3001/mcp
Transport: Streamable HTTP (stateless)
\`\`\`

---

## Key Files

### Agent Alpaca (Backend)

| File | Purpose |
|------|---------|
| \`internal/askg/tools.go\` | InProcessToolExecutor — 3 tools |
| \`internal/askg/orchestrator.go\` | Agent loop — model calls + tool execution |
| \`internal/askg/prompt.go\` | System prompt with guardrails |
| \`internal/askg/provider/anthropic/anthropic.go\` | Claude adapter with prompt caching |
| \`internal/askg/config.go\` | Config struct from env vars |
| \`internal/askg/result.go\` | Output contract (Result, Evidence, CallerScope) |
| \`internal/interface/rest/agent_controller.go\` | POST /agent/ask SSE endpoint |

### Agent Alpaca (Frontend)

| File | Purpose |
|------|---------|
| \`frontend/components/agent/AgentPanel.tsx\` | Sidebar chat drawer |
| \`frontend/components/agent/DomainCard.tsx\` | Rich domain lookup card |
| \`frontend/components/agent/TLDPricingCard.tsx\` | TLD config + pricing card |
| \`frontend/components/agent/EscalateCard.tsx\` | Amber escalation card |
| \`frontend/components/agent/EvidenceDrawer.tsx\` | Collapsible provenance section |
| \`frontend/lib/api/agent.ts\` | SSE streaming client |

### MCP Server

| File | Purpose |
|------|---------|
| \`cmd/mcp/main.go\` | Entry point — dual-mode transport |
| \`internal/interface/mcp/mcp_server.go\` | Server adapter — tool registration |
| \`internal/interface/mcp/mcp_server_test.go\` | 13 unit tests |
| \`Dockerfile.mcp\` | Container build |

### Knowledge Base

| File | Purpose |
|------|---------|
| \`internal/application/services/knowledge_service.go\` | BM25 in-process search |
| \`docs/index.yaml\` | Corpus manifest — which docs are indexed |

---

## Design Decisions

### Why not route Alpaca through MCP?

Agent Alpaca calls services **in-process** rather than going through MCP transport:

1. **Latency** — Alpaca runs inside the API server; an MCP round-trip over stdio/HTTP would add unnecessary IPC overhead to every tool call during interactive chat.
2. **Superset tools** — Alpaca has \`answer_system_question\` which is internal-only and doesn't belong in the external-facing MCP toolset.
3. **Schema consistency** — Both share the same Go output types (\`mcp.GetDomainOutput\`, etc.) so schemas stay aligned without the coupling.

### Why stateless HTTP mode?

The MCP server uses \`Stateless: true\` in Streamable HTTP mode because all tools are read-only and idempotent. Each request creates a temporary session with no overhead from session management, which is ideal for a containerised tool server.
`;
