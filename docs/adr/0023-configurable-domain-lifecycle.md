# ADR 0023 — The configurable domain lifecycle

- **Status:** Accepted (sections 1–4 describe what held before the implementation; the Decision is what changed)
- **Implemented:** 2026-11-02 — `entities.LifecyclePolicy` and the `lifecycle_policies` table (seeded with `gtld-standard`, phases migrated by value), `/lifecycle-policies` under `registry:policy:admin`, `PUT /tlds/{tld}/phases/{phase}/lifecycle-policy`, the Lifecycle Policies admin page. Two details settled in the implementation: a policy name follows the EPP policy rule (1–32 lower-case letters, digits and hyphens), since `migrated-<tld>-<phase>` does not fit a 16-character `ClIDType`; and the resolved policy is loaded with the TLD's phases on every read rather than cached per node, because every lifecycle decision already reads the TLD from the database.
- **Date:** 2026-11-02
- **Deciders:** Platform / Registry
- **Related:** [ADR 0004](0004-idempotent-domain-lifecycle-transitions.md) (how transitions run), [ADR 0018](0018-where-configuration-lives.md) (where settings live), [ADR 0006](0006-tenancy-model.md) (who may change a TLD)

## Context

Prospective operators ask four things about the lifecycle: what can be
configured, where a configuration applies, how it is managed, and what we can
show. This record answers them from the code as it stands, so the answers
can be reviewed as decisions rather than rediscovered.

## 1. What can be configured

**States and transitions are fixed; durations and a few switches are not.**

The states are the EPP and RGP ones (RFC 5731, RFC 3915): `ok`, `inactive`,
the client and server prohibitions, `pendingCreate/Delete/Transfer/Renew/Update`,
and the RGP statuses `addPeriod`, `renewPeriod`, `autoRenewPeriod`,
`transferPeriod`, `redemptionPeriod`, `pendingRestore`, `pendingDelete`
(`pkg/domain/entities/domainStatus.go`, `domainRGPStatus.go`). The graph
between them is code (`Domain.Renew`, `Expire`, `MarkForDeletion`,
`RequestRestore`, `CompleteRestore`, `RevertRestore`, the transfer methods).
No operator can add a state or a transition.

What an operator sets is the **phase policy** (`PhasePolicy`,
`pkg/domain/entities/phasePolicy.go`), one value object per phase of a TLD:

| Setting | Default | What it governs |
|---|---|---|
| `registrationGP` | 5 days | add grace period: delete after create earns a refund and skips redemption |
| `renewalGP` | 5 days | renew grace period |
| `autorenewalGP` | 45 days | auto-renew grace period |
| `transferGP` | 5 days | transfer grace period |
| `transferLockPeriod` | 60 days (0 allowed) | no transfer after a create or a transfer |
| `pendingTransferPeriod` | 5 days | losing registrar's window before the registry approves |
| `redemptionGP` | 30 days | restorable after delete or expiry |
| `pendingdeleteGP` | 5 days | after redemption, until purge |
| `pendingRestorePeriod` | 7 days | restore report due after a restore request |
| `restoreReportRequired` | true | false restores on the request alone |
| `allowAutorenew` | true | TLD-level switch for the expiry loop |
| `maxHorizon` | 10 years | furthest expiry a renew may reach |
| `minLabelLength`, `maxLabelLength`, `requiresValidation`, `baseCurrency`, contact data policy | | not lifecycle, same object |

Durations are whole days. A setting left at zero means "use the default" for
every field but the transfer lock, which is a pointer so that 0 (no lock) is
a value. Grace periods of zero days are therefore not expressible today.

**Exceptions** are per domain, not per policy:

- the standard prohibitions (`clientDeleteProhibited`, `serverRenewProhibited`, …)
  stop the corresponding command; `serverDeleteProhibited` also stops expiry,
  while the client prohibitions are cleared by expiry (`Domain.Expire`);
- `DropCatch` turns a purge into an NNDN reservation instead of a release;
- `GrandFathering` fixes a domain's price, with an expiry condition
  (transfer, delete, or a date). It does not alter timing.

**Cadence** is platform code: `expiry-loop` hourly at :00, `purge-loop` at
:30, `restore-loop` at :45, transfer approval at :15
(`internal/infrastructure/bootstrap/ensure.go`). A transition lands within an
hour of its due time, never at the second.

## 2. Where configuration applies

**Per TLD, through its current General Availability phase.** Every lifecycle
decision loads the TLD with its phases and calls `TLD.GetCurrentGAPhase()` at
the moment of the event (`domain_service.go`, 14 call sites). The phase in
force when a domain was registered is irrelevant to its later expiry; the
phase in force at expiry decides. Launch phases carry a policy too, but the
lifecycle reads only the GA one; a launch phase governs how a name is
*obtained*, through `TLD.RegistrationContextAt` ([ADR 0024](0024-launch-phases-and-epp-launch-extension.md)),
never its later lifecycle.

**Per registrar** there is exactly one knob: `Registrar.Autorenew`. The
expiry loop auto-renews a domain only when the TLD's policy allows it *and*
its sponsor has opted in; otherwise the domain expires into `pendingDelete`.
There are no per-registrar or per-customer grace periods, redemption periods
or lock periods. ADR 0019 chose the same shape for EPP limits (named policies,
no per-registrar overrides); the lifecycle has not yet gone that far: there is
no named lifecycle policy shared across TLDs, each phase carries its own copy.

**Platform-wide** are only the defaults in code (`NewPhasePolicy`) and the
loop cadence.

**After launch:** a policy can be changed as long as the phase has not ended
(`Phase.CanUpdate`; a historic phase is immutable). The change affects
transitions evaluated after it. Dates already written to a domain
(`RGPStatus.*End`, `PurgeDate`) are absolute timestamps and are not
recomputed, so a domain already in redemption keeps the window it entered
with. A **phase roll** (a new GA phase starting when the old one ends;
`TLD.AddPhase` rejects overlapping GA phases) is the orderly way to change
policy from a known instant, and the admin UI shows the diff between two
phases (`PhaseConfigDiff`).

## 3. How it is managed

- **Interface.** Admin REST: `PUT /tlds/{tld}/phases/{phase}/policy`
  (`phase_controller.go`), with phase create, end and delete beside it. Admin
  UI: the phase drawer's Policy section edits every field above
  (`frontend/components/phases/sections/PhasePolicySection.tsx`); the GA and
  launch timelines show phases over time. The database is never edited by
  hand (ADR 0018, point 2).
- **Who.** A caller with a valid admin token scoped to the TLD's operator
  (`X-Tenant-ID`, ADR 0006). Deleting a phase needs
  `registry:platform:admin`; *changing a policy needs no dedicated
  permission* beyond tenant membership. There is no approval step and no
  four-eyes rule.
- **Validation.** Structural only: the phase must exist and not be historic;
  GA phases may not overlap; zero fields are ignored rather than applied.
  There is **no range check** on durations (a negative or 10,000-day
  redemption period is accepted), no check that `minLabelLength ≤
  maxLabelLength`, and no check that a change is safe for domains in flight.
- **Traceability.** Every update publishes a `phase.updated` event carrying
  the previous and the new phase (`phase_service.go`) to the event stream;
  there is no dedicated policy history table, so "what was the policy on
  date X" is answered from the event stream, not from a query.

## 4. What we can demonstrate

**An example configuration** (a short-redemption TLD for a demo):

```json
PUT /tlds/example/phases/ga-2026/policy
{ "policy": {
    "registrationGP": 5, "autorenewalGP": 45, "renewalGP": 5,
    "transferGP": 5, "transferLockPeriod": 60, "pendingTransferPeriod": 5,
    "redemptionGP": 30, "pendingdeleteGP": 5,
    "pendingRestorePeriod": 7, "restoreReportRequired": true,
    "allowAutorenew": true, "maxHorizon": 10 } }
```

**A domain through its lifecycle**, with the dates that policy produces:

| Day | Event | Written to the domain |
|---|---|---|
| 0 | `<domain:create>` 1 year | `addPeriodEnd` = +5d, `transferLockPeriodEnd` = +60d, `expiryDate` = +1y |
| 0–5 | delete inside add grace | `purgeDate` = now; purged by the next purge loop, no redemption |
| 365 | expiry loop finds `expiryDate ≤ now` | registrar opted in and TLD allows → `expiryDate` += 1y, `autoRenewPeriodEnd` = +45d, one `domain.auto_renewed` event with its quote |
| 365 | same, registrar opted out | `pendingDelete`, `redemptionPeriodEnd` = expiry + 30d, `purgeDate` = + 5d; `<rgp:infData>` shows `redemptionPeriod` |
| 365–395 | `<rgp:restore op="request"/>` | `pendingRestore`, `pendingRestoreEnd` = +7d; purge held |
| +7 | report sent | restored, renewed 1 year, restore fee + renewal charged |
| +7 | no report | restore loop reverts to `pendingDelete`, poll message to the sponsor |
| 400 | purge loop finds `purgeDate ≤ now` | domain removed, tombstone written, NNDN if `DropCatch` |

**Showing it without waiting a year.** All three loops take `DryRun` and
`ReferenceTimeOverride` (`ExpiryLoopParams`), so a run can be previewed or
executed "as of" a future instant from the workflow launchpad; the result
reports counts of auto-renewed, expired, skipped and failed, with notes. The
domain page's lifecycle widget draws every period against the policy
(`DomainLifecycleWidget.tsx`), and `<domain:info>` over EPP returns the RGP
statuses the registrar would see.

## Decision

Decided 2026-11-02 against the questions above.

1. **Lifecycle timing becomes a named policy, as EPP limits are (ADR 0019).**
   A `LifecyclePolicy` is a named, platform-wide record holding the nine
   durations and the three switches: `registrationGP`, `renewalGP`,
   `autorenewalGP`, `transferGP`, `transferLockPeriod`,
   `pendingTransferPeriod`, `redemptionGP`, `pendingdeleteGP`,
   `pendingRestorePeriod`, `restoreReportRequired`, `allowAutorenew`,
   `maxHorizon`. A phase **names** its lifecycle policy instead of carrying a
   copy. Label lengths, `requiresValidation`, `baseCurrency`, prices, fees and
   the contact data policy stay on the phase: they are commercial and launch
   settings, not lifecycle. A change to a named policy lands on every phase
   that names it at once; a phase roll remains the way to switch a TLD to a
   different policy from a known instant.

2. **`gtld-standard` is the system default.** The migration seeds one
   lifecycle policy, `gtld-standard`, with today's defaults (5/5/45/5-day
   grace periods, 60-day transfer lock, 5-day pending transfer, 30-day
   redemption, 5-day pending delete, 7-day restore report, report required,
   auto-renew allowed, 10-year horizon). It is what `NewPhase` assigns when
   no policy is named, and what the code constants in `phasePolicy.go` become.
   It is editable like any other policy; it cannot be deleted while a phase
   names it, and the name is reserved by the seed.

3. **Every duration is a pointer.** nil means "the policy's default", 0 means
   zero days. The transfer lock's convention becomes the rule, and the
   "zero means unset" merge in `UpdatePolicy` goes. A zero-day grace period
   (an OT&E or RST TLD) is then a plain setting.

4. **The policy validates itself (INV-08).** Each duration is bounded (0 to
   an upper limit per field, to be set with the implementation; a redemption
   period over a year is a typo), `maxHorizon` is 1–10 years, and the phase
   checks `minLabelLength ≤ maxLabelLength`. A policy that fails validation
   is refused at the API; nothing is clamped silently.

5. **Changing a lifecycle policy needs `registry:policy:admin`,** a platform
   permission mirroring `epp:policy:admin`: the policy is platform-wide, so
   tenant membership cannot scope it. Assigning a policy to a phase stays a
   phase edit under the operator's tenant. There is **no approval step**;
   attribution comes from the permission and the `lifecycle_policy.updated`
   event with previous and new values.

6. **Migration groups by value.** Phases whose current settings equal
   `gtld-standard` name it; the remaining distinct sets each become a named
   policy (`migrated-<tld>-<phase>`, renamed by operators afterwards). No
   phase changes behaviour on deploy. The policy columns on `phases` are
   dropped once the assignment is verified.

7. **Hourly granularity is accepted** (ADR 0004): a transition lands within
   an hour of its due time.

The rest stands as described: states and transitions are code; the current
GA phase at the moment of the event decides; the registrar's only lifecycle
setting is the auto-renew opt-in.

## Consequences

**Positive**

- One place to change a lifecycle rule for every TLD that shares it; a
  per-phase copy can no longer drift.
- `gtld-standard` is a reviewable record instead of constants in code, and
  the same seed serves every installation.
- Zero-day periods, bounds and a permission close the three gaps in section 3.

**Negative / accepted trade-offs**

- A second table and entity, a migration (ADR 0010), a new admin page and a
  new scope, before any operator has asked for two policies. Accepted on the
  strength of ADR 0019's experience.
- A phase no longer answers "what is my redemption period" on its own; every
  reader of `phase.Policy` loads the named policy (cached and reloaded as EPP
  policies are).
- Renaming a migrated policy is manual.
- `GET /tlds/{tld}/phases/{phase}` changes shape (a policy name and the
  resolved policy, instead of inline fields); the frontend's policy section,
  diff view and lifecycle widget follow. Pre-v1, no compatibility layer.
- **The release that carries the migration is a cutover, not a rolling
  deploy** (a documented pre-v1 exception to ADR 0010's expand-then-contract):
  the migration drops the phases' inline columns in the same step that
  assigns the policies, so the previous release's services fail on those
  columns once it has run. Run `migrate` with the previous release's services
  stopped, then roll the new ones; a code rollback past this release needs
  the database restored from before the migration. Staging the drop into a
  later release was rejected: nothing would have written the inline columns
  in between, and the window would have carried two sources of truth.

**Out of scope here**

- Per-registrar lifecycle overrides (rejected, 2026-11-02).
- An approval step (rejected, 2026-11-02).
- Configurable states or transitions.
