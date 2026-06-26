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
3. **Explain likely causes** — for infrastructure errors (S3, Temporal, DB), include the 2-3 most common causes (e.g., "Check that MinIO is running and MINIO_ENDPOINT is configured").
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
