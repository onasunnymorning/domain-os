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
