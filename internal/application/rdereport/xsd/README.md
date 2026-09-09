# ICANN registry-interface schemas (test data)

These XSDs are used **only by tests** (`xsd_test.go`) to prove that the XML this
service emits is schema-valid. Nothing at runtime loads them.

| File | Source | Namespace |
|---|---|---|
| `rdeReport-1.0.xsd` | draft-lozano-icann-registry-interfaces-26 §9.2 | `urn:ietf:params:xml:ns:rdeReport-1.0` |
| `rdeNotification-1.0.xsd` | draft-lozano-icann-registry-interfaces-26 §9.3 | `urn:ietf:params:xml:ns:rdeNotification-1.0` |
| `iirdea-1.0.xsd` | draft-lozano-icann-registry-interfaces-26 §9.1 | `urn:ietf:params:xml:ns:iirdea-1.0` |
| `rdeHeader-1.0.xsd` | RFC 9022 §8 | `urn:ietf:params:xml:ns:rdeHeader-1.0` |
| `rde-1.0.xsd` | RFC 8909 §7 | `urn:ietf:params:xml:ns:rde-1.0` |
| `eppcom-1.0.xsd` | RFC 5730 §4.2 | `urn:ietf:params:xml:ns:eppcom-1.0` |
| `eve-schemas.xsd` | this repository | wrapper that loads the set in dependency order for `xmllint` |

Retrieved 2026-09-08 from `https://www.ietf.org/archive/id/draft-lozano-icann-registry-interfaces-26.txt`
and `https://www.rfc-editor.org/rfc/rfc{9022,8909,5730}.txt`; the `<CODE BEGINS>`/`<CODE ENDS>`
markers, page headers and RFC indentation were stripped, nothing else was changed.

The pinned version is `draft-lozano-icann-registry-interfaces-26`
(`rdereport.SpecVersion`). Bumping the draft means replacing the files here,
re-running the conformance tests, and adjusting the adapter — validation
semantics do not change.
