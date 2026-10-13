# ADR 0016 — EPP schema validation: Go models of the XSDs, proven against xmllint

- **Status:** Accepted
- **Date:** 2026-10-13
- **Deciders:** Platform / Registry
- **Composes with:** [ADR 0012](0012-escrow-runtime-schema-validation.md) (escrow validates with xmllint), [ADR 0013](0013-epp-server-architecture.md) (the EPP server), [ADR 0015](0015-rst-conformance-test-suites.md) (the pinned RST schemas)

## Context

RFC 5730 expects a server to reject a command that does not match the schemas with 2001, and the EPP architecture (§4) puts XSD validation between parsing and dispatch. RST raises `EPP_SCHEMA_VALIDATION_ERROR` on schema problems in what the server sends, and probes what it accepts.

Go has no complete XSD validator. There were three ways to validate at runtime:

1. **libxml2 through cgo** (`lestrrat-go/libxml2`, already vendored indirectly). Full fidelity by construction. But the EPP binary would need cgo, a C toolchain for every target architecture, and libxml2 in its image. libxml2 would parse untrusted input straight off the network. And the binding has been unmaintained since 2020.
2. **xmllint as a subprocess**, as escrow does (ADR 0012). Full fidelity with no cgo. But escrow validates once per deposit; EPP would spawn a process per command, and most at drop time, when commands arrive fastest.
3. **Go models of the schemas**, for the frames the server accepts, kept honest by a differential test against xmllint.

## Decision

1. **Option 3.** `internal/interface/epp/schema` mirrors each XSD the server accepts frames for: element names and order, occurrence bounds, choices, `##other` wildcards resolved strictly against the models, attributes, and the simple-type facets of every value (whitespace processing, lengths in characters, patterns, enumerations, ranges). It covers the client side of `epp-1.0`, `eppcom-1.0`, and each object or extension schema whose commands are bound. A command whose schema is not modelled is not bound, so it is answered 2101 before validation.

2. **The models are proven, not trusted.** A differential test validates a corpus of frames (`schema/testdata/frames`) and systematic mutations of each with both the models and xmllint on the RST-pinned schemas, and fails on any disagreement. The mutations remove, duplicate, reorder, rename and re-namespace every element, add unexpected children, text and attributes, and set every text and attribute value to each of a list of boundary values. A Go fuzz target extends the same comparison to arbitrary input. CI installs xmllint and the test fails there without it, as the RST suite does (ADR 0015).

3. **Every new model comes with corpus frames.** Binding a command means adding its model and at least one valid frame using every element of it. The mutator then covers the invalid side.

4. **Validation errors name the element and the rule, never the value.** The 2001 message is `Command syntax error; <path>: <rule>`. Values include passwords.

5. **Unsupported extensions are answered before validation.** An extension element whose namespace the session did not log in with is answered 2103, not 2001: it is unimplemented, not malformed.

## Consequences

- The EPP binary stays static and cgo-free, and no C parser sees network input. Validation costs one tree walk of a frame that is already parsed.
- Fidelity is a property of the tests. A model is only as good as its corpus; a reviewer adding a command should check that its frames exercise every element.
- `xs:anyType` content (`<hello>`, `<logout>`) is modelled as lax, as XSD defines it. Anything that is not namespace-well-formed is caught earlier, by the frame scanner and parser, so the comparison skips input libxml2 cannot parse.
- If a complete, maintained pure-Go XSD validator appears, it can replace the models; the differential test would then become its acceptance test.
