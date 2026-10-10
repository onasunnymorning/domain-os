package rdeschema

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The worker image is the only place the schema check runs for real, and its
// one external dependency is xmllint. A Dockerfile that stopped installing it
// would still build, still start, and then record ERROR on every deposit; this
// pins the install and the build-time proof that it runs.
func TestWorkerImageInstallsAndProvesXMLLint(t *testing.T) {
	_, self, _, ok := runtime.Caller(0)
	require.True(t, ok)
	root := filepath.Join(filepath.Dir(self), "..", "..", "..")
	raw, err := os.ReadFile(filepath.Join(root, "cmd", "workers", "unified", "Dockerfile"))
	require.NoError(t, err)
	df := string(raw)

	runStage := df[strings.LastIndex(df, "\nFROM "):]
	assert.Contains(t, runStage, "apk add --no-cache libxml2-utils", "the runtime stage must install xmllint")
	assert.Contains(t, runStage, "xmllint --version", "the build must fail if xmllint is not runnable")
	assert.Contains(t, df, "CGO_ENABLED=0", "the engine drives a process precisely because the worker is a static binary; adding cgo would change that")
}

// Every namespace the embedded set defines gets a display prefix, and the set
// is the one the wrapper schema loads — a schema added to xsd/ but not to the
// wrapper would be embedded and never enforced.
func TestDepositWrapperImportsEveryDepositSchema(t *testing.T) {
	wrapper, err := schemaFS.ReadFile("xsd/" + DepositSchemas)
	require.NoError(t, err)
	reportOnly := map[string]bool{
		ReportSchemas: true, DepositSchemas: true,
		"rdeReport-1.0.xsd": true, "rdeNotification-1.0.xsd": true, "iirdea-1.0.xsd": true,
	}
	entries, err := schemaFS.ReadDir("xsd")
	require.NoError(t, err)
	for _, e := range entries {
		if reportOnly[e.Name()] {
			continue
		}
		assert.Contains(t, string(wrapper), `schemaLocation="`+e.Name()+`"`, "%s is embedded but not imported by %s", e.Name(), DepositSchemas)
	}
}
