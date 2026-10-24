# EPP server: settings and policy reference

This page lists every setting that governs the EPP server: its default, where it lives, how to change it, and when a change takes effect. When you add, remove or re-default a setting, update this page in the same PR. The design behind these settings is in [ARCHITECTURE.md](ARCHITECTURE.md), and the decisions are in [ADR 0013](../adr/0013-epp-server-architecture.md), [ADR 0014](../adr/0014-epp-credential-rotation.md) and [ADR 0016](../adr/0016-epp-schema-validation-in-go.md).

Settings live in one of two places:

| Kind | Lives in | How to change | Takes effect |
|---|---|---|---|
| **Server settings**: how a node listens, terminates TLS and times out | Environment variables, read once at start by `internal/config/epp.go` (INV-17). Each is registered in `internal/config/env_registry.go` and published in `deploy/contract.json` (`make generate-contract`) | Set the variable in the deployment's environment (Doppler for local and dev) | On restart. An unusable value stops the server at start rather than being silently replaced by a default |
| **Policy limits**: what a registrar may do over its sessions | Database: named EPP policies, assigned to registrars and registrar families ([§ Policies](#policies)) | Admin API (`/epp/policies`, `/epp/families`, `/registrars/{clid}/epp/policy`) | Within `EPP_CREDENTIAL_REFRESH_INTERVAL`, without a restart |

A value marked *(planned)* does not exist yet. The phase that introduces it is named in [ARCHITECTURE.md § Roadmap](ARCHITECTURE.md#roadmap).

---

## Server settings: environment

Durations use Go syntax (`30s`, `10m`, `24h`). Every value is checked at start. A bad value, or an inconsistent combination, stops the server and lists every problem at once.

**Registry identity**

| Variable | Default | Meaning |
|---|---|---|
| `ROID_REPOSITORY_ID` | *(required)* | The registry's EPP repository ID from IANA's [EPP Repository Identifiers](https://www.iana.org/assignments/epp-repository-ids) registry: 1 to 8 ASCII letters or digits. Every ROID ends in it (`123_DOM-XYZ`). Read by every process that mints or shows ROIDs (admin API, EPP server, worker, whois, MCP, askg, JISC import), which will not start without it. Docker Compose and Tilt set `LOCAL`, CI sets `CITEST` ([ADR 0017](../adr/0017-roid-repository-id.md)) |

How it behaves:
- **One ID for the installation.** Contacts and hosts belong to no TLD, so a per-TLD ID would have no answer for them.
- **Only the ROID's number is stored.** The ID is appended whenever a ROID is read, so it is the same everywhere a ROID is shown.
- **Set it once, before the first start.** The first start records it in the database. A process configured with a different ID afterwards refuses to start, because running would silently rename every object.
- **ROIDs are always generated.** The admin API does not accept one on create, and escrow imports give every imported object a new one.
- **Before an RST or OT&E run,** register the ID with IANA (first come, first served) and set it here. RST epp-14 checks the registration with IANA.

**Listening and TLS**

| Variable | Default | Meaning |
|---|---|---|
| `EPP_PORT` | `700` | TCP port the EPP listener binds |
| `EPP_OPS_PORT` | `9700` | Plain-HTTP port for `/healthz` (process up), `/readyz` (accepting sessions; 503 while starting or draining) and `/metrics`. Keep it on the internal network |
| `EPP_TLS_CERT_PATH` | *(required)* | PEM server certificate, with its chain, presented to registrars |
| `EPP_TLS_KEY_PATH` | *(required)* | PEM private key for that certificate |
| `EPP_TLS_SELF_SIGNED` | `false` | Present a generated one-day self-signed certificate instead. **Local development only.** Conflicts with the two file variables. Docker Compose and Tilt set it |
| `EPP_TLS_RELOAD_INTERVAL` | `1m` | How often the certificate and key files are checked for a change |
| `EPP_SERVER_NAME` | *(unset)* | The hostname registrars connect to (RST's `epp.hostName`). When set, a certificate that does not cover it in a subjectAltName (exact or wildcard) is refused. **Set it in every deployment** |
| `EPP_PROXY_PROTOCOL` | `false` | Require a PROXY protocol (v1 or v2) header on every connection. **Must be `true` behind a load balancer**, or every per-IP control sees only the balancer's address |
| `EPP_PROXY_TRUSTED_CIDRS` | *(required when the above is true)* | Comma-separated CIDRs or IPs of the load balancers. A connection from anywhere else is dropped at accept |

Fixed in code (RST epp-01):
- **TLS versions:** 1.2 minimum; 1.3 is used when the client offers it.
- **TLS 1.2 cipher suites:** only the four RFC 9325 §4.2 recommends, ECDHE with AES-GCM for RSA and ECDSA keys. TLS 1.3 suites are all AEAD and not configurable in Go.
- **Certificate checks:** a certificate from file that is expired or not yet valid is refused, at start and on reload. So is one that misses `EPP_SERVER_NAME`. At start the server will not run; on reload it keeps the current certificate. The certificate file must contain the intermediates after the leaf, because clients are given exactly what is in the file.

**Rolling the server certificate.** Replace the certificate and key files in place; the server picks them up within `EPP_TLS_RELOAD_INTERVAL`:
- New handshakes get the new certificate. Sessions already open keep the one they negotiated, so nothing is dropped.
- A file that is missing, does not parse, or fails the checks above is ignored, and the current certificate stays in use. Each failure increments `epp_server_cert_reload_errors_total`, so a half-written renewal cannot take the server down.
- Write the key before the certificate. A certificate that does not match its key is treated as a failure and retried on the next check.
- `epp_server_cert_not_after_timestamp_seconds` exposes the expiry of the certificate in use.

**Per-node limits.** These hold without Redis, and each node enforces its own.

| Variable | Default | Meaning |
|---|---|---|
| `EPP_MAX_CONNECTIONS` | `2000` | Open connections on one node. Above this, new connections are closed at accept |
| `EPP_MAX_HANDSHAKES` | `200` | TLS handshakes in progress on one node. Above this, new connections are closed at accept. Must not exceed `EPP_MAX_CONNECTIONS` |
| `EPP_MAX_FRAME_BYTES` | `65536` | Largest frame accepted, including its 4-byte length header (minimum 4096). A create with 13 NS, 8 DS and extensions is about 8 KB, and a 50-name check is at most about 15 KB. A larger frame **closes the session**: the rest of it cannot be skipped safely |
| `EPP_XML_MAX_DEPTH` | `32` | Deepest element nesting. A deeper frame is answered 2001 and the session continues |
| `EPP_XML_MAX_ELEMENTS` | `2000` | Most elements in a frame. A larger one is answered 2001 and the session continues |

A frame containing a `<!DOCTYPE>` or any other `<!...>` declaration is always answered 2001. So is malformed XML, a root other than `<epp>`, a non-UTF-8 encoding, and a `clTRID` outside 3–64 characters. None of these close the session.

**Timeouts**

| Variable | Default | Meaning |
|---|---|---|
| `EPP_HANDSHAKE_TIMEOUT` | `10s` | TCP accept to completed TLS handshake, including reading any PROXY header |
| `EPP_LOGIN_TIMEOUT` | `30s` | Greeting to a successful `<login>`. A command already running finishes first |
| `EPP_IDLE_TIMEOUT` | `10m` | Waiting between commands |
| `EPP_READ_TIMEOUT` | `30s` | Reading one frame once its header has arrived |
| `EPP_WRITE_TIMEOUT` | `60s` | Writing one response |
| `EPP_MAX_SESSION_DURATION` | `24h` | Longest a session may stay open |
| `EPP_DRAIN_TIMEOUT` | `30s` | On SIGTERM, time for in-flight commands to finish. Idle sessions close at once, and sessions still open at the deadline are closed |

**Authentication**

| Variable | Default | Meaning |
|---|---|---|
| `EPP_REQUIRE_ALLOWLIST` | `true` | Drop a connection from outside **every** registrar's allowed networks before the TLS handshake, and refuse a login from outside **its own** registrar's. `false` only for local development, where Docker's NAT hides the client address. Docker Compose and Tilt set it |
| `EPP_CREDENTIAL_REFRESH_INTERVAL` | `30s` | How often a node reloads the union of allowed networks, and ends sessions that used a credential retired as compromised. A new allowed network can take this long to admit its first connection; a compromise retire can take this long to end sessions. Logins themselves always read the database |
| `EPP_MAX_CONCURRENT_LOGINS` | `8` | Password verifications in progress on one node. Each Argon2id verification holds 19 MiB, so this bounds login memory at about 150 MiB. More logins wait their turn |
| `EPP_MAX_LOGIN_ATTEMPTS` | `3` | Failed logins one session may make. The last is answered 2501 and the session closes |
| `DB_HOST`, `DB_PORT`, `DB_USER`, `DB_PASS`, `DB_NAME`, `DB_SSLMODE` | as for the other services | The registry database: credentials, domains for `<check>`, and the audit log. `SCHEMA_GUARD` applies as for the API |
| `EPP_DB_MAX_OPEN_CONNS` | `20` | Database connections one node may open, shared by logins, commands and the audit log |

**Commands**

| Variable | Default | Meaning |
|---|---|---|
| `EPP_CHECK_PARALLELISM` | `8` | Availability lookups one `<check>` runs at once. Must be below `EPP_DB_MAX_OPEN_CONNS`, so one check cannot take every connection |
| `EPP_MAX_CREATES_IN_FLIGHT` | `10` | `<domain:create>`s one node runs at once, let in fairly when they queue ([§ Create queue](#create-queue)). Must be below `EPP_DB_MAX_OPEN_CONNS`, so creates cannot take every connection |
| `EPP_CREATE_QUEUE_WAIT` | `5s` | The longest a `<domain:create>` waits for a slot before it is refused with 2400 |

**Audit log**

| Variable | Default | Meaning |
|---|---|---|
| `EPP_AUDIT_RETENTION` | `8760h` (365 days) | How long each command is kept. Every node deletes older rows hourly, in batches |
| `EPP_AUDIT_QUERY_RESPONSES` | `false` | Also keep the response frames of queries (`<hello>`, `<check>`, `<info>`, `<poll op="req">`, `<transfer op="query">`). Their requests are always kept, and transforms keep both frames. A 50-name check response with fee data is up to about 50 KB, so this is off by default |
| `EPP_AUDIT_BUFFER` | `10000` | Query records one node may hold before writing them in a batch. When it is full, records are written one at a time instead; none is dropped |

**Shared state and logging**

| Variable | Default | Meaning |
|---|---|---|
| `REDIS_HOST` | `localhost` | Redis holding the session registry and the lockouts. A single Redis (or one hash slot): the registry's scripts touch several keys at once. The admin API reads the same Redis to list and end sessions |
| `REDIS_PORT` | `6379` | Redis port |
| `REDIS_PASSWORD` | *(empty: no auth)* | Redis password. Secret |
| `LOG_LEVEL` | `info` | `debug`, `info`, `warn` (or `warning`), or `error`, in any case. `debug` logs every command with its clTRID, svTRID, result code and duration |

## Command handling

Fixed in code; the design is [ARCHITECTURE.md §4](ARCHITECTURE.md#4-frame--protocol-hygiene).

**The pipeline.** Every frame passes, in order: the frame scanner (size, DTD, depth and element caps; 2001), parsing, routing (2000 for an unknown command, 2101 for an RFC 5730 command that is not bound), the session state (2002 before login, or for a second login), the services negotiated at login (below), schema validation (2001), a check that an object command's element is that command's (2001: `<delete><host:info>` passes the schema, whose wildcard takes any object element), a check that no extension element is given twice (2001: a handler reads one, so a second `<secDNS:update>` would be ignored), the policy (the registrar's EPP policy, [§ Policies](#policies); `epp_policy_decisions_total{rule,action}`), and the handler. Then the command is audited and its response sent.

**Schema validation** ([ADR 0016](../adr/0016-epp-schema-validation-in-go.md)). Every `<hello>` and every bound command is validated against Go models of the EPP schemas, which a differential test keeps in agreement with xmllint on the RST-pinned XSDs. A failure is 2001, with a message naming the element and the rule it broke (`Command syntax error; /epp/command/login/pw: must be 6 to 16 characters`), never the value. A malformed `<login>` is not a failed login. `epp_frames_rejected_total{reason="schema"}` counts them.

**Services negotiated at login.**

| Case | Answer |
|---|---|
| `<login>` asks for an object service the greeting does not offer | 2307; not a failed login |
| `<login>` asks for an extension the greeting does not offer | 2103; not a failed login |
| A command on an object service the session did not ask for | 2307 |
| A command carrying an extension the session did not ask for | 2103 |

The greeting offers the domain, contact and host object services. It offers three extensions: DNSSEC (`secDNS-1.1`, RFC 5910) for DS data on `<domain:create>`, `<domain:info>` and `<domain:update>`; RGP (`rgp-1.0`, RFC 3915) for the grace period statuses on `<domain:info>`; and secure authorization information for transfer (`secure-authinfo-transfer-1.0`, RFC 9154), which defines no elements and says the registry keeps domain transfer codes as below.

**`<domain:check>`.** Each name is answered in the order asked, duplicates included, with `avail="1"` or `avail="0"` and a reason:

| Reason | When |
|---|---|
| `Invalid domain name` | Not a valid host name with at least two labels |
| `TLD not served by this registry` | No such TLD |
| `Not open for registration` | The TLD has no general-availability phase in force |
| `Not allowed by registry policy` | The current phase's label rules (length) exclude it |
| `In use` | Registered |
| `Reserved` | On the reserved and blocked names list (ADR 0011) |

Availability is the same for every registrar. Names are looked up `EPP_CHECK_PARALLELISM` at a time, and each TLD's phases are read once per command. A registry that cannot answer is 2400 for the whole command; there are no partial answers. `epp_check_names` is the histogram of names per check.

**`<domain:info>`.** Answered in full only to the sponsoring registrar (ADR 0006). Another registrar gets 2201 without the domain's transfer code, and is not told who the sponsor is; with the code, see *Transfer codes* below. Authorization information other than a password (`<domain:ext>`) is 2102, from any registrar. The `<infData>` carries the name, ROID, statuses, registrant and contacts, nameservers and subordinate hosts as the `hosts` attribute selects (`all`, the default; `del`; `sub`; `none`), the sponsor and creator, and the dates, `<domain:trDate>` once the domain has been transferred. When a transfer code is set it carries an empty `<domain:authInfo><domain:pw/></domain:authInfo>`, never the code; when none is set, no `<domain:authInfo>`. A session that logged in with RGP also gets `<rgp:infData>` with the domain's grace period statuses (`addPeriod`, `renewPeriod`, `autoRenewPeriod`, `transferPeriod`; or, once deleted, `redemptionPeriod`, `pendingRestore` or `pendingDelete`), when it is in one.

**`<domain:create>`.** The registrar must be accredited for the TLD (ADR 0006). The domain is registered in the TLD's current general-availability phase, through the same registration path as the admin API, which prices it, assigns its ROID and writes its `domain.registered` event with the session's login (`epp:<login>`) as actor and the svTRID as correlation ID. Every rule is applied in one order, by the registry, so that of several problems with a command the first in this order is the one reported:

| Refusal | Code |
|---|---|
| Not a valid domain name | 2005 |
| TLD not served, or not open (no GA phase in force) | 2306 |
| Registrar not accredited for the TLD | 2201 |
| A period in months that is not whole years | 2306 |
| Period outside 1 year to the phase's maximum horizon (10 by default); no period is 1 year | 2004 |
| The phase's label rules exclude the name | 2306 |
| Registered | 2302, also when another registrar registers it while the create runs |
| Reserved (ADR 0011) | 2308 |
| A `<domain:contact>` without a type | 2003 |
| Two contacts of one type | 2306 |
| A contact the phase requires is missing (by default: registrant and tech) | 2003 |
| A contact the phase prohibits is given | 2306. The admin path drops it silently; EPP tells the registrar |
| A contact that does not exist | 2303 |
| Another registrar's contact | 2201 |
| Host attributes (`<domain:hostAttr>`): this registry uses host objects | 2306 |
| A host object named more than once | 2306 |
| More than 10 nameservers (`entities.MaxHostsPerDomain`) | 2306 |
| A nameserver that is not one of the registrar's host objects | 2303 |
| `<secDNS:maxSigLife>`: not supported | 2102 (RFC 5910 §3.2) |
| The key data interface (`<secDNS:keyData>`): this registry takes DS data | 2306 (RFC 5910 §4) |
| More than 8 DS records | 2306 |
| A DS record that is malformed: key tag out of range, digest not matching its type's length | 2005 |
| A DNSSEC algorithm or digest type the registry does not accept (below) | 2306 |
| The same DS record twice | 2306 |
| `<domain:ext>` authorization information: the registry keeps only passwords | 2102 |
| A transfer code not random enough (below) | 2202 (RFC 9154 §5.2) |

Values are read as their schema types define them: identifiers whitespace-collapsed, and the password (a `normalizedString`) with tabs and line breaks as spaces, so the password stored is the one that was validated. Each refusal's message names the object or rule, after the standard text; anything else is 2400, with the detail only in the log. A phase that requires validation answers 1001 (`pendingCreate`). The ROID ends in the configured repository ID, `ROID_REPOSITORY_ID` (above, under Registry identity).

**DNSSEC.** The registry takes DS data (RFC 5910's DS data interface) on `<domain:create>`, and returns it in `<secDNS:infData>` on `<domain:info>` to a session that logged in with `secDNS-1.1`. A `<secDNS:keyData>` inside a `<secDNS:dsData>` is accepted as supporting information and not kept. What it accepts from registrars, fixed in code (`services.AcceptedDSAlgorithms`, `AcceptedDSDigestTypes`, `MaxDSRecords`): rules of the registry's data, the same for every registrar, so not in the EPP policies (ADR 0019):

| | Accepted | Refused |
|---|---|---|
| Algorithm | 8 RSASHA256, 10 RSASHA512, 13 ECDSAP256SHA256, 14 ECDSAP384SHA384, 15 ED25519, 16 ED448 | 1 RSAMD5, 3 DSA, 5 RSASHA1, 6 DSA-NSEC3-SHA1, 7 RSASHA1-NSEC3-SHA1, 12 ECC-GOST, and every unassigned number. RFC 8624 rules out MD5, DSA and GOST, and the SHA-1 algorithms are deprecated for signing |
| Digest type | 2 SHA-256, 4 SHA-384 | 1 SHA-1 and 3 GOST: RFC 8624 rules both out for delegations |
| Records per domain | up to 8 | |

The policy is for what registrars submit. A DS record imported from an escrow deposit is kept whatever its algorithm, as long as it is well formed: the domain model checks structure (a hex digest whose length matches its digest type), not policy. Both import paths carry them: the per-domain import through `CreateDomainCommand.FromRdeDomain`, and the bulk import (deposit → SQLite → `DirectDBImporter`), which skips a malformed record the way it skips a malformed host: a `SKIP DS RECORD` log line and an entry under `skipped_ds_records` in the importer's run report. The escrow import workflow saves that report as `run-report-domains.json` in the run folder, names it in `summary.json` (`importerReportKeys`), and counts the skipped records there as `ds_records_skipped`. A bulk re-import replaces each deposited domain's DS records with the deposit's, clearing them where the deposit has none; a staging database built without DS records (before they were staged) leaves them alone. A domain's DS records are published with the TLD's delegations, for domains that are active, not pending delete and linked to at least one nameserver: `GET /tlds/{tld}/dns/ds-records` (`?format=text` for zone file lines), beside `dns/domain-delegations` and `dns/glue-records`.

**`<domain:update>`, `<domain:renew>` and `<domain:delete>`.** Each is answered only for the sponsor: another registrar gets 2201, and a domain nobody has 2303. Each is checked and written on one reading of the domain: if another command changes the domain in between, including the lifecycle workflows, the write is refused and the command is read and checked again, up to three times. Pricing, the grace periods and the events (`domain.updated`, `domain.renewed`, `domain.marked_for_deletion`, with the session's login as actor) are the registration path's, as for create. The lifecycle processing (expiry, auto-renewal, restore) and the admin API's status changes also write only over the domain as they read it, so none of them overwrites a renew or update made meanwhile; a lifecycle write that loses the race is reported failed for that domain and picked up again on its next run.

A domain with `clientHold` or `serverHold` keeps its nameservers and DS records, but leaves the TLD's NS, DS and glue exports until the hold is removed. A pendingDelete domain is left out of all three too.

`<domain:update>` removes statuses, contacts, nameservers and DS records, then adds them, then makes the changes in `<domain:chg>`. The nameservers and DS records are written with the domain in one transaction, and every host linked or unlinked has its linked and in-bailiwick flags recomputed. A client sets only `clientDeleteProhibited`, `clientHold`, `clientRenewProhibited`, `clientTransferProhibited` and `clientUpdateProhibited`. It applies its rules in this order:
1. No `<domain:add>`, `<domain:rem>`, `<domain:chg>` or extension (RFC 5731 §3.2.5): 2003.
2. A status the client does not control: 2306.
3. A domain with a pending action, with `serverUpdateProhibited`, or with `clientUpdateProhibited` unless the update removes it: 2304.
4. Nameservers as host attributes: 2306. A `<secDNS:update urgent="true">` or a maxSigLife: 2102. Key data: 2306.
5. Removing a status, contact, nameserver or DS record the domain does not have, or adding a status it has: 2306.
6. Adding a contact of a type the domain has: 2306. A contact that does not exist: 2303; another registrar's: 2201. The phase's contact policy after the change (by default a registrant and a tech contact): 2003 if a required contact would be missing, 2306 if a prohibited one is given. An empty `<domain:registrant/>` removes the registrant, where the policy allows it.
7. Adding a nameserver that is not one of the registrar's host objects: 2303. One the domain has, or more than 10: 2306. A nameserver under the domain itself without an address: 2306.
8. Adding a DS record that is malformed: 2005. One with an algorithm or digest type the registry does not accept, one the domain has, or more than 8 in all: 2306.
9. `<domain:ext>` authorization information: 2102. A transfer code not random enough: 2202. An empty `<domain:pw/>` or `<domain:null/>` removes the code (RFC 9154 §4.2).

`<domain:renew>`, in this order:
1. A `<domain:curExpDate>` that is not the domain's expiry date: 2306. This makes a renew repeated by mistake fail rather than renew twice. A time zone on the date is ignored.
2. A period in months that is not whole years: 2306. No period is one year.
3. A domain with `clientRenewProhibited`, `serverRenewProhibited` or a pending action: 2304.
4. An expiry date more than the phase's maximum horizon (10 years by default) from now: 2306.

A renewed domain is in its renew grace period (`renewPeriod`, RFC 3915) and the answer carries the new expiry date in `<domain:renData>`.

`<domain:delete>` answers 1001: the domain becomes `pendingDelete` and goes through its redemption period, or, within its add grace period, is purged at once by the purge workflow. In this order:
1. A domain with a pending action, `clientDeleteProhibited` or `serverDeleteProhibited`: 2304. An update prohibition does not stop a delete: it stops updates only (RFC 5731 §2.3).
2. A host object under the domain: 2305, naming the hosts. Delete or rename them first. The check is made again in the delete's own transaction, and a host create or rename into a domain locks the domain's row first and is refused (2304) once the domain is pendingDelete, so neither can slip past the other.

**Restore** (RFC 3915 §4.2.5; [`entities.Domain.RequestRestore`](../../pkg/domain/entities/restore.go) describes the process). The user guide is the admin UI's *Domain Restore* page (`frontend/lib/constants/domainRestoreDoc.ts`). A deleted domain is restored with a `<domain:update>` that changes nothing (`<domain:chg/>`) and carries `<rgp:update>`, for a session that logged in with RGP (2103 otherwise), in two steps, as .com does:

| Step | `<rgp:restore op>` | Answer |
|---|---|---|
| Request | `request` | 1000, with `<rgp:upData><rgp:rgpStatus s="pendingRestore"/>` |
| Report | `report`, with `<rgp:report>` | 1000, the domain restored |

Both are answered only for the sponsor (2201 otherwise), and each is checked in this order:
1. Any other change in the same update (an `<domain:add>`, `<domain:rem>` or `<domain:chg>` with content, or `<secDNS:update>`): 2306. Send it separately once the domain is restored.
2. A request for a domain that is not `pendingDelete`, whose redemption period has ended, or whose restore is already pending: 2304. A report with no restore pending, or after the request lapsed: 2304.
3. A report with a required part missing (2003): `op="report"` without `<rgp:report>`; an empty `preData`, `postData` or `resReason`; fewer than two non-empty `statement`s; or no `delTime` or `resTime` (over EPP the schema requires both, 2001 without; the admin API's JSON can leave one out). A report whose times are not possible (2004): a `delTime` not before `resTime`, a `resTime` in the future, or a time whose year in UTC is outside 1 to 9999 (the report is kept as JSON with its event, which cannot hold such a year; `-0004-02-29T00:00:00Z` and `9999-12-31T23:00:00-14:00` are valid `xs:dateTime`s refused here). Each is checked before anything is written.
4. A restore the registry cannot price (its restore fee and the renewal): 2400.

During `pendingRestore` the EPP status stays `pendingDelete` (RFC 3915 §4), so every other update, renew, delete and transfer is still 2304, and `<rgp:infData>` shows `pendingRestore`. The report restores the domain in the command, written over the domain as read: `pendingDelete` and `pendingRestore` are cleared with the redemption period and purge date; the expiry is extended by a year, and by further years while it would still be past; no renew grace period starts. The report is kept as sent (XML included) on the `domain.restored` event. A restore is two charges, two events: `domain.restored` with the restore fee quote, and `domain.renewed` with the renewal quote for the years added (ICANN's RGP terms: the restore fee is in addition to the renewal). The request is recorded as `domain.restore_requested`, with no years and no quote. When the fees are taken, and whether a lapsed request is refunded, is the billing system's to decide.

A request whose report is not sent by its deadline (`pendingRestoreEnd`) lapses: the `Restore` workflow (schedule `restore-loop`, hourly at :45 on the `lifecycle` queue) returns the domain to `pendingDelete` as before the request, in its redemption period if that has not ended, with its original purge date, recorded as `domain.restore_reverted` without a charge. It re-checks each domain before reverting it, and pages with a (deadline, ROID) cursor, as the transfer approval sweep does. A domain with a restore pending is never purged, even past its purge date. The sponsor is told by a poll message queued in the same transaction as the revert: text alone (`<msg>` beginning `Restore lapsed:`, naming the domain, the deadline missed, and either the end of the redemption period, when a restore can be requested again, or the purge date), with no `<resData>`, since RFC 3915 defines no poll data.

Restores requested before this model (pendingRestore without pendingDelete or a deadline, completed by the old `Restore` loop without a report) are moved to it by the `migrate` job: pendingDelete again, the report due 7 days after the job ran, the redemption end and purge date kept.

A domain imported from an escrow deposit (RFC 9022) in its end of life keeps it ([`entities.Domain.ApplyDepositEndOfLife`](../../pkg/domain/entities/restore.go)). A deposit says which RGP period a domain is in, not when it ends, so the period starts in full at the import, under the TLD's current GA phase: `redemptionPeriod` is restorable for `redemptionGP`; `pendingRestore` keeps its restore, with the report due here after `pendingRestorePeriod` (and the redemption period started in full behind it, for if the report does not come); `pendingDelete` is purged after `pendingdeleteGP`. An EPP `pendingDelete` with none of these RGP statuses is read as `redemptionPeriod`, the longer reading of what is unknown. The bulk importer reads the phase; the admin-API import path uses the default periods.

| Phase policy setting | Default | Meaning |
|---|---|---|
| `redemptionGP` | 30 days | How long a deleted domain can be restored (`redemptionPeriod`) |
| `pendingdeleteGP` | 5 days | How long it then waits to be purged |
| `pendingRestorePeriod` | 7 days | How long the registrar has to send the report after a request. A phase stored before this setting existed has 0, read as 7 |
| `restoreReportRequired` | true | `false` restores a domain on its request alone, for registries (often ccTLDs) that do not require a report: the request answers 1000 with no `<rgp:upData>` and records the three events |

Set on the phase's policy (`PUT /tlds/{tld}/phases/{phase}/policy`, or the admin UI's phase page); a change applies to restores requested after it. The admin API acts for the sponsor with the same rules: `POST /domains/{name}/restore` requests, `POST /domains/{name}/restore/report` reports (JSON `preData`, `postData`, `delTime`, `resTime`, `resReason`, `statements`, `other`); 409 for a state that does not allow the step, 400 for a refused report.

**Transfer codes** (RFC 9154; [`entities.AuthInfoHash`](../../pkg/domain/entities/authInfoHash.go)). A domain's authorization information is its transfer code. The user guide is the admin UI's *Domain Transfers* page (`frontend/lib/constants/domainTransfersDoc.ts`).
- **Unset is the normal state.** A domain is created with no code unless `<domain:create>` gives a non-empty `<domain:pw>`; the sponsor sets one with `<domain:update>` when the registrant asks to transfer, and clears it with an empty `<domain:pw/>` or `<domain:null/>`. Escrow imports create domains with no code. A completed transfer clears the code: a code works once.
- **Kept as a hash.** A salted SHA-256 hash with a 128-bit random salt per code, `NULL` when none is set. The code itself is never stored, returned, logged or audited. SHA-256 rather than a slow password hash, because a code the registry accepts carries 128 bits or more, which no offline search exhausts. Codes stored before this were hashed by the `migrate` job (`migrateDomainAuthInfo`); they keep working until replaced. Deploying the release that adds this is a **cutover, not a rolling deploy**: earlier releases read and write plain-text codes, and a hash cannot be turned back into its code. Stop every process that writes domains (admin API, EPP, worker, MCP), run the `migrate` job, then start the new release; do not roll back past it. A plain-text code written after the job (by a process left running) leaves that domain unreadable until the job runs again and hashes it. The job's statements are logged without their parameter values, so no code reaches its logs.
- **Strength, fixed in code** (`entities.ValidateSecureAuthInfo`). Length times log2 of the alphabet the code uses (26 lower-case, 26 upper-case, 10 digits, 33 other printable ASCII), at least 128 bits; at least 8 different characters; printable ASCII; at most 128 characters. Otherwise 2202 on create and update. For example 20 random characters of all four classes (131 bits), or 25 random lower-case letters and digits (129 bits). The admin API and UI apply the same rule (`entities.ValidateDomainAuthInfo`): every path that sets a code does.
- **Checking a code** (RFC 9154 §5.3, ADR 0006 amendment of 2026-10-22). A `<domain:info>` from a registrar that does not sponsor the domain:

| The registrar gives | Answer |
|---|---|
| No `<domain:authInfo>` | 2201 |
| The domain's code | 1000 with name, ROID, statuses, sponsor, crDate and exDate only: no contacts, nameservers, hosts, authorization information or extensions |
| A wrong or empty code, or any code when none is set | 2202, the same for each |

The comparison is in constant time. The admin API returns `AuthInfoSet` (true or false), never the code or the hash. Contact transfer codes are not covered yet: they are kept as before until contact transfer is built.

**`<domain:transfer>`** (RFC 5731 §3.2.4; [`entities.DomainTransfer`](../../pkg/domain/entities/transfer.go) describes the process, [`services.EPPDomainService`](../../internal/application/services/epp_domain_transfer.go) who may do what). The user guide is the admin UI's *Domain Transfers* page.

| Op | Who | Answer |
|---|---|---|
| `request` | Any registrar accredited for the TLD but the sponsor, with the domain's code | 1001, the transfer pending |
| `approve`, `reject` | The sponsor (the losing registrar) | 1000 |
| `cancel` | The registrar that requested the pending transfer | 1000 |
| `query` | The sponsor, either registrar of the latest transfer, or any registrar giving the domain's code | 1000, the latest transfer |

Anyone else gets 2201 (without the code) or 2202 (with a wrong one) and learns nothing about the domain's transfers. Every answer carries `<domain:trnData>`: `trStatus` (`pending`, `clientApproved`, `clientRejected`, `clientCancelled`, `serverApproved`, `serverCancelled`), `reID` and `reDate`, `acID` and `acDate` (while pending, the losing registrar and the date the registry will approve it; afterwards, who acted and when), and `exDate` while pending or once approved. A request is checked in this order: the domain exists (2303); the registrar is accredited for the TLD (2201); a period in months is whole years (2306); the registrar is not the sponsor (2106); the code matches (2202, before any status, so a registrar without it learns nothing); no transfer pending (2300); no other pending action and no `clientTransferProhibited` or `serverTransferProhibited` (2304); out of the transfer lock (2106); the expiry plus the period (one year by default) within the phase's maximum horizon (2306); and a transfer quote for the registrar (2400 if none can be made; a phase with no transfer price quotes it at no charge, as it does a registration or a renewal). An update prohibition does not stop a transfer. A domain pending transfer refuses update, renew and delete (2304). Answering a transfer that is not pending is 2301. `<domain:ext>` authorization information is 2102.

On approval, in one transaction ([`entities.TransferResolution`](../../pkg/domain/entities/transfer.go)): the domain moves to the gaining registrar; its expiry is extended by the period; `trDate` is set; its code is cleared; the losing registrar's add, renew and auto-renew grace periods end and the transfer grace period (`transferPeriod`) begins; a new transfer lock begins; grandfathered pricing granted until transfer ends; client statuses stay. Its nameservers follow RFC 5732's rule that a host belongs to its superordinate domain's sponsor: the losing registrar's hosts under the domain (named as the domain, or below it) move with it, with their addresses; an external nameserver is re-linked to the gaining registrar's host of that name, created if it has none; a nameserver under another domain of this registry keeps its link, since only that domain's sponsor may hold a host there and a copy would publish diverging glue. The losing registrar's hosts left unused stay (cleanup is a later phase). Every host touched is locked in ROID order, as the other writes lock them; a nameserver renamed or replaced since the approval was planned, or a gaining registrar's host created or deleted meanwhile, makes the approval read and plan again. Contacts stay as they are: the gaining registrar is told to replace them (ADR 0006, amended 2026-10-22). The approval is recorded as a billable `domain.transferred` event carrying the gaining registrar's transfer quote; the request, rejection and cancellation as `domain.transfer_requested`, `domain.transfer_rejected` and `domain.transfer_cancelled`, with no years and no quote.

A transfer unanswered at its `acDate` is approved by the registry (`serverApproved`): the `TransferApprovalLoop` workflow runs hourly on the `lifecycle` queue (schedule `transfer-approval-loop`, at :15) and completes it as an approval by the losing registrar would, through the same transaction, with poll messages to both registrars and the `domain.transferred` event carrying the workflow ID as correlation ID and the run ID as trace ID (INV-16). It re-checks each transfer before approving it, so one answered meanwhile is left alone. Each hourly sweep pages through the due transfers with an (acDate, ID) cursor, so one that cannot be approved is passed over and retried by the next sweep; transfers of domains since deleted are left out. A change to `pendingTransferPeriod` applies to transfers requested after it; a pending transfer keeps its `acDate`.

| Phase policy setting | Default | Meaning |
|---|---|---|
| `pendingTransferPeriod` | 5 days | How long the losing registrar has to answer; the pending transfer's `acDate` |
| `transferGP` | 5 days | The transfer grace period (`transferPeriod`). A phase stored before this default existed has 0, read as 5 |
| `transferLockPeriod` | 60 days | No transfer this long after a registration or a transfer. **0 is a setting**: no lock, for an RST or OT&E TLD (RST epp-19 transfers a domain it has just created) |
| `maxHorizon` | 10 years | The furthest expiry a transfer may give |

All four are set on the phase's policy (`PUT /tlds/{tld}/phases/{phase}/policy`, or the admin UI's phase page) and apply to the next command.

**`<poll>`** (RFC 5730 §2.9.2.3). Each registrar has its own message queue (ADR 0006). A transfer step queues a message for each of its two registrars that did not take it, carrying the transfer as it stood in `<domain:trnData>`: the losing registrar is told of a request and a cancellation, the gaining registrar of an approval (told to replace the contacts) and a rejection, and both of the registry's own approval or cancellation. `<poll op="req">` answers 1301 with the oldest message (`<msgQ count id>`, `<qDate>`, `<msg>`) and leaves it queued, or 1300 when the queue is empty. `<poll op="ack" msgID="…">` removes it and answers 1000 with `<msgQ count id>`, the messages that remain; a message not in the registrar's queue, another registrar's or one already acknowledged, is 2303, and an ack without `msgID` 2003. Messages are kept until acknowledged.

**Host objects** (RFC 5732). This registry uses host objects (RST's `epp.hostModel` is `objects`), and they belong to a registrar:
- Each registrar has its own host object for a name, unique per registrar, and a domain's nameservers are host objects of its sponsor. Two registrars can each have a host named `ns1.example.net`.
- Every host command, check included, is scoped to the session's registrar (ADR 0006). A command on a host that only another registrar has is 2201, without naming the sponsor; on a host nobody has, 2303.
- A host is *internal* if it is under a TLD this registry serves, and *external* otherwise. When the registry serves both `uk` and `co.uk`, the longest TLD counts. An internal host's *superordinate domain* is its TLD plus one label.
- Only the sponsor of the superordinate domain can create an internal host or rename a host into it, with or without addresses. So no registrar can put glue, or a glueless host, under another's domain (RST epp-25 to epp-27, the "wide glue" risk).
- Only an internal host has addresses. An external host is created without them, and a host renamed out of this registry's TLDs loses its addresses in the same update.
- An address must be of the IP version its `ip` attribute names (`v4` when absent), and public unicast: loopback, unspecified, private, link-local, multicast and IPv4-mapped IPv6 addresses are refused. Addresses are stored, and shown, in canonical form. At most 10 per host (`entities.MAX_ADDRESSES_PER_HOST`).
- A client sets only `clientDeleteProhibited` and `clientUpdateProhibited`. `linked` is reported by `<host:info>` from the domains that use the host now.

**`<host:check>`.** Up to the registrar's policy's `check_names` names (50 by default), each answered in order with whether the session's registrar can create it:

| Reason | When |
|---|---|
| `Invalid host name` | Not a valid host name with at least two labels |
| `In use` | The registrar already has a host by that name |
| `Superordinate domain not yours` | Under a domain another registrar sponsors |

A name under a domain that is not registered is available, as RST epp-05 requires of an unregistered name; creating it is refused until the domain is registered. A name only another registrar has is available: the registrar can have its own.

**`<host:info>`.** The name, ROID (ending in `ROID_REPOSITORY_ID`), statuses, addresses with their IP version, sponsor and creator, creation date, and the last updater and update date once updated.

**`<host:create>`, `<host:update>` and `<host:delete>`.** Each writes a `host.created`, `host.updated` or `host.deleted` event with the session's login as actor. A create writes the host and its addresses in one insert, and an update writes all its changes in one transaction or none. An update is checked and written on one reading of the host: if another command changes the host in between, the write is refused and the update is read and checked again, up to three times, so no change is overwritten by a command that never saw it. A delete likewise removes the host only if it is still the one checked, the same sponsor and version, so a delete prohibition set meanwhile is honoured. A rename recomputes whether the host is a nameserver of its parent domain, which decides whether its addresses are published as glue. Each command applies its rules in the order below, and reports the first that fails:

`<host:create>`:
1. A name that is not a valid host name, an address that does not parse, or one that is not of the IP version its `ip` attribute names: 2005.
2. An address that is not public unicast, the same address twice, or more than 10: 2306.
3. The registrar has a host by that name: 2302.
4. An internal host whose superordinate domain is not registered: 2303. Under a domain another registrar sponsors: 2201.
5. An address on an external host: 2306.

`<host:update>`, which removes statuses and addresses, then adds them, then renames:
1. No `<host:add>`, `<host:rem>` or `<host:chg>` (RFC 5732 §3.2.5): 2003.
2. A name that is not a valid host name: 2005. Another registrar's host: 2201. A host nobody has: 2303.
3. An address that does not parse or is not of its IP version, or a new name that is not a valid host name: 2005. An address added that is not public unicast: 2306.
4. A status the client does not control (anything but `clientDeleteProhibited` and `clientUpdateProhibited`): 2306.
5. A host with a pending action, with `serverUpdateProhibited`, or with `clientUpdateProhibited` unless the update removes it: 2304.
6. Removing a status or address the host does not have, adding one it has, or more than 10 addresses: 2306.
7. A rename onto a name the registrar already has: 2302. A rename of a host that is a nameserver of another registrar's domain: 2305 (RST epp-23; only an escrow import can link one).
8. A rename to an internal name whose superordinate domain is not registered: 2303. Under a domain another registrar sponsors: 2201.
9. An address on an external host: 2306.
10. An internal host that is a nameserver left without an address: 2305.

`<host:delete>`:
1. A name that is not a valid host name: 2005. Another registrar's host: 2201. A host nobody has: 2303.
2. A host with a pending action, `clientDeleteProhibited` or `serverDeleteProhibited`: 2304.
3. A host that is a nameserver of a domain: 2305.

**Contact objects** (RFC 5733). This registry keeps contacts (RST's `general.registryDataModel` is not `minimum`):
- A contact ID is unique in the registry, whichever registrar sponsors it, so `<contact:check>` gives every registrar the same answer: `In use` if any contact has the ID, `Invalid contact ID` if it is not one.
- `<contact:info>`, `<contact:update>` and `<contact:delete>` are answered only for the sponsor (ADR 0006). Anyone else gets 2201, with or without the contact's authorization information, and is not told who the sponsor is. A contact nobody has is 2303.
- `<contact:info>` returns the ID, ROID (ending in `ROID_REPOSITORY_ID`), statuses (`linked` from the domains that use the contact now), postal addresses, phone numbers, email, sponsor and creator, the dates, the authorization information, and a `<contact:disclose flag="1">` listing what the contact discloses, when it discloses anything. By default it discloses nothing.
- **Phone extensions are not stored.** A `<contact:voice>` or `<contact:fax>` with an `x` attribute is refused with 2102. The registry collects no more personal data than it needs. For an RST run, declare `epp.supportedContactElements` as `org`, `street`, `sp`, `pc`, `voice` and `fax`, without `voice:ext` and `fax:ext`.
- Values are kept as the schema delivers them; postal lines are not trimmed of a trailing dot. A country code must be an ISO 3166-1 alpha-2 code (or `XK`), stored in upper case. An email address must be a single address, without a display name, whose local part is at most 64 characters and whose domain has at least two labels. An `int` postal address is in 7-bit ASCII (RFC 5733 §2.4).
- A client sets only `clientDeleteProhibited`, `clientTransferProhibited` and `clientUpdateProhibited`.
- Create, update and delete write `contact.created`, `contact.updated` and `contact.deleted` events with the session's login as actor; the authorization information in them is redacted. An update is checked and written on one reading of the contact, and read and checked again, up to three times, if another command changes it in between. A delete removes the contact only if it is still the one checked, the same ROID, sponsor and version: a contact deleted and created again with the ID by another registrar meanwhile is not deleted.
- A contact ID is taken as the schema delivers it: `abc` and `abc.` are two IDs. Postal lines are limited to 255 characters, not bytes.
- The disclosure flags are stored in `disclose_*` columns of their own. Until this release only the address flags were stored; the migrate job copies them across once (`migrateContactDisclose`), and the old `addr_int` and `addr_loc` columns are left for a later release to drop.

`<contact:create>`, checking the elements in the order the frame gives them:
1. The ID: 2005 if it is not a valid ID.
2. Each postal address: 2005 if a line or the country code is not valid; 2306 if it is a second of its type.
3. `<contact:voice>`, then `<contact:fax>`: 2306 if empty (leave the element out instead); 2102 with an extension; 2005 if not a valid number.
4. The email address: 2005 if not valid.
5. The authorization information: 2306 if it breaks the registry's rules.
6. An ID in use: 2302.

`<contact:update>`, which removes statuses, then adds them, then changes the values given:
1. No `<contact:add>`, `<contact:rem>` or `<contact:chg>` (RFC 5733 §3.2.5): 2003.
2. Another registrar's contact: 2201. A contact nobody has: 2303.
3. A status the client does not control: 2306.
4. A contact with a pending action, with `serverUpdateProhibited`, or with `clientUpdateProhibited` unless the update removes it: 2304.
5. Removing a status the contact does not have, or adding one it has: 2306.
6. Two postal addresses of one type: 2306. A postal address of a type the contact does not have yet, without a name and an address: 2003. Only the parts of a postal address that are given change; an empty `<contact:org>` removes the organisation.
7. A value that is not valid: 2005, or 2102 for a phone extension. An empty `<contact:voice>` or `<contact:fax>` removes the number.
8. Authorization information that breaks the registry's rules: 2306.

`<contact:delete>`:
1. Another registrar's contact: 2201. A contact nobody has: 2303.
2. A contact with a pending action, `clientDeleteProhibited` or `serverDeleteProhibited`: 2304.
3. A contact a domain uses, as registrant or as admin, tech or billing contact: 2305.

**Audit log.** Every command and its response is a row of `epp_transactions`: svTRID, clTRID, session, node, registrar, login (for a `<login>`, the login presented, accepted or not), source address, command, the object identifiers it names, result code, receipt time, duration, and the frames.
- Frames are redacted: the text of every `<pw>` and `<newPW>`, and every text and attribute value inside every `<authInfo>`, is replaced with `[redacted]`. A frame that could not be parsed is recorded without its request, which could not be redacted. The login ID and object identifiers are read as soon as a command passes validation, so a command the policy refuses is recorded with them.
- A transform's record (anything but a query, including `<login>` and `<logout>`) is written before its response is sent. If that write fails, the record is queued for a retry and the response is still sent: the registrar must learn the outcome of a change that has happened. `epp_audit_errors_total{stage="sync"}` counts these.
- Queries are queued and written in batches of up to 200, at least every second, by `EPP_AUDIT_BUFFER`. A failed batch is retried every second (`stage="batch"`); a record that can be neither queued nor written is lost and counted (`stage="lost"`). On shutdown, what is queued is written before the process exits, retrying for up to 10 seconds; anything still unwritten then is counted as `lost`, by record.
- It is a record, not telemetry (INV-01): it is never sampled.

## Credentials: admin API

A registrar's EPP logins, passwords, certificate pins and allowed networks are managed through the admin API, under `/registrars/{clid}/epp`. Every endpoint needs the `epp:credentials:admin` permission. Registrars are not owned by an operator, so this is a platform permission; with Auth0 disabled, the static admin token holds it. The model is [ARCHITECTURE.md §2](ARCHITECTURE.md#2-identity--credentials) and the decisions are [ADR 0014](../adr/0014-epp-credential-rotation.md).

| Action | Request |
|---|---|
| Show everything, with each credential's state now (never a password hash) | `GET /registrars/{clid}/epp` |
| Add a login. `role` is `transform` (default) or `readonly` | `POST /registrars/{clid}/epp/logins` `{"login_id": "...", "role": "..."}` |
| Change a login's `role`, `enabled` or `session_cap` | `PATCH /registrars/{clid}/epp/logins/{login}` |
| Add a password. With no body the server generates a 16-character password and returns it **once**; it cannot be retrieved again | `POST /registrars/{clid}/epp/logins/{login}/passwords` `{"password": "..."}` |
| Pin a client certificate's key. `logins` optionally restricts the pin | `POST /registrars/{clid}/epp/certificate-pins` `{"certificate_pem": "...", "logins": [...]}` |
| Allow a source network | `POST /registrars/{clid}/epp/allowed-networks` `{"network": "192.0.2.0/24", "note": "..."}` |
| Retire a password, pin or network | `POST .../passwords/{id}/retire`, `.../certificate-pins/{id}/retire`, `.../allowed-networks/{id}/retire` `{"kind": "normal" or "compromise", "at": "RFC 3339"}` |

Every change is written in one transaction with an outbox event (`epp.login.created`, `epp.password.added`, `epp.certificate_pin.retired`, and so on; subject: the registrar's ClID). Events never carry a password or a hash. Delivering them to the registrar's technical contact needs an outbox consumer that does not exist yet.

These rules are fixed in code (`pkg/domain/entities/eppRegistrarAccess.go`), the same for every registrar (ADR 0014):

| Rule | Value |
|---|---|
| Logins per registrar | 10, enabled or not |
| Login ID | 3 to 16 ASCII characters (`clIDType`), unique across registrars. It may be the registrar's own ClID, never another registrar's |
| Session cap per login | 0 (the policy default) to 100 |
| Usable passwords per login | 2. Retire one to add a third |
| Password | 6 to 16 printable ASCII characters (RFC 5730 `pwType`), no leading, trailing or repeated spaces. Stored as Argon2id (19 MiB, 2 passes, 1 lane). Prefer the generated password: 16 characters, about 92 bits |
| Usable certificate pins per registrar | 3 (current, next, and a spare for a staggered roll) |
| Pinned certificate | Any CA or self-signed. RSA keys of at least 2048 bits, ECDSA or Ed25519. An expired certificate is refused; one not yet valid is accepted, so the pin is ready before the switch |
| Usable allowed networks per registrar | 20 |
| Allowed network | A single address, or a CIDR no broader than /24 (IPv4) or /48 (IPv6) |
| Normal retire without `at` | Usable for 7 more days, shown as `retiring`. An `at` that is not in the future retires at once. New logins are refused from then on; open sessions run until they end |
| Compromise retire | At once, and every session that used the credential is ended. Also allowed on an already retired credential, which keeps its retire time |

A credential is never deleted. A retired one stays as the record of when, and from where, it was last used.

---

## Policies

The limits each registrar is held to are its **EPP policy**: data, changed through the admin API and reloaded by every node within `EPP_CREDENTIAL_REFRESH_INTERVAL`, without a restart ([ADR 0019](../adr/0019-epp-policies.md); [`entities.EPPPolicy`](../../pkg/domain/entities/eppPolicy.go)).

- **A policy is a named set of limits.** Two are built in: `default`, for every registrar not assigned another, and `probation`, for new registrars. The `migrate` job stores them with their starting limits (below) if they are not stored yet. Their limits can be changed; they cannot be deleted. Any other policy is custom.
- **A registrar is assigned one policy**, or none for `default`. There are no per-registrar overrides: a registrar that needs other limits gets a policy of its own.
- **A registrar family** groups affiliated registrars, often the accreditations of one company, so limits apply to them together as well as to each. A registrar is in at most one family. A family is assigned a policy too: its `sessions_per_family` caps the family's sessions and its `family_rates` the family's commands, while each member's own policy caps the member.
- **A login's own `session_cap`** ([Credentials](#credentials-admin-api)), when set, replaces its registrar's policy's `sessions_per_login`.

| Limit | `default` | `probation` | Applies to | Over it |
|---|---|---|---|---|
| `sessions_per_registrar` | 10 | 10 | A registrar's logged-in sessions, across every node and login | `<login>` 2502, and the session closes |
| `sessions_per_login` | 10 | 10 | One login's sessions, unless the login has its own cap | 2502, and the session closes |
| `sessions_per_family` | 40 | 30 | A family's sessions, across its members, from the family's policy | 2502, and the session closes |
| `check_names` | 50 | 50 | Names one `<domain:check>`, `<host:check>` or `<contact:check>` may carry | 2306, with no lookup |
| `rates` | [Command rates](#command-rates) | [Command rates](#command-rates) | A registrar's commands, a token bucket per class | Held up to 1s, then 2306; 2502 and close after 10 refusals in 10s |
| `family_rates` | 3× `rates` | 3× `rates` | A family's commands, across its members, from the family's policy | Held up to 1s, then 2306; 2502 and close after 10 refusals in 10s |
| `creates_per_day` | 0 (no cap) | 100 | A registrar's successful domain creates per UTC day | 2308 |
| `failed_creates` | 100 failures, 90% | 50 failures, 90% | A registrar's domain creates of registered names over 5 minutes | Held, then 2502, then the login suspended ([§ Failed creates](#failed-creates)) |
| `family_failed_creates` | 300 failures, 90% | 150 failures, 90% | A family's, across its members, from the family's policy | The same, for the member sending the create |

Every limit is at least 1, except `creates_per_day`, where 0 is no cap, and a `ratio_percent`, which is 1 to 100. A `check` burst, in `rates` and in `family_rates`, must be at least `check_names`, or a full check could never pass. The rules, in order, for each command of a logged-in session ([`epppolicy.Enforcer`](../../internal/application/epppolicy/enforcer.go)):

1. **Role.** A login with the `readonly` role ([Credentials](#credentials-admin-api)) may check, read (`<info>`, `<transfer op="query">`), poll and log out. Anything that changes the registry (create, update, delete, renew, transfer request, approve, reject or cancel) is 2201, *"this login is read-only"*.
2. **Check names.** A `<check>` naming more than `check_names` objects is 2306, *"at most N names per check"*, before any lookup. For a registrar in a family, the cap is also no more than the family's `check` burst, so a check the family's bucket could never pay for is refused for its size rather than as a rate it could never meet.
3. **Command rates.** The command takes its tokens from the registrar's bucket for its class and, if the registrar is in a family, the family's ([§ Command rates](#command-rates)).
4. **Failed creates.** A `<domain:create>` from a login suspended for failed creates, or while the registrar or its family is over its failed-create budget, is escalated ([§ Failed creates](#failed-creates)).
5. **Create quota.** A `<domain:create>` counts against the registrar's `creates_per_day` ([§ Quotas](#quotas)).

A command refused by one rule is not counted by the later ones. Session limits apply at `<login>`, in one atomic step in the session registry with the source cap's guarantees. `epp_policy_decisions_total{rule,action}` counts every decision by the rule that made it (`role`, `check_names`, `rate`, `family_rate`, `failed_creates`, `create_quota`, or `none` for a command allowed at once) and its action (`allow`, `delay`, `reject`, `close`).

**Reloading.** Each node loads every policy, assignment and family when it starts and every `EPP_CREDENTIAL_REFRESH_INTERVAL` after (`epp_policies_refreshed_timestamp_seconds`). A failed load keeps the set last loaded (`epp_policy_refresh_errors_total`); before the first, the built-in policies' starting limits apply. A change applies to the next command and the next login; sessions already open stay open under a lowered session cap. A family change also moves the sessions already open: at the reload, a registrar that left a family stops counting against it, and one that joined counts from then on (its open sessions stay open, even over the family's cap).

**Admin API** (permission `epp:policy:admin`; the static admin token holds it when Auth0 is off). Each change writes an outbox event (`epp.policy.saved`, `epp.policy.deleted`, `epp.policy.assigned`, `epp.family.saved`, `epp.family.deleted`) with the actor and the object before and after, in the same transaction. Policy changes are serialised by one database lock, and each reads what it replaces inside its transaction, so the before and after are exact under concurrent changes. Foreign keys tie assignments and family members to registrars: deleting a registrar deletes its assignment and its family membership.

| Action | Request |
|---|---|
| List policies, the built-in ones included | `GET /epp/policies` |
| One policy | `GET /epp/policies/{name}` |
| Create or replace a policy (every limit given) | `PUT /epp/policies/{name}` with `{"description": "…", "limits": {"sessions_per_registrar": 20, "sessions_per_login": 10, "sessions_per_family": 60, "check_names": 50, "rates": {"check": {"per_second": 500, "burst": 2000}, "info": {"per_second": 40, "burst": 200}, "transform": {"per_second": 20, "burst": 100}, "poll": {"per_second": 10, "burst": 40}}, "family_rates": {…the same four…}, "creates_per_day": 0, "failed_creates": {"failures": 100, "ratio_percent": 90}, "family_failed_creates": {"failures": 300, "ratio_percent": 90}}}`. A name is 1 to 32 lower-case letters, digits and hyphens, starting with a letter. 400 for a limit below 1 (below 0 for `creates_per_day`), or a check burst below `check_names`; the error names each |
| Delete a custom policy | `DELETE /epp/policies/{name}` → 204. 409 for a built-in policy, or one a registrar or family is assigned |
| What applies to a registrar | `GET /registrars/{clid}/epp/policy`: its policy, and its family with the family's policy |
| Assign a registrar its policy | `PUT /registrars/{clid}/epp/policy` with `{"policy": "gold"}`; `"default"` or `""` removes the assignment. 404 for an unknown registrar or policy |
| List families | `GET /epp/families` |
| One family | `GET /epp/families/{name}` |
| Create or replace a family, its members included | `PUT /epp/families/{name}` with `{"description": "…", "policy": "gold", "members": ["clid1", "clid2"]}`. 409 for a member of another family; 404 for an unknown registrar or policy |
| Delete a family | `DELETE /epp/families/{name}` → 204; its members belong to none |

### Sessions

The session caps per registrar, login and family are the registrars' policies' ([§ Policies](#policies)). These are server settings:

| Variable | Default | Meaning |
|---|---|---|
| `EPP_MAX_SESSIONS_PER_SOURCE_IP` | `20` | Live sessions from one source address, across every node. Over it, the connection closes before the greeting (`epp_connections_total{outcome="rejected_source_cap"}`). It applies before anyone has logged in, so it is not a policy |
| `EPP_SESSION_LEASE` | `90s` | How long a session stays in the registry without a renewal. A crashed node's sessions disappear after this. Lease times use Redis's clock, not the nodes', so clock skew between nodes does not matter. A closed session cannot be renewed back by a heartbeat that read it just before it closed |
| `EPP_SESSION_HEARTBEAT` | `30s` | How often a node renews its sessions. At most half the lease, so one missed renewal never drops a live session. The registry's command count and last command time are this fresh |
| `EPP_NODE_ID` | the hostname | The node's name in the registry |

**If Redis cannot answer** (each call is bounded to 500 ms), the node applies the same caps to its own sessions and sets `epp_policy_store_degraded{store="sessions"}`. A node's own count is a backstop, not the cluster's: with N nodes, up to N times a cap can be open until Redis returns. When it does, each node puts its sessions back within one heartbeat.

**Admin API** (permission `epp:sessions:admin`; the static admin token holds it when Auth0 is off):

| Action | Request |
|---|---|
| List live sessions, with node, source, registrar, login, credential IDs, connect and login times, command count and last command time | `GET /epp/sessions?clid=&login=&client_ip=&credential_id=`, cursor-paginated |
| One session | `GET /epp/sessions/{id}` |
| End a session | `DELETE /epp/sessions/{id}` → 202 |
| End every session of a registrar, login or source, or that used a credential | `DELETE /epp/sessions?clid=` (or `login`, `client_ip`, `credential_id`) → 202. At least one filter is required |

Ending a session first records an `epp.session.kill_requested` outbox event per session, with the operator as actor; if that cannot be written, nothing is sent and the request fails. Then it publishes a request every node receives, and the node holding the session closes it within moments (`epp_sessions_ended_total{reason="admin_kill"}`). The answer is 202: a node that cannot reach Redis at that instant misses the request. A compromise retire of a credential ends its sessions on its own, within `EPP_CREDENTIAL_REFRESH_INTERVAL`; ending them by `credential_id` is immediate.

### Authentication

In place. A `<login>` is checked in this order, against the database, so an admin API change applies to the next login on every node:

1. The login exists and is enabled.
2. The source address is in one of **its registrar's** usable allowed networks (unless `EPP_REQUIRE_ALLOWLIST=false`).
3. The client certificate's key is pinned by the registrar, for this login, and the certificate is within its validity period.
4. The (login, source address) pair is not locked out.
5. The password matches one of the login's usable passwords.

Every refusal is answered 2200 with the same text, so a client cannot tell which check failed; `epp_login_failures_total{reason}` and the log say which. Only a wrong password at step 5 counts towards a lockout: by then the client has shown an allowed address and a pinned certificate, so a stranger cannot lock a registrar out.

| Rule | Value | Where to change it |
|---|---|---|
| Failed logins per (login, source address) before lock | 5 within 15m. Failures that arrive while locked are not counted, so a burst cannot start the next, doubled lock | `lockout.DefaultPolicy` (code) |
| Lock duration | 15m, doubling on each further lock within 24h, up to 24h. A successful login clears it | `lockout.DefaultPolicy` (code) |
| While locked | 2501, and the session closes, even with the right password | |
| Failed logins per session | 3, then 2501 and the session closes | `EPP_MAX_LOGIN_ATTEMPTS` |
| Redis unavailable | Lockouts are counted on the node (fail-open, ADR 0013), with each Redis call bounded to 500 ms, and `epp_policy_store_degraded{store="lockout"}` is 1. The node-local count holds at most 100,000 (login, address) pairs and never drops a lock in force to make room. Redis being down at start is a warning, not a failure | |
| Credential store unavailable | 2400; the session stays open and may retry. Open sessions are unaffected | |
| Client certificate validity | Enforced at every login: no grace before `notBefore` or after `notAfter` | |
| Certificate expiry alerts | 30 and 7 days before expiry, from `epp_client_cert_not_after_timestamp_seconds{clid,login}` (alert rules: Phase 7) | |
| `<newPW>` | Strict RFC 5730: the new password replaces the one used to log in, which stops working at once. It is applied only once the session has been admitted under its session caps, so a login refused with 2502 leaves the password unchanged. An unacceptable `<newPW>` is 2306 and the login does not happen. Overlap is done with add-then-retire through the admin API | |
| Last used | Recorded for the password, pin and network of each successful login, written at most once a minute per node | |
| Compromise retire | Every session that authenticated with the credential, or set it through `<newPW>`, is ended within `EPP_CREDENTIAL_REFRESH_INTERVAL`. An already retired credential, such as a password replaced by `<newPW>`, can still be retired as compromised | |

### Command rates

Every command of a logged-in session, other than `<login>`, `<logout>` and `<hello>`, takes tokens from **token buckets**: its registrar's bucket for the command's class and, if the registrar is in a family, the family's bucket for that class. The buckets are the policies' `rates` and `family_rates` ([§ Policies](#policies)). A bucket refills at `per_second` tokens a second and holds at most `burst`, so a quiet registrar can send a burst at once and then its rate.

| Class | Commands | Cost | `default` rate / burst | `probation` rate / burst |
|---|---|---|---|---|
| `check` | `<check>` | One token **per name** | 250 / 1000 | 100 / 500 |
| `info` | `<info>`, `<transfer op="query">` | One per command | 20 / 100 | 10 / 50 |
| `transform` | create, update, delete, renew, transfer request, approve, reject, cancel | One per command | 10 / 50 | 5 / 25 |
| `poll` | `<poll>` (req and ack) | One per command | 5 / 20 | 5 / 20 |

A family's starting rates are 3× its policy's registrar rates (`family_rates`). `check` is metered in names so that batch size does not change capacity: a higher-volume registrar gets a policy with a larger budget, not a larger batch.

**Over the rate.**

| Wait for the tokens | Answer |
|---|---|
| None | The command runs |
| Up to 1s (`epppolicy.MaxRateDelay`, code) | The command is **held** until its tokens are there, then runs (`action="delay"`, `epp_policy_delay_seconds`). Its tokens are taken when it is held, so it keeps its place. A session that sends one command at a time is slowed to its rate and nothing fails |
| More than 1s | **2306**, *"Parameter value policy error; the registrar's check rate is exceeded, try again in 1.3s"* (or *"the registrar family's"*), with a machine-readable reason (below), and the session continues. No tokens are taken, so a refused command costs nothing. This happens to a client running many sessions at once past its rate |
| The session's 10th refusal within 10s (`epppolicy.RefusalsBeforeClose`, `RefusalWindow`, code) | **2502**, *"Session limit exceeded; server closing connection; 10 command rate refusals within 10s"*, and the session closes (`action="close"`). Its client is not backing off; reconnecting costs a handshake and a login, under the session caps and the lockout |

A rate refusal carries the reason in the result's `<extValue>` (RFC 5730 §2.6), so a client can tell it from any other 2306 and knows when to try again:

```xml
<result code="2306">
  <msg>Parameter value policy error; the registrar's check rate is exceeded, try again in 1.3s</msg>
  <extValue>
    <value><undef/></value>
    <reason>rate-limit; class=check; scope=registrar; retry-after-ms=1300</reason>
  </extValue>
</result>
```

`class` is `check`, `info`, `transform` or `poll`; `scope` is `registrar` or `family`. The 2502 that closes a session carries the same reason.

There is no standard EPP result code for a rate limit. A 23xx code says the client broke a policy, which clients do not retry blindly, unlike 2400 (an internal server error, which they may); 2306 is the policy error the registry already uses for the names per check. 2502 alone, as some registries use, would close the session at the first refusal.

**Where the buckets are kept.** In Redis, shared by every node, one key per bucket holding its GCRA *theoretical arrival time* (the time it would be full again), updated by one script for all of a command's buckets at once, on Redis's clock. A held command runs at one time for all its buckets, the earliest at which every one has its tokens, and each bucket is charged from that time, so a bucket that was not the one waited for cannot let a later command run first. **If Redis cannot answer** (each call is bounded to 200 ms), the node keeps buckets of its own and sets `epp_policy_store_degraded{store="rates"}`; for 5s after a failure, commands go straight to the node's buckets, so an unresponsive Redis does not slow every command. With N nodes, up to N times a rate can pass while Redis is down (fail-open, ADR 0013).

### Per-command object caps

| Cap | Default | On breach |
|---|---|---|
| Names per `domain:check`, `host:check`, `contact:check` | 50 | 2306. The policy's `check_names` ([§ Policies](#policies)) |
| Nameservers per domain | 10, fixed in the domain model (`entities.MaxHostsPerDomain`) | 2306 |
| DS records per domain | 8 (`services.MaxDSRecords`) | 2306 |

The nameserver and DS caps are rules of the registry's data, the same for every registrar, so they are not in the policies (ADR 0019).

### Quotas

| Quota | `default` | `probation` | Over it |
|---|---|---|---|
| `creates_per_day`: successful `<domain:create>`s per registrar per UTC day | 0 (no cap) | 100 | 2308, *"Data management policy violation; the registrar may create 100 domains a day (UTC), and has"* |

A create is counted when its policy check passes and given back if it does not succeed (any result of 2000 or more), so only domains created count; a refused or failed create does not. Each count is a reservation with its own ID, so a request to Redis retried after a lost reply neither takes a second slot nor gives back another create's. Host and contact creates do not count. The count is kept in Redis per registrar and UTC day; while Redis cannot answer it is kept on the node (`epp_policy_store_degraded{store="rates"}`), so a registrar can create up to N times its quota across N nodes. Creates of registered names are the failed-create budget's ([§ Failed creates](#failed-creates)).

### Failed creates

A dropcatcher sends creates for names that are about to be released, and most are refused because the name is still registered (2302). The **failed-create budget** ([§ Policies](#policies): `failed_creates`, and a family's `family_failed_creates`) is when that is a run rather than ordinary use.

**Counting.** Every `<domain:create>` answered 1000 or 1001 (a domain created) or 2302 (the name is registered) is counted, per registrar and per family, over the **last 5 minutes** (the current minute and the four before it). Other answers are not counted. A registrar or family is **over budget** when it has at least `failures` creates answered 2302 **and** they are at least `ratio_percent` of its counted creates. Both must hold: a busy registrar with some failures, or a quiet one with a few, is within it.

**Escalation.** While the registrar or its family is over budget, each `<domain:create>` the registrar sends is a **strike** against the one over budget (`epppolicy.DefaultEscalation`, code). A family's strikes are shared by its members, so a family over budget is escalated as one, however many accreditations send the creates. Strikes belong to the registrar or family, not the login: a login that carries a run on past the 20th strike is suspended at its first create.

| Strike | Answer |
|---|---|
| 1st to 10th | The create is **held** 1s per strike (1s, 2s, … 10s), on top of any rate wait, then runs (`rule="failed_creates"`, `action="delay"`) |
| 11th to 20th | **2502**, *"the registrar's creates of registered names are over their budget"* (or *"the registrar family's"*), and the session closes. `<extValue>` reason: `failed-creates; scope=registrar; suspended-for-ms=0` |
| 21st | The **login is suspended for 15 minutes**: 2502 and the session closes, with reason `failed-creates; scope=…; suspended-for-ms=900000`. The node's other sessions of the login end (`epp_sessions_ended_total{reason="login_suspended"}`); on other nodes, the login's next domain create closes its session the same way. A login while suspended is refused with 2501, *"the login is suspended for … for creating registered names"* (`epp_logins_total{result="suspended"}`). `epp_logins_suspended_total{clid}` counts suspensions: alert on any |

Strikes expire 5 minutes after the last; they stop while the registrar and its family are back within budget. A suspension's reason names the scope (`registrar` or `family`) whose strikes caused it. A suspension ends on its own; there is no API to lift one early yet (operator runbook, Phase 7).

**Accounting.** Each registrar's creates per UTC day, created and refused 2302, are kept in `epp_create_outcomes` as the basis for a future fee. Each node writes its counts every minute and when it stops. They are read with `GET /registrars/{clid}/epp/create-outcomes?from=YYYY-MM-DD&to=YYYY-MM-DD` (permission `epp:policy:admin`; the last 30 days by default, at most 366), one row per day with any creates.

**Where it is counted.** In Redis, shared by every node: per-minute counts per registrar and family, a registrar's strikes, and a login's suspension, all on Redis's clock. **If Redis cannot answer** (each call bounded to 200 ms, then skipped for 5s), the node counts on its own and sets `epp_policy_store_degraded{store="failed_creates"}`; a suspension made on the node holds after Redis returns.

### Create queue

Each node runs at most `EPP_MAX_CREATES_IN_FLIGHT` `<domain:create>`s at once ([§ Server settings](#server-settings-environment)), so a burst of creates, at a drop for instance, cannot take the database pool from checks, infos and the other commands. A create takes a slot after the policy has let it through (a create held for its rate or its failed-create strikes holds no slot while it waits) and gives it back when it has been answered.

**When every slot is taken**, creates wait in a queue per **registrar family**, or per registrar for one in no family, and a freed slot goes to the next queue **in turn**, not to the next create in line. A registrar, or a family of accreditations, sending many creates at once waits behind itself; one sending a few is let in at its next turn. A family is one queue however many accreditations it has, so splitting creates across accreditations does not buy more turns.

| Wait | Answer |
|---|---|
| A slot comes within `EPP_CREATE_QUEUE_WAIT` | The create runs (`epp_create_queue_wait_seconds`) |
| None comes | **2400**, *"Command failed; the registry is busy with creates, try again"*, with `<extValue>` reason `create-queue; waited-ms=5000`, and the session continues (`epp_create_queue_timeouts_total`). The create quota gives the create back, and the failed-create budget does not count it; a strike it took because the registrar was over budget stands, since a strike counts creates sent |

`epp_creates_in_flight` and `epp_creates_waiting` show the slots in use and the creates waiting. The queue is per node; across N nodes, up to N × `EPP_MAX_CREATES_IN_FLIGHT` creates run at once, and the database pool of each node bounds its share.

### Release of deleted names

A deleted domain is purged, and its name can be registered again, once its redemption and pending delete periods have ended (`<domain:delete>` under [§ Command handling](#command-handling)). The purge loop runs hourly; each scheduled run **waits a random offset within 45 minutes** before it purges, and **purges in a random order**, so the earliest purge date can be worked out from the periods but the moment a name is released cannot. A `<domain:create>` for a name not yet released is answered 2302, and counts towards the [failed-create budget](#failed-creates).

| Setting | Default | Where to change it |
|---|---|---|
| Release window | 45 minutes, inside the hourly schedule | `releaseWindowMinutes` on the `purge-loop` workflow's parameters; `0` purges at once (a manual run from the admin UI or `POST /workflows/launch` with `{"workflowType": "purge-loop", "params": {"releaseWindowMinutes": 0}}`) |
