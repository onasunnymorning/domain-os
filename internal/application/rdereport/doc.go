// Package rdereport is the narrow report adapter for issue #412: it turns a
// rdevalidate.Result into the ICANN `rdeReport:report` and
// `rdeNotification:notification` documents defined by
// draft-lozano-icann-registry-interfaces-26 (SpecVersion).
//
// Everything schema- and version-specific lives here so a draft bump changes
// this package and its XSD test data only; the validation semantics in
// rdevalidate are untouched by it. Conformance is proven in xsd_test.go by
// validating generated documents against the published XSDs with xmllint.
package rdereport
