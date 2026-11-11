# ADR 0025 — Chat-driven work tracking: outcomes and sub-issues on GitHub, three shared skills

- **Status:** Proposed (a draft for review; nothing in it is implemented, and no label, skill, workflow or issue has been created)
- **Date:** 2026-11-10
- **Deciders:** Platform / Registry
- **Related:** the *Workflow: push, review, merge* section of [`CLAUDE.md`](../../CLAUDE.md) (this record stops where that begins), [ADR 0015](0015-rst-conformance-test-suites.md) (the other record that decides how work is done rather than what is built), [`docs/epp/ARCHITECTURE.md`](../epp/ARCHITECTURE.md) and [`docs/rdap/ARCHITECTURE.md`](../rdap/ARCHITECTURE.md) (the roadmap tables this record keeps)

## Context

Work on this repository is planned and done in chats: Claude Code for most implementation, ChatGPT and Codex beside it. A chat ends, and what it knew ends with it. Three things are missing:

- a place to see **what outcomes we are working towards**, above the level of one protocol;
- a way to say **"defer this"** in the middle of something and trust that it was written down;
- a way for **another chat or another agent to pick work up** without being told the story again.

GitHub issues were meant to be that place. They stopped being it.

### What the repository shows (evidence, measured 2026-11-10)

| # | Finding | Where |
|---|---|---|
| E1 | The last issue is #432, opened 2026-09-14. 110 pull requests have merged since. None carries a closing keyword for an issue. | `gh issue list`, `gh pr list --state merged` |
| E2 | 107 issues are open. 16 have an empty body, 56 have no label. Several are umbrellas with no end: "Process improvements" (#314), "Frontend improvements" (#313), "Testing improvements" (#236). | `gh issue list --state open` |
| E3 | Nothing answers "what is next": no status labels, no milestones, no issue templates. The 21 labels are GitHub's defaults plus a few added by hand. | `gh label list`, `.github/` |
| E4 | Briefs written in a chat and pasted in lost their structure. #393 and #429 are stored as HTML. #393 names four labels (`tier-a`, `docs`, `onboarding-blocker`, `agent-task`) that do not exist on the repository. #429 links to file paths on one laptop. | the issue bodies |
| E5 | Sub-issues work here and were used: #244, #229, #205 and #154 are parents. The last was early 2025. | GraphQL `subIssuesSummary` |
| E6 | Planning did not stop; it moved. Protocol work is planned in ADRs and in the roadmap tables of `docs/epp/ARCHITECTURE.md` (phases 0 to 9c, each with a status) and `docs/rdap/ARCHITECTURE.md`, and shipped as PR series. That works. | those files |
| E7 | What comes next in a series is recorded where only one tool on one machine can read it. The note that sunrise (9b) and claims (9c) follow the TMCH data lives in Claude Code's per-user memory and points to a plan file under `~/.claude/plans`. | the memory directory |
| E8 | Agent instructions are in three files that do not refer to each other: `CLAUDE.md` (current), `.agents/AGENTS.md` (334 lines of project rules) and `.cursorrules` (which still says lint findings do not fail the build; `CLAUDE.md` says they do). None says anything about issues. | those files |
| E9 | One skill exists, `.agents/skills/data-pipeline-qa/SKILL.md`, in the `name` / `description` front-matter format. | that file |
| E10 | Two accounts are logged in to `gh`. The active one cannot write to this repository and has no `project` scope. `onasunnymorning` owns the repository and has both. The repository is public and owned by a user, not an organisation. | `gh auth status`, `gh repo view` |

### What is assumed, not shown

No file, branch or commit in this repository is called an *intent*. Whatever instructions told a chat to create issues lived outside it, and this record did not go looking: the decision was to start clean. So these are guesses about cause, and the decision below should not depend on any one of them being right:

- **A1. Capturing cost more than skipping.** Writing an issue meant leaving the work, and nothing checked that it was done.
- **A2. Nothing read the issues back.** No instruction tells an agent to start from an issue (E8 shows the absence, not the effect), so issues went stale and stopped being worth writing.
- **A3. Instructions were per tool.** What one chat product was told about issues, the others never saw.
- **A4. Briefs were written for one executor in one chat** and not updated when the work went elsewhere (E4 is consistent with this, not proof).

## Decision

### 1. An issue is the handoff; a chat is disposable

Work that is not yet done lives in GitHub issues on this repository, and nowhere else: not in a chat, a plan file or a tool's memory. The test for any issue is: **could an agent with this issue and the repository, and no chat, start?**

Two things keep their place. Decisions stay in ADRs. A protocol's phase roadmap stays in its `ARCHITECTURE.md` table; an issue links to the row and does not copy it.

### 2. Outcomes are parent issues

An outcome is something a person could be shown working. It is one issue with the label `outcome`:

- **Title:** the result, not the activity.
- **Why:** who needs it and what it unblocks.
- **Done when:** a check someone can run or watch.
- **Out of scope:** what this outcome will not grow into.
- **Links:** ADRs, roadmap rows, earlier issues.

An umbrella with no "done when" (E2) is not an outcome.

### 3. Work is sub-issues, one pull request each

Each piece of an outcome is a native GitHub sub-issue (`gh issue create --parent <n>`). It is sized to one pull request, which is already how series are shipped here. Its body:

- **Context:** enough to start cold, with `file:line` pointers.
- **Done when:** the observable change.
- **Origin** (deferred items only): what was being worked on when this was noticed.

The pull request says `Closes #<n>`. Merging it is what moves the issue to Done; nobody updates a status.

### 4. Inbox → Ready → Doing → Done, by label

| State | Meaning | How it is marked |
|---|---|---|
| Inbox | Captured. Nobody has agreed to do it. | open, label `inbox` |
| Ready | Shaped: it has a "done when" and passes the test in decision 1. | open, label `ready` |
| Doing | Someone has started. A branch or draft PR exists. | open, label `doing` |
| Done | Merged, or closed as not planned with a reason. | closed |

The same three labels apply to outcomes. **The product roadmap is `gh issue list --label outcome`**, grouped by state: what we are doing, what is agreed next, and ideas.

Three limits keep this small (decided 2026-11-10):

- **At most two outcomes are `doing` at once:** the main one, and one to switch to while the first waits on review or CI. Starting a third means moving one back to `ready` or finishing it.
- **No project board.** The labels are the status. A GitHub Project can be added later as a view over them if the label queries prove too thin.
- **No area labels.** The parent outcome is the only grouping.

**Only labelled issues are in the system.** The 107 open issues carry none of these labels, so every query the skills run passes over them. That gives a clean slate without deleting anything.

**The legacy issues stay open as a record of past intent, and are closed as work reaches them** (decided 2026-11-10). When an outcome is shaped, `shape` searches the unlabelled issues for ones that touch it. Whatever is still useful in each is carried into the new outcome or sub-issue. The legacy issue is then closed with a comment naming the issue that took it over, or saying that nothing in it was still needed. No legacy issue is closed without that cross-check, and none is closed in bulk.

### 5. Two modes

- **Roadmap mode** talks about outcomes: what, why, in what order. It creates and edits `outcome` issues and writes no code. It suits a chat with no checkout.
- **Execution mode** is scoped to one outcome. It takes the next `ready` sub-issue, does it, and defers everything else it notices.

A chat is in one mode at a time. Saying which is the first line of the chat.

### 6. "Defer this" is a contract

When the user says *defer this* (or an agent meets something out of scope), the agent:

1. **Asks nothing, and does not wait for a confirmation,** unless it cannot write a title. One `inbox` issue is cheap to close; a stop is what deferring is meant to avoid.
2. **Records the origin:** the issue or PR in hand, the branch, the commit, the files, and one line on why it is not being done now.
3. **Looks for a duplicate** among open issues and issues closed in the last 90 days (`gh issue list --state all --search "<terms> in:title,body"`). If one matches, it adds the new context there as a comment, labels it `inbox` if it is an unlabelled legacy issue, and creates nothing.
4. **Files it** with the label `inbox`, as a sub-issue of the current outcome only if it plainly belongs to it.
5. **Reads the issue back** (`gh issue view <n> --json url,state,labels`) and reports the URL from that response. A URL is never constructed or remembered.
6. **On any failure, says so** and prints the issue text into the chat, so the capture is not lost.
7. **Returns to the work** with a single line.

### 7. Three skills, adopted in order

| Skill | Verdict | Why |
|---|---|---|
| `capture` | **Adopt first.** | It is used most, it must be exact (steps 3, 5 and 6 above are the ones a chat skips), and it is identical in every tool. |
| `shape` | **Adopt second, as one skill.** | It turns an idea into an outcome, an outcome into sub-issues, and the inbox into `ready` or closed. Its value is the output it must produce (decisions 2 and 3), not a script. Shape one outcome by hand first and write the skill from what worked. It shows the list of issues it will create and waits for a yes, because this repository is public. |
| `work` | **Adopt last, and keep it thin.** | Most of what it would say is already in `CLAUDE.md` (push, review, merge). What is new is small: read the parent, the sub-issue and the latest handoff comment before starting; label `doing`; put `Closes #n` in the PR; leave a handoff comment when stopping. A skill is only read when invoked, so the part that must always hold goes in the instruction files (decision 8). If that proves enough, this skill is not needed. |

### 8. One shared copy, a thin entry point per tool

**Shared:**

- The skills live once, in `.agents/skills/<name>/SKILL.md`, beside the existing one (E9). They are written as `gh` commands and plain steps, naming no tool's own features.
- The rules that hold in every chat are a few lines: issues are the handoff; defer with `capture`; a PR closes its issue; write to GitHub as `onasunnymorning`. They are stated here. `CLAUDE.md` and the agents file **point** to this record; they do not restate it, for the reason `CLAUDE.md` already gives about a second copy drifting.

**Entry points:**

| Tool | How it reaches the skills | Its role |
|---|---|---|
| Claude Code | `.claude/skills/<name>` linking to `.agents/skills/<name>` | **Implementer.** Execution mode: sub-issues, branches, pull requests. |
| ChatGPT and Codex | Codex reads `.agents/skills` directly. ChatGPT reads the repository through its GitHub access and is pointed, in its own instructions, at this record, `docs/INVARIANTS.md` and the ADRs. | **Architect and reviewer**, one role for the second model. Roadmap mode: it creates and edits `outcome` issues. It reviews ADR drafts, shaped outcomes and pull requests against the invariants. It does not implement. |

**The roles do not overlap** (decided 2026-11-10): one model writes the change, the other shapes and reviews it. The reviewer is useful because it is a different model from the implementer; a second implementer would mean two writers and two sets of instructions to keep current.

Three rules for the second model:

- **It reads the rules first.** Nothing makes it open `docs/INVARIANTS.md`, so its instructions must. Advice that contradicts an invariant without citing it is a finding against the advice.
- **What it concludes lands in the repository:** an issue, a review comment, or an ADR draft as a pull request. An opinion left in its chat is lost the same way everything before this record was.
- **It cites `file:line`.** It sees what is pushed, not a worktree, so its claims must be checkable.
- **Its sub-issues start in `inbox`.** It may file sub-issues as well as outcomes. The implementer checks the pointers against the code and moves each to `ready` before starting it.

ChatGPT's write access to issues is confirmed by the owner (2026-11-10) and not tested for this record. Nobody pastes a chat into an issue by hand (E4).

### 9. GitHub access

- **One account, no machine user** (decided 2026-11-10). Every agent writes as `onasunnymorning`. Who actually filed an issue is its last line, written by the skill: `Filed by: <tool>, <mode>`. A dedicated account is worth its upkeep when a second person joins, when an agent's review should count towards merging, or when an agent runs unattended.
- **Identity in `gh`.** A skill takes that account's token for the one command (`GH_TOKEN=$(gh auth token --user onasunnymorning) gh issue create …`) and does not run `gh auth switch`, because sessions in other worktrees share the active account.
- **Scope.** `repo` covers issues, labels, comments and sub-issues. `project` is not needed, since no board is adopted (decision 4).
- **Hosted agents** need their GitHub app installed on this repository with read and write on Issues. ChatGPT has it. Codex cloud and Claude Code on the web are not checked.
- **Every writer follows decision 6's read-back,** ChatGPT included: an issue exists when its URL has come back from GitHub, not when a chat says it was created.
- **The repository is public.** An issue never contains real registry data, a customer's name, a secret or a host name of a deployment. Synthetic data is the default for every example.

### 10. How context survives a handoff

| What | Where it goes |
|---|---|
| The brief | The issue body, kept true as understanding changes |
| A decision made on the way | A comment on the issue; an ADR if it is architectural |
| Stopping before done | A comment headed **Handoff**: the state, the branch and PR, the next step, what was tried and ruled out |
| The code so far | The draft PR (`CLAUDE.md` workflow, step 2) |

A session starts by reading the outcome, the sub-issue, its last Handoff comment and the linked PR. It does not start from a tool's memory. Memory stays useful for how to work on this machine; it is not where the next step is kept (E7).

### Worked example: onboarding a registry operator with synthetic data

The pieces exist: `POST /registry-operators`, the tenant scope of ADR 0006, the offline seeder behind `make dev` (`cmd/api/ry-admin/seed.go`), and the synthetic deposit generator (`POST /escrow/synthetic`, `internal/application/rdesynth`). **The sub-issues below are illustrative.** Nobody has yet walked the path to find what is actually missing; that walk is the first sub-issue.

**Roadmap mode**, in any chat, produces the outcome:

> **A new registry operator can be onboarded end to end with synthetic data only** · `outcome` `ready`
> **Why:** to demonstrate and test onboarding without touching anyone's real registry data.
> **Done when:** on an empty installation, one documented command sequence creates an operator and its TLD, generates a synthetic deposit, imports it, and the admin UI shows the domains under that operator's scope and no other.
> **Out of scope:** real deposits; registrar onboarding; anything that needs a deployment.

**Shaping** it gives sub-issues, each one PR:

1. Walk the onboarding path as it stands and record every manual step and failure as a comment on the outcome (`ready`).
2. Create an operator and its first TLD in one request (`inbox` until 1 confirms it is needed).
3. Generate a synthetic deposit for a named operator's TLD and import it.
4. Show that a second operator sees none of the first one's objects.
5. Put the sequence behind one make target and document it.

**Execution mode**, in Claude Code on sub-issue 3. The agent notices that the generator always uses five registrars (`SYNTHETIC_REGISTRARS` in `frontend/lib/escrow/synthetic.ts`: "fixed server-side"). The user says *defer this*. `capture` finds no duplicate, files "Let a synthetic deposit take a registrar count" under the outcome with `inbox` and the origin (sub-issue 3, the branch, that file), reads it back, and answers with one line and the URL. Work on 3 continues.

**Handoff.** The session ends with the import half done. The agent leaves a Handoff comment on sub-issue 3 and the draft PR stays open. The next day a different agent is given the issue number and nothing else, and starts from there.

## Consequences

- **"What next?" and "what did we put off?" each become one query**, the same from any tool.
- **Deferring costs one sentence.** That is the property the rest depends on; if `capture` is slow or asks questions, the workflow fails the way the last one did (A1).
- **Labels can go stale.** A `doing` label on abandoned work is the likely drift. Nothing enforces it; `shape` lists `doing` issues with no open PR when it triages.
- **The duplicate check is a search, not a guarantee.** It will miss a duplicate worded differently and, less often, comment on the wrong issue. Both are cheap to fix by hand.
- **Issues are public.** Half-formed ideas and deferred defects are visible to anyone. Security findings do not go through `capture`.
- **ChatGPT needs its own pointer.** It does not load skills or `CLAUDE.md`; its project instructions must tell it to read this record and `docs/INVARIANTS.md` from the repository. That pointer lives outside the repository and is the one piece nobody can review here.
- **Implementation depends on one tool.** With the second model kept to shaping and review, there is no fallback implementer. That is the current practice, written down.
- **The legacy backlog shrinks only where work goes.** Issues in areas nobody touches stay open indefinitely. That is accepted: they cost nothing while unlabelled.
- **Shaping an outcome takes longer by one search** of the legacy issues, and that search is as fallible as the duplicate check.
- **Maintenance is four labels, three short files and two pointers.** No automation, no board, no service.

## Alternatives considered

- **Keep the status quo: ADRs, roadmap tables and PR series, no issues.** It works for a protocol series (E6) and is kept for that. It has no place for a deferral, an outcome that spans protocols, or a handoff to another tool (E7).
- **A GitHub Project as the system, with a Status field.** A better board and built-in automation. It needs the `project` scope the default account lacks (E10), GraphQL instead of one `gh issue` command, and is the least likely surface for a chat connector to reach. Kept as a view that can be added on top of the labels later.
- **A backlog file in the repository.** It travels with every checkout and needs no access. But `main` takes changes only through a PR with green CI, about six minutes, and work here runs in many worktrees at once. A deferral would need a PR and would conflict with the others. That ends "defer without interrupting".
- **An external tracker (Linear, Notion).** Another system and another login for every agent, further from the code and the PRs.
- **Milestones as outcomes.** A progress bar for free, but a milestone has no discussion, no sub-issues and no labels, so the outcome's own brief would have nowhere to live.
- **Issue types instead of an `outcome` label.** Issue types belong to organisations; this repository is owned by a user (E10).
- **Clean the legacy backlog first, or bulk-close it.** Cleaning first is the work that was being avoided. Bulk-closing throws away the record of what was once intended before anyone has checked it. Decision 4 does neither.

## Adoption plan

Each step is small and can be stopped after. None of it is done by this record.

0. **Check four things this record assumes:** that Claude Code follows a linked skill directory; which agents file Codex reads, given that this repository's is at `.agents/AGENTS.md` and not the root; which account ChatGPT's issues are created under, and whether it can set labels and a parent; that the per-command token in decision 9 works for sub-issues.
1. **Capture only.** Create the four labels. Write `capture`. Add the pointers. Use it for two weeks of normal work.
2. **Shape one outcome by hand:** the onboarding example above, with the templates in decisions 2 and 3, including the cross-check of legacy issues (#314 mentions pre-seeding registrars, for one). Then write `shape` from what worked.
3. **Two tests of the handoff.** Ask ChatGPT to review this record against the repository: that exercises the reviewer role on a document that needs review anyway. Then start a fresh Claude Code session on one sub-issue with the issue number and nothing else. If it can start, the design holds. If not, the gap is in the template, and that is the thing to fix.
4. **Decide on `work`,** and amend this record with what was built.

Three cheap fixes found on the way, independent of the above:

- Add a `Closes #` line to `.github/pull_request_template.md`.
- Move the PR rules that exist only in Claude Code's memory into `CLAUDE.md`: post as `onasunnymorning`, and read Copilot's review bodies before arming auto-merge.
- Correct or delete `.cursorrules` (E8).

**Review after the first outcome closes:** count captures, duplicates caught and missed, and PRs that closed an issue. If captures are near zero, the workflow has failed for reason A1 and should be withdrawn, not extended.

## Open questions

The owner answered seven on 2026-11-10; each is now in the decision: the legacy issues (decision 4), the board, area labels and the limit on outcomes in `doing` (decision 4), `capture` without confirmation (decision 6), ChatGPT's write access and sub-issues from the second model (decision 8), and the single account (decision 9).

One remains, and it waits on a check, not a choice:

1. **Where the pointer for Codex lives:** the existing `.agents/AGENTS.md`, or a new root `AGENTS.md`? It depends on which file Codex reads (adoption step 0).

## Not done

- No skill, label, issue, board, template or instruction file has been created or changed.
- The legacy issues have not been read beyond a sample, by decision.
- No hosted agent's access has been tested.
- The onboarding example has not been checked against a running system.
