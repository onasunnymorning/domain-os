# ADR 0007 — Escrow validation (EVE): one signed profile, keyring-shaped key boundary, immutable outcomes

- **Status:** Accepted
- **Date:** 2026-09-08
- **Deciders:** Platform / Registry
- **Supersedes:** none (composes with ADR 0006)
- **Resolves:** the design questions raised by issue [#412](https://github.com/onasunnymorning/domain-os/issues/412)

## Context

domain-os could stream-analyse an RDE XML deposit and durably **import** it,
but it could not act as an escrow verification (EVE) service: nothing verified
a detached OpenPGP signature, decrypted a `.ryde` archive, unpacked it safely,
turned the XML checks into a structured decision, persisted an immutable
validation record, or emitted the ICANN `rdeReport:report` and
`rdeNotification:notification` (DVPN / DVFN). The import path also mutates
registry data, so it cannot be the Stage 1 Data Escrow Agent (DEA) workflow.

Issue #412 asked for the first EVE vertical slice. Building it forced five
decisions that are cheaper to record now than to retrofit later.

## Decision

### 1. Exactly one profile, and only it can claim a verified pass

Stage 1 accepts a `.ryde` deposit plus its detached `.sig` and nothing else.
`entities.EscrowProfileRydeSig` is stamped on every run; `Verified()` is true
only for a PASS on that profile. When unsigned XML / XML.GZ compatibility
profiles arrive they will be *distinct* profiles that never yield
"cryptographically verified" (design constraint 5).

The payload inside the OpenPGP message may be raw XML, gzip-compressed XML, or
a tar archive (optionally gzip-compressed) holding **exactly one** XML file,
which may itself be gzip-compressed. Anything else — including split
multi-file deposits — fails closed with `ARCHIVE_UNSUPPORTED_LAYOUT`. This
layering is an assumption to be confirmed against an authorised real sample;
the peek-and-branch unpacker makes adding a layout a local change.

### 2. The key boundary is keyring-shaped; custody is someone else's job

Two responsibilities are deliberately separated:

| Responsibility | Owner | Status |
|---|---|---|
| Custody, access control, audit of reads and versioning of the **private key value** | Secrets service (Doppler today; the approved IAM-backed service once `alpaca-infra` names it) | Env adapter `internal/infrastructure/secrets` reads a Doppler-injected keyring. A secrets-service adapter is deferred until the service is decided; it replaces the adapter behind `interfaces.EscrowDecryptionKeyProvider` without touching validation code. |
| **Keyring semantics**: which private keys are live, try-all during rollover, which key decrypted each run, retirement | This application | Shipped: `DecryptionKeyring(ctx) (openpgp.EntityList, error)`, every run records the decrypting fingerprint. |
| **Trusted registry signing keys** per tenant/TLD with overlapping validity windows and retirement | This application (data, not secrets) | Shipped: `escrow_trusted_keys`, `/escrow/trusted-keys`, active-window check pinned to the verification time, signing fingerprint on every run. |
| Private-key rotation tooling and OpenPGP revocation-certificate ingestion | Follow-on | Deferred: depends on the backend and on how registries are notified. |

Why keyring-shaped: registries re-key on their own schedule, so during a
rollover deposits arrive encrypted to the previous service key *and* the new
one. A single-key port would force a port change and a run-record migration
at the first rotation; go-crypto already decrypts against an entity list, so
the keyring shape costs nothing today. A secrets service can version the
value, but it cannot know that two versions must be live at once, nor stamp
the fingerprint on a run — that is deposit logic, not secret logic.

**Guardrails.** Private key material never appears in source, workflow
history, logs, reports, run rows or general configuration. Error strings from
the key adapter are fixed text. Fingerprints are public and are logged once at
worker start so operators can confirm which keys a worker holds.

### 3. Fail closed, and never claim what was not established

- A deposit whose detached signature does not verify is **never decrypted**;
  the pipeline reads the ciphertext once to verify and only then a second time
  to decrypt → unpack → validate as one stream. Nothing is written to disk and
  plaintext is never persisted, only digested.
- Outcomes are three-valued. `PASS` → DVPN. `FAIL` (the deposit was received
  and is invalid: bad or untrusted signature, wrong recipient key, corrupt or
  unencrypted payload, unsafe archive, limit breach, malformed XML, invalid
  RDE object, count or TLD mismatch) → DVFN. `ERROR` (the *service* could not
  decide: our keyring unavailable, timeout, internal failure, artifact changed
  since intake) → **no notification**, the run is finalised ERROR and the
  workflow fails. This splits the issue's "decryption failure → DVFN" into
  deposit-side (DVFN) and service-side (ERROR) on purpose: a DVFN is a
  compliance claim about the registry and must not be made because *we* are
  misconfigured.
- Registry-specific entity rules (for example the repository id in a ROID or
  the URL validator) are `RDE_OBJECT_ENTITY_REJECTED` **warnings**, not
  failures. A domain-os rule is not an RDE rule and must not fail another
  registry's deposit.

### 4. Records are immutable; replay binds, never overwrites

`escrow_deposits` is unique on `(tenant_id, tld, ryde_sha256, sig_sha256)` and
has no update path. Replaying the same pair binds to the existing deposit and
opens a **new** run; the archived artifacts under
`escrow-validation/{tenant}/{tld}/{depositID}/` are copied once and never
replaced. A run is finalised by exactly one conditional `UPDATE … WHERE
outcome = 'RUNNING'`; a second finalisation is rejected and the first outcome
stands. `workflow_id` on the run is the correlation id (INV-16).

### 5. Tenancy and the TLD lookup

Per ADR 0006 the operator scope is derived once per request by
`OperatorScopeFromRequest` and passed as a typed parameter; every escrow query
carries `tenant_id = ?`. The TLD port gained a scoped
`GetByNameForOperator(ctx, scope, name)` enforced in SQL, used by every launch
surface **and re-checked inside `BindDeposit`**, so a launch path that forgets
the check still cannot create a record for a TLD the caller does not operate.

### Report adapter

`internal/application/rdereport` is the only code that knows
`draft-lozano-icann-registry-interfaces-26`. The XSDs live under
`rdereport/xsd/` as **test data**; conformance is proven by validating
generated documents with `xmllint --schema` (CI installs `libxml2-utils` and
the test refuses to skip when `CI` is set). The header inside the emitted
report is generated from what the agent observed, as the draft requires; for
DIFF/INCR deposits the RFC 8909 §5.2 dataset reconstruction is not implemented
and the observed counts are reported (documented limitation). The DVFN
`<results>` codes are this agent's own 4xxx vocabulary (`rdereport/codes.go`)
because the draft leaves verification result codes to the DEA; they are part
of the emitted document and are never renumbered. When a deposit could not be
opened far enough to read its own id/kind/watermark, the report is filled from
the ICANN file-name convention if the artifact followed it, else from the
artifact digest and receipt time, so a DVFN for an undecryptable artifact is
still schema-valid and still identifies the artifact.

## Explicitly deferred

- Secrets-service adapter and private-key rotation tooling (backend undecided).
- OpenPGP revocation certificates for trusted registry keys (retirement is the control today).
- Unsigned XML / XML.GZ compatibility profiles; split multi-file deposits.
- DRFN (missing-deposit) scheduling; `lastFullDate` tracking across runs.
- RFC 8909 §5.2 dataset reconstruction for DIFF/INCR headers.
- Submitting a notification to ICANN's interface; a pilot run against one real, authorised deposit.
- Removing the unused `entities.RDEReport` (wrong namespace declaration); superseded by `rdereport`.

## Consequences

**Easier:** rotation is a variable edit plus a public-key hand-off, with no
code or schema change; every run is auditable by tenant/TLD/deposit id, digest
and key fingerprints; a draft bump touches one package and its XSD test data.

**Harder:** two S3 reads per deposit (verify, then decrypt) — accepted, because
decrypting unverified data would violate fail-closed; the archive layout and
the draft's element set must be re-confirmed against real material before the
pilot run.

## Action Items

1. [ ] Confirm the `.ryde` layering against one authorised real sample before the pilot run.
2. [ ] Name the secrets service in `alpaca-infra` and add its adapter behind `EscrowDecryptionKeyProvider`.
3. [ ] Decide how registries are notified of a service public-key rollover and build the rotation runbook.
4. [ ] Remove `entities.RDEReport` once nothing references it.
