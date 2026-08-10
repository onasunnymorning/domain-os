# CLAUDE.md

## Architectural invariants

The architectural rules for this repository live in **[`docs/INVARIANTS.md`](docs/INVARIANTS.md)**.

Read that file before proposing structural changes. Do not restate its rules here — a second copy drifts, and a drifted agent-facing copy is worse than none.

Cite invariants by ID in review and in commit messages, e.g. "this violates INV-03".

| ID | Section |
|---|---|
| `INV-01` … `INV-06` | Invariants — confirmed rules |
| `PROP-01` … `PROP-10` | Proposed — **not rules.** Awaiting confirmation; do not enforce or cite as binding. |
| `UNR-01` … `UNR-03`, and `INV-02` | Unresolved — known contradictions with no decision yet. If your change touches one, say so and ask rather than picking a side. |

Two consequences worth knowing before you rely on CI:

- `golangci-lint` runs with `--issues-exit-code=0`, so lint findings do not fail the build.
- No import-restriction linter is configured, so layering rules are not mechanically enforced.

See the enforcement assessment at the end of `docs/INVARIANTS.md` for which rules are machine-checkable.

## Other project docs

- [`architecture.md`](architecture.md) and [`stack.md`](stack.md) — narrative overview. **Both contain claims contradicted by the code; see `UNR-03`.** Where they disagree with `docs/INVARIANTS.md`, the latter carries the evidence.
- [`docs/adr/`](docs/adr/) — decision records.
