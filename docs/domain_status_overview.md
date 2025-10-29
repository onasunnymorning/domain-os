# Domain Name Status Overview (per EPP RFCs)

Each domain object can have one or more **status values** that control what actions are allowed.  
Statuses are either **server-set** (by the registry) or **client-set** (by the registrar).

---

## 🔒 Server-Set Statuses

| Status | Meaning | Effect |
|--------|----------|--------|
| `serverHold` | Registry stops publishing in DNS. | Domain **not resolved** in DNS. |
| `serverDeleteProhibited` | Registry blocks deletion. | Prevents `delete` command. |
| `serverTransferProhibited` | Registry blocks transfer. | Prevents `transfer` command. |
| `serverUpdateProhibited` | Registry blocks update. | Prevents `update` command. |
| `serverRenewProhibited` | Registry blocks renewal. | Prevents `renew` command. |
| `serverRestoreProhibited` | Registry blocks restore. | Prevents restore from redemption. |

---

## 🧩 Client-Set Statuses

| Status | Meaning | Effect |
|--------|----------|--------|
| `clientHold` | Registrar requests DNS suspension. | Domain **not resolved** in DNS. |
| `clientDeleteProhibited` | Registrar blocks deletion. | Prevents `delete` command. |
| `clientTransferProhibited` | Registrar blocks transfer. | Prevents `transfer` command. |
| `clientUpdateProhibited` | Registrar blocks update. | Prevents `update` command. |
| `clientRenewProhibited` | Registrar blocks renewal. | Prevents `renew` command. |
| `clientRestoreProhibited` | Registrar blocks restore. | Prevents restore from redemption. |

---

## ⚙️ Lifecycle Statuses (set automatically)

| Status | Meaning | Typical Trigger |
|--------|----------|----------------|
| `ok` | Domain has no prohibitions. | Normal state. |
| `inactive` | No nameservers associated. | Added without hosts. |
| `pendingCreate` | Domain create in progress. | Awaiting completion. |
| `pendingDelete` | Domain deletion pending. | Awaiting removal or redemption. |
| `pendingTransfer` | Transfer in progress. | Waiting for approval/timeout. |
| `pendingRenew` | Renewal pending completion. | Registry-specific. |
| `pendingUpdate` | Update pending. | Registry-specific. |
| `redemptionPeriod` | Domain deleted, can still be restored. | 30-day redemption phase (typical). |
| `pendingRestore` | Restore requested but incomplete. | Requires report submission. |
| `pendingDelete` (final) | Domain will be purged soon. | After redemption phase. |

---

## 🧭 Usage Notes

- Multiple statuses can apply simultaneously.  
- Actions (create, delete, renew, transfer, update, restore) are allowed only if **no blocking status** is set.  
- Server statuses override client statuses when conflicts occur.  
- Lifecycle states (e.g., `redemptionPeriod`, `pendingTransfer`) are typically **system-driven**.
