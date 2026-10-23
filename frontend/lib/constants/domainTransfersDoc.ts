// The /docs/domain-transfers page. Keep it in step with
// pkg/domain/entities/authInfoHash.go (the code rules),
// pkg/domain/entities/transfer.go (the transfer process),
// internal/application/services/epp_domain_transfer.go (who may do what) and
// docs/epp/POLICY_REFERENCE.md (the EPP result codes).
export const DOMAIN_TRANSFERS_DOC_MARKDOWN = `# Domain Transfers

A **transfer** moves a domain from one registrar (the *losing* registrar, its current sponsor) to another (the *gaining* registrar). The registrant proves they want the move by giving the gaining registrar the domain's **transfer code**, which EPP calls *authorization information* or *AuthInfo*.

Domain OS follows **RFC 9154** (*Extensible Provisioning Protocol Secure Authorization Information for Transfer*) for transfer codes, **RFC 5731** for the EPP \`<domain:transfer>\` command, and the **ICANN Transfer Policy** for the process. This page explains how codes are set, kept and checked, then the transfer itself, step by step: what each registrar can do, what changes when a transfer completes, and the messages each registrar receives.

---

## Transfer codes in short

- **No code is the normal state.** A domain has no transfer code until its registrant asks to transfer it.
- **The sponsor sets a code when asked.** The registrant asks their current registrar, which sets a fresh random code and hands it to the registrant.
- **The registry never keeps the code itself.** It stores a salted hash and can only check a code given to it, never show one.
- **A code is used once.** It is cleared when the transfer completes, and the sponsor can clear it at any time.
- **A code must be random and strong.** At least 128 bits of entropy, so it cannot be guessed.

---

## How the registry keeps a code

| | |
|---|---|
| Stored as | A salted SHA-256 hash, with a new 128-bit random salt for every code |
| When no code is set | Nothing is stored (the column is empty) |
| Shown in the admin UI | Only **Set** or **Not set** |
| Returned by the admin API | Only \`AuthInfoSet: true\` or \`false\`, never the code or its hash |
| Returned by EPP \`<domain:info>\` | To the sponsor, an empty \`<domain:pw/>\` when a code is set, and no \`<domain:authInfo>\` when none is. Never the code. |
| Written to logs and the EPP audit trail | Never: passwords and authorization information are stripped from logged frames |

Because the code cannot be read back, **copy a code before you save it**. If it is lost, set a new one.

Codes stored before Domain OS adopted RFC 9154 were converted to hashes by the database migration, so they keep working. Domains loaded from an escrow deposit start with no code.

---

## Setting and clearing a code

| Who | How to set a code | How to clear it |
|---|---|---|
| The sponsoring registrar, over EPP | \`<domain:update>\` with \`<domain:chg><domain:authInfo><domain:pw>CODE</domain:pw></domain:authInfo></domain:chg>\` | The same with an empty \`<domain:pw/>\`, or with \`<domain:null/>\` in place of \`<domain:pw>\` |
| The sponsoring registrar, on create | \`<domain:create>\` with a \`<domain:pw>\`, or an empty one for no code | |
| Registry staff, in the admin UI or API | The **Transfer code** field when creating a domain, or \`AuthInfo\` in an update | Leaving the field empty keeps the current code |

Setting a new code replaces the old one; the old code stops working at once.

---

## How strong a code must be

The registry estimates a code's entropy the way RFC 9154 sizes codes: **its length times log₂ of the alphabet it draws from**. The alphabet is the sum of the character classes the code uses:

| Class | Characters counted |
|---|---|
| Lower-case letters | 26 |
| Upper-case letters | 26 |
| Digits | 10 |
| Other printable ASCII (punctuation, symbols, space) | 33 |

A code is accepted when:

1. the estimate is **at least 128 bits**;
2. it uses **at least 8 different characters**;
3. it is **printable ASCII** only; and
4. it is **at most 128 characters** long.

Some lengths that pass:

| A random code of | Alphabet | Bits |
|---|---|---|
| 20 characters using all four classes | 95 | 131 |
| 22 characters of upper, lower and digits | 62 | 131 |
| 25 lower-case letters and digits | 36 | 129 |
| 32 hexadecimal digits | 36 | 165 |

The estimate cannot tell a random code from a memorable one: \`Password1!Password1!\` uses four classes and enough length, but it is not random. **Always generate codes**, with a cryptographic random generator. The **Generate** button in the admin UI makes 24-character codes of about 150 bits.

The same rule applies everywhere a code is set: registrars over EPP, the admin API and the admin UI.

---

## Checking a code before a transfer

A registrar that does not sponsor a domain can check a transfer code with \`<domain:info>\`, giving the code in \`<domain:authInfo><domain:pw>\`:

| What the registrar gives | The registry answers |
|---|---|
| No code | **2201** Authorization error: only the sponsor sees a domain's details |
| The right code | **1000** with a limited view: name, ROID, statuses, sponsor, creation and expiry dates. No contacts, nameservers or authorization information. |
| A wrong code, or any code when none is set | **2202** Invalid authorization information |

This lets a gaining registrar confirm a code with the registrant before requesting the transfer. The limited view is an amendment to the registry's tenancy rules (ADR 0006): a correct transfer code is the registrant's consent to show the domain's basic details to the registrar they chose.

---

## The transfer flow

### 1. Before the request

1. The registrant asks the **losing** registrar for a transfer code.
2. The losing registrar sets a fresh random code (\`<domain:update>\`, see *Setting and clearing a code*) and gives it to the registrant.
3. The registrant gives the code to the **gaining** registrar. The gaining registrar may check it first with \`<domain:info>\` (see *Checking a code before a transfer*).

### 2. The request

The gaining registrar sends \`<transfer op="request">\` with the domain's name, its code in \`<domain:authInfo><domain:pw>\`, and optionally a \`<domain:period>\`: the years the transfer adds to the expiry, **one year** if none is given (months are accepted in whole years).

The registry checks, in this order, and refuses at the first problem:

| Check | Refused with |
|---|---|
| The domain exists | 2303 |
| The gaining registrar is accredited for the domain's TLD | 2201 |
| The gaining registrar is not already the sponsor | 2106 |
| The code matches. This is checked before anything about the domain's state, so a registrar without the code learns nothing | 2202 |
| No transfer is already pending | 2300 |
| No other pending action (pending delete, restore…), no \`clientTransferProhibited\` or \`serverTransferProhibited\` | 2304 |
| The domain is out of its **transfer lock** period | 2106 |
| The new expiry stays within the TLD phase's **maximum horizon** (10 years by default) | 2306 |
| The transfer can be quoted for the gaining registrar (a phase with no transfer price quotes it at no charge, as it does a registration or a renewal) | 2400 |

An update prohibition (\`clientUpdateProhibited\`, \`serverUpdateProhibited\`) does **not** stop a transfer.

The registry answers **1001** (action pending), with the transfer in \`<domain:trnData>\`.

### 3. Pending

- The domain has the status \`pendingTransfer\`. While pending, the sponsor cannot update, renew or delete it (2304).
- The losing registrar receives a poll message: *Transfer requested*.
- The transfer's \`acDate\` is the end of the **pending transfer period** (5 days by default). If the losing registrar has not answered by then, **the registry approves the transfer** (\`serverApproved\`), as the ICANN Transfer Policy requires. A scheduled job checks every hour, so this happens within the hour after the \`acDate\`, and the transfer completes exactly as if the losing registrar had approved it.

### 4. The answer

| Op | Who | Result |
|---|---|---|
| \`approve\` | The losing registrar (the sponsor) | The transfer completes: \`clientApproved\` |
| *(no answer)* | The registry, when the pending period ends | The transfer completes: \`serverApproved\` |
| \`reject\` | The losing registrar | The domain stays: \`clientRejected\` |
| \`cancel\` | The gaining registrar that requested it | The domain stays: \`clientCancelled\` |
| \`query\` | The sponsor, either registrar of the latest transfer, or anyone giving the domain's code | The latest transfer, pending or not |

Any other registrar gets **2201** and learns nothing. Answering a transfer that is not pending is **2301**. Each answer is **1000**, with the transfer in \`<domain:trnData>\`.

### 5. What an approval changes

Everything below happens in one database transaction, so the domain, its nameservers and the messages never disagree.

| What | After the transfer |
|---|---|
| Sponsor | The gaining registrar |
| Expiry date | Extended by the transfer period (one year by default) |
| Transfer date | Set (\`<domain:trDate>\`) |
| Transfer code | **Cleared**: a code works once (RFC 9154). The gaining registrar sets a new one when the registrant next asks |
| Grace periods | The losing registrar's add, renew and auto-renew grace periods end. The domain enters the **transfer grace period** (RGP status \`transferPeriod\`, 5 days by default) |
| Transfer lock | A new lock starts (60 days by default; 0 in test TLDs) |
| Statuses | The client statuses (such as \`clientUpdateProhibited\`) stay; \`pendingTransfer\` is removed |
| Grandfathered pricing | Ends if it was granted *until transfer* |
| Contacts | **Stay as they are**, still the losing registrar's contact objects. The gaining registrar should replace them with its own; its poll message says so |
| Billing | The gaining registrar is charged the TLD's transfer price for the period, recorded on a \`domain.transferred\` event |

**Nameservers.** Host objects belong to a registrar, so each of the domain's nameservers is handled by where it lives:

| The nameserver is… | What happens |
|---|---|
| Under the transferred domain (\`ns1.example.test\` for \`example.test\`) | The host object **moves** with the domain to the gaining registrar, with its addresses (RFC 5732: a host belongs to its domain's sponsor). Other domains that use it keep using it |
| Outside the registry's TLDs (\`ns1.dns-provider.net\`) | The domain is linked to the gaining registrar's host of the same name, which is **created** for it if it has none. The losing registrar's host stays |
| Under another domain of the registry (\`ns.other.test\`) | The link **stays**. Only that domain's sponsor may hold a host under it; a copy would publish a second, diverging set of glue records |

Hosts the losing registrar no longer uses are left in place; cleaning them up is a later release.

### 6. Rejection and cancellation

The domain loses \`pendingTransfer\` and nothing else changes: same sponsor, expiry and transfer code.

---

## Poll messages

Each step of a transfer queues a message for the registrar that did **not** take it; when the registry acts, both are told.

| Step | Message | To |
|---|---|---|
| Request | *Transfer requested* | The losing registrar |
| Approval | *Transfer approved; the domain's contacts are still the losing registrar's: replace them with your own* | The gaining registrar |
| Rejection | *Transfer rejected* | The gaining registrar |
| Cancellation | *Transfer cancelled* | The losing registrar |
| Approval by the registry, at the end of the pending period | *Transfer approved by the registry* (the gaining registrar's adds the contacts note) | Both |

Every message carries the transfer as it stood at that step, as \`<domain:trnData>\`.

A registrar reads its queue with \`<poll op="req">\`: **1301** with the oldest message and the number waiting (\`<msgQ count id>\`), or **1300** when the queue is empty. Reading does not remove a message: \`<poll op="ack" msgID="…">\` does, answering **1000** with how many remain. A queue is the registrar's own: another registrar's message ID, or one already acknowledged, is **2303**.

---

## Settings per TLD phase

| Setting | Default | What it does |
|---|---|---|
| Pending transfer period | 5 days | How long the losing registrar has to answer |
| Transfer grace period | 5 days | The \`transferPeriod\` after a transfer |
| Transfer lock period | 60 days | No transfer for this long after a registration or a transfer. Set it to **0** for an RST or OT&E test TLD, where a domain must be transferable at once |
| Maximum horizon | 10 years | A transfer cannot take the expiry further out than this |

Change them on the TLD's phase, under **Policy**.

---

## EPP result codes

| Code | When |
|---|---|
| 1001 | A transfer request was accepted and is pending |
| 1300, 1301 | \`<poll op="req">\`: no messages; a message to acknowledge |
| 2003 | \`<poll op="ack">\` without a \`msgID\` |
| 2102 | Authorization information that is not a password (\`<domain:ext>\`): the registry keeps only hashed passwords |
| 2106 | A transfer request by the sponsor, or in the transfer lock period |
| 2201 | A registrar with no part in the domain (without its code, for info and query) |
| 2202 | A code too weak to set; a wrong code, or a code for a domain with none |
| 2300 | A transfer request while one is pending |
| 2301 | An answer to, or query of, a transfer that is not pending or does not exist |
| 2303 | A domain, or a poll message in the registrar's queue, that does not exist |
| 2304 | A transfer request blocked by the domain's statuses; a change to a domain pending transfer |
| 2306 | A transfer period that would exceed the maximum horizon |

---

## References

- RFC 9154, Extensible Provisioning Protocol (EPP) Secure Authorization Information for Transfer
- RFC 5730, Extensible Provisioning Protocol (\`<poll>\`, transfer statuses)
- RFC 5731, EPP Domain Name Mapping (\`<domain:info>\`, \`<domain:update>\`, \`<domain:transfer>\`)
- RFC 5732, EPP Host Mapping (hosts belong to their domain's sponsor)
- RFC 3915, Domain Registry Grace Period Mapping (\`transferPeriod\`)
- ICANN Transfer Policy
`;
