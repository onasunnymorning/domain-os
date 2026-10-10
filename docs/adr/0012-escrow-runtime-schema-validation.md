# ADR 0012 — Escrow validation enforces the pinned XML schemas at runtime

- **Status:** Accepted
- **Date:** 2026-10-10
- **Deciders:** Platform / Registry
- **Amends:** [ADR-0007](0007-escrow-validation-profile-and-key-boundary.md) (decision 3, and the "Report adapter" note that the XSDs are test data)

## Context

On v0.12.0 two malformed deposits received PASS with zero findings: one with no
`rde:rdeMenu`, one with an unexpected `rde:unexpected` before `rde:watermark`.
Both fail the published deposit XSDs. The Go validator decodes the elements it
has structs for and checks the fields it knows are required; anything outside
that — unknown elements, missing structure, wrong order, wrong types — is
invisible to it. The XSDs under `internal/application/rdeschema` were test data.

A PASS is a compliance claim (a DVPN to ICANN for a signed deposit). It has to
mean the XML conforms, not that the elements we happen to read are present.

## Decision

### 1. Enforce the pinned schema set with libxml2, in streaming mode

Every run pipes the unpacked deposit XML, as it is read, into
`xmllint --stream --schema deposit-schemas.xsd -`. The Go checks and libxml2
consume one stream in parallel through a tee; nothing is written to disk and the
signed pipeline's no-persisted-plaintext contract is unchanged.

Why a subprocess: the worker is a static, cross-compiled `CGO_ENABLED=0` binary
and no pure-Go XSD validator is complete enough to trust. Why xmllint's stream
mode: it validates with a bounded window, so memory is constant in the size of
the deposit. Measured on a 1.17 GB, 1.5 M-domain deposit (macOS, libxml2 2.9.13): xmllint
alone 10.6 s at 2.8 MB peak RSS; the whole `Run` 27 s with the schema check
and 31 s without it (the Go checks are the bottleneck, so running the schema
check beside them costs no measurable wall-clock time).

### 2. The check cannot be skipped, and a broken engine is never a PASS

- `rdevalidate.Input.Schema` is required. No engine, an engine that fails to
  start, one that crashes, exits unexpectedly, is killed, or returns a verdict
  the code does not define → `XML_SCHEMA_ENGINE_UNAVAILABLE`, an error-class
  code → outcome **ERROR**, no notification.
- PASS requires xmllint to exit 0. The text `- validates` is never evidence:
  macOS's libxml2 prints it for a document it then fails to parse.
- The engine proves itself at construction: it validates a known-good and a
  known-bad document. A binary that accepts everything, or a schema set that
  does not compile, is unavailable from the first run.
- The worker image installs `libxml2-utils` and runs `xmllint --version` in the
  build, so an image without it cannot be built. A test pins both lines.
- The schemas are embedded (`go:embed`), so the binary cannot ship without them.

### 3. Schema-invalid is FAIL, with an actionable but value-free finding

`XML_SCHEMA_INVALID` (ERROR severity, stage `xml`, DVFN result code 4404). Each
finding has a class-level **Rule** (`schema: element is not allowed at this
position`, `…a required attribute is missing`, `…value is not valid for its
type`, …), a `line=N` **Locator**, and a **Message** that names what the schema
itself says: the element, and the elements it would have accepted.

libxml2's sentences quote the offending value, so none is passed on. Names reach
Message only when the pinned schemas declare them; a name the deposit invented is
cleaned, bounded and carried in `Object` — the single field the finding model
already reserves for deposit-sourced text, absent from logs and from the DVFN.

Violations are bounded: the engine keeps the first 1000 and stops after 10000,
stating so with `XML_SCHEMA_CHECK_TRUNCATED`.

### 4. Extension policy

The pinned set is the policy. A deposit may use exactly what these schemas
declare:

- the RFC 8909 wrapper and the RFC 9022 object mappings (header, domain, host,
  contact, registrar, IDN table reference, NNDN, EPP parameters, policy);
- the EPP mappings they compose with (`epp`, `eppcom`, `domain`, `host`,
  `contact`), `secDNS-1.1` and `rgp-1.0`;
- optional elements and attributes those schemas allow.

Content from any other namespace is accepted only where a schema's own wildcard
allows it (`epp:extension`, `rgp`'s lax wildcard). `rde:contents` is a
substitution group on an abstract element, so a vendor element there is a FAIL,
not a skip. Adding a supported extension means adding its XSD to `xsd/` and
importing it from `deposit-schemas.xsd`; a test fails if one is embedded and not
imported.

### 5. Deliberately unsupported

- **Document type declarations.** libxml2 resolves a DOCTYPE's external
  entities and subsets — from the local filesystem, and with no option to turn
  that off — so a deposit with one is refused before any byte reaches xmllint
  (`XML_DTD_NOT_SUPPORTED`). RFC 8909 deposits are plain XML documents.
- **Non-UTF-8 documents** (`XML_PROLOG_NOT_SUPPORTED`): the DTD refusal is only
  as good as the ability to read the prolog, so UTF-16/32 signatures and a
  declared non-UTF-8 encoding are refused too. The Go checks already reject
  these.
- **`authInfo` and every other element RFC 9022 does not define.** A deposit
  carrying one now FAILs. See the consequence below.

### 6. Unchanged

Cryptographic verification, the Go content checks, profile distinctions
(`Verified()` is still signed-PASS only), resource limits, cancellation and
timeouts (the process is bound to the run's context), and immutable run history.
The check is independent of registry import and mutation.

## Consequences

- A deposit that the old validator passed may now FAIL. That is the point; it
  will surface deposits that were never conformant.
- **The derivative sanitiser only accepts a PASS source**, and RFC 9022 defines
  no `authInfo`, so a source carrying one is no longer a PASS and cannot be
  sanitised. The sanitiser keeps removing `authInfo` (and every non-standard
  credential block) from whatever it is given — the rewriter and conformance
  tests still drive it with such sources — but real deposits no longer reach it
  that way. Activity-level sanitiser tests use standard-conformant sources only.
- A namespace the pinned schemas do not declare can no longer be a PASS source
  either, so the sanitiser's "unknown namespace" quarantine is unreachable from
  validated input. Its "unknown element" quarantine was reachable: the profile
  had no entry for several elements and attributes the standard defines, so a
  conformant deposit validated and then quarantined. That is fixed in the same
  change — sanitisation profile `rde-baseline-v2`, see the amendment in
  [ADR-0008](0008-escrow-derivative-sanitisation.md) — and a test derives the
  full set from the XSDs so the profile cannot fall behind the schemas again.
  Because tokens are keyed by policy version, v2 derivatives' pseudonyms do not
  join v1's.
- The Go validator still requires an `rdeHeader:tld` for a TLD-bound run, so a
  registrar-escrow header (`registrar`/`ppsp`/`reseller`) FAILs there. The
  sanitisation profile classifies those headers regardless; the validator's
  rule is existing behaviour and is unchanged.
- The worker image gains one package (`libxml2-utils`, ~0.5 MB plus libxml2);
  local `go run` of the worker, and any test that exercises validation, needs
  `xmllint` on PATH.
- The libxml2 version is not pinned by this repository: Alpine's package is. The
  startup log records the version and the schema-set digest.

## Not done

- The schema-set digest is logged at start-up but not yet stamped on each run
  record.
- No Alpine/libxml2 2.13 run was possible from the development machine; the
  message parser degrades to class `other` and the verdict logic to ERROR on
  anything it does not recognise, so a reworded message cannot turn into a PASS,
  but exact wording on 2.13 should be confirmed in the image.
