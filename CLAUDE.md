# CLAUDE.md

## Architectural invariants

The architectural rules for this repository live in **[`docs/INVARIANTS.md`](docs/INVARIANTS.md)**.

Read that file before proposing structural changes. Do not restate its rules here — a second copy drifts, and a drifted agent-facing copy is worse than none.

Cite invariants by ID in review and in commit messages, e.g. "this violates INV-03".

| ID | Section |
|---|---|
| `INV-01` … `INV-14` | Invariants — confirmed rules. The only IDs citable as rules. |
| `UNR-02` … `UNR-03` | Unresolved — known contradictions with no decision yet. If your change touches one, say so and ask rather than picking a side. |

IDs are never reused. **The `PROP-nn` proposal space is fully retired** — all ten were answered on 2026-08-10 and 2026-08-25 (six promoted to `INV-07`…`INV-13`, four dropped); no `PROP` ID is live or citable, and the disposition table in `docs/INVARIANTS.md` records where each went. `INV-02` has left the Unresolved list: it is resolved by [ADR-0006](docs/adr/0006-tenancy-model.md) and is now a Class B invariant. `UNR-01` is retired — it was promoted to `INV-14` (the domain layer does not import inward), which supersedes the looser "dependency-free" wording in `architecture.md`.

Two consequences worth knowing before you rely on CI:

- `golangci-lint` runs with `--issues-exit-code=0`, so lint findings do not fail the build.
- No import-restriction linter is configured, so layering rules are not mechanically enforced.

See the enforcement assessment at the end of `docs/INVARIANTS.md` for which rules are machine-checkable.

## Other project docs

- [`docs/adr/0006-tenancy-model.md`](docs/adr/0006-tenancy-model.md) — the tenancy model behind `INV-02`. Read it before adding a tenant column, a scope parameter, or an EPP transactional verb; it carries the guardrails those changes are reviewed against.
- [`architecture.md`](architecture.md) — structural overview: layers, entry points, patterns. Describes shape only and delegates every rule to `docs/INVARIANTS.md` by ID. Rewritten 2026-08-25; accurate as of then.
- [`stack.md`](stack.md) — technology choices. **Contains at least one claim contradicted by the code (it names a message broker that does not exist); see `UNR-03`.** Where it disagrees with `docs/INVARIANTS.md`, the latter carries the evidence.
- [`docs/adr/`](docs/adr/) — decision records.
