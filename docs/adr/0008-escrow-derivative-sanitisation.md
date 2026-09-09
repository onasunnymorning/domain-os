# ADR 0008 — Escrow derivatives: a sanitized-pseudonymized copy, produced by rewriting rather than re-modelling

- **Status:** Accepted
- **Date:** 2026-09-09
- **Deciders:** Platform / Registry
- **Composes with:** ADR 0006 (tenancy), ADR 0007 (EVE profiles and the key boundary)
- **Resolves:** the design questions raised by issue [#415](https://github.com/onasunnymorning/domain-os/issues/415)

## Context

EVE could decide whether a deposit was good (#412) but had no way to keep a
copy that is safe to analyse. Escrow data exists to support recovery: ICANN's
RDE material requires the archived deposited copy to stay unmodified, so the
answer to "we want to query this data" cannot be "widen access to the escrow
bucket".

Issue #415 asked for a **separate derivative** — labelled
`sanitized-pseudonymized`, produced by a workflow that cannot write registry
data and cannot modify the deposit. Building it forced six decisions.

## Decision

### 1. The derivative is never the deposit

The raw accepted deposit remains the authoritative custody artifact. The
sanitisation workflow reads it, records its checksum, and writes somewhere
else. It never modifies, moves, re-checksums or re-labels the source, and a
derivative must never be reported as an accepted escrow file or used for an
ICANN / RSP / EBERO release. `escrow_sanitization_runs` records which version
of which source a derivative came from; the relationship only ever points one
way.

The output label is `entities.EscrowDerivativeLabel` —
`sanitized-pseudonymized`, deliberately **not** "anonymous". Deterministic
tokens stay linkable within a tenant, the HMAC key is sensitive material, and
the analytical fields the ticket asks us to retain (country, city, postal code,
exact timestamps, status combinations) can re-identify in combination. NIST
SP 800-188 is explicit that masking alone does not establish de-identification.

### 2. Rewrite tokens; do not decode into structs

The RDE entity structs in `pkg/domain/entities` cannot round-trip. They model
no `authInfo` at all — `RDEDomain.ToEntity` *fabricates* a literal password —
and no `secDNS:keyData`; several fields carry no XML tag; and several
`ToEntity` methods mutate their receiver. Decoding a deposit into them and
re-marshalling would silently drop published DNSSEC key material and emit a
fabricated credential.

So the sanitizer is a streaming token-level rewriter. It is driven by
`xml.Decoder.RawToken` rather than `Token` because `RawToken` leaves namespace
prefixes as the source wrote them; Go's encoder does not round-trip a
namespaced document, and re-encoding through it would produce a derivative
whose element names differ from the deposit it claims to mirror. Prefixes are
resolved against an explicit scope stack instead.

The consequence worth stating: **everything the profile does not deliberately
change passes through byte-for-byte**, which is why a derivative validates
against the published RFC 8909 / 9022 / EPP schemas.

### 3. The profile is in-repo, versioned, and fails closed

`rdesanitize/profile.go` is the whole policy in one reviewable table, keyed by
parent and child because `rdeContact:org` means a registrant's organisation
under `postalInfo` and a disclosure preference under `disclose`. An element,
attribute or namespace it does not classify **quarantines the run and publishes
nothing**: an unclassified field is not a field we may assume is safe. A vendor
extension is exactly the thing that must not pass through by default.

Changing the table means changing `PolicyVersion` in the same commit, and the
database enforces one derivative per `(tenant, source validation run, policy
version)` — so a re-run under a new policy is a separate, separately traceable
object and never an overwrite.

Four readings a reviewer might want to overturn, all recorded next to the
entries they govern:

- **Mandatory direct identifiers are replaced, optional ones removed.** A
  schema-valid derivative still needs a contact name and email, so those become
  deterministic synthetic values; `org`, `street`, `voice` and `fax` are
  optional and are dropped outright. Removal is preferred wherever the schema
  permits it.
- **Country, city, state and postal code are retained**, because the ticket
  lists them as the analytical metadata the approved analysis needs. They are
  also the main reason the output is pseudonymized rather than anonymous.
- **Registrar records are retained in full.** A registrar is an organisation
  whose contact details ICANN already publishes, and the ticket lists the agreed
  registrar identifier as operational metadata to keep.
- **Contact ROIDs are tokenised; domain and host ROIDs are not.** A contact ROID
  is a second handle for the same person and would defeat tokenising the contact
  id. A domain ROID identifies an object whose name the derivative keeps by
  design, so tokenising it would buy nothing.

`secDNS` `dsData` **and** `keyData` are retained: DNSKEY public-key material is
published in DNS, and the private material RFC 8909 warns about has no element
in a deposit to carry it.

### 4. Pseudonyms are derived, not stored

Tokens are HMAC-SHA256 under a subkey derived from one master key as
`HMAC(master, tenant ‖ purpose ‖ policyVersion)`. That makes them deterministic
inside a tenant — a contact shared by a thousand domains stays one contact —
different across tenants, and different across policy versions. **No mapping
table is written anywhere**: re-identification requires the master key.

Every token shape stays inside the schema type it replaces (`eppcom:clIDType`
is 3–16 characters, `eppcom:roidType` needs its hyphen), so the derivative still
validates as RDE rather than merely parsing.

Key custody follows the split ADR-0007 drew: the value belongs to the secrets
service, the *meaning* belongs here. An env-backed adapter sits behind
`interfaces.SanitizationTokenKeyProvider`, its errors are fixed text that cannot
echo the value, and the key **fingerprint** — a MAC over a fixed public string —
is logged at startup and recorded on every manifest, so a rotation shows up as a
visible break in joinability instead of a silent one.

### 5. Verify the outcome, not the policy, and publish only what passed

The rewriter is driven by the profile, so a second check driven by the profile
would find nothing new. `rdesanitize.Scan` therefore checks the *result*: no
element the profile removes, no unclassified element, no name still carrying the
source suffix, and synthetic fields that actually look synthetic. It is followed
by structural re-validation through the ordinary RDE validator, bound to the
synthetic suffix, and the aggregate statistics are counted from the derivative
rather than from the rewriter, so the manifest describes what was written.

A derivative is staged under a `pending/` prefix, verified there, and only then
server-side copied to `sanitized/`. When the profile refuses the source the
upload is aborted rather than completed, so a quarantined derivative is never
stored at all.

Outcomes are three-valued, mirroring ADR-0007's DVFN-versus-ERROR split.
`PASS` publishes. `QUARANTINED` means *this agent will not transform this
source under this policy* — a decision, not a failure, so the workflow
completes. `ERROR` means the *service* could not decide (token key unavailable,
escrow keyring gone, the source artifact changed since validation, timeout) and
fails the workflow. Quarantining is a statement about a deposit and must not be
made because our own key store is down.

### 6. Deviation: the derivative shares the escrow bucket

The ticket asks for a distinct bucket and a restricted IAM role. **The user
overrode that on 2026-09-09**: the derivative is written alongside the deposit,
under a distinct `sanitized/` key prefix.

What is lost is the ability to grant analysts derivative access without granting
escrow access, and to give the derivative its own shorter retention. What is not
lost is confidentiality: the derivative is strictly less sensitive than the
source already in that bucket. The custody boundary is therefore the key prefix
plus the manifest label, and keeping both prefixes in one bucket is also what
makes publication a server-side copy instead of a full re-upload.

Reversing this is one change: a `STORAGE_SANITIZED_BUCKET` client passed as the
publish target. Revisit it when analyst access is actually granted.

## Also decided here: the unsigned validation profile

ADR-0007 deferred "unsigned XML / XML.GZ compatibility profiles". #415's input
is a plaintext deposit and its first step requires an accepted validation for
that profile, so it is no longer deferrable and ships here as
`entities.EscrowProfilePlaintextXML`.

It runs the same unpack limits and the same strict RDE checks. What it cannot
establish is who sent the deposit, so it never yields `Verified()` and never
emits an `rdeNotification`: a DVPN or DVFN is a compliance claim about a signed
deposit, and `EscrowValidationRun.Finalize` now refuses a notification on an
unsigned profile rather than trusting callers to remember.

`EscrowDeposit` was renamed for roles rather than for one profile's file
extension (`Artifact*` / `Signature*`) and carries its profile, because an
`.xml.gz` key stored in a column called `ryde_object_key` is a permanent lie.

## Explicitly deferred

- A separate bucket and workload identity for derivatives (see decision 6).
- A lifecycle rule expiring the `pending/` prefix — infrastructure, not code.
- Profile entries for vendor extensions: the first real deposit that carries one
  will quarantine, which is the intended behaviour, and extending the profile is
  a small reviewed diff.
- The purpose, access, retention and controller/processor roles for the
  analytics this enables. Sanitisation does not by itself establish a legal
  basis, and the ticket says so.
- Deriving from a DIFF/INCR deposit is supported mechanically but no dataset
  reconstruction is attempted, exactly as in validation.

## Consequences

**Easier:** a field-policy change is one table plus a version bump, and the
database guarantees the old derivative survives it. A derivative is provably
schema-valid, so a consumer needs only the published schemas. Every run is
traceable to a source checksum, a policy version and a key fingerprint.

**Harder:** a signed source is read and decrypted a second time — the price of
never having persisted the plaintext, and worth paying. The profile must be
kept ahead of real deposits or they quarantine. And the RDE schemas compose EPP
types, so an object's children are not always in the object's own namespace;
classification is by resolved namespace for that reason, and getting it wrong is
caught by the conformance test rather than by review.

## Action Items

1. [ ] Provision a lifecycle rule expiring `escrow-validation/**/pending/`.
2. [ ] Decide and document the purpose, access and retention policy for derivative data before any analyst is granted access.
3. [ ] Run one real accepted deposit through the profile and triage whatever quarantines.
4. [ ] Revisit decision 6 when derivative access is granted to anyone who should not see the escrow bucket.
