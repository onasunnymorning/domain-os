# ADR 0006 — Tenancy model: two sides, two scopes, one enforcement layer

- **Status:** Accepted
- **Date:** 2026-08-25
- **Deciders:** Platform / Registry
- **Supersedes:** none (composes with ADR 0002)
- **Resolves:** `INV-02` in `docs/INVARIANTS.md`

## Context

`INV-02` ("tenant scope required on every executor call") was filed as a
**Class D contradiction**: the rule as written did not map onto the codebase,
and three parts of the code did incompatible things. One vertical slice (zone
slaving / serial drift) threaded a `tenantID string` through service, port and
SQL; the entire core registry surface — 25 service interfaces in
`internal/application/interfaces/` — threaded nothing; and the one place
tenant scope *was* enforced was also the one place a caller could choose their
own tenant, via a self-asserted `X-Tenant-ID` header with a `?tenant_id=` query
fallback.

That contradiction is not resolvable by picking one of the three. It is
resolvable by writing down what a tenant *is* here, which had never been
recorded.

### This is a two-sided marketplace

domain-os is Airbnb-shaped, and the two sides are not variations of one tenant
kind — they are different tenants with different keys:

- **Registry Operators are the supply side.** They own TLDs.
  `RegistryOperator.RyID` (`pkg/domain/entities/registryOperator.go:18`) →
  `TLD.RyID` (`pkg/domain/entities/tld.go:49`) → `Domain.TLDName`
  (`pkg/domain/entities/domain.go:62`). Note what this chain does *not* do:
  `RyID` is never denormalized below TLD, so reassigning a TLD to a different
  operator is a one-row update.
- **Registrars are the consumer side.** They consume TLDs, sponsor domains, and
  create the contacts and hosts used on them. Later, registrar *groups*.

**Consumer-side tenancy already exists in the schema, and it is called
sponsorship.** `ClID` on `Domain` (`domain.go:59`), `Contact`
(`contact.go:66`), and `Host` (`host.go:42`) is not denormalization to be
regretted — it is ownership, mandated by the registry/EPP data model. Registrar
scoping therefore needs **zero schema change**.

This dissolves the trap that an operator-only reading of tenancy runs into.
Contacts and hosts have no TLD, so they cannot be scoped through the TLD join,
and adding an operator column to them would be exactly the denormalization the
supply chain avoids. Under the two-sided model the question does not arise:
their tenant is the sponsoring registrar. What remains is a *visibility*
question — whether an operator can read registrar-sponsored contact data
through domains in their TLDs — and that is policy, not schema.

**Accreditation is the bridge between the two sides**: registrar ⋈ TLD (⋈
operator via `TLD.RyID`) — `internal/application/services/accreditation_service.go:40`,
`Registrar.TLDs`.

### The launch state, and why the timing is lucky

Today there is **one operator**, and every admin-API caller presents the same
shared root token (`cmd/api/ry-admin/ryAdminAPI.go:91`). Enforcement machinery
would currently protect nothing from nobody.

But the EPP server already authenticates per registrar: `registrarIDKey` is
stamped into the session context at `cmd/epp/eppServer.go:319`, using the typed
context-key idiom (`eppServer.go:28`). And it implements only greeting, login,
logout, and domain-check (`eppServer.go:100-118`; contact-info is commented
out). The session's registrar identity is used **only for rate limiting**.

**The transactional verbs do not exist yet.** That is the whole reason to write
this ADR now: the invariant that governs those verbs can be established
*before* the surface it governs is built, which is the only time establishing
it is free.

The read side is likewise pre-fitted: `ListDomainsFilter` already carries both
scope hooks — `TldEquals` (operator side) and `ClidEquals` (registrar side) —
at `internal/application/queries/listDomainsFilter.go:16,18`.

### Relationship to ADR 0002

ADR 0002 (Proposed) defines typed principals (`user` / `service` / `schedule`)
and names attribute rules as its escalation path. This ADR composes with it
rather than competing: registrar EPP sessions and operator API principals
become two principal flavours that *carry scope*. When ADR 0002's Auth0 work
lands, the tenant claim rides the same token, and the "only TLDs they operate"
attribute rule it anticipates is exactly the operator scope defined here.

## Decision

### The gold standard

1. **Two tenant kinds, one per side of the marketplace.**
   - **Operator scope (`RyID`)** — administrative plane. Sees their TLDs and
     everything within them, *derived via the TLD join*, never via
     denormalized columns.
   - **Registrar scope (`ClID`)** — transactional plane (EPP, future registrar
     portal). Sees and manages **only objects they sponsor**; the sponsorship
     columns are the enforcement key. Accreditation gates which TLDs they may
     transact in.
   - **Staff/global is an explicit third kind**, not the absence of a filter.
     "No scope" must never be spelled the same way as "all scopes".
2. **Scope is a claim on the authenticated principal** — an Auth0 claim or an
   EPP login — never a caller-supplied header or parameter.
3. **Tenant-scoped service methods are `(ctx, scope, ...)`** — ctx first, scope
   second, typed, never optional, never ambient (never fished out of
   `context.Context`).
4. **Enforcement lives in repository SQL.** Operator side: `WHERE tld.ry_id = ?`
   (join). Registrar side: `WHERE clid = ?` (sponsorship). A service that
   filters in Go is a service that will one day forget to.
5. **Every EPP transactional verb checks sponsorship — or accreditation, for
   creates — in the command layer.** Info / update / delete / transfer: the
   object's sponsor must equal the session registrar, EPP result code `2201`
   (authorization error) on mismatch. Create: the session registrar must be
   accredited for the TLD. This rule is established now, before those verbs are
   written.
6. **Cross-side visibility is an explicit policy decision, deferred.** Whether
   an operator may read registrar-sponsored contact data through domains in
   their TLDs is undecided until a real second tenant exists on either side.
   Deferred means undecided, not permitted.
7. **Registrar groups (later) are a set of registrar ClIDs** — an
   admin/reporting concept resolved to the ClID set at scope-derivation time.
   Enforcement stays at ClID granularity. No schema change and no signature
   change is anticipated for groups.

### What ships in this pass

Deliberately not a service-surface migration. Decisions are cheap now and
expensive later; enforcement is the reverse — it scales in value with tenant
count, and today the count is one. So this pass ships the decision plus the
guardrails that keep the second tenant, on either side, a ratchet rather than a
rewrite:

- **`pkg/domain/entities/scope.go`** — `OperatorID` and `RegistrarClID`, both
  `ClIDType`-shaped, both **distinct defined types, not aliases**, with
  constructors that reuse `NewClIDType` validation. Being distinct is the point:
  an operator scope, a registrar scope, and a plain object ClID are three
  different meanings wearing the same shape, and the compiler is the cheapest
  place to keep them apart. `RegistrarClID` is defined and documented but not
  yet threaded — it exists so the EPP buildout starts typed.
- **`internal/interface/rest/tenant.go`** — `OperatorScopeFromRequest`, the
  single admin-plane derivation point.
- **The zone-slaving slice converts `tenantID string` → `entities.OperatorID`
  end to end**: application interface, service, port, Postgres adapter,
  Temporal params, activities, and controller. It is the template slice, so it
  is the one that must exhibit the target shape.

Nothing else moves. The core registry service surface is untouched, and no EPP
code is written — there, the invariant, not the implementation, is this pass's
deliverable.

### Guardrails

These are the rules a reviewer can cite, and the reason this ADR is worth more
than the code it ships with.

1. **Never denormalize `RyID` below TLD.** It keeps TLD→operator reassignment a
   one-row update, and it keeps the operator scope honest — derived, not
   copied.
2. **Never add operator-tenant columns to registrar-sponsored data.**
   Sponsorship (`ClID`) *is* the consumer-side tenancy key; accreditation is
   the bridge to the operator side. A `ry_id` column on `contacts` is the
   canonical wrong answer to a question this model has already answered.
3. **No EPP transactional verb ships without its sponsorship/accreditation
   check.** The check is part of the verb's definition of done, and it belongs
   in the command layer, not in a middleware that a future verb can forget to
   register.
4. **New tenant-aware code copies the zone-slaving signature shape** with the
   typed scope: `(ctx, scope entities.OperatorID, ...)` down to SQL.
5. **One derivation point per plane.** Admin API: reads of `X-Tenant-ID` are
   confined to `OperatorScopeFromRequest` — root-asserted for now, replaced by
   the Auth0 claim later, one function either way. EPP: scope comes only from
   the authenticated session key, never from command XML.
6. **Scope is a parameter, not context state.** Passing it through
   `context.Context` makes it optional at every call site and invisible in every
   signature, which is how the core surface ended up with no scope at all.

### Noted limitations

- **Events carry no tenant.** `entities.Event` has no scope field; tenant
  attribution for audit is derived at query time from the object referenced.
  Accepted — adding a scope column to the event stream would create a second
  copy of the tenancy key, which guardrail 1 exists to prevent.
- **The workflow launchpad supplies operator scope as a launch parameter**
  (`internal/interface/rest/workflow_controller.go`, `serial-drift` case)
  rather than deriving it from the caller. This is a second self-asserted
  source. It is now typed and validated, so an unusable scope is rejected
  before a schedule is created against it, but it is not yet a claim. Closing
  it is part of the Auth0 tenant-claim work.
- **`askg`'s `CallerScope`** (`internal/askg/result.go:62`) is a `UserID`, not a
  tenant, and is not threaded into tool execution. It is out of scope here and
  remains as `INV-02` described it.

## Explicitly deferred

Listed so that "not done" is on the record rather than inferred:

- Threading scope through the core registry services.
- SQL enforcement outside the zone-slaving slice.
- The Auth0 tenant claim (lands with ADR 0002).
- Operator ↔ registrar cross-visibility policy.
- Registrar groups.
- Row-level security and per-tenant encryption keys.
- EPP verb implementation (the *rule* for those verbs is decided here; the
  verbs are not).

## Consequences

**Easier:**

- The second tenant on either side is an additive change. The operator side has
  a signature shape and an enforcement layer to copy; the registrar side has a
  key that already exists in the schema.
- EPP verbs get their authorization requirement at design time instead of in a
  retrofit after the first cross-registrar data leak.
- Review has something to cite. "This violates ADR-0006 guardrail 2" is a link,
  not an argument.

**Harder:**

- Two scope types mean explicit conversions where a bare string used to flow.
  That friction is the mechanism, not a side effect.
- Every new tenant-aware method now owes a decision about which plane it is on.
  There is no default, by design — staff/global is a third explicit kind.

**Behaviour changes in this pass:**

- The `?tenant_id=` query-parameter fallback on `/zone-slavings*` is **removed**;
  the `X-Tenant-ID` header is the only source. The frontend already sends the
  header (`frontend/lib/api/zone-slavings.ts`), so no client change is needed.
- The operator scope is now validated as a `ClIDType` (3–16 ASCII characters)
  rather than merely non-empty, on both `/zone-slavings*` and the `serial-drift`
  launchpad path. Since the value is a `RegistryOperator.RyID` — itself a
  `ClIDType` — every legitimate value already passes.

## Action Items

1. [ ] EPP transactional verbs enforce sponsorship/accreditation in the command
       layer. Open this as an issue on the "Architectural invariants" board so
       the rule lives where the EPP buildout works from, not only in this
       document.
2. [ ] Replace the root-asserted `X-Tenant-ID` read in
       `OperatorScopeFromRequest` with the Auth0 tenant claim, alongside ADR
       0002.
3. [ ] Derive the `serial-drift` launchpad's operator scope from the caller
       instead of the request body.
4. [ ] Decide operator ↔ registrar cross-visibility policy when a second tenant
       exists on either side.
