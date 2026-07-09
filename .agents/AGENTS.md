# Project Rules

## Secrets & Environment Variables

- This project uses **Doppler** for secrets management. Never hardcode secrets or read `.env` files directly.
- When running Go binaries locally, prefix commands with `doppler run --` to inject environment variables (e.g. `doppler run -- go run ./cmd/askg "..."`).
- When adding a new secret (e.g. an API key), instruct the user to add it via `doppler secrets set KEY=value` — do not create `.env` files or suggest `export` commands.
- Docker Compose services receive secrets via `${VAR}` interpolation from Doppler-managed `.env` files. Do not inline secret values in `docker-compose.yml`.

## Dependency Management

- Whenever changing a dependency (Go modules, npm packages, Docker base images), always run the full CI pipeline via `make ci-local` before committing. This runs lint, backend tests, frontend tests, and security scans (govulncheck, npm audit, Trivy image scan) to ensure no known vulnerabilities are introduced.

## Workflow Documentation (Definition of Done)

Every Temporal workflow **must** have a sidecar documentation file. Documentation is not optional — it is an integral part of "done" for any workflow operation.

### When to update documentation

| Operation | Documentation Action |
|-----------|---------------------|
| **Creating** a workflow | Create a matching `<workflowName>.doc.md` sidecar file using the template in `internal/application/workflows/WORKFLOW_DOC_TEMPLATE.md` |
| **Modifying** a workflow | Update the sidecar doc to reflect changes — steps, signals, inputs, retry policies, failure modes |
| **Retiring** a workflow | Mark the sidecar doc as `Status: RETIRED` with a retirement date and migration notes |

### Documentation location & naming

- Sidecar docs live **next to** the workflow code: `internal/application/workflows/<workflowName>.doc.md`
- Template: `internal/application/workflows/WORKFLOW_DOC_TEMPLATE.md`
- The sidecar doc is the single source of truth for that workflow's behavior, architecture, and operational runbook

### What the sidecar doc must contain

1. **Metadata header** — status, queue, category, tags, owner
2. **Mermaid flow diagram** — visual representation of the workflow's step sequence
3. **Input/output contract** — Go struct definitions and JSON examples
4. **Step-by-step breakdown** — what each activity does, with timeout and retry info
5. **Signals & human-in-the-loop** — if applicable, the signal name, expected payload, and UI behavior
6. **Failure modes** — what can go wrong and how the workflow handles it
7. **Operational notes** — scheduling, monitoring, and manual intervention guidance

### Registry & Searchability

- Every workflow must also be registered in `internal/application/workflows/workflow_registry.go` with accurate `Name`, `Description`, `Tags`, and `Steps`.
- The workflow registry powers the **⌘K global search** in the UI — workflows are searchable by name, description, tags, and category. If a workflow isn't in the registry, users can't find or launch it.
- When creating or modifying a workflow, update both the sidecar doc **and** the registry entry in the same change.

## Error Handling

This is **internal tooling** — every error message must be written for developers and operators, not end users. Unhelpful errors waste debugging time.

### Requirements for all error messages

1. **State what failed** — identify the operation, phase, or component (e.g., "Failed to initialize multipart upload", not "Something went wrong").
2. **Include the underlying cause** — always propagate the original error message. In Go: `fmt.Errorf("operation X: %w", err)`. In TypeScript: include `err.message` or `err.response.data.error`.
3. **Explain likely causes** — for infrastructure errors (S3, Temporal, DB), include the 2-3 most common causes (e.g., "Check that MinIO is running and STORAGE_ENDPOINT is configured").
4. **Include diagnostic data** — add context that helps debugging: URLs, keys, IDs, HTTP status codes, hostnames. Truncate or redact secrets, but don't strip useful info.
5. **Suggest next steps** — when possible, tell the user what to check or do (e.g., "For MinIO CORS issues, ensure MINIO_API_CORS_ALLOW_ORIGIN is set").

### Backend (Go) patterns

- **Wrap errors with context**: `fmt.Errorf("PresignUploadPart(key=%s, part=%d): %w", key, partNumber, err)`
- **HTTP handlers**: Return structured JSON errors with `error` field. Include the operation name and propagated error: `gin.H{"error": "failed to init multipart upload: " + err.Error()}`
- **Never return bare `500 Internal Server Error`** without a message body.

### Frontend (TypeScript) patterns

- **API calls**: Catch errors and extract `err.response.data.error` (backend message) or `err.message` (network-level).
- **XHR/fetch to external services (S3)**: Distinguish between HTTP errors (parse status + response body) and network errors (CORS, DNS, connectivity). Include the target URL/origin.
- **S3 errors**: Parse the XML error response to extract `<Code>` and `<Message>` when available.
- **Show error context in the UI**: The FileUpload component and toast notifications should display the full error message, not a generic "Upload failed".

## UI/UX Design Principles

This is an **internal admin tool for registry operators**. Every screen should feel calm, confident, and professional — not cluttered or "demo-ware".

### Core rules

1. **Visually self-explanatory** — UI elements should communicate their purpose through layout, iconography, and hierarchy alone. Remove labels, descriptions, and helper text that merely restate what is already obvious from context.
2. **Calm, not cluttered** — Generous whitespace, breathing room between sections. Never squeeze elements together. If a layout feels dense, remove content or increase spacing — don't shrink fonts.
3. **Balanced detail** — Show enough information to be useful at a glance, but don't overwhelm. Prefer progressive disclosure (expand/collapse, hover tooltips) over dumping everything on screen.
4. **No stating the obvious** — Omit `CardDescription` text like "Manage registry operators" under a card titled "Registry Operators". If the title is clear, the description is noise.
5. **Warm, premium feel** — Use the project's sunset/desert design tokens. Subtle gradients, soft borders, and micro-animations (hover lifts, fade-ins) over hard edges and abrupt transitions.
6. **Data over decoration** — Prefer real data density (tables, counts, event streams) over placeholder illustrations or decorative charts with no actionable insight.
7. **Consistent component patterns** — Reuse shadcn/ui primitives (`Card`, `Badge`, `Button`, `Skeleton`) with the project theme. Don't introduce one-off styled components.
8. **Number Formatting** — Format large numbers (like domain counts, registrar counts) using the compact abbreviation pattern (e.g., 1.2K, 3.4M) via the `formatCompactNumber` helper in `frontend/lib/utils/numberUtils.ts` to ensure consistency and clean data presentation. Use native tooltips (`title` attribute) to expose the full exact number on hover.

## In-App Documentation (Definition of Done)

This project has a **built-in documentation portal** at `/docs` in the frontend UI. Documentation is rendered with full markdown support (GFM tables, Mermaid diagrams, syntax-highlighted code blocks, table of contents) and is **searchable via ⌘K global search**. Keeping it current is part of "done" for any significant change.

### When to update documentation

| Operation | Documentation Action |
|-----------|---------------------|
| **Adding** a significant architectural feature, strategy, or pattern | Create a new reference guide doc |
| **Modifying** a system covered by an existing doc | Update the relevant doc to reflect the changes |
| **Retiring** a system or strategy | Delete or mark the doc as superseded |

### What qualifies as "significant"

Write documentation for topics that a future developer or operator would need to understand to make informed decisions. Examples:
- Database indexing strategies, schema design rationale
- Infrastructure architecture (caching, storage tiers, event pipelines)
- Policy enforcement rules (contact data, lifecycle, access control)
- Integration patterns (S3, Temporal, external APIs)
- Performance optimization strategies and trade-offs

Do **not** create docs for trivial changes (bug fixes, UI tweaks, dependency bumps).

### How to add a new doc

1. **Content**: Create a markdown constant in `frontend/lib/constants/<docName>Doc.ts` exporting a template literal
2. **Page**: Create `frontend/app/docs/<doc-slug>/page.tsx` using the pattern in `frontend/app/docs/contact-data-policy/page.tsx`
3. **Index**: Add a card to the Reference Guides sidebar in `frontend/app/docs/page.tsx`
4. **Search**: Add an entry to the `STATIC_DOCS` array in `frontend/lib/api/search.ts` with a descriptive `name`, `description`, and `tags` — this powers ⌘K discoverability
5. **Rendering**: Use `<WorkflowDocViewer markdown={YOUR_CONSTANT} />` — it handles ToC, Mermaid diagrams, code blocks, and anchor links automatically

### Existing reference guides

| Doc | Route | Content |
|-----|-------|---------|
| Contact Data Policy | `/docs/contact-data-policy` | Enforcement levels, validation, compliance |
| PostHog Analytics | `/docs/posthog-analytics` | Event tracking, session recordings, error capture |
| Database Index Strategy | `/docs/database-index-strategy` | PostgreSQL indexing for scale, storage budgets, query optimization |
| Event Consumer Cloud | `/docs/event-consumer` | Tiered event lifecycle, relay workflows, S3 archival, pruning |

Workflow documentation (sidecar `.doc.md` files) is served automatically from the workflow registry — see the "Workflow Documentation" section above.

## PostHog Analytics (Definition of Done)

PostHog is the frontend analytics layer. Every change to event tracking, the PostHog SDK configuration, or custom events must be reflected across **three surfaces**:

### When to update PostHog documentation

| Operation | Documentation Action |
|-----------|---------------------|
| **Adding** a new `posthog.capture()` event | Add to the event inventory table in `frontend/lib/constants/posthogAnalyticsDoc.ts` |
| **Removing** an event | Remove from the event inventory table |
| **Changing** PostHog SDK config (e.g. `instrumentation-client.ts`, `next.config.ts` rewrites) | Update the Architecture and Configuration sections in the doc |
| **Adding/changing** PostHog-related environment variables | Update all three: (1) the doc, (2) the Cloud page env vars, (3) `frontend/Dockerfile`, (4) `render.yaml` |

### PostHog files to keep in sync

| File | What it contains |
|---|---|
| `frontend/instrumentation-client.ts` | PostHog SDK init (capture settings, proxy config) |
| `frontend/next.config.ts` | Reverse proxy rewrites for `/ingest` |
| `frontend/lib/constants/posthogAnalyticsDoc.ts` | In-app documentation (event inventory, architecture, config) |
| `frontend/app/cloud/page.tsx` | Cloud Infrastructure page (service card + env vars in the `Analytics (PostHog)` tab) |
| `frontend/lib/api/search.ts` | ⌘K search index (`STATIC_DOCS` entry for `posthog-analytics`) |
| `frontend/.env.local` | Local dev env vars (`NEXT_PUBLIC_POSTHOG_*`) |
| `frontend/Dockerfile` | Build args for `NEXT_PUBLIC_POSTHOG_*` |
| `render.yaml` | Render deployment env vars for `NEXT_PUBLIC_POSTHOG_*` |

## Cloud Infrastructure Page (Definition of Done)

The Cloud Infrastructure page (`frontend/app/cloud/page.tsx`) is the single source of truth for external services and environment variables. Keep it in sync:

### When to update the Cloud page

| Operation | Cloud Page Action |
|-----------|------------------|
| **Adding** a new external service (SaaS, managed DB, etc.) | Add a service card to the `services` array with name, description, icon, URL, and key items |
| **Adding** a new environment variable | Add to the appropriate category in `envVarsByCategory` — or create a new category if needed |
| **Removing** a service or env var | Remove the corresponding entry |
| **Changing** an env var's purpose or scope | Update `description` and `services` fields |

### Rules for env var entries

- Mark vars as `secret: true` if they contain credentials, API keys with write/read access, or connection strings with passwords
- `NEXT_PUBLIC_*` vars are **not** secrets (they're baked into the client bundle)
- The `services` array should list which deployment targets use the var: `'API'`, `'Frontend'`, or `'Worker'`

## Temporal Worker & Queue Architecture

The project uses Temporal for workflow orchestration with a queue-per-workload-profile design. Understanding this is critical when adding new workflows.

### Queue Assignment Rules

When creating a new workflow, assign it to the correct queue based on its workload profile:

| Queue | Use For | Key Trait |
|-------|---------|----------|
| `fast-ops` | Operator-facing, sub-minute activities (DNS checks, HTTP fetches) | Latency-sensitive — must start within seconds |
| `scheduled` | Periodic background work (event relay, sync, prune) | Tolerates 10-30s of scheduling delay |
| `heavy-batch` | Multi-hour activities (escrow import, snapshots, TLD cleanup) | Resource-intensive — must NOT block other queues |
| `lifecycle` | Domain state-machine transitions (expiry, purge, restore) | Steady hourly cadence with moderate durations |

### When Adding a New Workflow

1. **Classify the workload** — What's the expected activity duration? Is it operator-facing (latency-sensitive) or background (tolerant)? Is it scheduled or ad-hoc?
2. **Assign the correct queue** — Use the table above. Never put a multi-hour activity on `fast-ops` or `scheduled`.
3. **Register on the correct worker** — In `cmd/workers/unified/main.go`, register the workflow and activities on the worker for the chosen queue.
4. **Update the workflow registry** — In `internal/application/workflows/workflow_registry.go`, set the `Queue` field to the correct constant from `internal/infrastructure/temporal/queues.go`.
5. **If adding a new scheduled workflow** — Add it to `internal/infrastructure/bootstrap/ensure.go` with the correct queue.

### Anti-Patterns to Avoid

- **Mixing fast and slow on the same queue** — A 30-second activity and a 10-hour activity should never share a queue. The long activity starves the short one of pollers.
- **Default `worker.Options{}`** — Always specify `MaxConcurrentWorkflowTaskPollers`, `MaxConcurrentActivityTaskPollers`, and `MaxConcurrentActivityExecutionSize` explicitly.
- **Heartbeat timeout < expected queue delay** — If your activity might sit in the queue for 2 minutes waiting for a poller, a 2-minute heartbeat timeout will fire before the activity even starts. Set heartbeat timeouts with queue delay headroom.

### Reference

- Queue constants: `internal/infrastructure/temporal/queues.go`
- Worker config: `cmd/workers/unified/main.go`
- Workflow registry: `internal/application/workflows/workflow_registry.go`
- Bootstrap schedules: `internal/infrastructure/bootstrap/ensure.go`
- In-app documentation: `/docs/worker-queue-architecture`

## Deployment Contract (Definition of Done)

The file `deploy/contract.json` is the **machine-readable interface** between this application repo and any infrastructure repo that deploys domain-os. It is **generated** from `internal/config/contract.go` — never edit it by hand.

### When to regenerate

Run `make generate-contract` after any of these changes:

| Change | Why it matters |
|--------|---------------|
| Adding/removing/renaming an env var in `env_registry.go` | Infra repo may reference the var by name |
| Changing `Required` from `false` to `true` | Infra repo must now provide a value |
| Changing a `Default` value | Infra repo's assumptions about defaults may break |
| Changing a service port or health check in `contract.go` | Infra repo's probes/listeners will fail |
| Adding or removing a service/image in `contract.go` | Infra repo needs to add/remove a deployment |
| Bumping the `VERSION` file | Contract's `app_version` must match |

The CI test `TestContractDrift` (run via `make ci-envcheck`) will fail if the committed `contract.json` doesn't match what the generator produces. Fix it by running `make generate-contract`.

### What breaks the contract (for infra repo consumers)

These changes are **breaking** — they require coordinated updates in the infra repo:

- **New required env var** — infra must supply a value or deployment fails at startup
- **Removed env var** — infra should stop setting it (harmless but noisy)
- **Changed port** — infra must update service/ingress/listener config
- **Changed health check path** — infra must update readiness/liveness probes
- **Renamed or removed image** — infra must update image references
- **New service** — infra must create a new deployment/service/task

These changes are **non-breaking** — infra repo doesn't need updates:

- New optional env var with a sensible default
- Changed description text
- New default value for an optional var (existing deployments keep working)
- Version bump (infra pins to a tag, bumps at its own pace)

### Reference

- Contract source: `internal/config/contract.go` (types + `GenerateContract()`)
- Service metadata: `serviceMetaRegistry` in `internal/config/contract.go`
- Generator CLI: `cmd/tools/gencontract/main.go`
- Generated output: `deploy/contract.json`
- Drift test: `internal/config/contract_test.go`
