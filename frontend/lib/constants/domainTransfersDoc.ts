// The /docs/domain-transfers page. Keep it in step with
// pkg/domain/entities/authInfoHash.go (the rules) and
// docs/epp/POLICY_REFERENCE.md (the EPP result codes).
export const DOMAIN_TRANSFERS_DOC_MARKDOWN = `# Domain Transfers

A **transfer** moves a domain from one registrar (the *losing* registrar, its current sponsor) to another (the *gaining* registrar). The registrant proves they want the move by giving the gaining registrar the domain's **transfer code**, which EPP calls *authorization information* or *AuthInfo*.

Domain OS follows **RFC 9154** (*Extensible Provisioning Protocol Secure Authorization Information for Transfer*) for transfer codes. This page explains how codes are set, kept and checked. The transfer commands themselves are described under *The transfer flow*, below.

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

The admin API also accepts codes under the registry's earlier rule (8 to 22 characters with upper case, lower case and a symbol), so existing tooling keeps working. Registrars over EPP, and the admin UI, must meet the RFC 9154 rule.

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

## EPP result codes

| Code | When |
|---|---|
| 2202 | A code too weak to set (create or update); a wrong code, or a code for a domain with none (info, transfer) |
| 2201 | A registrar that does not sponsor the domain gives no code |
| 2102 | Authorization information that is not a password (\`<domain:ext>\`): the registry keeps only hashed passwords |

---

## The transfer flow

The EPP transfer commands (\`<domain:transfer>\` request, query, approve, reject and cancel) and the poll queue that tells both registrars about a transfer are being built in the next release. This section will then describe each step. What is decided:

- **Request.** The gaining registrar requests the transfer with the domain's code. A wrong code is refused (2202). A domain under a transfer prohibition, in another pending state, or in its transfer lock period after registration or a previous transfer, cannot be transferred.
- **Pending.** The domain is \`pendingTransfer\`. Both registrars get a poll message. The losing registrar may approve or reject; the gaining registrar may cancel.
- **Approval.** On approval the domain moves to the gaining registrar and its transfer code is cleared, so the code cannot be used again.
  - **Nameservers.** Hosts belong to a registrar, so the domain's nameservers are linked to the gaining registrar's hosts of the same names, which are created for it if it has none (with their addresses, for hosts under the registry's TLDs). The losing registrar's hosts are left in place.
  - **Contacts.** Contacts stay as they are: the domain keeps pointing at the same contact objects, still sponsored by the losing registrar. The gaining registrar should replace them with its own, as the poll message says.
- **Transfer lock.** After registration and after each transfer, a domain cannot be transferred again for the phase's transfer lock period (60 days by default, settable per TLD phase; 0 for test TLDs).
- **Auto-approval.** A transfer the losing registrar does not answer within the transfer grace period is approved automatically. This follows in a later release.

---

## References

- RFC 9154, Extensible Provisioning Protocol (EPP) Secure Authorization Information for Transfer
- RFC 5731, EPP Domain Name Mapping (\`<domain:info>\`, \`<domain:update>\`, \`<domain:transfer>\`)
- ICANN Transfer Policy
`;
