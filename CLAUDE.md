# CLAUDE.md

## Architectural invariants

The architectural rules for this repository live in **[`docs/INVARIANTS.md`](docs/INVARIANTS.md)**.

Read that file before proposing structural changes. Do not restate its rules here — a second copy drifts, and a drifted agent-facing copy is worse than none.

Cite invariants by ID in review and in commit messages, e.g. "this violates INV-03".

| ID | Section |
|---|---|
| `INV-01` … `INV-07` | Invariants — confirmed rules |
| `PROP-01`…`PROP-03`, `PROP-05`…`PROP-10` | Proposed — **not rules.** Awaiting confirmation; do not enforce or cite as binding. |
| `UNR-01` … `UNR-03` | Unresolved — known contradictions with no decision yet. If your change touches one, say so and ask rather than picking a side. |

IDs are never reused. `PROP-04` is retired — it was promoted to `INV-07`. `INV-02` has left the Unresolved list: it is resolved by [ADR-0006](docs/adr/0006-tenancy-model.md) and is now a Class B invariant.

Two consequences worth knowing before you rely on CI:

- `golangci-lint` runs with `--issues-exit-code=0`, so lint findings do not fail the build.
- No import-restriction linter is configured, so layering rules are not mechanically enforced.

See the enforcement assessment at the end of `docs/INVARIANTS.md` for which rules are machine-checkable.

## Other project docs

- [`docs/adr/0006-tenancy-model.md`](docs/adr/0006-tenancy-model.md) — the tenancy model behind `INV-02`. Read it before adding a tenant column, a scope parameter, or an EPP transactional verb; it carries the guardrails those changes are reviewed against.
- [`architecture.md`](architecture.md) and [`stack.md`](stack.md) — narrative overview. **Both contain claims contradicted by the code; see `UNR-03`.** Where they disagree with `docs/INVARIANTS.md`, the latter carries the evidence.
- [`docs/adr/`](docs/adr/) — decision records.
