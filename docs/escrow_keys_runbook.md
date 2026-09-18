# Escrow keys — operator runbook

How to run escrow keys on the key registry introduced by issue
[#429](https://github.com/onasunnymorning/domain-os/issues/429). The design and
its reasons are in [ADR-0009](adr/0009-escrow-parties-keys-arrangements.md); this
page is only the procedures.

## The model in one paragraph

Keys belong to **parties**, not TLDs.

- **Our identities** hold private keys in the key store. EVE, our Data Escrow
  Agent, holds a *decryption key* that registries encrypt deposits to, and a
  *pseudonymisation key* for sanitized copies.
- **Registry service providers** are counterparties. We only hold their public
  *signing keys*.

An **arrangement** says, for a TLD, which provider deposits and which of our
identities receives. It is set once as a platform default or an operator
default, and overridden per TLD only where a TLD differs.

Every key has **versions** that move through these states:

```
STAGED → ACTIVE → HISTORICAL,   and REVOKED / DESTROYED
```

- **HISTORICAL** means "not for new deposits, still opens older ones".
- **REVOKED** means "unusable, even for runs already in progress".

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
   choosing *Platform registry* under **Escrow keys** loads the registry
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
  import proves the API can write, and its automatic probe proves a worker can
  read. An import failing with `KEY_STORE_UNAVAILABLE` points at the API role;
  a probe failing with it points at the worker role or the KMS key policy.
  Check that the prefix in the variables matches `PREFIX` in the policies.

**Local development** runs the same flow against LocalStack, with no Auth0 or
IAM setup:

1. Set `ESCROW_CUSTODY_BACKEND="aws-secrets-manager"` in `.env` (or in your
   Doppler config, if that is where the api and worker read their environment).
2. Run `make keystore`. It starts LocalStack and restarts the api and worker
   against it; `docker-compose.yml` supplies the endpoint, prefix and dummy
   credentials.
3. Manage keys in the admin UI under **Escrow keys**. With Auth0 disabled the
   static token holds both permissions, so nothing else is needed.

Without step 1 everything except private keys still works: public signing keys
and arrangements are database rows. Importing or probing a private key reports
`KEY_STORE_NOT_CONFIGURED`. A probe needs the worker and Temporal, and the api
and worker must share one database and one key store.

## First-time setup

1. **Open the platform registry.** In **Escrow keys**, choose *Platform
   registry* (this needs the platform permission).
2. **Create our identity.** Add a DEA identity, for example "EVE".
3. **Add EVE's decryption key.** Paste the armored private key and its
   passphrase. The key stays passphrase-protected in the key store, and the
   form clears as soon as it is submitted. A worker probes the key
   automatically; once the probe shows **passed**, **Activate** it.
4. **Generate EVE's pseudonymisation key.** Wait for the probe to pass, then
   activate it.
5. **Set the platform default receiver.** In **Escrow arrangements**, set it to
   EVE.
6. **Set each operator's depositor.** For each operator, add its registry
   service provider(s). Either add them under the operator, or add them once in
   the platform registry as catalogue entries and use them from the operator.
   Add each provider's public signing key and activate it (public keys need no
   probe). Then set the operator's **default depositor**.
7. **Add TLD overrides only where needed.** A TLD whose provider or receiver
   differs gets an override. Check a TLD's **Escrow** tab: each side shows where
   it is inherited from.

## Rotating EVE's decryption key

1. **Add and activate the new version.** Add it on EVE's decryption key and
   wait for its probe to pass. Activate it; both versions are now ACTIVE.
2. **Hand the new public key to the registries.** Use the download button on
   the version, and give each registry the date from which to encrypt to it.
3. **Deactivate the old version after the overlap.** It becomes HISTORICAL: new
   deposits no longer select it, but deposits received before deactivation
   still open with it, so revalidating a retained deposit keeps working.
4. **Destroy only when nothing depends on it.** Destroy the old version only
   once no retained deposit depends on it, and repeat its fingerprint to
   confirm. Check with **runs** on the version, which lists the validation runs
   that used it.

## Rotating a registry's signing key

1. **Add the provider's new public key and activate it.** Both versions verify
   during the overlap.
2. **Deactivate the old version.** Do it when the provider stops signing with
   it. Retained deposits signed before then still verify on revalidation.

## Rotating the pseudonymisation key

Generate a new version, wait for the probe to pass, and activate it. The UI asks
you to confirm the replacement, because tokens in new sanitized copies do not
join with older ones.

- The previous version becomes HISTORICAL.
- A derivative run already in progress finishes under the key it started with.
- Every run and manifest records the version it used.

## Compromise

1. **Revoke the version and tick "may be known to someone else".** Revocation
   takes effect immediately, including for retries of runs that already
   selected the version. Those end ERROR with `DECRYPT_KEY_UNAVAILABLE` (or
   `TOKEN_KEY_UNAVAILABLE`); they never send a DVFN.
2. **Replace a decryption key urgently.** Import a replacement, probe it,
   activate it, and send the registries the new public key.
3. **Handle a compromised signing key with the provider.** Revoke our copy of
   the key and add the provider's replacement.
4. **Destroy after investigation.** When the investigation no longer needs the
   version, destroy it. Its record, fingerprint and audit trail stay.

## Where to look

| Question | Answer |
|---|---|
| Which keys did a run use? | The run page's **Keys used** section: parties, arrangement revisions, and the verifying and decrypting versions. |
| Which runs used a version? | **runs** on the version, or `GET /escrow/validations?keyVersionId=`. |
| Who changed what, when? | The **History** tab of the party. The same events go to the event outbox as `escrow.*`. |
| Why can't a version be activated? | Its probe has not passed. **Probe on a worker** again and read the result. Codes: `KEY_STORE_NOT_CONFIGURED`, `KEY_STORE_UNAVAILABLE`, `SECRET_NOT_FOUND`, `MATERIAL_UNREADABLE`, `FINGERPRINT_MISMATCH`, `ROUND_TRIP_FAILED`. |
