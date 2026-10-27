# CLAUDE.md

## Architectural invariants

The architectural rules for this repository live in **[`docs/INVARIANTS.md`](docs/INVARIANTS.md)**.

Read that file before proposing structural changes. Do not restate its rules here — a second copy drifts, and a drifted agent-facing copy is worse than none.

Cite invariants by ID in review and in commit messages, e.g. "this violates INV-03".

| ID | Section |
|---|---|
| `INV-01` … `INV-17` | Invariants — confirmed rules. The only IDs citable as rules. |

IDs are never reused. **The `PROP-nn` proposal space is fully retired** — all ten were answered on 2026-08-10 and 2026-08-25 (six promoted to `INV-07`…`INV-13`, four dropped); no `PROP` ID is live or citable, and the disposition table in `docs/INVARIANTS.md` records where each went. `INV-02` has left the Unresolved list: it is resolved by [ADR-0006](docs/adr/0006-tenancy-model.md) and is now a Class B invariant. **All three `UNR` IDs are retired and the Unresolved section is empty**: `UNR-01` → `INV-14` (domain layer does not import inward, superseding the looser "dependency-free" wording), `UNR-02` → `INV-15` (typed context keys), and `UNR-03` resolved by correcting `architecture.md`, `stack.md` and `.cursorrules`.

Three things to know about CI:

- **Architectural rules are enforced.** The `arch-lint` job runs `.golangci.arch.yml` and fails the build on any violation of `INV-01`, `INV-03`, `INV-07`, `INV-12` (workflows), `INV-14`, `INV-15` or `INV-17`. `INV-06` is enforced by an architecture test. Deliberate exceptions are allowlisted by filename in that config with a note saying what will remove them — do not add to it without one.
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

## Workflow: push, review, merge

CI runs the full suite on every push to a PR (about six minutes) and cancels the run it supersedes. Reviewer attention, human or Copilot, is the scarce resource, so the flow is built to spend CI freely and reviews once:

1. **Before every push**, run `make ci-arch ci-lint` (and `make secrets` if gitleaks is installed; `ci-lint-go` alone if the worktree has no `node_modules`). `make hooks` installs a pre-push hook that does this for you, once per clone, and covers every worktree. `make ci-local` is for when you want the whole pipeline locally; it is not required before a push.
2. **Open the PR as a draft** (`gh pr create --draft`). CI runs on drafts; Copilot review does not. Iterate on the draft until CI is green.
3. **Mark it ready** (`gh pr ready`) when the work is complete. That is what requests the review.
4. **Batch review fixes** into one push rather than one commit per comment. Every push is a CI run and a fresh review pass.
5. **Merge only when green.** The `main` ruleset requires every CI job to pass and the branch to be up to date, so the merge button enforces this; `gh pr merge --auto` merges the moment it is.

## Other project docs

- [`docs/adr/0006-tenancy-model.md`](docs/adr/0006-tenancy-model.md) — the tenancy model behind `INV-02`. Read it before adding a tenant column, a scope parameter, or an EPP transactional verb; it carries the guardrails those changes are reviewed against.
- [`docs/epp/ARCHITECTURE.md`](docs/epp/ARCHITECTURE.md) — EPP server: target design, what holds today, and the phased roadmap. Decisions in [ADR-0013](docs/adr/0013-epp-server-architecture.md), [ADR-0014](docs/adr/0014-epp-credential-rotation.md) and [ADR-0016](docs/adr/0016-epp-schema-validation-in-go.md) (schema validation: Go models proven against xmllint; bind a command only with its model and corpus frames), [ADR-0017](docs/adr/0017-roid-repository-id.md) (ROID repository ID: one per installation from `ROID_REPOSITORY_ID`, appended to the stored number on read, guarded against change), [ADR-0019](docs/adr/0019-epp-policies.md) (EPP policies: named limits assigned to registrars and registrar families, reloaded by every node; no per-registrar overrides); every setting and how to change it in [`docs/epp/POLICY_REFERENCE.md`](docs/epp/POLICY_REFERENCE.md); what to do when an EPP alert fires in [`docs/epp/RUNBOOK.md`](docs/epp/RUNBOOK.md). Read these before changing `internal/interface/epp` or the EPP limiter.
- [`docs/rdap/ARCHITECTURE.md`](docs/rdap/ARCHITECTURE.md) — RDAP server: target design, the phased roadmap to RST v2.0 `StandardRDAP`, and the pre-test checklist. Decisions in [ADR-0020](docs/adr/0020-rdap-server-architecture.md) (separate read-only binary, TLS on the node, one answer per lookup, contacts only inside a domain response), [ADR-0021](docs/adr/0021-rdap-publication-and-redaction.md) (the TLD's *current* policy decides what is published, field by field, before serialisation; RFC 9537 redaction) and [ADR-0022](docs/adr/0022-rdap-access-policies.md) (low rate limits per network on the shared EPP limiter; 429, never 503). The field-access matrix is [`docs/rdap/PUBLICATION_REFERENCE.md`](docs/rdap/PUBLICATION_REFERENCE.md). Read these before adding anything under `internal/interface/rdap`.
- [`architecture.md`](architecture.md) — structural overview: layers, entry points, patterns. Describes shape only and delegates every rule to `docs/INVARIANTS.md` by ID. Rewritten 2026-08-25; accurate as of then.
- [`stack.md`](stack.md) — technology choices. **Contains at least one claim contradicted by the code (it names a message broker that does not exist); see `UNR-03`.** Where it disagrees with `docs/INVARIANTS.md`, the latter carries the evidence.
- [`docs/adr/`](docs/adr/) — decision records.
- [`docs/adr/0018-where-configuration-lives.md`](docs/adr/0018-where-configuration-lives.md) — where a new setting goes: environment (bootstrap and deployment), database (business and policy, typed tables), or installation record (identity that must never change). Read it before adding a setting.
- [`docs/adr/0015-rst-conformance-test-suites.md`](docs/adr/0015-rst-conformance-test-suites.md) — how test suites for RST-covered protocols are built: one test per ICANN RST v2.0 case ID, a pinned spec release, a completeness test, and a `make rst-<protocol>` scoreboard (`make rst-epp` today). Read it before adding tests for EPP, RDAP, DNS, RDE or IDN.
