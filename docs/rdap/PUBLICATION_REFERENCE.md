# RDAP publication reference: the field-access matrix

What the RDAP server publishes for each contact field, per publication policy ([ADR 0021](../adr/0021-rdap-publication-and-redaction.md)). This page is the shared specification for the server, compliance review and a future authenticated tier. Change it in the same PR as the built-in policies change.

> **Status: draft, awaiting the policy owner's sign-off.** The server applies these policies, which are stored and editable (Phase 3b). Neither built-in may be used for a live TLD until the policy owner has confirmed it and the sign-off is recorded below ([Sign-off](#sign-off)). This is an operational requirement: the server does not check for a sign-off before serving a policy.

## Scope

This page covers the **public, unauthenticated response** of the **registry's** RDAP server. Every requester is public today: the decision takes the requester as an input but ignores it until the authenticated tier ([ADR 0021](../adr/0021-rdap-publication-and-redaction.md), RFC 9560) exists.

It is not a registrar baseline. Some Registration Data Policy obligations fall on registrars, not on the registry: for example §9.2.3, offering an email address or web form through which a redacted registrant can be contacted. Do not reuse these policies for a registrar's RDAP service without checking those obligations.

## How a field is decided

1. **Role.** The role is served only if the TLD's current GA phase allows it (`mandatory` or `optional` in its `ContactDataPolicy`). A prohibited role, or a TLD with no current GA phase, means no entity for that role and nothing listed in `redacted`.
2. **Rule.** The TLD's publication policy gives the field one rule:

   | Rule | Meaning |
   |---|---|
   | `publish` | Published |
   | `publish-if-disclosed` | Published only if the contact's stored disclose setting for the field is true. Otherwise **not published, and listed in `redacted`**, with method `removal` except as in step 5 |
   | `withhold` | Not published, and listed in `redacted` |

3. **Disclose settings** can only narrow a `publish-if-disclosed` field. They never widen a `withhold`. For the name, organisation and address, the setting that counts is that of the postal form shown (the internationalised one when a contact has both).

   The stored setting is not the raw EPP flag. The registry's default is **not disclosed**: a new contact discloses nothing. A `<contact:disclose flag="1">` from the sponsoring registrar sets the listed elements to disclosed, and `flag="0"` sets them back (RFC 5733 §2.9 flags are exceptions to the server's default). A missing setting therefore means not disclosed. The server reads a disclosed setting as the contact's agreement to **public** RDAP publication of that field; the policy owner confirms that reading at sign-off.
4. **Absent data** (for example no fax number) is simply absent, and is not listed in `redacted`. The jCard can still hold an empty slot for it where its structure requires one: an `adr` property has seven parts, so a published country keeps the other parts as empty strings, and a jCard always has an `fn`. An empty slot for absent data is not a redaction and has no `redacted` entry.

5. **Kept jCard structure.** Some withheld fields are emptied rather than removed even where their rule says `removal` (or where an undisclosed `publish-if-disclosed` field would be removed), because the jCard keeps a slot for them:
   - The **name** is always `emptyValue` when withheld: a jCard always has an `fn`.
   - The **street, city, state or province and postal code** are `emptyValue` when withheld, if any part of the address is shown or emptied (the `adr` property is then present, and its value slots stay as empty strings). The **country** is not affected: it is carried in the address's parameters, so it can still be removed.

   The `redacted` entry then records `emptyValue`, so it matches the response.

Redaction methods (RFC 9537): `removal` (the member is left out), `emptyValue` (the member is present but empty), and `replacementValue` (the member is left out and another member carries a substitute value). The only replacement in use is for email: the `email` property is removed, the contact form URI is published in a separate `contact-uri` jCard property, and the `redacted` entry's `replacementPath` points at that property. A URL is never placed in an `email` property.

## Not supported: privacy and proxy services

The Registration Data Policy (§9.2.5) requires the full registration data of an affiliated or accredited privacy or proxy service to be published, unredacted. **These policies do not support that case.** The server cannot tell a privacy or proxy registration from any other, so such a registration is withheld like every other contact. Until this is implemented, a TLD that must meet §9.2.5 is not ready for live use under `gtld-rdp-2024`, and the sign-off below records this as a known gap.

## Built-in policies

`P` = publish, `D` = publish-if-disclosed, `W` = withhold (method in brackets).

A role a policy does not list is **not published**: no entity is served for it and nothing about it is listed in `redacted`, the same as a role the TLD prohibits.

A TLD is assigned a policy through the admin API ([POLICY_REFERENCE.md § Publication policies](POLICY_REFERENCE.md#publication-policies)). A TLD with none has `gtld-rdp-2024` if it is generic and `cctld-minimal` otherwise. The built-ins' starting rules are code (`entities.BuiltInRDAPPublicationPolicies`), seeded by the migration and editable afterwards; this page lists the starting rules and must match that code. An IDN TLD (`xn--…`) with no assignment has `cctld-minimal` whatever its stored type: the registry infers the type from the name and files IDN ccTLDs as generic, so when the type is uncertain the less revealing policy applies.

That fallback is a safety net, not a configuration. **Before a TLD goes live it must be classified and explicitly assigned a policy.** This matters most for an IDN gTLD: without an assignment it gets `cctld-minimal` and withholds the state or province, which the gTLD policy requires to be published (Registration Data Policy §§9.1.4–9.1.5).

### `gtld-rdp-2024`: ICANN Registration Data Policy baseline

| Field | Source | Registrant | Technical |
|---|---|---|---|
| Handle (ROID) | `Contact.RoID` | W (removal) | W (removal) |
| Name | `PostalInfo.Name` | W (emptyValue) | W (emptyValue) |
| Organisation | `PostalInfo.Org` | D | W (removal) |
| Street | `Address.Street1-3` | W (emptyValue) | W (removal) |
| City | `Address.City` | W (emptyValue) | W (removal) |
| State or province | `Address.SP` | P | W (removal) |
| Postal code | `Address.PC` | W (emptyValue) | W (removal) |
| Country | `Address.CC` | P | W (removal) |
| Phone | `Contact.Voice` | W (removal) | W (removal) |
| Fax | `Contact.Fax` | W (removal) | W (removal) |
| Email | `Contact.Email` | W (replacementValue: contact form URI) | W (replacementValue: contact form URI) |

The administrative and billing roles are not published.

The gTLD profile fixes some methods, and ICANN's conformance tool checks them: the registrant's name, street, city and postal code are emptied; the organisation and phone are removed; the email is replaced by a contact form or removed. A withheld phone or fax is always removed: an empty `tel` is not a valid jCard property, so a policy that empties one is refused.

A `replacementValue` email needs the policy's contact form URI (`contactFormUri`). The built-ins start without one, so their emails are removed until a URI is set. With a URI set, the jCard gets a `contact-uri` property holding it, and the email's `redacted` entry names that property as its `replacementPath`. Leaving the email out without a form is acceptable for this registry server: the §9.2.3 duty to offer an alternative contact channel is the registrar's (see [Scope](#scope)).

The internationalised (`int`) postal information is used when a contact has both forms.

### `cctld-minimal`: country only

Only the registrant role is published. The country is `P`. The name, street, city, state or province and postal code are `W (emptyValue)`: the country is published in the jCard's address, whose other parts are then kept as empty slots, and a jCard always has a name. Every other field is `W (removal)`.

## Sign-off

A sign-off approves **one revision** of a policy, not its name. The policies can be edited after seeding, so record what was approved: the policy's `updatedAt` as the admin API returns it (`GET /rdap/publication-policies/{name}`), which every save through the API sets, or a hash over **both** of its publication-affecting members, the field rules (`roles`) and the contact form URI (`contactFormUri`), the TLDs or TLD class it covers, the approver and the date.

**Any change to a signed-off policy's rules or contact form URI needs a new sign-off** before it is served on a live TLD. A row whose revision no longer matches the stored policy is not a sign-off.

The policy owner also records, for each policy:

- the basis for redacting under Registration Data Policy §9.2.1;
- that a disclosed setting is treated as agreement to public publication (see [How a field is decided](#how-a-field-is-decided), step 3);
- the privacy and proxy gap ([Not supported](#not-supported-privacy-and-proxy-services)), and whether it blocks the TLDs in scope.

| Policy | Revision (`updatedAt`, or hash of `roles` and `contactFormUri`) | Scope (TLDs) | §9.2.1 basis | Confirmed by | Date | Notes |
|---|---|---|---|---|---|---|
| `gtld-rdp-2024` | | | | | | |
| `cctld-minimal` | | | | | | |
