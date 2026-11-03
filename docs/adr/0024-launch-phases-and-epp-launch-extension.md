# ADR 0024 — Launch phases and the EPP launch extension

- **Status:** Proposed (sections 1 and 2 are implemented; sections 3 and 4 are the design the following PRs build)
- **Implemented so far:** 2026-11-03 — `entities.LaunchSettings` on `Phase` (`launch_*` columns on `phases`), `TLD.RegistrationContextAt` as the one rule for which phase governs a registration, `Domain.RegistrationPhaseName`, `PUT /tlds/{tld}/phases/{phase}/launch`, time-aware activity on `Phase` (`IsActiveAt`, `HasStartedAt`, `HasEndedAt`) and the `GetCurrentLaunchPhases` fix (roadmap 8a). The `launch-1.0` extension over EPP for first-come phases: the schema models, the greeting, `<launch:check type="avail">` and `<launch:create>` (roadmap 8b).
- **Date:** 2026-11-03
- **Deciders:** Platform / Registry
- **Related:** [ADR 0023](0023-configurable-domain-lifecycle.md) (the lifecycle reads only the GA phase), [ADR 0018](0018-where-configuration-lives.md) (typed columns, not JSON), [ADR 0016](0016-epp-schema-validation-in-go.md) (how an EPP extension is bound), [ADR 0017](0017-roid-repository-id.md) (identifiers), [ADR 0006](0006-tenancy-model.md) (who may change a TLD)

## Context

Before this record, every domain command, over EPP and the admin API alike,
resolved the TLD's *current GA phase*: `EPPDomainService.tldForCheck`,
`DomainService.RegisterDomain`, and the fourteen lifecycle call sites ADR
0023 counts. Launch phases existed as data (a `Phase` of type `Launch` with
dates, prices, fees, a premium list and the shared `PhasePolicy`) but
governed nothing. The admin API could name a phase on registration and did
not check that it was active, or a launch phase at all. Nothing parsed or
emitted RFC 8334's `launch-1.0` namespace; the RST catalogue asks for it in
the greeting (epp-02/8).

Launching a TLD needs sunrise, landrush and claims periods a registrar can
address over EPP. This record decides what a launch phase *is* in the model,
how a registration chooses its phase, and how RFC 8334 maps onto both. Sunrise
and claims verification (signed marks, notices, the TMCH data feeds) are
**deferred, not dropped**: the model leaves them a typed place and EPP refuses
them until they land (roadmap 9a–9c in [`docs/epp/ARCHITECTURE.md`](../epp/ARCHITECTURE.md)).

## 1. The model (implemented)

**A launch phase carries `LaunchSettings`** (`pkg/domain/entities/launch_settings.go`),
a value object beside `PhasePolicy`, as typed columns on `phases` (ADR 0018):

| Field | Values | Meaning |
|---|---|---|
| `Mode` | `open`, `landrush`, `custom`, `sunrise`, `claims` | RFC 8334 §2.3: the value a registrar sends in `<launch:phase>`. Sunrise and claims `RequiresVerification()`; EPP refuses registrations in them (2306) until the verifier exists. |
| `Model` | `first-come`, `application` | A create registers the domain at once, or files an application the registry allocates later (§2.1). |
| `AcceptSignedMarks`, `AcceptCodeMarks` | bool | Which trademark evidence a sunrise accepts (§2.6). A sunrise must accept one. Stored now, read when sunrise lands. |
| `RetainMarks` | bool | Whether marks are kept with applications (§3.1.2 `includeMark`). |

A new launch phase is `open` / `first-come` / signed marks accepted. A GA
phase carries the zero value and refuses the settings. **The mode is fixed
once the phase has started; the model once it has started or has
applications** (`Phase.SetLaunchSettings`), because applications already
filed cannot become registrations or the reverse. Prices, fees and the mark
settings may change until the phase ends, as today. `PhaseType` stays
`GA`/`Launch`: GA overlap rules and the `activeGAPhaseFilter` key on it.

**One rule for which phase governs a registration:** `TLD.RegistrationContextAt(ref, now)`.
A `PhaseRef` names a phase by name (the admin API's `PhaseName`, the `name`
attribute of `<launch:phase>`), by mode (the element's value), or both. No
ref is the GA phase in force (`ErrNoActivePhase`); GA is never named
(`ErrPhaseNotLaunch`). A name must be a launch phase of this TLD, of the mode
given if one is, and active at `now`. A mode alone picks the one active
launch phase of that mode: none is `ErrPhaseNotActive`, two or more
`ErrLaunchPhaseAmbiguous`, so a registrar need name the phase only when two
of one mode are open at once. The result is a `RegistrationContext{Phase,
Claims}`: `Claims` is the active claims-mode phase, if any, whatever `Phase`
is, because a claims period usually overlays GA and a name on the claims list
then needs a notice however it is registered (§3.1.3). Nothing reads `Claims`
yet; it is there so the rule does not change when claims support arrives.

The admin API's `POST /domains/{name}/register` and `GET /domains/{name}/available`
follow this rule (an ended, future or GA phase named is a 400). **Renew,
transfer, delete and restore of a registered domain stay with the GA phase in
force** (ADR 0023 §2): a launch phase governs how a name is *obtained*, not
its later lifecycle.

**Phase activity takes the instant as a parameter.** `Phase.IsActiveAt(now)`,
`HasStartedAt`, `HasEndedAt`, `TLD.GAPhaseAt(now)` and
`TLD.ActiveLaunchPhases(mode, now)` are the rule; the clock-reading forms
wrap them. The interval is `[Starts, Ends)` everywhere (the start instant
belongs to the phase, the end instant to the next); `GetCurrentGAPhase` used
to exclude the start instant and now agrees. An allocation after a launch
phase has ended is expressed by registering in the phase as recorded, not by
asking whether it is active.

**A domain records the launch phase it was registered in**
(`Domain.RegistrationPhaseName`, `domains.registration_phase_name`, NULL for
GA and for every domain from before). Lifecycle decisions do not read it.

**`PhasePolicy.RequiresValidation`** keeps its meaning for first-come
registrations (the domain starts `pendingCreate`) and gains one for
applications: an application starts `pendingValidation` when it is set, else
`pendingAllocation`.

## 2. RFC 8334 over EPP (first-come phases implemented; applications next)

The server offers `urn:ietf:params:xml:ns:launch-1.0` in the greeting and
accepts it at login; a command carrying the extension on a session that did
not negotiate it is 2103, as for every extension. The `launch-1.0`,
`mark-1.0`, `signedMark-1.0` and XML Signature schemas are pinned from the
RST release so xmllint can load the launch schema, and the launch elements
a client sends are modelled per ADR 0016. Of the trademark evidence a
`<launch:create>` may carry, only `<smd:encodedSignedMark>` (the form SMD
files take) and `<launch:codeMark>` with a `<launch:code>` are modelled; a
decoded `<smd:signedMark>` or inline `<mark:mark>` is 2001 until sunrise
support models the mark and signature schemas it verifies.

**`<launch:phase>` names the phase** as `RegistrationContextAt` resolves it:
the element's value is the mode, its `name` attribute the phase's name. A
registrar sends `<launch:phase>landrush</launch:phase>` when one landrush
phase is open, and names the phase only when two of one mode are.

| Command | In a first-come phase | In an application phase |
|---|---|---|
| `<domain:check>` + `<launch:check type="avail">` | availability under the phase's label rules, answered in the plain `<domain:chkData>` with no extension (§3.1.1.2) | the same; filed applications do not make a name unavailable |
| `<launch:check>` of the claims form (the default) or the trademark form | 2102 until claims support lands | |
| `<domain:create>` + `<launch:create>` (type `registration` or none) | registered at once under the phase's prices, fees and policy, answered as a GA create is: `<launch:creData>` is for applications only (§3.3.1) | 2306 until applications land |
| `<domain:create>` + `<launch:create type="application">` | 2306 `ErrLaunchCreateTypeMismatch` | 1001 and `<launch:creData>` with a server-assigned `applicationID` (next) |
| `<domain:create>` in a sunrise or claims phase, or carrying marks, codes or notices | 2306 | 2306 |
| `<domain:info>` + `<launch:info>` with `applicationID` | 2306 until applications land | `<domain:infData>` from the application and `<launch:infData>` with its status (next) |
| `<domain:update>` / `<domain:delete>` + `<launch:update>` / `<launch:delete>` | 2306 until applications land | changes or withdraws a non-final application; a final one is 2304 (next) |

A plain `<domain:check>` or `<domain:create>` without the extension is the
GA phase's, as before.

Application IDs are server-assigned and `roidType`-valid, carrying the
installation's ROID repository ID (ADR 0017) with a prefix of their own.
Statuses (§2.4): `pendingValidation → pendingAllocation → allocated |
rejected`, plus `invalid` when validation fails; `validated` and `custom` are
not used. A refusal's result code is in
[`docs/epp/POLICY_REFERENCE.md`](../epp/POLICY_REFERENCE.md) as each command
lands.

## 3. Applications and allocation (next PRs)

An application holds what a create holds (contacts, nameservers, DS records,
the period, the authorization code) plus the quote computed when it was
filed, checked against the fee extension as a registration's is. **Nothing
is charged at filing.** An operator **allocates one application per name
through the admin API** (`validate`, `allocate`, `reject`); allocation
registers the domain in the application's phase, which may have ended,
rejects the other applications for the name in the same transaction, and
sends each applicant a poll message carrying `<launch:infData>`. The
`domain.registered` event, with the stored quote, is published at
allocation. There is no allocation workflow: the decision (auction, lottery,
manual) is the operator's, and a loop cannot take it.

## 4. Deferred: sunrise, claims and TMCH (roadmap 9a–9c)

A `LaunchVerifier` port (verify a signed mark, verify a claims notice) will
sit in the domain layer with its implementation in infrastructure. It needs
platform-wide data that is not a phase's: the TMCH CA certificates, the SMD
revocation list and the claims (DNL) list, refreshed on a schedule. Until
then sunrise- and claims-mode phases exist, can be priced and planned, and
refuse registrations over EPP with 2306.

## Consequences

- A TLD can run open and landrush launch phases over EPP, by first come or
  by application, with their own prices and policy, before or beside GA.
- One rule chooses the phase everywhere; the admin API no longer accepts an
  inactive or GA phase by name (a pre-v1 break, deliberate).
- A registrar can register in an open or landrush first-come phase over EPP
  today; the greeting's `launch-1.0` satisfies RST epp-02/8.
- `Phase` gains one value object, not a new shape; sunrise and claims will
  add a verifier and data feeds, not entity fields.
- `GetCurrentGAPhase` now includes the start instant; a registration at
  exactly a phase's start belongs to it.
- Invariants: entities import nothing inward (INV-14); the actor stays on
  the typed context key (INV-15); no new workflow (INV-06 untouched).
