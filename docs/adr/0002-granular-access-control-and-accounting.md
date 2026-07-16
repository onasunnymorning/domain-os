# ADR 0002 — Granular access control and accounting for the Admin API

- **Status:** Proposed
- **Date:** 2026-07-11
- **Deciders:** Platform / API
- **Supersedes:** none

## Context

The Admin API (`cmd/api/ry-admin`) authenticates every request through a single
middleware, `rest.Auth0Middleware` (`internal/interface/rest/auth_middleware.go`),
which validates an Auth0 JWT or falls back to one shared legacy bearer token
(`legacy-admin`). Authentication is binary: any valid token grants access to
every endpoint, including the most destructive ones.

Three surfaces make this a real risk rather than a theoretical one.

**The Workflow Launchpad exposes high-impact verbs to any authenticated
caller.** `POST /workflows/launch`, `POST /workflows/:id/signal`, and
`POST /workflows/:id/terminate` (`internal/interface/rest/workflow_controller.go`)
can start an escrow import, confirm a TLD cleanup (irreversible asset deletion),
or kill a running workflow. Signal-gated HITL workflows (`tld-cleanup`,
`escrow-import`) treat the confirmation signal as a safety gate, but the signal
endpoint has the same access requirements as a read-only status query.

**The authorization transport already exists but is never evaluated.**
`CustomClaims.Scope` is parsed from every Auth0 token and then discarded —
nothing in the codebase reads it. We pay for granular claims and enforce none
of them.

**Accounting cannot attribute or explain actions.** The `entities.Event`
audit record (`pkg/domain/entities/event.go`) captures `User`, `Action`,
`ObjectType`, `ObjectID`, and an execution `Result` (Success/Failure), but:

- `stream_middleware.go` hardcodes `userid = "admin"`, masking the caller.
- Scheduled workflow runs created by `bootstrap.EnsureTemporalInfrastructure()`
  (`internal/infrastructure/bootstrap/ensure.go` — expiry-loop, purge-loop,
  restore-loop, sync-registrars, update-fx, sync-spec5, event-relay,
  event-prune) have no principal at all; a schedule-driven domain purge and a
  human-triggered one are indistinguishable in the event log.
- There is no record of *authorization* outcomes. `Details.Result` says whether
  an action worked, not whether it was permitted. Denied requests are never
  logged, so the most security-relevant rows in an audit trail simply don't
  exist.

### Constraints

- Must be **simple to operate and update**: permission changes should not
  require a deploy or a re-login.
- Must fit the existing stack (Gin, Postgres/GORM, Auth0, Temporal) and the
  hexagonal architecture rules in `architecture.md` (repositories defined in
  the domain layer, implementations in infrastructure).
- The workflow registry (`internal/application/workflows/workflow_registry.go`)
  is the single source of truth for workflow metadata; the access-control
  scheme must not introduce a second registry that can drift.
- Must leave a credible upgrade path to attribute-based rules (e.g. "may only
  launch cleanup for TLDs they operate") without a rewrite.

## Decision

Adopt a **layered scheme**: Auth0 for authentication and coarse role
assignment, a **Postgres-backed RBAC table** for the granular authorization
decision, and an explicit **`AccessDecision`** recorded in every accounting
event. Casbin is the designated escalation path if attribute-based conditions
become necessary; the data model below is chosen to migrate into a Casbin
policy store mechanically.

### 1. Principals

Every request and every workflow execution carries a typed principal:

| Kind | Example | Source |
|------|---------|--------|
| `user` | `auth0\|abc123` | Auth0 JWT `sub` claim |
| `service` | `svc:frontend` | Client-credentials token |
| `schedule` | `svc:schedule/expiry-loop` | Stamped by bootstrap into scheduled workflow input |

The hardcoded `userid = "admin"` in `stream_middleware.go` is removed.
Bootstrap-created schedules stamp their synthetic principal into workflow
context so activity-emitted events are attributable.

### 2. Permission vocabulary — derived from the workflow registry

Permissions are strings of the form `action:resource`, with resources taken
from existing registry keys rather than a new namespace:

```
workflows:launch:tld-cleanup
workflows:signal:escrow-import
workflows:terminate:*
workflows:read:*
```

The last segment supports `*` wildcard matching. Because the resource segment
is the registry `Key`, a newly registered workflow automatically has a
permission key — there is no second list to update, only a role to grant.

### 3. RBAC storage

A `Role → Permission` mapping lives in Postgres, following the repository
pattern: interface in the domain layer, GORM implementation in
infrastructure, cached in memory with a short TTL (≤30 s) so the hot path is
a map lookup. Roles are assigned in Auth0 (the JWT carries role names); what
each role *means* is owned and edited in our own table via the admin UI, so a
permission change takes effect within the cache TTL — no deploy, no re-login.

### 4. The `AccessDecision`

The output of every authorization evaluation is an explicit value, produced
in middleware (not in handlers) and attached to both the request context and
the accounting event:

```go
type DecisionOutcome string

const (
    DecisionAllow       DecisionOutcome = "allow"        // a rule explicitly permitted
    DecisionDeny        DecisionOutcome = "deny"         // a rule explicitly forbade
    DecisionDenyDefault DecisionOutcome = "deny_default" // no rule matched — fail closed
    DecisionError       DecisionOutcome = "error"        // evaluation failed — fail closed
)

type AccessDecision struct {
    Decision  DecisionOutcome
    Principal string // "auth0|abc123", "svc:schedule/expiry-loop"
    Action    string // "workflows:signal"
    Resource  string // "escrow-import"
    PolicyID  string // which role/permission row matched
    Reason    string // human-readable; returned in the 403 body
}
```

Semantics:

- Anything other than `allow` blocks the request. The three non-allow values
  exist to make the audit log diagnosable: an explicit deny, a policy gap,
  and a broken policy engine are different operational problems.
- The decision is **always** recorded — including denies, which never reach a
  handler. This is why evaluation and logging happen in middleware.
- `DecisionOutcome` is deliberately distinct from the existing
  `EventDetails.Result` (Success/Failure): *permitted* and *worked* are
  independent facts. A denied request is `deny` + no result; a permitted
  request that crashed is `allow` + `Failure`.

### 5. Accounting extensions

`entities.Event` gains `Decision`, `PolicyID`, and `PrincipalKind` alongside
the existing fields. The existing event pipeline (Postgres publisher →
event-relay workflow → S3 archive, pruned by event-prune) then provides a
durable audit archive with no new infrastructure.

## Options Considered

### Option A: Auth0 scopes/roles enforced per-route in middleware

| Dimension | Assessment |
|-----------|------------|
| Complexity | Low |
| Runtime updatability | Poor — edits in the Auth0 dashboard require token refresh/re-login to take effect |
| Granularity | Route-level only; cannot distinguish `tld-cleanup` from `sync-registrars` on the same endpoint |
| Team familiarity | High (Auth0 already integrated) |

**Pros:** Nearly free to implement; no new storage; scopes already arrive in the token.
**Cons:** Per-workflow granularity would require one scope per workflow managed in Auth0 — a second registry that drifts; changes are not immediate.

### Option B: Postgres-backed RBAC, permission keys derived from the workflow registry ⭐

| Dimension | Assessment |
|-----------|------------|
| Complexity | Low–Medium (one table, one repository, one middleware) |
| Runtime updatability | Excellent — admin UI edit, effective within cache TTL |
| Granularity | Per-action × per-workflow, with wildcards |
| Team familiarity | High (Postgres/GORM/repository pattern is the house style) |

**Pros:** Registry stays the single source of truth; fits the hexagonal rules; trivially auditable (it's a table); migrates mechanically to Casbin.
**Cons:** Cannot express attribute conditions ("only TLDs they own"); we own the evaluator code.

### Option C: Casbin embedded

| Dimension | Assessment |
|-----------|------------|
| Complexity | Medium (policy model files, adapter, watcher) |
| Runtime updatability | Excellent |
| Granularity | RBAC + ABAC + conditions |
| Team familiarity | Low |

**Pros:** Handles ABAC when we need it; Go-native; Postgres adapter exists.
**Cons:** Policy-model DSL is a new concept to operate and review; overkill while all rules are `role → action:resource`.

### Option D: OPA/Rego (sidecar or embedded) — rejected

Full policy language and decision logging, but a second runtime, a new
language for reviews, and network hops in the request path. Pays off with
many services; we have one API.

### Option E: ReBAC (SpiceDB/Zanzibar-style) — rejected

Relationship graphs solve user-owned-resource sharing at scale. We have a
handful of operator roles and a few dozen resources. Highest operational cost
of all options for capability we don't need.

## Trade-off Analysis

Weighted for the stated priorities (simplicity 30%, runtime updatability 25%,
granularity 20%, stack fit 15%, ops burden 10%; scores 1–5):

| Option | Simple | Updatable | Granular | Stack fit | Ops | **Weighted** |
|--------|--------|-----------|----------|-----------|-----|--------------|
| A. Auth0 scopes only | 5 | 2 | 3 | 5 | 5 | 3.9 |
| **B. Postgres RBAC** | 4 | 5 | 4 | 5 | 4 | **4.4** |
| C. Casbin | 3 | 5 | 5 | 4 | 4 | 4.2 |
| D. OPA | 2 | 4 | 5 | 2 | 2 | 3.1 |
| E. ReBAC | 1 | 4 | 5 | 2 | 1 | 2.6 |

B wins on the criteria that matter now; C is the only option whose extra
capability we can foresee needing. The decision is therefore **B with a
designed migration path to C**, not B alone.

### Migration path B → C

The B data model is a strict subset of a Casbin RBAC model, so escalation is
mechanical, not a rewrite:

1. **Policy rows map 1:1.** Each `role → action:resource` row becomes a
   Casbin `p, role, resource, action` policy line; role assignments from the
   JWT become `g, principal, role` grouping rules. A one-shot migration
   script converts the table; the Casbin GORM adapter can even keep reading
   from Postgres.
2. **The evaluator is behind an interface.** The middleware depends on a
   domain-layer `AccessPolicyEvaluator` interface (`Evaluate(principal,
   action, resource, attrs) AccessDecision`), per the house repository
   pattern. Step C swaps the homegrown implementation for a Casbin-backed
   one; middleware, controllers, and accounting are untouched.
3. **`AccessDecision` is engine-agnostic.** `PolicyID` carries a row ID today
   and a Casbin rule reference later; the audit schema does not change.
4. **The `attrs` parameter is present from day one** (unused by the RBAC
   evaluator) so ABAC conditions like TLD ownership can be added in Casbin
   `keyMatch`/custom matchers without changing call sites.

**Trigger for the migration:** the first genuine requirement for a
condition-bearing rule (attribute constraints, time windows, per-tenant
ownership). Until then, Casbin's model DSL is pure operating cost.

## Consequences

**Easier:**

- Granting a new operator read-only or operate-level access becomes a role
  assignment instead of sharing the admin token.
- Audits can answer "who tried to do what, was it allowed, and by which
  rule" — including denied attempts, which are currently invisible.
- Scheduled runs become attributable (`svc:schedule/expiry-loop`),
  separating machine actions from human ones in the event archive.
- New workflows get access control for free via the registry-derived
  permission key.

**Harder:**

- Every protected route needs its action/resource declared; the launchpad
  launch/signal/terminate endpoints derive the resource from the request
  body/workflow ID, which needs care (derive *before* the decision, from
  validated input).
- A cache TTL on permissions means revocation is eventually consistent
  (bounded by TTL, ≤30 s).
- The legacy shared token must be scoped or retired; while it exists it maps
  to a named role rather than implicit full access.

**To revisit:**

- Whether EPP and MCP interfaces adopt the same `AccessPolicyEvaluator`
  (EPP has its own registrar credential model today).
- Casbin migration when the first ABAC requirement lands (see trigger above).
- Whether the frontend should surface per-user capability (hide launch
  buttons the caller can't use) via a `GET /me/permissions` endpoint.

## Action Items

1. [ ] Remove the hardcoded `userid = "admin"` in `stream_middleware.go`; propagate the authenticated principal.
2. [ ] Stamp schedule principals (`svc:schedule/<id>`) in `bootstrap.EnsureTemporalInfrastructure()` workflow args/context.
3. [ ] Add `Decision`, `PolicyID`, `PrincipalKind` to `entities.Event`; emit decision events from middleware, including denies.
4. [ ] Define `AccessPolicyEvaluator` interface + `AccessDecision` in the domain layer.
5. [ ] Implement Postgres role/permission repository + cached RBAC evaluator in infrastructure; seed roles `viewer`, `operator`, `admin`.
6. [ ] Wire evaluation middleware into the ry-admin router, starting with `/workflows/*` (launch, signal, terminate), fail-closed.
7. [ ] Map the legacy token to a named role; plan its retirement.
8. [ ] Admin UI for role/permission editing (table CRUD).
9. [ ] Document the permission-key convention in `docs/WORKFLOW_LAUNCHPAD_ARCHITECTURE.md` and the workflow "Definition of Done".
