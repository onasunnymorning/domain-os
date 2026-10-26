# Pinned RST v2.0 artifacts

The files in this directory, except this one, are copied unmodified from ICANN's RST test specifications. The RDAP conformance tests (`../../rst_rdap_test.go`) run against them. ADR 0015 explains why they are pinned rather than fetched.

| | |
|---|---|
| Source | https://github.com/icann/rst-test-specs |
| Release | `v2026.08` (the same release the EPP suite pins) |
| Commit | `906f221f46dedf95b74d4f9cdce10c7948c46ffa` |
| Copied | 2026-10-26 |
| License | BSD-3-Clause; see `LICENSE` (copied from the same commit) |

| File here | Path in the source repository |
|---|---|
| `rdap-cases.yaml` | `inc/rdap/cases.yaml` |
| `rdap-errors.yaml` | `inc/rdap/errors.yaml` |
| `rdap-inputs.yaml` | `inc/rdap/inputs.yaml` |

## The RDAP Conformance Tool

RST grades rdap-01 to rdap-04 with ICANN's [RDAP Conformance Tool](https://github.com/icann/rdap-conformance-tool) (`rdapct`). The suite runs the same tool (`../../conformance_test.go`).

| | |
|---|---|
| Tool | `rdapct-3.1.0.jar`, release `v3.1.0`, SHA-256 `e0d0d2002e8d2e933426c25712be5137bc629d3b081e042d6ffdcc42309e1a42` |
| Where it lives | Not in the repository (29 MB). `make rdapct` downloads it into `.cache/rdapct`, checks the checksum, and fetches the IANA datasets it validates against |
| Configuration | `rdapct_config.json` here. It is generated, not copied, from the release's own `rdapct_config.json` by the rules in `generate-rdapct-config.pl` (copied unmodified from `tools/generate-rdapct-config.pl` at the commit above), with its `RSP` usage: RST's Registry Service Provider evaluation |
| Copied | 2026-10-26 |

The generator fetches the tool's template from its default branch; the configuration here was generated from the `v3.1.0` release's template instead, so it matches the pinned tool. To regenerate it, apply the generator's `ERROR`, `WARNING`, `IGNORE` and `IGNORE_RSP` lists to the new release's template, in that order.

## Moving to a newer release

1. Pick the release tag in the source repository, and read its `ChangeLog.yaml` entries for RDAP since `v2026.08`.
2. Copy the files above from that tag, and update the release, commit and date in this file.
3. Run `make rst-rdap`. `TestRST_RDAPCatalogueIsComplete` fails for every case the new release adds or removes. Account for each one in `rst_rdap_test.go`: implement it, or list it in `rstPendingCases` with the phase that delivers it.
4. Changed requirements in an existing case show up as failing subtests, or not at all. So read the ChangeLog: a changed requirement needs a changed test.
5. Regenerate the conformance tool's config from the same release, since its ignore and warning lists change with it. A new tool release is its own bump too: change the version and checksum in `../../conformance_test.go` and the `Makefile`, and regenerate the config from its template.
6. Land the bump as its own PR, so a conformance change is never hidden inside a feature.
