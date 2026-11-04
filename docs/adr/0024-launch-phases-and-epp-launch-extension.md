# ADR 0024 — Launch phases and the EPP launch extension

- **Status:** Accepted (sections 1 to 3 are implemented: the model in #537, first-come phases over EPP in #538, applications in #539, allocation in #540, the admin UI in the PR after; section 4.1, the TMCH data, is implemented; 4.2, sunrise and claims, is deferred to roadmap 9b–9c)
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

## 2. RFC 8334 over EPP

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
| `<domain:create>` + `<launch:create>` (type `registration` or none) | registered at once under the phase's prices, fees and policy, answered as a GA create is: `<launch:creData>` is for applications only (§3.3.1) | with no type or `type="application"`: an application is filed, 1001 with `<launch:creData>` carrying the server-assigned `applicationID`; `type="registration"` is 2306 `ErrLaunchCreateTypeMismatch` |
| `<domain:create>` + `<launch:create type="application">` | 2306 `ErrLaunchCreateTypeMismatch` | as above |
| `<domain:create>` in a sunrise or claims phase, or carrying marks, codes or notices | 2306 | 2306 |
| `<domain:info>` + `<launch:info>` with `applicationID` | 2303: the ID names no application | `<domain:infData>` from the application (its ID as the ROID, `pendingCreate` while not final) and `<launch:infData>` with the phase, the ID and its status; another registrar's is 2201 |
| `<domain:update>` / `<domain:delete>` + `<launch:update>` / `<launch:delete>` | 2303 | changes (contacts, nameservers, DS records, code) or withdraws a non-final application, 1000; a final one is 2304; statuses and restores are 2306 |

A `<launch:phase>` the server sends back (`<launch:creData>`, `<launch:infData>`)
carries the phase's mode as its value and the phase's name as its `name`
attribute, so a registrar can name the phase back either way.

A plain `<domain:check>` or `<domain:create>` without the extension is the
GA phase's, as before.

Application IDs are server-assigned and `roidType`-valid, carrying the
installation's ROID repository ID (ADR 0017) with a prefix of their own.
Statuses (§2.4): `pendingValidation → pendingAllocation → allocated |
rejected`, plus `invalid` when validation fails; `validated` and `custom` are
not used. A refusal's result code is in
[`docs/epp/POLICY_REFERENCE.md`](../epp/POLICY_REFERENCE.md) as each command
lands.

## 3. Applications and allocation

An application (`entities.LaunchApplication`, the `launch_applications`
table) holds what a create holds (contacts, nameservers by name, DS records,
the period, the authorization code hashed as a domain's is) plus the quote
computed when it was filed, in the phase's base currency. **Nothing is
charged at filing.** Its ID is minted as a ROID with the `APP` object
identifier (`<n>_APP-<repository>`), so it is `roidType`-valid and stands
as the `<domain:roid>` in `<domain:infData>`. A create passes every rule a
registration passes (the name is neither registered nor reserved, the
phase's label rules, the contacts and hosts are the registrar's) before it
is filed; filed applications do not make a name unavailable to other
applicants, and a registrar may hold **one open application per name and
phase** (a partial unique index over the non-final statuses), filing again
once it is final or withdrawn. The registrar that filed an application is
the only one that may read, change or withdraw it (ADR 0006); a change
replaces what a domain update would (contacts, nameservers, DS records, the
code), never statuses, and is refused once the application is final. A
phase with applications filed can no longer change its registration model.

An operator **allocates one application per name through the admin API**
(`POST /launch-applications/{id}/validate|allocate|reject`, validate taking an
explicit `valid` true or false with a reason when false; listing them at
`GET /tlds/{tld}/phases/{phase}/applications`); deciding needs the
`registry:launch:admin` permission within the TLD's tenant, reading only the
tenant. Allocation registers the domain in the application's phase, which may
have ended, through the registration path (`DomainService.RegisterApplication`),
with the applicant's hashed code and **the quote from filing**, so the
`domain.registered` event bills what the applicant was quoted; it rejects the
other open applications for the name and queues each applicant a poll message
carrying `<launch:infData>` with the application's new status, all in one
transaction. Validation (`pendingValidation` → `pendingAllocation`, or
`invalid` with a reason) and rejection queue a message too. There is no
allocation workflow: the decision (auction, lottery, manual) is the
operator's, and a loop cannot take it.

## 4. Sunrise, claims and TMCH (roadmap 9a–9c)

Sunrise and claims need data that is not a phase's and not a TLD's: what
the Trademark Clearinghouse publishes to every registry. 9a stores it; 9b
and 9c read it.

### 4.1 TMCH data (9a, implemented)

The Clearinghouse publishes three data sets, each as a complete file with
its generation time, never a delta:

- the **TMCH CA certificate bundle** (PEM), which a signed mark's XML
  signature (RFC 7848) is verified against;
- the **SMD revocation list** (SMDRL; CSV: version and generation time,
  then `smd-id,insertion-datetime`), the signed marks the Clearinghouse has
  revoked;
- the **DNL list** (CSV: version and generation time, then
  `DNL,lookup-key,insertion-datetime`), the labels that match a trademark
  record, each with the key a registrar presents for the claims notice.

They are **platform-wide data in typed tables** (ADR 0018): `tmch_sets`
(one row per set: generation time, fetch time, source, entry count, and
the bundle's expiry), `tmch_certificates`, `tmch_smd_revocations` keyed by
SMD ID and `tmch_dnl_labels` keyed by label, which is what the EPP path
will look up: one indexed row, never the list. `entities.TMCHCertificates`,
`SMDRevocationList` and `DNLList` validate what is parsed (a CA bundle with
no valid CA certificate, a list of an unknown version, a label that is not
a label, a duplicate entry are refused whole).

**Refresh.** The `refresh-tmch` schedule (every `TMCH_REFRESH_INTERVAL`,
6h by default, at least hourly) runs one activity through
`services.TMCHService.Refresh`: fetch each set and have the repository
store it when it supersedes the stored one (a later generation; for the CA
bundle, which has no generation of its own, also different contents). The
repository decides under the set's advisory lock, re-reading the stored
header there, and replaces the set whole in one transaction with its
`tmch.<set>.refreshed` outbox event (INV-01): a reader never sees a partial
list, the same generation fetched twice writes nothing but its fetch time,
and a slow refresh behind a fast one cannot put an older list back. A row
of a list that is not an entry (an invalid label, a duplicate) is left out
and counted rather than failing the list; a file that loses more rows than
it keeps is refused. A set that fails leaves the stored one in place and
fails the run, which Temporal retries. The admin API runs the same refresh
on `POST /tmch/refresh` (`registry:policy:admin`).

**Where the feed is configured.** The URLs and the registry's TMDB
credentials are deployment settings (ADR 0018: environment, `TMCH_CA_URL`,
`TMCH_SMDRL_URL`, `TMCH_DNL_URL`, `TMCH_USERNAME`, `TMCH_PASSWORD`,
`TMCH_REFRESH_INTERVAL`), because they differ between the pilot and the
production Clearinghouse and the password is a secret. The URLs have
defaults, so the credentials are what makes an installation a TMCH client:
without them the feed is not configured, the schedule exists and does
nothing, the admin API says so, and no freshness alert fires, since such an
installation runs no sunrise or claims phase. The credentials go only to
the configured hosts: a redirect to another host or scheme is refused.

**Freshness.** `GET /tmch` reports each set's generation and fetch time.
The admin API serves them as `tmch_set_generated_timestamp_seconds`,
`tmch_set_fetched_timestamp_seconds`, `tmch_set_entries` and
`tmch_certificates_not_after_timestamp_seconds`, read from the table on
each scrape (the worker that refreshes has no metrics port, and the table
is what every process reads). `prometheus/rules/tmch.rules.yml` warns when
a stored list's generation is over 36 hours old (both lists are published
at least daily and fetched every 6 hours, so this is a day of failed
refreshes), when a configured feed has never loaded a set, and 30 days
before the CA bundle expires; each links to its section of
[`docs/epp/RUNBOOK.md`](../epp/RUNBOOK.md#tmch-data-is-stale).

### 4.2 Deferred: sunrise and claims (9b, 9c)

A `LaunchVerifier` port (verify a signed mark against the stored bundle and
SMDRL, verify a claims notice) will sit in the domain layer with its
implementation in infrastructure, reading the repository above. Until then
sunrise- and claims-mode phases exist, can be priced and planned, and
refuse registrations over EPP with 2306.

## Consequences

- A TLD can run open and landrush launch phases over EPP, by first come or
  by application, with their own prices and policy, before or beside GA.
- One rule chooses the phase everywhere; the admin API no longer accepts an
  inactive or GA phase by name (a pre-v1 break, deliberate).
- A registrar can register in an open or landrush first-come phase over EPP
  today; the greeting's `launch-1.0` satisfies RST epp-02/8.
- `Phase` gains one value object, not a new shape; the TMCH data is
  platform-wide tables, and sunrise and claims will add a verifier, not
  entity fields.
- `GetCurrentGAPhase` now includes the start instant; a registration at
  exactly a phase's start belongs to it.
- Invariants: entities import nothing inward (INV-14); the actor stays on
  the typed context key (INV-15); no new workflow (INV-06 untouched).
