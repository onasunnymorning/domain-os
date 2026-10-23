// The /docs/domain-restore page. Keep it in step with
// pkg/domain/entities/restore.go (the restore process and its rules),
// internal/application/services/domain_service_restore.go (charges and
// events), internal/application/workflows/restoreWorkflow.go (the revert
// loop) and docs/epp/POLICY_REFERENCE.md (the settings and EPP result codes).
export const DOMAIN_RESTORE_DOC_MARKDOWN = `# Domain Restore

A **restore** brings back a domain that was deleted, or that expired and was not renewed, before it is purged and released for anyone to register. Domain OS follows **RFC 3915** (*Domain Registry Grace Period Mapping for EPP*), ICANN's Redemption Grace Period policy, and the way .com applies it.

---

## A deleted domain's end of life

When a registrar deletes a domain after its add grace period, or a domain expires and is not auto-renewed, it does not disappear at once:

| Stage | Length (default) | EPP status | RGP status | What can happen |
|---|---|---|---|---|
| Redemption period | 30 days | \`pendingDelete\` | \`redemptionPeriod\` | The sponsor can request a restore. Nothing else can change. |
| Pending restore | up to 7 days, after a request | \`pendingDelete\` | \`pendingRestore\` | The sponsor sends the restore report. |
| Pending delete | 5 days | \`pendingDelete\` | \`pendingDelete\` | Nothing: the domain waits to be purged. |

A domain is **not in the zone** for the whole of its end of life, so it does not resolve. A domain deleted within its add grace period skips all of this and is purged at once.

---

## Restoring a domain, step by step

### 1. The restore request

The sponsor sends an EPP \`<domain:update>\` that changes nothing, with the RGP extension's restore request:

\`\`\`xml
<domain:update xmlns:domain="urn:ietf:params:xml:ns:domain-1.0">
  <domain:name>example.com</domain:name>
  <domain:chg/>
</domain:update>
...
<rgp:update xmlns:rgp="urn:ietf:params:xml:ns:rgp-1.0">
  <rgp:restore op="request"/>
</rgp:update>
\`\`\`

The domain must be in its redemption period. It becomes **pendingRestore**, an RGP status; its EPP status stays \`pendingDelete\` (RFC 3915 §4). The response lists \`pendingRestore\` in \`<rgp:upData>\`, and \`<domain:info>\` shows it in \`<rgp:infData>\`. The **restore report** is now due within the phase's pending restore period, 7 days by default; the domain page shows the deadline as *Restore Report Due*.

A request is refused if it changes anything else (2306: send the changes in a separate update), if the domain is not deleted or its redemption period is over (2304), or if a restore is already pending (2304).

### 2. The restore report

The sponsor sends a second update with \`<rgp:restore op="report">\` and the report ICANN requires:

| Part | What it holds |
|---|---|
| \`preData\` | The registration data before the delete |
| \`postData\` | The registration data as restored |
| \`delTime\` | When the domain was deleted |
| \`resTime\` | When the restore was requested |
| \`resReason\` | Why the domain is restored |
| \`statement\` (two) | That the registrar has not restored the domain to use or sell it itself, and that the report is factual |
| \`other\` | Anything else (optional) |

The registry checks that every required part has content (2003 otherwise), and that \`delTime\` is before \`resTime\`, which is not in the future (2004 otherwise). The report is kept, as sent, with the domain's *restored* event.

A report is refused with 2304 when no restore was requested, or when the request has lapsed.

### 3. The domain is restored

The report completes the restore at once:

- \`pendingDelete\` and \`pendingRestore\` are cleared, and the domain leaves its end of life: it is back in the zone if it has nameservers.
- It is **renewed for a year**, and for further years while its expiry date would still be in the past (a domain that expired more than a year ago). Its expiry date is always in the future afterwards.
- No renew grace period starts: deleting the domain again earns no refund.

### If the report is not sent in time

The request **lapses** at its deadline. Within the hour, the *Revert Lapsed Restores* workflow returns the domain to where it was: \`pendingDelete\`, in its redemption period if that has not ended, with its original purge date, and sends the registrar a poll message (*Restore lapsed: no restore report for … by …*) saying where the domain now stands. Nothing is charged. The registrar can request a restore again while the redemption period lasts. A domain with a pending restore is never purged, even past its purge date: it is reverted first.

### Domains imported from an escrow deposit

A deposit says which period a deleted domain is in, but not when that period ends. On import, the period **starts in full**, under the TLD's current phase:

| In the deposit | After the import |
|---|---|
| \`redemptionPeriod\` | Restorable for the full redemption period, then pending delete |
| \`pendingRestore\` | The restore stays requested: the registrar sends its report **here**, within the pending restore period. If it does not, the request lapses and the domain is back in a full redemption period |
| \`pendingDelete\` (past redemption) | Purged after the pending delete period |

A deleted domain the deposit gives no RGP status is treated as in its redemption period.

---

## Charges

A completed restore is **two charges**, each recorded as its own event so billing sees two line items:

| Event | Transaction | Priced at |
|---|---|---|
| \`domain.restored\` | restore | the phase's restore price |
| \`domain.renewed\` | renewal | the renewal price, for the years the restore added |

ICANN's RGP terms treat the restore fee as separate from, and in addition to, the renewal. The request (\`domain.restore_requested\`) and a lapsed request (\`domain.restore_reverted\`) are recorded without a charge. When the fees are taken, at the request or at the report, and whether a lapsed request is refunded, is the billing system's to decide.

A restore the registry cannot price is refused when it is requested.

---

## Settings

Both are set per TLD phase, on the phase's *Policy* section:

| Setting | Default | Meaning |
|---|---|---|
| Redemption | 30 days | How long a deleted domain can be restored |
| Pending Delete | 5 days | How long it then waits to be purged |
| Pending Restore | 7 days | How long the registrar has to send the restore report |
| Restore Report Required | on | Off restores a domain on its request alone, for registries (often ccTLDs) that do not require a report |

A change applies to restores requested after it: a pending restore keeps its deadline.

---

## From the admin API

The registry's operator can act for a domain's sponsor:

- \`POST /domains/{name}/restore\` requests the restore.
- \`POST /domains/{name}/restore/report\` sends the report, as JSON (\`preData\`, \`postData\`, \`delTime\`, \`resTime\`, \`resReason\`, \`statements\`, \`other\`).

Both follow the same rules as EPP: 409 when the domain's state does not allow the step, 400 for an incomplete or impossible report.
`;
