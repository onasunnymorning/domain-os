// Package rdeschema holds the published XML schemas for RDE deposits, the
// registry-interface report/notification documents, and the EPP object
// mappings they compose with — and the runtime engine that enforces the
// deposit schemas on a deposit under validation.
//
// The schemas have two consumers:
//
//   - tests, which prove with xmllint (via Path) that what this service emits
//     is what a real consumer would accept, a claim that is otherwise only an
//     assertion about our own structs; and
//   - the EVE validator, which runs every deposit through Engine so that a
//     PASS means the XML conforms to the pinned schemas and not merely that
//     the elements the Go checks look at are present. See engine.go.
//
// The schemas are embedded in the binary (see embed.go) so they cannot be
// missing from a deployed worker, and the engine resolves them only from that
// embedded copy: nothing is fetched, and nothing the deposit says can select a
// different schema.
//
// They live in one package rather than beside each consumer because both the
// report writer and the deposit sanitizer need overlapping subsets, and a
// second copy of eppcom-1.0.xsd would eventually drift from the first.
package rdeschema

import (
	"path/filepath"
	"runtime"
)

// Wrapper schemas. Each loads a set of namespaces in dependency order, which
// is what lets libxml2 satisfy the imports the published schemas make by
// namespace alone.
const (
	// ReportSchemas covers rdeReport, rdeNotification and iirdea.
	ReportSchemas = "eve-schemas.xsd"
	// DepositSchemas covers a complete RDE deposit and its EPP object mappings.
	// It is the schema set the runtime engine enforces.
	DepositSchemas = "deposit-schemas.xsd"
)

// Path resolves a schema file to an absolute path, so a test in any package can
// reach it without a chain of parent directories that breaks when the caller
// moves. It resolves against this source file's directory and is therefore
// only meaningful when the source tree is present — which is exactly the case
// tests run in. Runtime code must use the embedded copy (Engine) instead.
func Path(name string) string {
	_, self, _, ok := runtime.Caller(0)
	if !ok {
		return name
	}
	return filepath.Join(filepath.Dir(self), "xsd", name)
}
