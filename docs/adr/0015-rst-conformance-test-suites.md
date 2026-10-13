# ADR 0015 — RST conformance suites: one test per ICANN case, a pinned spec release, a readiness scoreboard

- **Status:** Accepted
- **Date:** 2026-10-13
- **Deciders:** Platform / Registry
- **Supersedes:** `docs/RSP_TESTING_NORTH_STAR.md` (a working principle; this ADR makes it a decision, and that file now points here)
- **Composes with:** [ADR 0012](0012-escrow-runtime-schema-validation.md) (schema validation with libxml2's xmllint), [ADR 0013](0013-epp-server-architecture.md) and [ADR 0014](0014-epp-credential-rotation.md) (the EPP server the first suite tests)

## Context

ICANN's Registry System Testing (RST v2.0) is the test an RSP has to pass. It is a published catalogue of cases with stable IDs, across EPP, RDAP, DNS, DNSSEC, RDE, IDN, the SRS gateway and integration between them. Each case lists numbered requirements, the inputs ICANN needs, and the errors it can raise.

`docs/RSP_TESTING_NORTH_STAR.md` asked for test suites to adopt RST's case IDs, so a passing run would double as readiness evidence. It was a principle with no mechanism behind it, so nothing stopped a suite drifting from it. Nothing showed how far from RST the system was, and nothing noticed when ICANN changed the catalogue.

The EPP server is the first protocol to get a suite. Writing that suite against the catalogue found two defects that our own tests had missed:
- The greeting had no `<dcp>` element, so it failed the EPP schema. RST rejects that.
- The server offered CBC-mode cipher suites. RST requires the RFC 9325 recommendations.

## Decision

1. **One test per RST case, named by its ID.** For example, `TestRST_epp01_ServiceConnectivity`. Each numbered requirement in the case is a subtest named after it, such as `epp-01/5 TLSv1.1 and older refused`. A failure therefore names the clause that would fail at ICANN.

2. **Every case is accounted for, including the ones we cannot run yet.** A requirement that depends on unbuilt functionality is a subtest that **skips with the phase that delivers it**. One that depends only on a deployment (DNS records, the firewall, the certificate actually installed) **skips as operational**. Nothing is silently absent.

3. **The spec is pinned, not fetched.** Each suite keeps a copy of the RST release it targets in its `testdata/rst` directory: the case catalogue, the schemas and the extension registry. A `PROVENANCE.md` records the release tag, the commit and the update procedure. Tests never reach the network. A new RST release reaches us as a reviewed PR, not as a surprise red build.

4. **A completeness test ties the code to the catalogue.** `TestRST_<protocol>CatalogueIsComplete` fails when the pinned catalogue has a case that the suite neither implements nor lists as pending, or the other way round. Bumping the pinned release is what surfaces new cases.

5. **Every frame is schema-checked.** For EPP, every frame the server sends in any test is validated against the pinned XSDs with xmllint, the tool ADR 0012 already uses. CI installs xmllint and the check fails there if it is missing. A machine without xmllint logs that it skipped the check rather than skipping the test.

6. **Test inputs take RST's shape.** Fixtures follow RST's input parameters (`epp.clid01`, `epp.clid02`, `epp.pwd01`, and so on, with RST's length limits), so swapping in ICANN's values needs no restructuring.

7. **The scoreboard is a make target.** `make rst-<protocol>` (first: `make rst-epp`) runs only the suite and prints passed, failed and pending counts, with every pending reason. It is the answer to "how close are we to passing RST?"

8. **The suites run in the normal test job.** They are ordinary Go tests in the package they test. A conformance regression fails CI like any other regression.

## Consequences

- The EPP suite covers epp-01 to epp-03 today. The rest of the catalogue (epp-04 to epp-27) is pending, each case with its phase.
- New RDAP, DNS and RDE work starts by pinning that protocol's part of the catalogue and adding its completeness test, before any implementation.
- A case that RST marks "if applicable" stays in the suite. Where our configuration makes it not applicable, it skips with that reason, so the decision is visible.
- Operational requirements are listed, not tested. They need a pre-RST checklist for an actual test run, which is not part of this ADR.
- Pinning means we can fall behind ICANN. The ChangeLog review in `PROVENANCE.md` is the control. Bumping the release is cheap because the completeness test does the bookkeeping.

## Alternatives considered

- **Fetch the spec at test time.** This is always current, but makes CI depend on GitHub's availability and turns ICANN's release schedule into ours. Rejected.
- **Keep it a principle.** That was the state before this ADR. Rejected, because nothing kept the EPP suite honest.
- **Run ICANN's own RST tooling in CI.** That is the real test, but it needs a deployed, DNS-reachable environment and ICANN's involvement. It belongs to an OT&E run, not to every commit. These suites are what make such a run unsurprising.
