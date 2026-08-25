# CLAUDE.md

## Architectural invariants

The architectural rules for this repository live in **[`docs/INVARIANTS.md`](docs/INVARIANTS.md)**.

Read that file before proposing structural changes. Do not restate its rules here — a second copy drifts, and a drifted agent-facing copy is worse than none.

Cite invariants by ID in review and in commit messages, e.g. "this violates INV-03".

| ID | Section |
|---|---|
| `INV-01` … `INV-16` | Invariants — confirmed rules. The only IDs citable as rules. |

IDs are never reused. **The `PROP-nn` proposal space is fully retired** — all ten were answered on 2026-08-10 and 2026-08-25 (six promoted to `INV-07`…`INV-13`, four dropped); no `PROP` ID is live or citable, and the disposition table in `docs/INVARIANTS.md` records where each went. `INV-02` has left the Unresolved list: it is resolved by [ADR-0006](docs/adr/0006-tenancy-model.md) and is now a Class B invariant. **All three `UNR` IDs are retired and the Unresolved section is empty**: `UNR-01` → `INV-14` (domain layer does not import inward, superseding the looser "dependency-free" wording), `UNR-02` → `INV-15` (typed context keys), and `UNR-03` resolved by correcting `architecture.md`, `stack.md` and `.cursorrules`.

Two things to know about CI:

- **Architectural rules are enforced.** The `arch-lint` job runs `.golangci.arch.yml` and fails the build on any violation of `INV-01`, `INV-03`, `INV-12` (workflows) or `INV-14`. `INV-06` is enforced by an architecture test. Deliberate exceptions are allowlisted by filename in that config with a note saying what will remove them — do not add to it without one.
- **General lint findings are advisory.** The main `golangci-lint` pass still runs with `--issues-exit-code=0` and reports ~1255 issues across seven linters, so it will not fail your build. Do not read a green Lint job as a clean bill of health.

See the enforcement assessment at the end of `docs/INVARIANTS.md` for which rules are machine-checkable.

## Other project docs

- [`docs/adr/0006-tenancy-model.md`](docs/adr/0006-tenancy-model.md) — the tenancy model behind `INV-02`. Read it before adding a tenant column, a scope parameter, or an EPP transactional verb; it carries the guardrails those changes are reviewed against.
- [`architecture.md`](architecture.md) — structural overview: layers, entry points, patterns. Describes shape only and delegates every rule to `docs/INVARIANTS.md` by ID. Rewritten 2026-08-25; accurate as of then.
- [`stack.md`](stack.md) — technology choices. **Contains at least one claim contradicted by the code (it names a message broker that does not exist); see `UNR-03`.** Where it disagrees with `docs/INVARIANTS.md`, the latter carries the evidence.
- [`docs/adr/`](docs/adr/) — decision records.
