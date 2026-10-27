# ADR 0021 — RDAP publication: the TLD's current policy decides, field by field, before serialisation

- **Status:** Accepted
- **Date:** 2026-10-26
- **Deciders:** Platform / Registry
- **Composes with:** [ADR 0006](0006-tenancy-model.md) (amended: the public read plane), [ADR 0019](0019-epp-policies.md) (the policy-as-data pattern copied here), [ADR 0020](0020-rdap-server-architecture.md) (the server)
- **Roadmap:** [`docs/rdap/ARCHITECTURE.md`](../rdap/ARCHITECTURE.md); the field-access matrix in [`docs/rdap/PUBLICATION_REFERENCE.md`](../rdap/PUBLICATION_REFERENCE.md)

## Context

An RDAP domain response can carry contact data: registrant, administrative, technical and billing contacts, each with a name, organisation, address, phone, fax and email. What may be published depends on whose TLD it is:

- for a gTLD, ICANN's Registration Data Policy and the gTLD RDAP Profile;
- for a ccTLD, its own governance;
- for every TLD, the applicable privacy law.

None of these depends on the registrar, and they change over time. A TLD can move from thick to thin.

The registry already holds two relevant things:

- **Which contact roles a TLD collects.** Each phase's `ContactDataPolicy` marks a role as mandatory, optional or prohibited.
- **RFC 5733 disclose flags on each contact**, set by the registrar over EPP and false (disclose nothing) by default.

Nothing yet maps these to what RDAP may show. Contacts are linked to domains by ID and outlive any change of policy: a registrant linked while a TLD was thick is still linked after it goes thin.

RFC 9537 lets a response say that a field was withheld by policy rather than being absent, so a client can tell the two apart.

## Decision

1. **What a domain response contains is decided by the TLD's current policy, never by the data stored or by the registrar.** A contact role is served only if the policy of the TLD's current GA phase allows it (mandatory or optional). A role the current policy prohibits is never served, even when a contact is still linked to the domain from an earlier, thicker policy. A TLD with no current GA phase serves no contacts. When there is no rule to apply, personal data is withheld.

2. **Within an allowed role, each field follows the TLD's field-access matrix.**
   - The matrix is a named **publication policy**. Each policy has, for each role and each field (handle, name, organisation, street, city, state or province, postal code, country, phone, fax, email), one rule:
     - `publish`
     - `publish-if-disclosed`: published only when the contact's disclose flag for that field is set
     - `withhold`

     A withheld field also has a redaction method from RFC 9537 (removal, empty value or replacement value; the replacement for email is a contact form URI).
   - Disclose flags can only narrow what a policy publishes, never widen it. A registrar cannot publish what the TLD's policy withholds.
   - Publication policies are stored in the database and assigned to TLDs, one assignment per TLD, with the built-in default when none is assigned. They follow the shape of ADR 0019:
     - typed and validated by the entity (INV-08);
     - edited through the admin API under a new `rdap:policy:admin` scope;
     - every change is an outbox event carrying the actor and the policy before and after, serialised by an advisory lock;
     - reloaded by every node by polling, with the last good set kept if a load fails;
     - no per-TLD overrides.
   - Two built-in policies are seeded from code and can be edited but not deleted:
     - `gtld-rdp-2024`: the Registration Data Policy's publication and redaction baseline;
     - `cctld-minimal`: only the country is published.

     The policy owner must confirm both before a TLD goes live.

3. **The decision is a pure function, applied before serialisation.**
   - `rdappolicy.Decide(requester, TLD policy, object)` returns, for every field, whether to publish it and how to redact it.
   - The response builders only carry the decision out. They do not decide.
   - Every path through the server goes through the same decision: a domain's nested entities, links, and errors (an error never echoes withheld data).
   - Every role × rule × disclose combination is tested without a server. The thick-to-thin case (a linked contact for a now-prohibited role) is tested first.

4. **Redaction is signalled per RFC 9537.**
   - A withheld field is listed in the response's `redacted` array, with its JSONPath, method and reason.
   - A field with no data is simply absent and not listed.
   - A role the current policy prohibits is treated as not collected: no entity is served for it and nothing about it is listed in `redacted`.
   - The response's notices say which roles the TLD publishes.

5. **The response is not the place to record disclosure.**
   - Redactions are counted in metrics by TLD, field and method, never with the values.
   - The policy in force is identified by its name and last change, never by the data it covered.
   - Personal data is not written to logs.

6. **Every requester is `public` for now; the requester type is built for more.**
   - `Decide` takes a requester. Today the server builds only an anonymous one, which resolves to the `public` tier.
   - Authenticated access (RFC 9560 OpenID Connect) and a reviewed disclosure workflow are later phases (ADR 0020 decision 8). They will add tiers and grants as inputs to the same function, without changing the response builders.
   - An identity, even a verified one, is not a grant: a tier gives fields only through a grant that names the TLD, the fields and the purpose.
   - Responses to authenticated requests will never be shared-cached (ADR 0020 decision 7).

7. **RST's data model follows from the policy.** The `general.registryDataModel` declared for a TLD in an RST run is:
   - `maximum` when its current policy allows a registrant;
   - `minimum` when it prohibits one.

   So the conformance tool grades each TLD against the rules its policy actually produces.

## Consequences

- Changing a TLD's contact policy changes its RDAP responses within one reload interval, with no data migration. Contacts stored for a role that is now prohibited stay in the database (their retention is a separate decision) and are no longer served.
- ADR 0006 is amended with the public read plane (see its Amendments). Publication policies hang off the TLD, the operator side, and RDAP serves only what a TLD's policy publishes to everyone, so it does not decide the deferred cross-side visibility question (ADR 0006 rule 6).
- `docs/rdap/PUBLICATION_REFERENCE.md` is the human-readable field-access matrix: field, source, rule per built-in policy, redaction method and reason. It is the shared specification for the API, compliance review and a future authenticated tier.
- The contents of the built-in policies are a compliance decision, not an engineering one. The policy owner signs off each built-in before a TLD goes live, and that sign-off is recorded in the reference.

## Amendments

### 2026-10-27: loading the set (Phase 3b)

- A node does not serve until it has loaded the stored publication policies once. The built-ins are not a safe stand-in: a stored policy may withhold what a built-in publishes. A reload that fails later keeps the set last loaded.
- Policies and assignments are read in one consistent snapshot, and a set in which an assignment names a policy it does not hold is refused, so a TLD is never served its built-in in place of a policy created at the same moment.
