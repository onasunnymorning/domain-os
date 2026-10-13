# RSP Testing North Star — superseded

This working principle is now a decision: **[ADR 0015](adr/0015-rst-conformance-test-suites.md)**, RST conformance suites.

In short:
- Each ICANN RST v2.0 case is a test named by its case ID.
- Every case is accounted for, as passing, pending with the phase that delivers it, or operational.
- The spec release is pinned in the repository with its provenance.
- A completeness test keeps each suite in step with the pinned catalogue.
- `make rst-<protocol>` prints the readiness scoreboard.

The first suite is EPP (`make rst-epp`).

This file is kept so existing links still resolve. Read the ADR.
