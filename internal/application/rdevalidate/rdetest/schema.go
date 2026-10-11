package rdetest

import (
	"os/exec"
	"sync"
	"testing"

	"github.com/onasunnymorning/domain-os/internal/application/rdeschema"
)

var (
	schemaOnce   sync.Once
	schemaEngine *rdeschema.XMLLint
)

// SchemaEngine returns the real schema engine — the embedded schemas run
// through the installed xmllint — shared by every test in the process.
//
// A test that exercises validation exercises the same enforcement a worker
// runs; there is no fake that accepts everything. Where xmllint is missing the
// test fails rather than skips — the engine is not optional in production, so a
// suite that can pass without it would be testing a product that does not
// exist. (It is also why this does not branch on CI: the environment is read
// only at the composition root, INV-17, and a helper that skipped locally would
// let a missing install go unnoticed until CI.)
func SchemaEngine(t testing.TB) *rdeschema.XMLLint {
	t.Helper()
	if _, err := exec.LookPath("xmllint"); err != nil {
		t.Fatal("xmllint is required to test validation (apt-get install libxml2-utils, or apk add libxml2-utils); schema enforcement cannot be skipped")
	}
	schemaOnce.Do(func() { schemaEngine = rdeschema.NewXMLLint(rdeschema.Config{}) })
	if err := schemaEngine.Err(); err != nil {
		t.Fatalf("rdetest: the schema engine is unavailable: %v", err)
	}
	return schemaEngine
}
