# Pinned RST v2.0 artifacts

The files in this directory, except `xsd/epp-schemas.xsd` and this file, are copied unmodified from ICANN's RST test specifications. The EPP conformance tests (`../../rst_epp_test.go`) run against them. ADR 0015 explains why they are pinned rather than fetched.

| | |
|---|---|
| Source | https://github.com/icann/rst-test-specs |
| Release | `v2026.08` |
| Commit | `906f221f46dedf95b74d4f9cdce10c7948c46ffa` |
| Copied | 2026-10-13 |
| License | BSD-3-Clause; see `LICENSE` (copied from the same commit) |

| File here | Path in the source repository |
|---|---|
| `epp-cases.yaml` | `inc/epp/cases.yaml` |
| `epp-extensions.yaml` | `epp-extensions/epp-extensions.yaml` |
| `xsd/epp-1.0.xsd`, `eppcom-1.0.xsd`, `domain-1.0.xsd`, `host-1.0.xsd`, `contact-1.0.xsd`, `secDNS-1.1.xsd`, `rgp-1.0.xsd` | `epp-extensions/xsd/` |
| `xsd/epp-schemas.xsd` | *Not from the source.* Written here: it imports the pinned schemas with their locations, because they import one another by namespace only |

`launch-1.0.xsd` is not pinned yet. It imports the mark and signedMark schemas, which nothing here uses; it arrives with launch-phase support.

## Moving to a newer release

1. Pick the release tag in the source repository, and read its `ChangeLog.yaml` entries for EPP since `v2026.08`.
2. Copy the files above from that tag, and update the release, commit and date in this file.
3. Run `make rst-epp`. `TestRST_EPPCatalogueIsComplete` fails for every case the new release adds or removes. Account for each one in `rst_epp_test.go`: implement it, or list it in `rstPendingCases` with the phase that delivers it.
4. Changed requirements in an existing case show up as failing subtests, or not at all. So read the ChangeLog: a changed requirement needs a changed test.
5. Land the bump as its own PR, so a conformance change is never hidden inside a feature.
