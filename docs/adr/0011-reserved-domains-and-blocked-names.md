# ADR 0011 — Reserved domains and blocked names: two concepts, two nouns

- **Status:** Proposed
- **Date:** 2026-10-02
- **Deciders:** Platform / Registry
- **Supersedes:** none
- **Composes with:** ADR 0006 (sponsorship is the registrar-side tenant key), ADR 0010 (schema changes ship through the single migrator)

## Context

In registry industry usage, a **reserved domain** is a name held for the
Registry Operator's own use or for later release. This codebase already models
that situation, but it never uses the word for it, and it uses the word for
something else.

### Glossary: two vocabularies

"Reserved" means different things in industry usage and in the Registry
Agreement. This ADR keeps the industry meaning for the product and maps it to
the contract's terms, so each side is used where it belongs.

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

### What exists today

- Every TLD auto-provisions two registrar accounts through which the Registry
  Operator acts as registrar (`provisionOperatorRegistrars`,
  [`tld_service.go:243`](../../internal/application/services/tld_service.go#L243)):
  `9998-{tld}` named "Reserved - Billable" and `9999-{tld}` named
  "Reserved - Non-Billable", both with `IANAStatus = Reserved`. A domain
  sponsored by one of them is a normal domain object that the operator owns.
  "Operator account" is used below as shorthand for these two.
- Nothing stops an operator account from being accredited for a TLD other than
  its own. `Registrar.AccreditFor`
  ([`registrar.go:262`](../../pkg/domain/entities/registrar.go#L262)) has no
  notion of operator accounts: for a gTLD it rejects them (their `IANAStatus`
  is not `Accredited`), while the auto-provisioning path writes through the
  repository directly and so never runs that check.
- NNDN (`pkg/domain/entities/nndn.go`) is the RFC 9022 *non-standard domain
  name*: a label the registry declines to turn into a domain object. It carries
  an RFC state (`blocked` / `withheld` / `mirrored`) and a free-text `Reason`
  whose own comment calls it "a basic form of categorization".
- The UI calls the NNDN list "Blocking" in the sidebar
  ([`Sidebar.tsx:32`](../../frontend/components/layout/Sidebar.tsx#L32)) but
  titles the TLD tile built on it **"Reserved Inventory"**
  ([`TLDReservedInventoryWidget.tsx`](../../frontend/components/tlds/TLDReservedInventoryWidget.tsx)).
  So the operator-owned concept in the data model and the blocked concept in
  the UI are both candidates for the word "reserved", and the UI picked the
  wrong one.

### What is wrong with the current shape

1. *The vocabulary collides.* A user who asks "how many reserved domains do I
   have?" gets the NNDN count, not the count of domains the operator owns.
2. *Operator ownership has no domain-layer expression.* "Is this an operator
   account?" is answered by `GurID == 9998 / 9999` literals in services
   (`escrow_service.go:1227`) rather than by the registrar entity.
3. *Operator accounts are not bound to their TLD.* See above.
4. *NNDN categories are not usable.* `Reason` is free text typed as `ClIDType`
   (a client-ID type, evidently reused for convenience). Values in the wild are
   whatever a caller typed: `"RDE-import"` (`NNDNReasonEscrowImport`),
   `"Domain.DropCatch is true"` (set in two places in `domain_service.go`), and
   anything an operator enters by hand. Free text cannot back a filter,
   a count, or a dashboard.
5. *The RA's cap on operator-held names is not modelled* (see Decision 6).

### What the Registry Agreement says

Researched 2026-10-02 against the base RA (21 Jan 2024) and the IANA registrar
ID registry. The Specification 5 §3.1/§3.2 wording should be checked against
the agreement text by someone with RA authority before the implementing change.

- **IANA:** 9998 is "Reserved for billable transactions where Registry
  Operator acts as Registrar"; 9999 is "Reserved for non-billable transactions
  where Registry Operator acts as Registrar".
- **Specification 5 §3.2 (billable, capped).** The Registry Operator may
  *activate in the DNS* up to 100 names (plus their IDN variants) necessary for
  the operation or promotion of the TLD. The operator must be the registered
  name holder. These names are Transactions for the RA 6.1 fee. They are
  either registered through an ICANN-accredited registrar or self-allocated
  and identified with Registrar ID **9998**.
- **Specification 5 §3.1 (non-billable, technical).** `nic` (required),
  `www`, `rdds` and `whois`, plus IDN translations of `nic`, may be
  self-allocated without a registrar and are not Transactions. They are
  identified with Registrar ID **9999**.
- **The cap counts activation, not holding.** Specification 5 §3.3 and RA 2.6
  let the operator withhold other names, or allocate them to itself, provided
  they are not activated. Those are not capped.
- **Beyond the cap there is no overflow account.** A name past 100 cannot be
  activated through 9998. It stays unactivated (in our terms, a blocked name in
  state `withheld`), or is registered to the operator through an ordinary
  accredited registrar, which is the out-of-scope use case below.
- **Applicability.** The RA binds gTLDs only. `.brand` TLDs (Specification 13)
  are not exempt from the cap. Position for ccTLDs was not researched.

## Decision

### 1. Two concepts, never sharing a word

| | **Reserved domain** | **Blocked name** |
|---|---|---|
| What it is | A real `Domain` object | An `NNDN`, no domain object exists |
| Who holds it | The operator, as sponsoring registrar | Nobody; the label is unavailable |
| Lifecycle | Full: delegation, renewal, transfer-out on release | None |
| Appears in zone/DNS | Yes, if delegated | No |
| Billing | 9998 billable, 9999 non-billable | n/a |
| Typical purpose | Operator's own use; operation or promotion of the TLD | Policy, Spec 5, drop-catch, future inventory, IDN variants |

"Reserved" means *the operator owns the domain*. "Blocked" means *the registry
refuses to register the name*. Neither word is used for the other anywhere:
UI, API fields, docs, code identifiers.

Billing is decoupled from the registry core and consumes domain events, so it
is not changed by this ADR.

### 2. A reserved domain is derived from sponsorship, not stored

A domain is reserved when its sponsoring registrar is a registrar through which
the Registry Operator acts as registrar, identified by GurID 9998 or 9999. The
GurID is the source of truth. `IANAStatus = Reserved` is descriptive and is not
consulted for this decision.

No new column on `Domain`. A flag would duplicate what `ClID` already says
(ADR 0006: sponsorship *is* the registrar-side ownership key) and could drift
from it.

The domain layer gains the concept explicitly, in `pkg/domain/entities`, using
the RA's terminology:

- `Registrar.ActsForRegistryOperator()` — true for GurID 9998/9999. The
  literals live in one place in the domain layer instead of in services
  (INV-14 direction: the domain declares the concept; outer layers use it).
- A small `ReservedKind` (billable / non-billable) derived from the same.
- The `9998`/`9999` literals in `escrow_service.go` and
  `registrar_repository.go` move to these.

A reserved domain keeps its ordinary lifecycle and status machinery. There is
no "reserved" domain status. Releasing one is an ordinary transfer from the
operator account to a customer registrar.

Reserved domains are exported in RDE like any other domain, with the operator
account as the sponsoring registrar. They get no special handling.

**Scope.** "Reserved = sponsored by 9998/9999" and nothing wider. A name the
operator registers through an ordinary registrar for its own use is a different
use case, deliberately **out of scope**, to be designed separately. If it is
later modelled, it will be an explicit mechanism of its own and will not widen
the meaning of "reserved" here.

### 3. Blocked names stay NNDN, gain a typed category

Add a typed `Category` to the NNDN alongside the existing RFC `NameState`.

- `NameState` stays the protocol-facing value (RFC 9022 `blocked` / `withheld`
  / `mirrored`), unchanged, because RDE and RDAP semantics depend on it.
- `Category` is our own business-level classification, a closed enum owned by
  the domain layer, so it can drive filters, counts and dashboards.
- `Reason` stays as optional human-readable free text (a note), retyped to
  `string`. It is no longer the categorisation mechanism.

Proposed initial categories (to be finalised in the implementing change; the
point of the enum is that this list can grow deliberately):

| Category | Replaces / covers |
|---|---|
| `drop-catch` | `"Domain.DropCatch is true"` |
| `escrow-import` | `"RDE-import"` (`NNDNReasonEscrowImport`) |
| `spec5` | names blocked by RA Specification 5 (cf. `Spec5Label`) |
| `policy` | names blocked by the registry's own policy |
| `future-inventory` | names held back for later launch or release (withheld) |
| `idn-variant` | variants of an existing name (`mirrored`) |
| `other` | everything uncategorised, including all pre-existing rows |

Existing rows migrate to `other` with their old `Reason` text preserved as the
note, as an additive column through the normal migrator (ADR 0010). Known
writers (`domain_service.go` ×2, `direct_db_importer.go`, `nndn_commands.go`)
move to the typed values.

### 4. Moving between the three states is explicit

- **Reserved → released:** transfer from 9998/9999 to a customer registrar.
  Already possible; no new verb.
- **Blocked → reserved:** delete the NNDN and create the domain under
  9998/9999. This is a genuinely new operation. It must be one auditable
  action, because between the two steps the name is briefly registrable by
  anyone.
- **Blocked → available:** delete the NNDN. The name is then registrable under
  the normal phase and launch rules. This already works as a plain delete, and
  should emit an audit event carrying the category it had, so releasing
  inventory is traceable. `mirrored` names are the exception: they exist
  because of the base name they mirror and are released with it, not by hand.

### 5. Presentation

- Sidebar gains **Reserved** (domain list filtered to operator sponsorship;
  for a gTLD it shows usage against the cap, e.g. "37 of 100 active") next to
  **Blocked names** (the current "Blocking" page). "Blocked names" is a
  proposal: it matches the noun used everywhere else and carries the
  Category. UX can still change the label without affecting this ADR.
- **NNDN does not appear in the UI.** Today the only user-visible occurrence is
  the "NNDNs" field label on the synthetic escrow form
  ([`escrow/synthetic/page.tsx:141`](../../frontend/app/escrow/synthetic/page.tsx#L141)).
  That becomes "Blocked names". "NNDN" stays the technical term in code, the
  database and the API, where it is the RFC name. The page route moves from
  `/nndns` to a name that matches the label, with a redirect.
- The RFC state (`blocked` / `withheld` / `mirrored`) shows as a secondary
  "Status" column, not as the primary grouping. Category is the primary
  grouping.
- The TLD page shows two tiles: **Reserved domains** (count, billable vs
  non-billable) and **Blocked names** (count, by category). The existing widget
  is repointed to the blocked noun and renamed.
- Domain detail shows a **Reserved** badge when the sponsor is an operator
  account.

### 6. The RA's 100-name cap is a TLD policy, enforced on 9998

- A TLD carries an operator activation cap. For a gTLD (`TLDTypeGTLD`) it
  defaults to 100. For other TLD types it defaults to unlimited, since the RA
  does not bind them.
- The cap counts domains sponsored by `9998-{tld}` that are *active*, meaning
  published in the zone. A reserved domain that is created but not delegated,
  or is on hold, does not count. The precise definition of "active" is settled
  in the implementing change against the domain status model.
- The rule lives in the domain layer; the service supplies the count. It is
  enforced at the moment a 9998 domain becomes active, and creating one
  inactive is always allowed.
- Breaching the cap is rejected with an error that names the alternatives:
  keep the name unactivated as a blocked (`withheld`) name, or register it
  through an accredited registrar.
- 9999 is not subject to the cap, and is **not restricted** to the
  Specification 5 §3.1 labels. The platform does not police which names an
  operator places under 9999. Keeping those names within §3.1 is the
  operator's compliance responsibility, not a registry-software rule.

### 7. Operator accounts are bound to their own TLD

- The TLD an operator account serves is the one its ClID names
  (`9998-{tld}`, `9999-{tld}`). `Registrar.AccreditFor` gains the rule: an
  operator account can be accredited for that TLD only, for every TLD type,
  and is exempt from the "must be ICANN accredited" check for gTLDs because it
  is not a third-party registrar.
- `provisionOperatorRegistrars` goes through `AccreditFor` instead of calling
  the repository directly, so the guard cannot be bypassed by the one path that
  currently skips the domain layer.
- The accreditation REST path already runs `AccreditFor`, so it picks the rule
  up without further change.

## Deferred (decided, but not part of this change)

- **`withheld` and `blocked` stay distinct** for now. The target state is
  fluent category handling in the UI, where the operator works in terms of
  category and the RFC state is derived or defaulted from it. That is a UX
  evolution on top of `Category`, not a prerequisite for it.
- **Operator-owned names under ordinary registrars** (see Decision 2, scope).

## Consequences

- One word, one meaning. The wrong "Reserved Inventory" label is corrected at
  the source rather than papered over.
- No new table and no flag on `Domain`: reserved stays a pure projection of
  sponsorship, so it cannot disagree with it.
- One additive NNDN column and a one-time backfill. Category filters and counts
  become reliable instead of substring matches on free text.
- A new TLD-level policy value (the activation cap) and a count query on
  9998-sponsored active domains.
- Costs: a rename across UI and docs; every NNDN writer must choose a category;
  the "blocked → reserved" operation needs to be built as a single audited
  action; the cap rule needs a stable definition of "active".
- Reserved domains remain real domains, so they still count toward registry
  totals. Reporting that wants "registered excluding operator" needs to filter
  on sponsorship explicitly.

## Open questions

- **Cap position for ccTLDs and other non-gTLD types** was not researched.
  Decision 6 defaults them to unlimited.
- **The exact meaning of "active"** for the cap, as noted in Decision 6.
