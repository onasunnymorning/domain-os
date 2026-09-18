# ADR 0009 — Escrow keys belong to parties; TLDs choose parties; EVE owns the lifecycle

- **Status:** Accepted
- **Date:** 2026-09-12
- **Deciders:** Platform / Registry
- **Composes with:** ADR 0006 (tenancy), ADR 0007 (EVE key boundary), ADR 0008 (derivatives)
- **Supersedes:** ADR 0007 §2 row "secrets-service adapter deferred" and its action item 2; ADR 0008 §4 "an env-backed adapter"
- **Resolves:** the design questions raised by issue [#429](https://github.com/onasunnymorning/domain-os/issues/429)

## Context

EVE held keys in two shapes, and both were scoped along the wrong axis:

- **Private decryption keys** were one worker-wide keyring read from
  environment variables (`ESCROW_VALIDATION_PRIVATE_KEYS` plus one shared
  passphrase). The keyring had no owner or lifecycle, rotation meant a worker
  restart, and a retried activity used whatever the keyring held at that moment.
- **Trusted signing keys** (`escrow_trusted_keys`, #412) were registered **per
  tenant and TLD**.
- **The sanitisation HMAC key** (#415) was a third environment variable with
  the same weaknesses.

Escrow does not work per TLD. A Data Escrow Agent has **one** encryption key,
whatever TLD a deposit is for, and a registry service provider signs **every**
deposit with one signing key. Outbound escrow (the deferred "escrow target")
follows the same pattern: our RSP signing key, the DEA's public encryption key,
and a transport. Issue #429 asks EVE to own which keys exist, who they belong
to, what they may do and when, while a secrets service protects the private
material.

## Decision

### 1. Keys belong to parties; purpose follows from the party

A **party** is an organisation on one side of an escrow flow:

- **Kind:** `RSP` or `DEA`.
- **Side:** `self` (an identity we operate, so we hold its secrets) or
  `external` (a counterparty, so we only ever hold its public keys).
- **Owner:** the platform or one operator (§3).

What a party's keys are for is **derived, not chosen**:

| Party | Purpose | Material | Use |
|---|---|---|---|
| DEA × self (e.g. EVE) | `decrypt-inbound` | OpenPGP private (secret backend) | Decrypt `.ryde` deposits sent to us |
| DEA × self | `pseudonymise` | symmetric (secret backend) | Master HMAC key for derivatives (ADR 0008) |
| RSP × external | `verify-inbound` | OpenPGP public (database) | Verify the detached `.sig` |
| RSP × self | `sign-outbound` | OpenPGP private | Reserved for escrow targets |
| DEA × external | `encrypt-outbound` | OpenPGP public | Reserved for escrow targets |

"Key" is `(party, purpose)`. It has no table of its own, because nothing would
live on it. A **version** is one cryptographic key with its lifecycle. An EVE
version is one OpenPGP identity (or one symmetric secret). The backend's own
version of the stored value is recorded separately
(`EscrowSecretRef.BackendVersionID`), so rewrapping a secret or changing a
passphrase does not create a new key.

The roles reserved for escrow targets cannot be created yet
(`ErrEscrowPartyRoleNotSupported`). Allowing them now would invite data nothing
reads.

### 2. One lifecycle; per-purpose policy for everything that differs

```
STAGED ──Activate──▶ ACTIVE ──Deactivate──▶ HISTORICAL
  └──────── Revoke(reason, compromised) — any live state ────────▶ REVOKED
STAGED | HISTORICAL | REVOKED ──Destroy──▶ DESTROYED (metadata kept forever)
```

- **"Not for new deposits" (HISTORICAL) is not "unavailable" (REVOKED).**
  Escrow has to open retained deposits long after a key stops being advertised.
- **Compromise never takes the retirement path.** Revoke applies immediately,
  including to a retry of a run that already selected the version: emergency
  disablement wins over workflow history.

Everything that varies between kinds of key is one row of
`EscrowKeyPurposePolicy`:

| Purpose | Probe before activation | Max ACTIVE | HISTORICAL still used for |
|---|---|---|---|
| `decrypt-inbound` | yes | unbounded (rollover overlap) | deposits received before deactivation |
| `verify-inbound` | no (public) | unbounded | deposits received before deactivation |
| `pseudonymise` | yes | **1** | only a retry of a run that recorded it |

Pseudonymisation was folded in precisely to test that the model is not shaped
around OpenPGP. It differed only in that table. Replacing its active version
needs explicit confirmation (`ErrEscrowKeyReplaceNotConfirmed`), because tokens
in a new derivative stop joining with older ones.

### 3. Platform and operator ownership (ADR 0006)

- **Platform-owned** parties are the installation's own identities (EVE's DEA
  identity) and a shared catalogue of RSPs that serve many operators. They are
  managed under a new explicit third scope kind, `entities.PlatformScope`,
  whose zero value is invalid. "The whole installation" is never spelled like
  "no scope".
- **Operator-owned** parties belong to one operator and are invisible to every
  other operator.
- **Visibility,** enforced in SQL through `entities.EscrowKeyScope`:
  - An operator sees and *uses* its own parties and the platform's, and
    manages only its own.
  - The platform scope sees and manages only platform parties. It is not a
    superuser over operators.

### 4. Arrangements: TLDs choose parties, with inheritance

An **arrangement** names the depositor (an external RSP) and the receiver (one
of our DEA identities) for the inbound direction. It exists at one of three
levels: platform default, operator default or TLD override. It is the inbound
half of the future escrow target.

- **Resolution** takes the most specific level, independently for each side
  (`ResolveEscrowArrangement`). A typical installation sets EVE as the platform
  receiver and the backend RSP as each operator's default depositor, and most
  TLDs set nothing at all.
- **Revisions.** Rows are immutable. A change supersedes the live row and
  writes the next revision, and a partial unique index allows only one live row
  per slot. A run records the revision of every level it consulted.
- **Visibility.** A platform arrangement may only name platform parties, and an
  operator arrangement may only name parties that operator can see.
- **Derivatives.** A derivative uses the `pseudonymise` key of the source
  deposit's effective receiver. The sanitiser needs no concept of its own.

### 5. Every change is audited and emitted, in the same transaction

Every party, version or arrangement change writes three rows in one
transaction:

1. the change itself;
2. an `escrow_key_audit_events` row, the tenant-scoped history the UI reads;
3. a `domain_events` outbox row of type `escrow.<action>`.

This path deliberately does not use `EventPublisher`, whose insert is separate
from the business write and whose errors callers swallow (INV-01 defects 1 and
2, #408). A revocation must never commit without its audit trail, and an event
must never describe a change that rolled back. The audit row and the event are
built from one value (`EscrowKeyAuditEvent.DomainEvent`) and carry identifiers,
fingerprints, states and reasons only, never material.

### 6. Custody: AWS Secrets Manager behind a port, with split IAM

- **Port and adapter.** Private and symmetric material lives in AWS Secrets
  Manager behind `interfaces.EscrowSecretStore`: one secret per EVE version,
  named `{prefix}/escrow-keys/{platform|op-<RyID>}/{partyID}/{versionID}`. The
  database stores only the reference.
- **IAM split.** The API role may create, put, tag, describe and delete
  secrets, but **not read them**. The worker role may only read and describe.
- **Proving a key works.** The API holds a private key only for the duration of
  an import request. That a worker can actually use a version is proven on the
  worker by `EscrowKeyProbeWorkflow`, which fetches, unlocks, matches the
  fingerprint and does a round trip (for a symmetric key, derives a tokenizer).
  - **Every probe is audited, passed or failed.** A probe is a worker reading
    secret material, which is exactly what custody audit is for.
  - **A key store that stays unreachable through the retries is recorded as a
    failed probe** (`KEY_STORE_UNAVAILABLE`). A database failure records
    nothing, because it says nothing about the key.
  - **The probe is not in the Launchpad registry,** because only the key API,
    after its permission check, may start it.
- **Configuration.** The `ESCROW_CUSTODY_*` variables configure the store:
  backend, region, name prefix, KMS key, endpoint and recovery window. The
  names avoid "KEY" and "SECRET" because they are configuration, not
  credentials. Credentials come only from the AWS default chain.
- **The stored value is a versioned JSON payload.** An OpenPGP key is kept
  exactly as imported, still passphrase-protected, so the backend's encryption
  is not the only layer.
- **Who may change keys.** Mutations require Auth0 permissions:
  `escrow:keys:admin` for operator-owned parties, and
  `escrow:platform-keys:admin` for platform-owned parties. They are read from
  the access token's RBAC `permissions` claim only, never from `scope`: with
  RBAC off, Auth0 grants any scope a client requests, so `scope` would let a
  user grant themselves the permission. Without RBAC the claim is absent and
  every change fails closed. With Auth0 disabled altogether (local development)
  the static admin token holds both, because it is the only principal and it
  already reaches every endpoint.

### 7. Runs bind to versions, not to "the latest key"

- **Resolution.** At execution time the workflow resolves the effective
  arrangement and the usable versions in an activity, and passes references
  through history.
- **Retries** reuse that selection but drop any version revoked since.
- **Revalidation** is a new run that resolves again.
- **Recorded on the run:** the depositor and receiver parties, the arrangement
  revisions, and the verifying, decrypting and pseudonymising version ids,
  alongside the fingerprints that are already recorded. A worker checks that a
  fetched secret matches the recorded fingerprint and fails closed if it does
  not.

**How phase 3 implements this.** The implementation has these parts:

- **Resolution.** `internal/application/escrowkeys.Resolver` resolves the
  selection. It lives outside `internal/application/services` because the
  escrow pipelines may not import services
  (`escrow_validation_isolation_test.go`).
- **Validation.** The validation workflow calls `ResolveEscrowKeys` once after
  binding.
- **Sanitisation.** The sanitisation workflow calls `ResolveSanitizationKeys`,
  which reuses the source run's recorded verification and decryption versions
  and resolves the receiver's pseudonymisation version.
- **Re-checks.** Before use, `StillUsable` re-reads every selected version.
- **Store outages.** A key store outage is returned for Temporal to retry.
  Material that fails its fingerprint check is skipped, so an empty keyring
  surfaces as the pipeline's own `DECRYPT_KEY_UNAVAILABLE` /
  `TOKEN_KEY_UNAVAILABLE` (ERROR, never a DVFN or a quarantine).
- **No other key source.** The registry is the only source, and a signed
  validation or a derivative without a selection is refused. Without a
  receiver, a signed deposit ends ERROR with `DECRYPT_KEY_UNAVAILABLE` and a
  derivative with `TOKEN_KEY_UNAVAILABLE`. The sanitisation activities register
  without any key present.
- **No workflow versioning.** The resolve step is unconditional rather than
  behind `workflow.GetVersion`: nothing ran in production before it, so there
  are no histories to replay without it.
- **Run records.** A validation run records `EscrowRunKeyEvidence`, and
  `signing_key_version_id` / `decryption_key_version_id` are indexed columns
  behind the `keyVersionId` run filter. A derivative records
  `TokenKeyFingerprint` and `TokenKeyVersionID`, on the run and in the
  manifest.

**How phase 4 implements the API.**

- **Scope.** A request with `X-Tenant-ID` acts for that operator. A request
  without it acts for the platform, and that requires
  `escrow:platform-keys:admin` even to read, because the platform view is
  staff-only.
- **Permission.** Changes require `escrow:keys:admin` or
  `escrow:platform-keys:admin` to match the scope. The service also refuses a
  change to a party the scope can see but does not own, with 403.
- **Not found.** A party or version the scope cannot see is 404, exactly like
  one that does not exist.
- **Material in requests.** Import bodies are capped at 64 KB. Bind errors
  never quote the body, and no response carries key material or a secret
  reference.
- **Destroy.** Destroy requires the fingerprint repeated. It schedules the
  store deletion before recording `DESTROYED`, so a version never claims to be
  destroyed while material remains.
- **API test doubles.** The API test suite grants scopes through a test-only
  header read by its mock authentication, and runs against an in-memory key
  store and a recording probe starter.

### 8. The trusted-key table and the env adapters are removed, not kept alongside

Two sources of truth for "whom do we trust" would drift. None of them held
production data when the registry replaced them, so they are deleted rather
than migrated:

- **`escrow_trusted_keys`** (#412), its `/escrow/trusted-keys` routes, entity
  and repository. The migration drops the table (`DROP TABLE IF EXISTS`).
  Registry signing keys are added as `verify-inbound` versions on a provider
  party.
- **The env keyring** (`ESCROW_VALIDATION_PRIVATE_KEYS*`) and **the env HMAC
  key** (`ESCROW_SANITIZE_HMAC_KEY`), with their ports.

**Meaning of retirement.** A trusted key was judged against the verification
time, so retiring it stopped it verifying anything. A HISTORICAL version still
verifies deposits *received before* its deactivation, which is what
revalidating a retained deposit needs. Compromise is expressed with Revoke.

## Delivery

| Phase | Scope | Status |
|---|---|---|
| 1 | This ADR; parties, versions, arrangements, audit entities; `PlatformScope`; repositories with transactional audit and outbox | Shipped |
| 2 | `EscrowSecretStore` port, AWS Secrets Manager adapter (`ESCROW_CUSTODY_*`), LocalStack (`--profile keystore`), worker loader with fingerprint check, probe workflow | Shipped |
| 3 | Resolution in the validation and sanitisation workflows; run key evidence; token-key version on derivatives; stop reading `escrow_trusted_keys` | Shipped |
| 4 | REST API (`/escrow/parties`, `/escrow/key-versions`, `/escrow/arrangements`) with Auth0 permission checks; `keyVersionId` run filter; remove `/escrow/trusted-keys`, its entity, repository and table | Shipped |
| 5 | Frontend: Escrow keys (parties grouped by role, key cards with lifecycle actions), Escrow arrangements, Escrow tab on the TLD page, key evidence on runs, runs-by-key-version | Shipped |
| 6 | Remove the env keyring (`ESCROW_VALIDATION_PRIVATE_KEYS*`), the env HMAC key (`ESCROW_SANITIZE_HMAC_KEY`) and their ports; the registry is the only key source; [runbook](../escrow_keys_runbook.md) | Shipped |

## Explicitly deferred

- Escrow targets and outbound deposits (their roles and purposes are reserved above).
- SSH transport credentials, which will be a purpose on the target, not on a party.
- Remote cryptography or an HSM; that would change the crypto boundary, not just the adapter.
- Expiry warnings, scheduled rotation and compliance policies. Lifecycle dates and audit are captured now, so these can be built later.
- OpenPGP revocation certificates for counterparty keys.
- Operator-owned receivers are allowed by the model, but no operator needs one today.

## Consequences

**Easier:**
- Rotation is an operator action with a probe and an audit trail, not an env edit and a restart.
- One registry backend used by many TLDs is registered once.
- Every run names the exact key versions and arrangement revisions it used.
- A compromised key can be withdrawn from in-flight retries.
- Adding a purpose is one policy row.

**Harder:**
- The platform-versus-operator split has to be carried through every read, through a typed scope that the compiler checks.
- The first private key needs an Auth0 principal with the right permission, and the Auth0 API needs RBAC with permissions in the access token; the shared legacy token cannot import one.

## Action Items

1. [x] Model and persistence (phase 1).
2. [ ] Provision the Secrets Manager prefix, KMS key and the two IAM roles in `alpaca-infra` ([runbook](../escrow_keys_runbook.md#environment-setup-once-per-environment), steps 2–3).
3. [ ] Register `escrow:keys:admin` and `escrow:platform-keys:admin` on the Auth0 API, enable RBAC with permissions in the access token, and assign roles (runbook, step 1).
4. [ ] Set `ESCROW_CUSTODY_*` on the admin API and the worker, then do first-time setup: the platform EVE party, its decryption and pseudonymisation keys, the platform default receiver (runbook).
