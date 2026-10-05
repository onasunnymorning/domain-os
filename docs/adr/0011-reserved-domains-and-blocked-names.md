# ADR 0011 — Reserved domains and blocked names: two concepts, two nouns

- **Status:** Proposed
- **Date:** 2026-10-02
- **Deciders:** Platform / Registry
- **Supersedes:** none
- **Composes with:** [ADR 0003](0003-dns-zone-production-and-publication.md) (zone eligibility defines "active"), [ADR 0006](0006-tenancy-model.md) (sponsorship is the registrar-side tenant key), [ADR 0010](0010-database-migrations.md) (schema changes ship through the single migrator)

## Context

In registry industry usage, a **reserved domain** is a name held for the
Registry Operator's own use or for later release. This codebase already models
that situation, but it never uses the word for it, and it uses the word for
something else.

> **Terminology note.** This is industry usage. "Reserved" has two other
> meanings this ADR does not adopt:
>
> - The Registry Agreement titles Specification 5 "Schedule of Reserved Names"
>   and uses the word for the labels on it (our `Spec5Label`). For the
>   9998/9999 arrangement the RA and IANA say "Registry Operator acts as
>   Registrar".
> - IANA's registrar ID registry gives IDs 9995–9999 the status *Reserved*.
>   We keep that word verbatim wherever we mirror IANA (`IANAStatus`, the
>   `*ReservedGurIDs` maps in `reserved_registrars.go`).
>
> The vocabulary rule in Decision 1 governs our own nouns: UI, API fields,
> docs and new identifiers. Docs should not assume either of the other two
> meanings.

**Glossary: two vocabularies**

The product keeps the industry meaning of "reserved" and maps it to the
contract's terms, so each side is used where it belongs.

| Concept | Product vocabulary (UI, docs, API) | Contract vocabulary (RA, ICANN-facing) |
|---|---|---|
| A domain the operator owns, sponsored by 9998/9999 | **Reserved domain** | Name for which the Registry Operator acts as Registrar (Spec 5 §3) |
| A label the registry will not register | **Blocked name** | "Reserved Names" (the Spec 5 schedule), and withheld names (Spec 5 §3.3, RA 2.6) |
| The ICANN schedule of reserved labels | **Specification 5 names** (category `spec5`) | Specification 5, Schedule of Reserved Names |

Rules that keep the two apart:

- The product uses "reserved" only for domains the operator owns. IANA already
  lists 9998 and 9999 as "Reserved for … Registry Operator acts as Registrar",
  so on the registrar side the product and ICANN agree.
- The ICANN schedule is labelled **Specification 5 names** in the UI. No
  label on the Blocked names page says "reserved", otherwise the clash returns
  inside that page.
- Outputs that follow the RA's wording (ICANN-facing reports and exports) use
  the contract vocabulary.

**What exists today**

- **Two operator registrar accounts per TLD.** Every TLD auto-provisions two
  registrar accounts through which the Registry Operator acts as registrar
  (`provisionOperatorRegistrars`,
  [`tld_service.go:243`](../../internal/application/services/tld_service.go#L243)):
  - `9998-{tld}`, named "{tld} - Reserved - Billable"
  - `9999-{tld}`, named "{tld} - Reserved - Non-Billable"

  Both have `IANAStatus = Reserved`. A domain sponsored by one of them is a
  normal domain object that the operator owns. "Operator account" is used
  below as shorthand for these two.
- **The domain layer only knows these GurIDs as data.**
  [`reserved_registrars.go`](../../pkg/domain/entities/reserved_registrars.go)
  declares two maps that mirror IANA's descriptions:
  - `TLDScopedReservedGurIDs` for 9998/9999
  - `SpecialReservedGurIDs` for the system-wide 9995–9997 (pre-delegation
    testing and ICANN SLA monitoring)

  Nothing has a predicate built on them, and `TLDScopedReservedGurIDs` has no
  callers. Services test `GurID == 9998 / 9999` literals instead
  ([`escrow_service.go:1227`](../../internal/application/services/escrow_service.go#L1227)).
- **Escrow import can quietly put domains on 9999.** The same block maps
  GurID `0` and `119` onto `9999-{tld}`; the code's own TODO says it is "a bit
  dangerous". Any domain imported that way becomes operator-held without
  anyone deciding it should.
- **GurID lookups for operator accounts are ambiguous.** Every TLD has its own
  9998/9999 account, so `GetByGurID(9999)` returns an arbitrary TLD's account
  ([`registrar_repository.go:63`](../../internal/infrastructure/db/postgres/registrar_repository.go#L63)
  has a TODO for this).
- **Operator accounts are not bound to their own TLD.** `Registrar.AccreditFor`
  ([`registrar.go:262`](../../pkg/domain/entities/registrar.go#L262)) has no
  notion of operator accounts:
  - For a gTLD it rejects them, because their `IANAStatus` is not
    `Accredited`.
  - The auto-provisioning path calls `accRepo.CreateAccreditation` directly,
    so it never runs that check.
- **NNDN carries free-text reasons.** NNDN
  ([`nndn.go`](../../pkg/domain/entities/nndn.go)) is the RFC 9022
  *non-standard domain name*: a label the registry declines to turn into a
  domain object. It carries an RFC state (`blocked` / `withheld` /
  `mirrored`) and a free-text `Reason`. The field's own comment calls the
  Reason "a basic form of categorization".
- **Spec 5 labels are a reference list, not a guard.** The global
  `spec5_labels` table is refreshed by `SyncSpec5` from ICANN's
  [`ReservedNames.xml`](https://www.icann.org/sites/default/files/packages/reserved-names/ReservedNames.xml)
  plus hardcoded lists in
  [`icannspec5/`](../../internal/infrastructure/web/icannspec5/).
  - Each label's `Type` is either the XML registry id (`IOC`, `red-cross1`,
    `red-cross2`, `red-cross-international`, `IGOs`, `IGOs-2`) or a section
    tag from the **2013** RA numbering (`spec5_1`, `spec5_2`, `spec5_4`,
    `spec5_5`).
  - Registration does not consult the table. `Spec5Sweep` reports
    domains that already match a label, after the fact.
- **The UI picked the wrong word.** The UI calls the NNDN list "Blocking" in
  the sidebar
  ([`Sidebar.tsx:32`](../../frontend/components/layout/Sidebar.tsx#L32)), but
  titles the TLD tile built on it **"Reserved Inventory"**
  ([`TLDReservedInventoryWidget.tsx`](../../frontend/components/tlds/TLDReservedInventoryWidget.tsx)).
  Both the operator-owned concept in the data model and the blocked concept in
  the UI are candidates for the word "reserved", and the UI chose the blocked
  one.

**What is wrong with the current shape**

1. *The vocabulary collides.* A user who asks "how many reserved domains do I
   have?" gets the NNDN count, not the count of domains the operator owns.
2. *Operator ownership has no domain-layer behaviour.* The GurIDs are declared
   as data, but "is this an operator account?" is still answered by literals in
   services.
3. *Operator accounts are not bound to their TLD,* and are looked up by an
   identifier that is not unique. See above.
4. *NNDN categories are not usable.*
   - `Reason` is free text typed as `ClIDType`, a client-ID type evidently
     reused for convenience.
   - Values in the wild are whatever a caller typed: `"RDE-import"`
     (`NNDNReasonEscrowImport`), `"Domain.DropCatch is true"` (set in two
     places in `domain_service.go`), and anything an operator enters by hand.
   - Free text cannot back a filter, a count, or a dashboard.
5. *Spec 5 is not enforced at registration.* For a gTLD, a Spec 5 label can be
   registered, and only the sweep notices.
6. *The RA's cap on operator-held names is not modelled* (see Decision 6).

### What the Registry Agreement says

This was researched on 2026-10-02 against the base RA (21 Jan 2024), the IANA
registrar ID registry, and ICANN's two-character label authorizations.
Section numbers below follow the 2024 RA. The readings marked *our reading*
should be confirmed by someone with RA authority before the implementing
change.

- **IANA.** 9998 is "Reserved for billable transactions where Registry
  Operator acts as Registrar". 9999 is "Reserved for non-billable transactions
  where Registry Operator acts as Registrar".
- **Specification 5 §3.2 (billable, capped).**
  - The Registry Operator may *activate in the DNS* up to 100 names (plus
    their IDN variants) necessary for the operation or promotion of the TLD.
  - The operator must be the registered name holder.
  - These names are Transactions for the RA §6.1 fee.
  - They are either registered through an ICANN-accredited registrar, or
    self-allocated and identified with Registrar ID **9998**.
- **Specification 5 §3.1 (non-billable, technical).**
  - `nic`, `www`, `rdds` and `whois` must be withheld or allocated to the
    operator. `nic` must be allocated and activated, as an NS zone cut.
  - IDN translations of `nic` are also allowed.
  - These names may be self-allocated without a registrar, are not
    Transactions, and are identified with Registrar ID **9999**.
- **9999 is named only for §3.1.** Specification 3 (monthly reporting) says
  self-allocations use "either 9998 or 9999 … depending on registration
  type".
  - *Our reading:* every self-allocated name that is not a Transaction and not
    activated is non-billable, so it belongs on 9999. 9998 is for activated
    §3.2 names only.
- **The cap counts activation, not holding.**
  - Specification 5 §3.3 and RA §2.6 let the operator withhold other names, or
    allocate them to itself without a registrar and without a Transaction,
    provided they are not activated. Those are not capped.
  - Operating evidence agrees. A registry that never used 9998 was flagged by
    an ICANN audit for having more than 100 operator names *delegated*. The
    audit looked at what resolved, not at which registrar ID held it.
- **Self-allocations must be visible.** Specification 5 (preamble): "If using
  self-allocation, the Registry Operator must show the registration in the
  RDDS."
- **ICANN can ask for the list.** Specification 5 §3.3: on ICANN's request,
  the operator must list every name withheld or allocated to itself under
  RA §2.6.
- **Beyond the cap there is no overflow account.** A name past 100 cannot be
  activated through 9998. It has two options:
  - stay unactivated, either as a blocked name or as a reserved domain on 9999
    that is not active
  - be registered to the operator through an ordinary accredited registrar,
    which is the out-of-scope use case below
- **Two-character labels (§2) are no longer a live reservation.**
  - ICANN released all non-letter two-character labels (digit+letter,
    letter+digit, digit+digit) on 1 Dec 2014.
  - It released all letter/letter labels on 13 Dec 2016, "not otherwise
    required to be reserved pursuant to Specification 5, Section 6". The
    release is conditional on the registry applying ICANN's measures to avoid
    confusion with country codes (Appendix A of that authorization).
  - The only two-letter labels that remain reserved are the five on ICANN's
    IGO Acronyms List: `au`, `ec`, `ep`, `eu`, `un`. They are reserved under
    §6, not §2.
- **Applicability.**
  - The RA binds gTLDs only.
  - Specification 5 §3.4 says the ICANN SLA-monitoring name neither counts
    toward the 100 nor affects .BRAND qualification. That wording implies
    `.brand` TLDs (Specification 13) are subject to the cap.
  - The position for ccTLDs was not researched.

## Decision

### 1. Two concepts, never sharing a word

|                     | **Reserved domain**                                   | **Blocked name**                                           |
| ------------------- | ----------------------------------------------------- | ---------------------------------------------------------- |
| What it is          | A real `Domain` object                                | An `NNDN`, no domain object exists                         |
| Who holds it        | The operator, as sponsoring registrar                 | Nobody; the label is unavailable                           |
| Lifecycle           | Full: delegation, renewal, transfer-out on release    | None                                                       |
| Appears in RDDS     | Yes (Specification 5 requires it)                     | No                                                         |
| Appears in zone/DNS | Yes, if active                                        | No                                                         |
| Billing             | 9998 billable, 9999 non-billable                      | n/a                                                        |
| Typical purpose     | Operator's own use; operation or promotion of the TLD | Spec 5, policy, drop-catch, future inventory, IDN variants |

"Reserved" means *the operator owns the domain*. "Blocked" means *the registry
refuses to register the name*. Neither word is used for the other anywhere:
UI, API fields, docs, code identifiers.

Billing is decoupled from the registry core and consumes domain events, so it
is not changed by this ADR.

### 2. A reserved domain is derived from sponsorship, not stored

A domain is reserved when its sponsoring registrar is a registrar through which
the Registry Operator acts as registrar, identified by GurID 9998 or 9999. The
GurID is the source of truth. `IANAStatus = Reserved` is descriptive and is not
consulted for this decision. 9995–9997 also have that status, and they are not
operator accounts.

No new column on `Domain`. A flag would duplicate what `ClID` already says
(ADR 0006: sponsorship *is* the registrar-side ownership key) and could drift
from it.

The domain layer gains the concept as behaviour, built on the existing
`TLDScopedReservedGurIDs` map in `reserved_registrars.go` and named in the
RA's terms:

- `Registrar.ActsForRegistryOperator()` returns true for GurID 9998/9999. The
  literals are declared once, in the domain layer, instead of in services
  (INV-14 direction: the domain declares the concept, outer layers use it).
- A small `ReservedKind` (billable / non-billable) is derived from the same
  GurID.
- The literals in `escrow_service.go` move to these. The import's `0`/`119` →
  9999 fallback becomes an explicit, reported decision instead of a silent
  default.

A reserved domain keeps its ordinary lifecycle and status machinery. There is
no "reserved" domain status. Releasing one is an ordinary transfer from the
operator account to a customer registrar.

Reserved domains are exported in RDE and served by RDAP like any other domain,
with the operator account as the sponsoring registrar. Neither path filters
them out: Specification 5 requires self-allocations to show in RDDS.

**Scope.** "Reserved = sponsored by 9998/9999" and nothing wider.

- Domains sponsored by 9995–9997 (pre-delegation testing, ICANN SLA
  monitoring) are not reserved domains and do not count toward the cap.
- A name the operator registers through an ordinary registrar for its own use
  is a different use case, deliberately **out of scope**, to be designed
  separately. If it is later modelled, it will be an explicit mechanism of its
  own and will not widen the meaning of "reserved" here.

### 3. Blocked names stay NNDN, gain a typed category

Add a typed `Category` to the NNDN alongside the existing RFC `NameState`.

- `NameState` stays the protocol-facing value (RFC 9022 `blocked` / `withheld`
  / `mirrored`), unchanged, because RDE and RDAP semantics depend on it.
- `Category` is our own business-level classification: a closed enum owned by
  the domain layer, so it can drive filters, counts and dashboards.
- `Reason` stays as optional human-readable free text (a note), retyped to
  `string`. It is no longer the categorisation mechanism. The one exception
  is `spec5`, below.

Proposed initial categories. The list is finalised in the implementing
change; the point of the enum is that it can grow deliberately.

| Category           | Replaces / covers                                         |
| ------------------ | --------------------------------------------------------- |
| `spec5`            | labels blocked by RA Specification 5 (see below)          |
| `policy`           | names blocked by the registry's own policy (RA §2.6)      |
| `future-inventory` | names held back for later launch or release (withheld)    |
| `drop-catch`       | `"Domain.DropCatch is true"`                              |
| `escrow-import`    | `"RDE-import"` (`NNDNReasonEscrowImport`)                 |
| `idn-variant`      | variants of an existing name (`mirrored`)                 |
| `other`            | everything uncategorised, including all pre-existing rows |

Existing rows migrate to `other`, with their old `Reason` text preserved as the
note. The column is additive and ships through the normal migrator (ADR 0010).
The known writers move to the typed values:

- `domain_service.go` (×2)
- `direct_db_importer.go`
- `nndn_commands.go`

#### Specification 5 is materialised as blocked names

For a gTLD, Specification 5 is enforced by writing its labels into that TLD's
NNDN table. Registration then refuses them through the existing blocked-name
check (`CheckDomainIsBlocked`), and nothing new is needed on the registration
path. For other TLD types, materialising is optional and off by default.

- **When.**
  - At gTLD creation, next to `provisionOperatorRegistrars`.
  - After every `SyncSpec5` run. ICANN can add IOC, Red Cross and IGO names on
    ten days' notice, so additions are propagated to every gTLD, and labels
    ICANN drops are removed.
  - Existing gTLDs are backfilled once. A label that is already registered as a
    domain is skipped and reported; the existing sweep already produces that
    report.
- **Shape.**
  - `Category = spec5`, `NameState = blocked`. In this codebase `withheld`
    means "potentially a future registrable domain", which does not describe
    these labels.
  - `Reason` is the **2024** Specification 5 section (`§1`, `§3.1`, `§4`,
    `§5`, `§6`), validated for this category. This is the one category where
    Reason is not free text.
- **The `spec5_labels` table remains the reference list.** Its `Type` maps to
  the 2024 section as follows:

  | `spec5_labels.Type` | Labels | 2024 section |
  | --- | --- | --- |
  | `spec5_1` | `example` | §1 |
  | `spec5_4` | `www`, `rdds`, `whois` (and `nic`, see below) | §3.1 |
  | `spec5_5` | country and territory names | §4 |
  | `IOC`, `red-cross1`, `red-cross2`, `red-cross-international` | IOC and Red Cross identifiers | §5 |
  | `IGOs`, `IGOs-2` | IGO identifiers and acronyms (includes `au`, `ec`, `ep`, `eu`, `un`) | §6 |
  | `spec5_2` | the same five two-letter labels | §6, not §2 (see Context) |

  `spec5_2` is already redundant. The five labels also appear in `IGOs-2`, and
  `removeDuplicates` keeps the XML entry because it is read first. The
  implementing change drops the hardcoded list and corrects its comment.
- **`nic` is the exception.** §3.1 requires it to be allocated and activated,
  so it is a reserved domain on `9999-{tld}`, not a blocked name. Creating it
  is a TLD onboarding step.
- **Release is governed by section and sub-list.** The sub-list is recovered
  by joining the label back to `spec5_labels`.
  - **Never released to a third party:** §1, §3.1, IGO acronyms (`IGOs-2`) and
    Red Cross acronyms (`red-cross2`).
  - **Released with government agreement and ICANN approval:** §4.
  - **Released through the exception procedure** in ICANN's IGO/INGO
    protection policy: the remaining §5 and §6 lists.

  A `spec5` row on a gTLD cannot be deleted by hand. It leaves only through
  one of these routes.

**The §3.3 listing.** If ICANN asks for every name withheld or allocated under
RA §2.6, the answer is:

- NNDN rows in categories `policy` and `future-inventory`, plus
- domains sponsored by `9999-{tld}` whose label is not a §3.1 label.

`spec5` rows are not part of it: ICANN publishes those lists itself.

### 4. Moving between states is explicit

- **Reserved → released:** a transfer from 9998/9999 to a customer registrar.
  This is already possible; no new verb.
- **Blocked → reserved:** delete the NNDN and create the domain under
  9998/9999.
  - This is a genuinely new operation. It must be one auditable action,
    because between the two steps the name is briefly registrable by anyone.
  - For `spec5` rows it is allowed only where the section permits the
    operator to hold the name.
- **Non-billable → billable (9999 → 9998):** a sponsor change between the two
  operator accounts of the same TLD.
  - It is the only way a non-§3.1 reserved domain becomes active on a gTLD
    (Decision 6), so it is one audited action that passes the cap check.
  - The reverse (9998 → 9999) is allowed only while the domain is not active.
- **Blocked → available:** delete the NNDN. The name is then registrable under
  the normal phase and launch rules.
  - This already works as a plain delete. It should emit an audit event
    carrying the category the name had, so releasing inventory is traceable.
  - `mirrored` names are the exception: they exist because of the base name
    they mirror and are released with it, not by hand.
  - `spec5` rows follow the release rules in Decision 3.

### 5. Presentation

- **Sidebar.**
  - It gains **Reserved**: the domain list filtered to operator sponsorship.
    For a gTLD it shows usage against the cap, e.g. "37 of 100 active".
  - **Blocked names** sits next to it (the current "Blocking" page). "Blocked
    names" is a proposal: it matches the noun used everywhere else and carries
    the Category. UX can still change the label without affecting this ADR.
- **NNDN does not appear in the UI.**
  - Today the only user-visible occurrence is the "NNDNs" field label on the
    synthetic escrow form
    ([`escrow/synthetic/page.tsx:141`](../../frontend/app/escrow/synthetic/page.tsx#L141)).
    That becomes "Blocked names".
  - "NNDN" stays the technical term in code, the database and the API, where
    it is the RFC name.
  - The page route moves from `/nndns` to a name that matches the label, with
    a redirect.
- **Columns.** Category is the primary grouping. The RFC state (`blocked` /
  `withheld` / `mirrored`) shows as a secondary "Status" column. For `spec5`
  rows the section is shown with the category. The `spec5` category is
  labelled **Specification 5 names**, never "reserved" (see the glossary).
- **TLD page.** It shows two tiles:
  - **Reserved domains:** count, billable vs non-billable.
  - **Blocked names:** count, by category.

  The existing widget is repointed to the blocked noun and renamed.
- **Domain detail** shows a **Reserved** badge when the sponsor is an operator
  account.

### 6. The RA's 100-name cap is a TLD policy, enforced on activation

- **The cap value.** A TLD carries an operator activation cap. For a gTLD
  (`TLDTypeGTLD`) it defaults to 100. For other TLD types it defaults to
  unlimited, since the RA does not bind them.
- **"Active" means eligible for the zone, as defined by ADR 0003.** That is: at
  least one linked host and no `serverHold` / `clientHold`. The predicate lives
  in the domain layer and is shared by the zone generator and the cap. If the
  cap used its own definition, it could disagree with what is actually
  published, and what is published is what an audit measures.
- **9998.** The cap counts domains sponsored by `9998-{tld}` that are active.
  - A 9998 domain may always be created inactive.
  - The cap is checked at the moment a 9998 domain becomes active, and
    whenever a domain moves to 9998 while active.
- **9999 on a gTLD.** A 9999 domain may become active only if its label is a
  §3.1 label (`nic`, `www`, `rdds`, `whois`, or an IDN translation of `nic`).
  - Any other 9999 domain may exist, since §3.3 allows allocation, but must
    stay inactive.
  - To activate it, move it to 9998 (Decision 4), which applies the cap.
  - This closes the route the audit found, where unbounded activations went
    through the non-billable account. It does not forbid lawful
    allocation-without-activation. This rests on *our reading* of
    Specification 3 above.
- **Other TLD types.** 9999 is unrestricted.
- **The rules live in the domain layer.** The service supplies the count and
  the label check.
- **Errors name the alternatives.** Breaching the cap, or activating a
  non-§3.1 name on 9999, is rejected with an error that names what can be done
  instead:
  - keep the name inactive (on 9999, or as a blocked name)
  - move it to 9998 within the cap
  - register it through an accredited registrar

### 7. Operator accounts are bound to their own TLD

- **One TLD per account.** The TLD an operator account serves is the one its
  ClID names (`9998-{tld}`, `9999-{tld}`). `Registrar.AccreditFor` gains the
  rule:
  - An operator account can be accredited for that TLD only, for every TLD
    type.
  - It is exempt from the "must be ICANN accredited" check for gTLDs, because
    it is not a third-party registrar.
- **No bypass at provisioning.** `provisionOperatorRegistrars` goes through
  `AccreditFor` instead of calling the repository directly, so the one path
  that currently skips the domain layer can no longer bypass the guard.
- **REST picks it up for free.** The accreditation REST path already runs
  `AccreditFor`.
- **Look up by ClID, not GurID.** An operator account is always resolved by
  its ClID (`9999-{tld}`), never by GurID, because the GurID is shared by
  every TLD's account. `GetByGurID` is not used for 9998/9999.

## Deferred (decided, but not part of this change)

- **`withheld` and `blocked` stay distinct** for now. The target state is
  fluent category handling in the UI, where the operator works in terms of
  category and the RFC state is derived or defaulted from it. That is a UX
  evolution on top of `Category`, not a prerequisite for it.
- **Operator-owned names under ordinary registrars** (see Decision 2, scope).

## Consequences

- **One word, one meaning.** The wrong "Reserved Inventory" label is corrected
  at the source rather than papered over.
- **No new table and no flag on `Domain`.** Reserved stays a pure projection
  of sponsorship, so it cannot disagree with it.
- **One additive NNDN column and a one-time backfill.** Category filters and
  counts become reliable, instead of substring matches on free text.
- **Specification 5 becomes enforced at registration for gTLDs,** through the
  existing blocked-name check. The sweep remains as a safety net.
- **The audit failure mode becomes impossible.** Activating an operator name
  beyond the cap, or through 9999, is refused at the point of activation.
- **New TLD-level state.** A new policy value (the activation cap), plus a
  count query on active domains sponsored by 9998.
- **Costs.**
  - A rename across UI and docs.
  - Every NNDN writer must choose a category.
  - Spec 5 rows per gTLD (a few thousand, scaling with the IGO/IOC lists)
    must be kept in step with `SyncSpec5`.
  - "Blocked → reserved" and "9999 → 9998" must be built as single audited
    actions.
  - The cap depends on ADR 0003's zone-eligibility predicate being extracted
    into the domain layer.
- **Registry totals still include reserved domains,** because they remain
  real domains. Reporting that wants "registered excluding operator" needs to
  filter on sponsorship explicitly.

## Open questions

- **Who confirms the RA reading.** These rest on our reading and should be
  checked by someone with RA authority before Decision 6 is implemented:
  - 9999 for non-activated, non-§3.1 self-allocations
  - activation as the measure of the cap
- **Materialised rows vs a lookup.** Decision 3 writes Spec 5 into each gTLD's
  NNDN table, which keeps registration on one check and makes the rows visible
  in the UI. The alternative is a registration-time lookup against
  `spec5_labels`. It avoids the per-TLD copies, but the labels would then be
  invisible in the blocked-names inventory. Revisit if the per-TLD volume
  becomes a problem.
- **Release workflows** for §4 (government agreement) and for the IGO/INGO
  exception procedure are not designed here. Until they are, those rows are
  released only by an administrator, with the evidence attached to the audit
  event.
- **Two-character confusion measures.** Appendix A of ICANN's 2016
  authorization should become a registration-policy check on two-letter
  labels. Its content has not yet been reviewed for this ADR.
- **Cap position for ccTLDs and other non-gTLD types** was not researched.
  Decision 6 defaults them to unlimited.
