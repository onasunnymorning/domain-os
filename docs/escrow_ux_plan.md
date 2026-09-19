# Escrow configuration UX — what shipped, what is deferred

The escrow configuration screens were reworked against a UX brief whose premise
is that an operator should be able to configure escrow knowing only four
things: where deposits come from, where they go, which keys each direction
needs, and whether the configuration is ready. The model behind the screens is
unchanged — [ADR-0009](adr/0009-escrow-parties-keys-arrangements.md) still
describes it exactly — and the procedures are still in the
[runbook](escrow_keys_runbook.md).

This page records the vocabulary that pass settled on, the work it deliberately
left, and where the backend would be worth aligning with it.

## The vocabulary

The registry's own classification (kind × side) is a fine data model and a poor
label. The user-facing name follows from it, one to one, and the mapping is the
whole translation:

| Record (`escrow_parties`) | Called, in the interface | Holds |
|---|---|---|
| `RSP` × `external` | **Escrow source** | their public verification key |
| `DEA` × `self` | **Our receiving identity** | our private decryption key (and the pseudonymisation key) |
| `DEA` × `external` | **Escrow destination** *(not built)* | their public encryption key |
| `RSP` × `self` | **Our sending identity** *(not built)* | our private signing key |

| Purpose | Called, in the interface |
|---|---|
| `verify-inbound` | verification key |
| `decrypt-inbound` | decryption key |
| `pseudonymise` | pseudonymisation key |

| State | Called, in the interface |
|---|---|
| `STAGED` | Added — *not in use* |
| `ACTIVE` | Active |
| `HISTORICAL` | Retired — *still opens deposits received before it was retired* |
| `REVOKED` | Revoked — *blocked immediately, including runs in progress* |
| `DESTROYED` | Destroyed |

Arrangement sides are read as directions: `depositor_party_id` is "deposits come
from", `receiver_party_id` is "received by".

Two rules came out of this. **Acronyms do not appear in the interface** — a
source may or may not be a registry service provider, and assuming it is puts a
wrong word in front of the user. **The registry's own names stay reachable**:
the exact lifecycle state, purpose, version, fingerprint, probe result and
arrangement revision are all one click away under *Technical details*, because
support and audit work needs them and plain language would be a loss there.

## Readiness

Readiness is derived, not stored: `frontend/lib/escrow/readiness.ts` turns
lifecycle states and probe results into one of Ready · Rotation in progress ·
Ready to activate · Testing key · Setup incomplete · Action required · Revoked,
each with a sentence naming what is missing.

Two judgements in it are worth keeping if it ever moves:

- A missing **pseudonymisation** key does not make a receiving identity
  incomplete. It is a capability, not a precondition for receiving deposits, and
  counting it would flag every installation that never sanitizes.
- **Two active keys is a rotation, not a fault.** Overlap is how a correct
  rotation looks; colouring it as a problem teaches people to ignore the colour.

## Deferred work

In rough order of value per unit of effort.

### 1. Failure → configuration diagnostics (brief §18)

From a failed validation run, reach the source, the exact verification key
version, the receiving identity, the decryption key version and the arrangement
that was in effect; from a key version, reach the runs that used it.

Everything needed is already served: `EscrowRunKeyEvidence` (the run's `keys`
field) carries the depositor and receiver party IDs, the signing and decryption
key version IDs, the candidates that were considered and the three arrangement
revisions, and the run list filters by key version. What is missing is the
linking, and names beside the identifiers — the response gives IDs, so the page
has to resolve them to the source and version a reader recognises.

It is the highest-value item on the list: diagnosing a `SIG_KEY_UNTRUSTED`
failure currently means reading `key_evidence` JSON out of the database.

*Frontend, unless resolving the names server-side turns out cheaper. Small to
medium.*

### 2. Escrow configuration on the TLD page (brief §11)

The TLD escrow card shows the effective arrangement and links away to change
it. Make each side overridable in place, with the effective value, where it came
from, what an override would change, and how to return to the inherited value
("Use the operator default", not "Inherit").

*Frontend over the existing `PUT/DELETE /escrow/arrangements/tlds/:tld`. Small
to medium.*

### 3. Guided first-time source setup (brief §6, §20)

The four steps exist as separate controls on two pages: add the source, paste
their verification key, hand them our public encryption key, choose where it
applies. A guided flow would make the order obvious and end with a completion
state that says either "we can verify deposits from this source and they have
what they need from us" or exactly what is still missing.

*Frontend over existing endpoints. Medium.* The brief's acceptance test (§20)
is this flow, performed by someone who has never seen a purpose identifier.

### 4. Rotation as a guided sequence (brief §9, open question 5)

Add replacement → test → activate → keep both active → retire the old one →
destroy when safe. The individual actions all exist; what is missing is the
sequence and the explanation of why the overlap is deliberate.

*Frontend. Medium.*

### 5. Persistent working scope (brief §5)

Scope is currently chosen per page. It should be one working context —
"Escrow / Operator: Example Registry" — that follows the reader across sources,
identities, defaults and the TLD card, with platform-owned configuration marked
*Provided by platform · Read-only* rather than silently disabled.

*Frontend, but it touches the escrow shell rather than one page.* Open question:
whether this is an escrow-local context or the app's existing operator context,
which the TLD and domain pages already have their own version of.

### 6. Dependencies before destroy (brief §9)

Destroy asks for the fingerprint but does not yet show what depends on the key:
validation runs that used it, and configuration still pointing at its party.
The run filter by key version already exists.

*Frontend. Small.*

### 7. Permission-aware presentation (brief §16)

Read-only is explained on the party page; the platform area is still offered to
principals who cannot use it, and the refusal arrives as a 403 after the click.
Surfacing the scope claim to the client would let the interface say so first.

*Backend (scopes in a `/me`-style response) plus frontend. Small to medium.*

### 8. Escrow destinations and outbound deposits (brief §13)

Reserved throughout — purposes `sign-outbound` and `encrypt-outbound` exist,
`groupParties` has a `destinations` bucket, and the section shows a factual "not
available yet" with no fake controls. Building it is the deferred escrow-target
work from the #429 plan, not a UX task.

*Backend and frontend. Large.*

### 9. Readiness derived server-side (brief §14)

Readiness lives in the client today. Moving it behind an additive `readiness`
field on the party response would let list endpoints, the runs view and any
future client share one definition. Worth doing when a second consumer appears,
not before — the rules are still settling.

*Backend, additive. Small to medium.*

## Aligning the backend language

Worth doing where the backend produces something a person reads, and worth
refusing where it would churn identifiers that systems depend on.

### Worth doing — done

All four shipped together; the entries below record what changed and why, so
the reasoning survives the diff.

- **Human-readable strings in validation findings.** A `SIG_KEY_UNTRUSTED`
  finding said "no active trusted signing key is registered for this
  tenant/TLD"; the interface calls that a *verification key* belonging to a
  *source*. These strings are free text next to a stable result code, they are
  what an operator reads when something fails, and they were the one place the
  two vocabularies visibly disagreed. Reworded in
  `internal/application/rdevalidate/pgp.go` and `pipeline.go` (signature and
  decryption) and in `internal/application/activities/escrow_sanitize.go` (the
  pseudonymisation and reopen failures). The two "no decryption key" sites —
  the pipeline short-circuit and `Decrypt` itself — now share one constant, so
  one missing key cannot produce two different sentences. A test in
  `pgp_test.go` asserts the refusals contain neither "trusted" nor "service
  key" nor "tenant/TLD", which locks the words without freezing the sentences.
- **A `role` field on the party response** (`source` | `receiving-identity` |
  `destination` | `sending-identity`), additive. Derived in the entity
  (`EscrowParty.Role`), returned by `toPartyResponse`, and preferred by the
  frontend's `partyRole`, which still falls back to deriving from kind and side
  — for an older server, and for a role a future build has no words for.
- **The mapping table, in the entity source.** The doc comment on
  `escrowPartyPurposes` now carries kind × side → role → purposes → what the
  interface calls it, in one table, with the key names beside it.
- **The runbook, aligned with the screens** (`docs/escrow_keys_runbook.md`):
  sources and receiving identities instead of providers and DEA identities,
  verification key instead of signing key, retire instead of deactivate, test
  instead of probe, Escrow Setup instead of Escrow keys. The AWS, IAM and
  configuration sections keep their own vocabulary — they describe the system,
  not the screen.

One boundary held deliberately: the DVFN result messages in
`internal/application/rdereport/codes.go` are unchanged. That table is part of
an emitted document with a different audience — ICANN and the registry, in the
escrow industry's words — and it is declared append-only. The finding message
still reaches the DVFN as the free-text description, which is where our prose
belongs.

### Not worth doing

- **Renaming `depositor` / `receiver`** in columns, ports, resolver, run records
  and audit events. Both names are accurate, already directional, and appear in
  persisted evidence; the migration and the churn buy a synonym.
- **Renaming purposes.** `verify-inbound` and `decrypt-inbound` say the
  direction and the operation. They are good wire names and map one to one.
- **Renaming `RSP` / `DEA` / `self` / `external`.** They are the industry's own
  classification and they are correct in the record. The interface simply does
  not show them.
- **Renaming domain event types** (`escrow.key_version.activated` and the rest)
  or REST paths. Consumers, the S3 event archive and API clients depend on both,
  and the benefit is cosmetic.

The line: *rename what a person reads, keep what a system matches on.*

## Open questions from the brief

| # | Question | Where it stands |
|---|---|---|
| 1 | Assign a source to domains during creation? | Offered, never required. A source is routinely registered before anything points at it. |
| 2 | Should "our identities" be visible navigation? | Kept as a section under escrow setup rather than its own nav entry; revisit if rotation becomes frequent. |
| 3 | Do defaults belong to sources, or to configuration? | Left under defaults and overrides — people describe them as routing rules, not properties of a source. |
| 4 | How prominent is the scope switcher? | Deferred item 5. |
| 5 | Is rotation a wizard? | Deferred item 4. |
| 6 | How is readiness calculated? | Settled and tested; see above. Moving it server-side is item 9. |
| 7 | A counterparty that is both source and destination | Unanswered, and cheap to leave so: it becomes real with outbound escrow, and the record can carry one organisation shown in two directional contexts. |
| 8 | How much cryptography belongs in the briefing? | Enough to prevent the public/private mistake, no more: the briefing names which key is secret and which is meant to be handed out. |
