# Tech Stack

Technologies actually in use in `domain-os`. Verified against `go.mod`, `frontend/package.json` and the code at the time of writing.

For architectural shape see [`architecture.md`](architecture.md); for architectural rules see [`docs/INVARIANTS.md`](docs/INVARIANTS.md).

## Backend

- **Language**: **Go** 1.26 — performance, concurrency, type safety.
- **Architecture**: Domain-Driven Design / hexagonal — see [`architecture.md`](architecture.md).
- **Web framework**: **Gin**, with CORS, Prometheus and zap logging middleware.
- **Database**: **PostgreSQL**, via **GORM** (`pgx` and `lib/pq` drivers). **SQLite** (`modernc.org/sqlite`) is used as a staging store for escrow imports, not as a production backend.
- **Workflow engine**: **Temporal** — long-running, restart-safe flows: escrow import, lifecycle sweeps, zone serial drift, FX updates, event relay.
- **Event delivery**: **transactional outbox**, not a message broker. Domain events are written to a `domain_events` table and drained by a Temporal relay workflow that archives to object storage. **There is no AMQP/RabbitMQ, Kafka, NATS, SNS or SQS anywhere in this codebase** — see `INV-01`.
- **Caching / rate limiting**: **Redis** — currently backing EPP rate limiting.
- **Object storage**: **MinIO** (S3-compatible) — escrow deposits, snapshots, event archives.
- **Authentication**: **Auth0** — JWT validation via `go-jwt-middleware/v2`, with a legacy static-token fallback.
- **Registry protocols**: **EPP** (`dotse/epp-client`, `internetstiftelsen-oss/epp-lib`), **WHOIS** (`likexian/whois`, `whois-parser`), **DNS** (`miekg/dns`).
- **AI / agents**: **Anthropic SDK** for the "Ask G" agent, and the **Model Context Protocol** Go SDK for the MCP server. Vendor LLM SDKs are confined to a single adapter package — see `INV-03`.
- **IDs**: **Snowflake** (`bwmarrin/snowflake`) for distributed ID generation.
- **CLI**: **Cobra**, plus `urfave/cli` and `kingpin` in older tools.
- **API docs**: **Swaggo** — generated OpenAPI/Swagger served from the admin API.
- **Observability**: **zap** structured logging, **Prometheus** metrics and **New Relic** APM, all wired at the composition root in `cmd/api`. Deliberately *not* on the event-delivery path — see `INV-01`.

## Frontend

- **Framework**: **Next.js 15** (App Router).
- **Library**: **React 19**.
- **Language**: **TypeScript 5**.
- **Styling**: **Tailwind CSS v4**.
- **UI components**: **Radix UI** primitives (shadcn-style composition).
- **State management**: **Zustand 5**.
- **Data fetching**: **TanStack Query 5**.
- **Forms**: **React Hook Form** + **Zod 4** validation.
- **Authentication**: **Auth0** (`@auth0/auth0-react`).
- **Testing**: **Vitest 3** + **React Testing Library**.

## Infrastructure & tooling

- **Containerization**: **Docker** / Docker Compose, with profiles for the local stack.
- **Local orchestration**: **Tilt**, plus Skaffold and Helm charts under `deploy/`.
- **Task runner**: **Makefile**. `make test` starts a throwaway PostgreSQL on port 5433 rather than reusing the dev stack.
- **CI**: GitHub Actions — tests with race detection, Go-native API integration tests against real PostgreSQL, gitleaks secret scanning, Trivy image scanning, release-please. Note `golangci-lint` runs with `--issues-exit-code=0`, so lint findings do not fail the build.
- **Go testing**: **Testify** for unit tests; **Ginkgo/Gomega** for the REST integration suites.

## Key concepts

- **Decoupled frontend and backend**, communicating over the REST admin API. The backend additionally serves EPP, WHOIS and MCP, which are not frontend-facing.
- **Interfaces at the layer boundaries** — the domain declares ports, infrastructure implements them, so storage and external clients are swappable and mockable. See `INV-14`.
