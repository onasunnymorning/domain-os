package epp

import (
	"bytes"
	"os"
	"os/exec"
	"sync"
	"testing"
)

// eppSchemas is the entry point for the XSDs pinned from ICANN's RST
// release; see testdata/rst/PROVENANCE.md.
const eppSchemas = "testdata/rst/xsd/epp-schemas.xsd"

var xmllint = sync.OnceValues(func() (string, error) { return exec.LookPath("xmllint") })

// requireSchemaValid fails the test unless frame validates against the
// pinned EPP schemas (RST epp-02: EPP_SCHEMA_VALIDATION_ERROR). It uses
// libxml2's xmllint, as escrow validation does (ADR 0012). CI installs it;
// a developer machine without it skips the check and says so, rather than
// skipping the test.
func requireSchemaValid(t *testing.T, frame []byte) {
	t.Helper()
	bin, err := xmllint()
	if err != nil {
		if os.Getenv("CI") != "" {
			t.Fatalf("xmllint is required in CI for EPP schema validation: %v", err)
		}
		t.Logf("xmllint not found; EPP schema validation skipped on this machine")
		return
	}
	cmd := exec.Command(bin, "--noout", "--nonet", "--schema", eppSchemas, "-") // #nosec G204 -- test-only: fixed arguments, binary from PATH
	cmd.Stdin = bytes.NewReader(frame)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("frame does not validate against the EPP schemas: %v\n%s\nframe:\n%s", err, out, frame)
	}
}
