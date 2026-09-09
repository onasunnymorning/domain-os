// Package rdeschema holds the published XML schemas for RDE deposits, the
// registry-interface report/notification documents, and the EPP object
// mappings they compose with.
//
// The schemas are test data: nothing at runtime loads them, and this service
// emits XML from typed structs rather than from a schema. They exist so tests
// can prove with xmllint that what we emit is what a real consumer would
// accept — a claim that is otherwise only an assertion about our own structs.
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
	DepositSchemas = "deposit-schemas.xsd"
)

// Path resolves a schema file to an absolute path, so a test in any package can
// reach it without a chain of parent directories that breaks when the caller
// moves. It resolves against this source file's directory and is therefore
// only meaningful when the source tree is present — which is exactly the case
// tests run in.
func Path(name string) string {
	_, self, _, ok := runtime.Caller(0)
	if !ok {
		return name
	}
	return filepath.Join(filepath.Dir(self), "xsd", name)
}
