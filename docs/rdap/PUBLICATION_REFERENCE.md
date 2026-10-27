# RDAP publication reference: the field-access matrix

What the RDAP server publishes for each contact field, per publication policy ([ADR 0021](../adr/0021-rdap-publication-and-redaction.md)). This page is the shared specification for the server, compliance review and a future authenticated tier. Change it in the same PR as the built-in policies change.

> **Status: draft, awaiting the policy owner's sign-off.** The server does not exist yet (Phase 2 builds the decision; Phase 3 makes policies data). Neither built-in may be used for a live TLD until the policy owner has confirmed it and the sign-off is recorded below.

## How a field is decided

1. **Role.** The role is served only if the TLD's current GA phase allows it (`mandatory` or `optional` in its `ContactDataPolicy`). A prohibited role, or a TLD with no current GA phase, means no entity for that role and nothing listed in `redacted`.
2. **Rule.** The TLD's publication policy gives the field one rule:

   | Rule | Meaning |
   |---|---|
   | `publish` | Published |
   | `publish-if-disclosed` | Published only if the contact's RFC 5733 disclose flag for the field is set |
   | `withhold` | Not published, and listed in `redacted` |

3. **Disclose flags** can only narrow a `publish-if-disclosed` field. They never widen a `withhold`.
4. **Absent data** (for example no fax number) is simply absent, and is not listed in `redacted`.

Redaction methods (RFC 9537): `removal` (the member is left out), `emptyValue` (the member is present but empty), and `replacementValue` (another value is shown instead, for example a contact form URI in place of an email address).

## Built-in policies

`P` = publish, `D` = publish-if-disclosed, `W` = withhold (method in brackets).

### `gtld-rdp-2024`: ICANN Registration Data Policy baseline

| Field | Source | Registrant | Technical | Admin | Billing |
|---|---|---|---|---|---|
| Handle (ROID) | `Contact.RoID` | W (removal) | W (removal) | W (removal) | W (removal) |
| Name | `PostalInfo.Name` | W (emptyValue) | W (emptyValue) | W (removal) | W (removal) |
| Organisation | `PostalInfo.Org` | D | W (removal) | W (removal) | W (removal) |
| Street | `Address.Street1-3` | W (emptyValue) | W (removal) | W (removal) | W (removal) |
| City | `Address.City` | W (emptyValue) | W (removal) | W (removal) | W (removal) |
| State or province | `Address.SP` | P | W (removal) | W (removal) | W (removal) |
| Postal code | `Address.PC` | W (emptyValue) | W (removal) | W (removal) | W (removal) |
| Country | `Address.CC` | P | W (removal) | W (removal) | W (removal) |
| Phone | `Contact.Voice` | W (emptyValue) | W (emptyValue) | W (removal) | W (removal) |
| Fax | `Contact.Fax` | W (removal) | W (removal) | W (removal) | W (removal) |
| Email | `Contact.Email` | W (replacementValue: contact form URI) | W (replacementValue: contact form URI) | W (removal) | W (removal) |

### `cctld-minimal`: country only

Every field of every role is `W (removal)`, except the registrant's country, which is `P`.

## Sign-off

| Policy | Confirmed by | Date | Notes |
|---|---|---|---|
| `gtld-rdp-2024` | | | |
| `cctld-minimal` | | | |
