# EPP operator runbook

What to do when an EPP alert fires, and how to carry out the routine operations. Every setting and limit named here is in [POLICY_REFERENCE.md](POLICY_REFERENCE.md); this page says what to look at and what to change, and links there for the rules.

- **Alerts** are in [`prometheus/rules/epp.rules.yml`](../../prometheus/rules/epp.rules.yml). Each links to its section below. `critical` pages someone now, `warning` is looked at the same day, `info` is for a ticket.
- **The dashboard** is *EPP server* in Grafana ([`grafana/dashboards/epp.json`](../../grafana/dashboards/epp.json)), provisioned with the local stack (`docker compose --profile full up`).
- **Admin API calls** below need the permission named; with Auth0 off, the static admin token holds them all. `$API` is the admin API, `$CLID` the registrar, `$LOGIN` an EPP login.

---

## Alerts

### EPP server down

`EPPServerDown`: Prometheus cannot scrape a node's ops port for 2 minutes.

1. Is the process up? `GET /healthz` on the ops port (`EPP_OPS_PORT`, 9700). If it answers, the scrape path is the problem (network, Prometheus config), not the server.
2. If not, read the node's last logs. A node that will not start says why on its last lines: a configuration error (every one is listed, ADR 0018), the database unreachable, the schema guard, the repository ID changed (ADR 0017), or the server certificate missing.
3. `GET /readyz` answers 503 while a node starts or drains; the load balancer stops sending it sessions, which is expected during a deploy.

Registrars' sessions on a node that died are gone; their leases expire from the session registry within `EPP_SESSION_LEASE` (90s), after which they count against no cap.

### Rotate the server certificate

`EPPServerCertExpiresIn30Days` (warning), `EPPServerCertExpiresIn7Days` (critical), `EPPServerCertReloadFailing`.

Write the new key, then the new certificate, over the files at `EPP_TLS_KEY_PATH` and `EPP_TLS_CERT_PATH`. Each node picks them up within `EPP_TLS_RELOAD_INTERVAL` (1m) without dropping a session ([POLICY_REFERENCE.md § Server settings](POLICY_REFERENCE.md#server-settings-environment)). Check `epp_server_cert_not_after_timestamp_seconds` has moved on every node.

If `EPPServerCertReloadFailing` fires, the files on that node do not parse, do not match, or fail the checks; the node keeps serving its current certificate. Fix the files; the next check picks them up.

### A registrar certificate is expiring

`EPPClientCertExpiresIn30Days` (info), `EPPClientCertExpiresIn7Days` (warning), per registrar and login, from the certificate each login last logged in with. That is per node and kept until the login logs in again or the node restarts: a login that has switched to a new certificate but not logged in since still shows the old one, so check the registrar's pins (`GET $API/registrars/$CLID/epp`) before chasing it. The registry also records an `epp.certificate.expiring` event 30 and 7 days before a pin's end ([§ Events](POLICY_REFERENCE.md#events)).

The registrar pins its next certificate before the current one expires: `POST $API/registrars/$CLID/epp/certificate-pins` with the new certificate (`epp:credentials:admin`). Up to three pins are usable at once, so the next can be pinned ahead of the switch. Once the registrar has switched, retire the old pin (`POST …/certificate-pins/{id}/retire`, `{"kind": "normal"}`). See [§ Rotate a registrar's credentials](#rotate-a-registrars-credentials).

### A policy store is degraded

`EPPPolicyStoreDegraded`: a node cannot reach Redis for `store` (`sessions`, `lockout`, `rates`, `failed_creates`) and is counting on its own (fail-open, ADR 0013). `EPPSessionRegistryRenewalsFailing` usually fires with it.

Nothing is refused because of it, but caps hold per node rather than across the cluster: with N nodes, up to N times a session cap, rate or quota can pass. Restore Redis (`REDIS_HOST`, `REDIS_PORT`). When it answers, each node puts its sessions back within one heartbeat, and the gauge returns to 0 on its own. Lockouts and suspensions a node made on its own while degraded last their time and cannot be lifted through the API.

### Credentials or policies are stale

`EPPCredentialsStale`, `EPPPoliciesStale`: a node has not loaded registrar credentials, or EPP policies, for over 5 minutes (it tries every `EPP_CREDENTIAL_REFRESH_INTERVAL`, 30s).

The node keeps the set it last loaded, so open sessions are unaffected, but admin API changes do not reach it: a retired credential still works there, a new limit does not apply. New logins still check the database directly. Look for the database being unreachable from that node (`epp_credential_refresh_errors_total`, `epp_policy_refresh_errors_total`, and the node's logs).

### Audit records are failing

`EPPAuditRecordsLost` (critical): a node could not write EPP audit records at all. `EPPAuditWritesRetrying`: transform records could not be written before their response and were queued, or a queued batch keeps failing. `EPPEventsLost`: outbox events (lockouts, suspensions, quota, closes) could not be written; what they record still happened.

The audit log is in the database (`epp_transactions`), so these are database failures from that node: connections (`EPP_DB_MAX_OPEN_CONNS`), disk, or permissions. Lost records cannot be recovered; the node's logs name the svTRIDs. Fix the database, and the retrying records are written.

### A login was suspended

`EPPLoginSuspended`: a login was suspended for creating registered names past its failed-create budget ([§ Failed creates](POLICY_REFERENCE.md#failed-creates)). `EPPFailedCreateSpike` (info) shows a registrar with many such creates before any suspension.

1. See who and why: `GET $API/epp/suspensions?clid=$CLID` (`epp:policy:admin`) gives the scope (the registrar's own strikes or its family's) and when it ends. The `epp.login.suspended` event has the source address and strikes.
2. Look at the registrar's daily counts: `GET $API/registrars/$CLID/epp/create-outcomes`.
3. Usually nothing more: the suspension ends after 15 minutes. If the registrar has fixed its client, lift it ([§ Lift a suspension or a lockout](#lift-a-suspension-or-a-lockout)). If it keeps happening, give the registrar a stricter policy ([§ Change a registrar's limits](#change-a-registrars-limits)).

### A lockout storm

`EPPLockoutStorm`: many logins refused while locked out. `EPPLoginFailureStorm`: many failed logins.

`epp_login_failures_total{reason}` says why logins fail: a wrong password (counts towards lockouts), an unknown login, a source not allowed, a certificate not pinned. A storm of one registrar's wrong passwords is usually a client deployed with an old password; a storm of unknown logins or unpinned certificates from many sources is someone probing, and is refused before any password is checked. `GET $API/epp/lockouts?clid=` lists the locked (login, source) pairs.

For a registrar's own misconfigured client: once it is fixed, lift the lockouts ([§ Lift a suspension or a lockout](#lift-a-suspension-or-a-lockout)).

### A cap is saturated

- `EPPNodeConnectionCapReached`: a node refused connections at `EPP_MAX_CONNECTIONS` or `EPP_MAX_HANDSHAKES`. Add nodes, or raise the cap if the node has room (memory, file descriptors).
- `EPPSessionCapsSaturated` (info): logins refused at a registrar's, login's or family's session cap. If it is one registrar that needs more, give it a policy with more sessions; `GET $API/epp/sessions?clid=` shows what is open.
- `EPPCreateQueueTimeouts`: domain creates refused because no create slot freed within `EPP_CREATE_QUEUE_WAIT` ([§ Create queue](POLICY_REFERENCE.md#create-queue)). Usually a drop: expected for minutes, not hours. If it persists, the database is slow (`epp_command_duration_seconds{command="domain:create"}`), or `EPP_MAX_CREATES_IN_FLIGHT` is too low for the pool.

### Commands are slow

`EPPCheckLatencyHigh`: p99 `<domain:check>` over 250ms for 10 minutes. A check runs `EPP_CHECK_PARALLELISM` lookups at once against the database, so this is the database or the node's pool: look at the database's load and slow queries, and at whether the node's connections are all in use (creates and checks share `EPP_DB_MAX_OPEN_CONNS`).

---

## Tasks

### Rotate a registrar's credentials

All under `$API/registrars/$CLID/epp` (`epp:credentials:admin`); the rules are in [§ Credentials](POLICY_REFERENCE.md#credentials-admin-api).

- **Password.** `POST …/logins/$LOGIN/passwords` with no body generates one and returns it once. Two passwords are usable at a time, so the registrar can switch over; then retire the old one with `{"kind": "normal"}`. A registrar can also change its password itself with `<newPW>` at login.
- **Certificate.** Pin the new one (`POST …/certificate-pins`), let the registrar switch, retire the old pin.
- **Allowed network.** Add the new one (`POST …/allowed-networks`), then retire the old.
- **Compromise.** Retire the credential with `{"kind": "compromise"}`: it stops working at once, and every session that used it ends within `EPP_CREDENTIAL_REFRESH_INTERVAL`.

Every change is recorded as an outbox event.

### End sessions

`epp:sessions:admin`. `GET $API/epp/sessions?clid=$CLID` lists live sessions across every node. `DELETE $API/epp/sessions/{id}` ends one; `DELETE $API/epp/sessions?clid=$CLID` (or `login`, `client_ip`, `credential_id`) ends every one that matches. Both answer 202: the node holding a session closes it within moments. See [§ Sessions](POLICY_REFERENCE.md#sessions).

### Change a registrar's limits

`epp:policy:admin`. A registrar's limits are its policy's ([§ Policies](POLICY_REFERENCE.md#policies)); there are no per-registrar overrides.

1. `GET $API/registrars/$CLID/epp/policy` shows what applies: its policy, and its family's.
2. Create or change a policy: `PUT $API/epp/policies/{name}` with every limit.
3. Assign it: `PUT $API/registrars/$CLID/epp/policy` `{"policy": "{name}"}`; `"default"` removes the assignment.

Every node applies the change within `EPP_CREDENTIAL_REFRESH_INTERVAL`, to the next command and the next login. Open sessions stay open under a lowered session cap.

### Lift a suspension or a lockout

- **Suspension** (failed creates; `epp:policy:admin`): `DELETE $API/registrars/$CLID/epp/logins/$LOGIN/suspension`. It ends at once on every node and clears the strikes that caused it; if the registrar is still over budget, escalation starts again from the first strike.
- **Lockout** (failed passwords; `epp:credentials:admin`): `DELETE $API/registrars/$CLID/epp/logins/$LOGIN/lockouts` clears the login's lockouts from every source.

Both are recorded first as outbox events with you as actor. A suspension or lockout a node made on its own while Redis was down is not reached, and ends on its own.

### Drain a node

Send it SIGTERM (a rolling deploy does). `/readyz` answers 503 at once, so the load balancer stops sending it sessions; idle sessions close, in-flight commands get `EPP_DRAIN_TIMEOUT` (30s) to finish, and what is still open then is closed. Registrars reconnect to the other nodes. Their sessions leave the session registry as they close, so session caps free up straight away.
