# Published XML schemas (test data)

These XSDs are used **only by tests**, to prove that the XML this service emits
is schema-valid against the published standards rather than against our own Go
structs. Nothing at runtime loads them, and no runtime CGO is involved.

Two wrapper schemas load a set of namespaces in dependency order. The published
schemas import each other by namespace without a `schemaLocation`, so without a
wrapper libxml2 cannot resolve them.

| Wrapper | Covers | Used by |
|---|---|---|
| `eve-schemas.xsd` | `rdeReport`, `rdeNotification`, `iirdea` and their dependencies | `rdereport`, escrow-validation activity tests |
| `deposit-schemas.xsd` | a complete RDE deposit and the EPP object mappings it composes with | `rdesanitize` derivative conformance |

## Sources

| File | Source | Namespace |
|---|---|---|
| `rdeReport-1.0.xsd` | draft-lozano-icann-registry-interfaces-26 §9.2 | `urn:ietf:params:xml:ns:rdeReport-1.0` |
| `rdeNotification-1.0.xsd` | draft-lozano-icann-registry-interfaces-26 §9.3 | `urn:ietf:params:xml:ns:rdeNotification-1.0` |
| `iirdea-1.0.xsd` | draft-lozano-icann-registry-interfaces-26 §9.1 | `urn:ietf:params:xml:ns:iirdea-1.0` |
| `rde-1.0.xsd` | RFC 8909 §7 | `urn:ietf:params:xml:ns:rde-1.0` |
| `rdeHeader-1.0.xsd` | RFC 9022 §8 | `urn:ietf:params:xml:ns:rdeHeader-1.0` |
| `rdeDnrdCommon-1.0.xsd` | RFC 9022 §8 | `urn:ietf:params:xml:ns:rdeDnrdCommon-1.0` |
| `rdeDomain-1.0.xsd` | RFC 9022 §8 | `urn:ietf:params:xml:ns:rdeDomain-1.0` |
| `rdeHost-1.0.xsd` | RFC 9022 §8 | `urn:ietf:params:xml:ns:rdeHost-1.0` |
| `rdeContact-1.0.xsd` | RFC 9022 §8 | `urn:ietf:params:xml:ns:rdeContact-1.0` |
| `rdeRegistrar-1.0.xsd` | RFC 9022 §8 | `urn:ietf:params:xml:ns:rdeRegistrar-1.0` |
| `rdeIDN-1.0.xsd` | RFC 9022 §8 | `urn:ietf:params:xml:ns:rdeIDN-1.0` |
| `rdeNNDN-1.0.xsd` | RFC 9022 §8 | `urn:ietf:params:xml:ns:rdeNNDN-1.0` |
| `rdeEppParams-1.0.xsd` | RFC 9022 §8 | `urn:ietf:params:xml:ns:rdeEppParams-1.0` |
| `rdePolicy-1.0.xsd` | RFC 9022 §8 | `urn:ietf:params:xml:ns:rdePolicy-1.0` |
| `epp-1.0.xsd`, `eppcom-1.0.xsd` | RFC 5730 §4 | `urn:ietf:params:xml:ns:epp-1.0`, `…:eppcom-1.0` |
| `domain-1.0.xsd` | RFC 5731 §4 | `urn:ietf:params:xml:ns:domain-1.0` |
| `host-1.0.xsd` | RFC 5732 §4 | `urn:ietf:params:xml:ns:host-1.0` |
| `contact-1.0.xsd` | RFC 5733 §4 | `urn:ietf:params:xml:ns:contact-1.0` |
| `secDNS-1.1.xsd` | RFC 5910 §4 | `urn:ietf:params:xml:ns:secDNS-1.1` |
| `rgp-1.0.xsd` | RFC 3915 §5 | `urn:ietf:params:xml:ns:rgp-1.0` |
| `eve-schemas.xsd`, `deposit-schemas.xsd` | this repository | wrappers for `xmllint` |

Retrieved 2026-09-08 (registry-interface drafts) and 2026-09-09 (RFC set) from
`https://www.ietf.org/archive/id/draft-lozano-icann-registry-interfaces-26.txt`
and `https://www.rfc-editor.org/rfc/rfc{9022,8909,5730,5731,5732,5733,5910,3915}.txt`.
The `<CODE BEGINS>`/`<CODE ENDS>` (or `BEGIN`/`END`) markers, page furniture and
the uniform RFC indentation were stripped; nothing else was changed.

The pinned registry-interface version is
`draft-lozano-icann-registry-interfaces-26`. A draft bump changes `rdereport`
and these files, and nothing else.

## What the deposit schemas taught us

RFC 9022 defines no `authInfo` anywhere: a deposit that carries one is already
non-conformant. `rdesanitize` removes it, so a derivative validates against
these schemas even when its source does not — a property the conformance test
asserts in both directions.

RDE composes EPP types, so an object's children are not always in the object's
own namespace: a contact's `postalInfo` and `disclose` content is
`contact-1.0`, and `rdeEppParams`' service and policy content is `epp-1.0`.
The sanitisation profile classifies by resolved namespace for exactly this
reason.
