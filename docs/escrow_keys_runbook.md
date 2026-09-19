# Escrow keys — operator runbook

How to run escrow keys on the key registry introduced by issue
[#429](https://github.com/onasunnymorning/domain-os/issues/429). The design and
its reasons are in [ADR-0009](adr/0009-escrow-parties-keys-arrangements.md); this
page is only the procedures.

## The model in one paragraph

Keys belong to the organisation they identify, not to a TLD. The screens are
arranged in the direction deposits travel, and this page uses their words; the
record's own classification (`kind`, `side`, `purpose`) is in
[ADR-0009](adr/0009-escrow-parties-keys-arrangements.md) and stays in the API.

- **Escrow sources** send deposits to us. We hold each source's **verification
  key** — its public key — and check every deposit against it.
- **Our receiving identities** are the identities deposits are encrypted to.
  EVE, our Data Escrow Agent, holds a **decryption key** whose private half
  stays in the key store, and a **pseudonymisation key** for sanitized copies.
- **Escrow destinations** receive deposits from us. Sending deposits out is not
  built yet, so the section is empty.

**Defaults and overrides** say, for a TLD, where its deposits come from and
which of our identities receives them. Set once as a platform default or an
operator default, and overridden for a single TLD only where that TLD differs.

Every key has **versions**. The screens name the states plainly, and the record
keeps the name in brackets:

```
Added (STAGED) → Active (ACTIVE) → Retired (HISTORICAL),   and Revoked / Destroyed
```

- **Retired** means "not for new deposits, still opens older ones".
- **Revoked** means "unusable, even for runs already in progress".

## Permissions

| Action | Needs |
|---|---|
| Read an operator's registry (UI, `X-Tenant-ID`) | an authenticated principal |
| Read or change the platform registry (no `X-Tenant-ID`) | Auth0 permission `escrow:platform-keys:admin` |
| Change an operator's parties, keys or arrangements | Auth0 permission `escrow:keys:admin` |

- **Where they are read.** The API reads permissions only from the access
  token's RBAC `permissions` claim, never from `scope`. If RBAC is off, Auth0
  puts any scope a client asks for into `scope`, so trusting it would let users
  grant themselves permissions.
- **Operators are not bound yet.** `escrow:keys:admin` applies to whichever
  operator the request names in `X-Tenant-ID`, until ADR-0002 ties principals
  to operators. Grant it only to staff.
- **With Auth0 disabled** (`AUTH0_ENABLED=false`, local development) the static
  `ADMIN_TOKEN` principal holds both permissions, because it already reaches
  every other endpoint. With Auth0 enabled that token is not accepted at all.

## Environment setup (once per environment)

Do these in order. Until step 4 is deployed the key store is off
(`ESCROW_CUSTODY_BACKEND=none`). Public keys and unsigned deposits still work,
but importing or probing a private key reports `KEY_STORE_NOT_CONFIGURED`.

### 1. Auth0: register and grant the permissions

In the Auth0 dashboard of the tenant named by `AUTH0_DOMAIN`:

1. **Add the permissions.** Go to **Applications → APIs** and open the API
   whose *Identifier* equals `AUTH0_AUDIENCE`. On **Permissions**, add:

   | Permission | Description |
   |---|---|
   | `escrow:keys:admin` | Change operators' escrow parties, keys and arrangements |
   | `escrow:platform-keys:admin` | Read and change the platform escrow key registry |

2. **Turn on RBAC.** On the same API's **Settings**, under **RBAC Settings**,
   enable **Enable RBAC** and **Add Permissions in the Access Token**, then
   **Save**. Without the second switch the token has no `permissions` claim,
   and every change returns 403.
   - Nothing else in the API checks permissions today, so existing logins keep
     working.
   - Machine-to-machine clients keep the scopes their client grant gives them.
3. **Create roles.** Under **User Management → Roles**, create:
   - **Escrow key admin**, with `escrow:keys:admin`;
   - **Escrow platform key admin**, with both permissions.
4. **Assign the roles.** Give each role to the people who manage keys (the
   role's **Users** tab). Keep the platform role to the few people who hold
   EVE's keys.
5. **Refresh the token.** Those people sign out of the admin UI and back in;
   an existing token does not gain the claim.
6. **Check.** Decode the access token locally, without pasting it into a
   website:

   ```bash
   cut -d. -f2 <<<"$TOKEN" | tr '_-' '/+' | base64 -d 2>/dev/null | jq .permissions
   ```

   The list should hold the permissions of the role. For a platform key admin,
   choosing *Platform registry* under **Escrow Setup** loads the registry
   instead of the permission notice.

### 2. AWS: choose the name prefix and encryption key

- **Name prefix.** One prefix per environment, for example `domain-os-prod`.
  Every secret is named `<prefix>/escrow-keys/{platform|op-<RyID>}/<partyID>/<versionID>`
  and is created by the API on import. There is nothing to create in advance,
  and the IAM policies below are written against the prefix.
- **Region.** Use the region the workers run in.
- **Encryption key (recommended).** Create a customer-managed symmetric KMS key,
  for example alias `alias/domain-os-prod-escrow-keys`. The key's policy
  should allow only the API and worker roles below, and your break-glass
  administrators. Without one, secrets use the account's `aws/secretsmanager`
  key, which every principal with Secrets Manager access in the account can use.
- **Rotation.** Leave the Secrets Manager rotation schedule off. Keys are
  rotated as new versions through EVE, not by Lambda.

### 3. AWS: the two IAM roles

The API writes keys but can never read them back. Only workers read them.
Attach these to the identities the admin API and the unified worker run as
(an EKS IRSA role, ECS task role or instance profile). Replace `REGION`,
`ACCOUNT`, `PREFIX` and `KMS_KEY_ARN`.

**Admin API role** (ry-admin API):

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Sid": "EscrowKeysWrite",
      "Effect": "Allow",
      "Action": ["secretsmanager:CreateSecret", "secretsmanager:TagResource", "secretsmanager:DeleteSecret"],
      "Resource": "arn:aws:secretsmanager:REGION:ACCOUNT:secret:PREFIX/escrow-keys/*"
    },
    {
      "Sid": "EscrowKeysNeverRead",
      "Effect": "Deny",
      "Action": ["secretsmanager:GetSecretValue", "secretsmanager:BatchGetSecretValue"],
      "Resource": "arn:aws:secretsmanager:REGION:ACCOUNT:secret:PREFIX/escrow-keys/*"
    },
    {
      "Sid": "EscrowKeysEncrypt",
      "Effect": "Allow",
      "Action": ["kms:GenerateDataKey", "kms:Decrypt"],
      "Resource": "KMS_KEY_ARN",
      "Condition": { "StringEquals": { "kms:ViaService": "secretsmanager.REGION.amazonaws.com" } }
    }
  ]
}
```

**Worker role** (unified worker):

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Sid": "EscrowKeysRead",
      "Effect": "Allow",
      "Action": ["secretsmanager:GetSecretValue"],
      "Resource": "arn:aws:secretsmanager:REGION:ACCOUNT:secret:PREFIX/escrow-keys/*"
    },
    {
      "Sid": "EscrowKeysDecrypt",
      "Effect": "Allow",
      "Action": ["kms:Decrypt"],
      "Resource": "KMS_KEY_ARN",
      "Condition": { "StringEquals": { "kms:ViaService": "secretsmanager.REGION.amazonaws.com" } }
    }
  ]
}
```

- **These are all the calls the adapter makes.** Creating a secret with tags
  needs `TagResource`; `DeleteSecret` is only used by **Destroy**, and always
  with a recovery window.
- **The explicit Deny** keeps the API unable to read keys even if a broader
  policy is attached to its role later.
- **The KMS statements go only with a customer-managed key.** Without one,
  drop them. The API's `kms:Decrypt` is needed by `CreateSecret` and, because
  it is limited to Secrets Manager, still cannot read a secret without
  `GetSecretValue`.
- **Workers read keys only inside probes and validation, sanitisation and
  derivative activities.** CloudTrail records every `GetSecretValue` with the
  secret name, so worker reads are auditable per key version.

### 4. Configuration

Set on the **admin API and the unified worker**, in Doppler or the deployment's
env (see `deploy/contract.json`):

| Variable | API | Worker | Value |
|---|---|---|---|
| `ESCROW_CUSTODY_BACKEND` | ✓ | ✓ | `aws-secrets-manager` |
| `ESCROW_CUSTODY_AWS_REGION` | ✓ | ✓ | the region from step 2 |
| `ESCROW_CUSTODY_NAME_PREFIX` | ✓ | ✓ | the prefix from step 2, the same on both |
| `ESCROW_CUSTODY_KMS_ID` | ✓ | | the KMS key id, ARN or alias, if you made one |
| `ESCROW_CUSTODY_RECOVERY_WINDOW_DAYS` | ✓ | | 7–30, default 30 |
| `ESCROW_CUSTODY_ENDPOINT` | | | leave empty in AWS; LocalStack only |

- **No credential variables.** Credentials come only from the AWS default
  chain, meaning the role from step 3.
- **Check the wiring.** Redeploy, then run *First-time setup* below. The first
  import proves the API can write, and the test it starts proves a worker can
  read. An import failing with `KEY_STORE_UNAVAILABLE` points at the API role;
  a test failing with it points at the worker role or the KMS key policy.
  Check that the prefix in the variables matches `PREFIX` in the policies.

**Local development** runs the same flow against LocalStack, with no Auth0 or
IAM setup:

1. Set `ESCROW_CUSTODY_BACKEND="aws-secrets-manager"` in `.env` (or in your
   Doppler config, if that is where the api and worker read their environment).
2. Run `make keystore`. It starts LocalStack and restarts the api and worker
   against it; `docker-compose.yml` supplies the endpoint, prefix and dummy
   credentials.
3. Manage keys in the admin UI under **Escrow Setup**. With Auth0 disabled the
   static token holds both permissions, so nothing else is needed.

Without step 1 everything except private keys still works: verification keys
and the defaults and overrides are database rows. Importing or testing a private
key reports `KEY_STORE_NOT_CONFIGURED`. A test needs the worker and Temporal,
and the api and worker must share one database and one key store.

## First-time setup

1. **Open the platform registry.** In **Escrow Setup**, choose *Platform
   registry* (this needs the platform permission).
2. **Add a receiving identity**, for example "EVE".
3. **Add its decryption key.** Paste the armored private key and its
   passphrase. The key stays passphrase-protected in the key store, and the
   form clears as soon as it is submitted. A worker tests the key
   automatically; once the test shows **passed**, **Activate** it.
4. **Generate its pseudonymisation key.** Wait for the test to pass, then
   activate it.
5. **Set who receives by default.** Under **Defaults & overrides**, set
   *Received by* to EVE.
6. **Add each operator's source.** For each operator, add the organisation that
   will deposit. Either add it under the operator, or add it once in the
   platform registry, where every operator can use it. Add that source's
   **verification key** and activate it — a public key needs no test — then set
   the operator's *Deposits come from*.
7. **Add overrides only where needed.** A top-level domain whose source or
   receiving identity differs gets an override. Its **Escrow** tab shows each
   side and where it is inherited from.

## Rotating a receiving identity's decryption key

1. **Add the replacement and activate it.** Use **Add replacement** on the
   decryption key and wait for its test to pass. Activate it; both versions are
   now active, and the card reads *Rotation in progress*.
2. **Hand the new public key to each source.** Use *Download the public key to
   give out* on the version, and tell each source the date from which to
   encrypt to it.
3. **Retire the old version after the overlap.** *Retire (keep for older
   deposits)*: new deposits no longer select it, but deposits received before
   it was retired still open with it, so revalidating a retained deposit keeps
   working.
4. **Destroy only when nothing depends on it.** Destroy the old version once no
   retained deposit depends on it, and type its fingerprint to confirm. Check
   with **runs** on the version, which lists the validation runs that used it.

## Rotating a source's verification key

1. **Add the source's new public key and activate it.** Both versions verify
   during the overlap.
2. **Retire the old version** when the source stops signing with it. Retained
   deposits signed before then still verify on revalidation.

## Rotating the pseudonymisation key

Generate a new version, wait for the test to pass, and activate it. You are
asked to confirm the replacement, because tokens in new sanitized copies do not
join with older ones.

- The previous version is retired.
- A derivative run already in progress finishes under the key it started with.
- Every run and manifest records the version it used.

## Compromise

1. **Revoke the version and tick "may be known to someone else".** Revocation
   takes effect immediately, including for retries of runs that already
   selected the version. Those end ERROR with `DECRYPT_KEY_UNAVAILABLE` (or
   `TOKEN_KEY_UNAVAILABLE`); they never send a DVFN.
2. **Replace a decryption key urgently.** Import a replacement, test it,
   activate it, and send every source the new public key.
3. **Handle a compromised signing key with the source.** Revoke our copy of its
   verification key and add the replacement the source gives you.
4. **Destroy after investigation.** When the investigation no longer needs the
   version, destroy it. Its record, fingerprint and audit trail stay.

## Where to look

| Question | Answer |
|---|---|
| Is escrow ready? | The status beside each source and receiving identity, on **Escrow Setup** and at the top of its page. It reports the worst thing true of the keys that entity needs. |
| Which keys did a run use? | The run page's **Keys used** section: the source, the receiving identity, the arrangement revisions, and the verifying and decrypting versions. |
| Which runs used a version? | **runs** on the version, or `GET /escrow/validations?keyVersionId=`. |
| Who changed what, when? | The **History** tab of the source or identity. The same events go to the event outbox as `escrow.*`. |
| Why can't a version be activated? | Its test has not passed. **Test on a worker** again and read the result. Codes: `KEY_STORE_NOT_CONFIGURED`, `KEY_STORE_UNAVAILABLE`, `SECRET_NOT_FOUND`, `MATERIAL_UNREADABLE`, `FINGERPRINT_MISMATCH`, `ROUND_TRIP_FAILED`. |
