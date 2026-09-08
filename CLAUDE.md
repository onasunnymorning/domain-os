# CLAUDE.md

## Architectural invariants

The architectural rules for this repository live in **[`docs/INVARIANTS.md`](docs/INVARIANTS.md)**.

Read that file before proposing structural changes. Do not restate its rules here — a second copy drifts, and a drifted agent-facing copy is worse than none.

Cite invariants by ID in review and in commit messages, e.g. "this violates INV-03".

| ID | Section |
|---|---|
| `INV-01` … `INV-16` | Invariants — confirmed rules. The only IDs citable as rules. |

IDs are never reused. **The `PROP-nn` proposal space is fully retired** — all ten were answered on 2026-08-10 and 2026-08-25 (six promoted to `INV-07`…`INV-13`, four dropped); no `PROP` ID is live or citable, and the disposition table in `docs/INVARIANTS.md` records where each went. `INV-02` has left the Unresolved list: it is resolved by [ADR-0006](docs/adr/0006-tenancy-model.md) and is now a Class B invariant. **All three `UNR` IDs are retired and the Unresolved section is empty**: `UNR-01` → `INV-14` (domain layer does not import inward, superseding the looser "dependency-free" wording), `UNR-02` → `INV-15` (typed context keys), and `UNR-03` resolved by correcting `architecture.md`, `stack.md` and `.cursorrules`.

Three things to know about CI:

- **Architectural rules are enforced.** The `arch-lint` job runs `.golangci.arch.yml` and fails the build on any violation of `INV-01`, `INV-03`, `INV-07`, `INV-12` (workflows), `INV-14` or `INV-15`. `INV-06` is enforced by an architecture test. Deliberate exceptions are allowlisted by filename in that config with a note saying what will remove them — do not add to it without one.
- **General lint findings are enforced too.** The `Lint` job runs `.golangci.yml` and fails the build. It was advisory until #411 cleared the backlog. Two conventions came out of that: silence a finding at the site with its reason (`//nolint:<linter> // why`, or `#nosec Gxxx -- why`) and never a bare `nolint`; and if a whole rule does not fit this codebase, argue it once in `.golangci.yml` rather than suppressing it at dozens of call sites. The config records the reasoning for every rule it disables, including what would make it worth turning back on.
- **Secrets are scanned, but only on new commits.** The `Secret Scan` job runs `gitleaks` over the commits a push or PR introduces, against [`.gitleaks.toml`](.gitleaks.toml). It cannot see what is already in the tree or in history — that is how three live credentials sat in this public repo for over a year before #411 found them with `gosec` G101. Use `make secrets` for the whole tree. Anything `make secrets-history` reports has to be **rotated**, not edited away.

Run the gates locally before pushing:

```bash
make ci-arch          # architecture gate only — fast
make ci-lint          # the general lint pass, also blocking
make secrets          # whole working tree, not just the diff CI sees
make ci-local         # full pipeline, mirrors GitHub Actions
```

See the enforcement assessment at the end of `docs/INVARIANTS.md` for which rules are machine-checkable.

## Other project docs

- [`docs/adr/0006-tenancy-model.md`](docs/adr/0006-tenancy-model.md) — the tenancy model behind `INV-02`. Read it before adding a tenant column, a scope parameter, or an EPP transactional verb; it carries the guardrails those changes are reviewed against.
- [`architecture.md`](architecture.md) — structural overview: layers, entry points, patterns. Describes shape only and delegates every rule to `docs/INVARIANTS.md` by ID. Rewritten 2026-08-25; accurate as of then.
- [`stack.md`](stack.md) — technology choices. **Contains at least one claim contradicted by the code (it names a message broker that does not exist); see `UNR-03`.** Where it disagrees with `docs/INVARIANTS.md`, the latter carries the evidence.
- [`docs/adr/`](docs/adr/) — decision records.
- [`docs/RSP_TESTING_NORTH_STAR.md`](docs/RSP_TESTING_NORTH_STAR.md) — working principle (not an invariant, not enforced) for aligning new test suites to ICANN's RST v2.0 case catalog as RDAP/DNS/RDE/IDN/SRS Gateway support gets built.
