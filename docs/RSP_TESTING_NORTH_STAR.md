# RSP Testing North Star

ICANN's RST v2.0 (Registry System Testing) defines the test-case catalog an
RSP is measured against — RDAP, EPP, DNS, RDE, IDN, SRS Gateway — each with
its own numbered case IDs (e.g. RDAP's `Test-Suite-StandardRDAP`: domain/
nameserver/registrar/help queries as `rdap-01`…`04`, HEAD support as
`05`…`07`, non-existent object handling as `08`…`10`, TLS/port conformance
as `91`/`92`).

**Principle:** when we build or extend a test suite for a protocol RST
covers, we adopt RST's case IDs and naming instead of inventing our own.
A passing test run should double as standing RSP-readiness evidence, not
a one-off internal check we'd have to re-derive against the spec later.

**How to apply:**
- New test files for an RST-covered protocol should name cases after their
  RST ID (`rdap-01`, ...) alongside a descriptive name, so the mapping to
  the spec is visible without cross-referencing a separate document.
- Where RST specifies inputs (e.g. `rdap.testDomains` / `testEntities` /
  `testNameservers`), seed fixtures from the same shape rather than
  ad-hoc test data, so fixtures stay swappable with ICANN's own.
- This is not yet enforced anywhere — no CI gate depends on it, and most
  of the catalog (RDAP, DNS, RDE, IDN, SRS Gateway) has no implementation
  in this codebase today. Treat it as the default shape for testing work
  in these areas as they get built, not a retroactive requirement.

This is a working principle, not an architectural invariant — it doesn't
belong in [`docs/INVARIANTS.md`](INVARIANTS.md) and isn't lint-enforced.
Promote pieces of it there only if/when concrete enforcement exists.
