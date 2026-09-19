package dataqa

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestQAReport_OnlyAnErrorSeverityFailureClosesTheGate(t *testing.T) {
	r := NewQAReport("test-pipeline", "src", nil)
	require.True(t, r.Passed, "a report with no checks is open")

	r.AddCheck(QACheck{Rule: "a_warning", Severity: SeverityWarning, Passed: false})
	r.AddCheck(QACheck{Rule: "an_info", Severity: SeverityInfo, Passed: false})
	r.AddCheck(QACheck{Rule: "a_passing_error", Severity: SeverityError, Passed: true})
	require.True(t, r.Passed, "warnings, info and passing checks must not close the gate")
	require.Empty(t, r.Failed())

	r.AddCheck(QACheck{Rule: "a_failing_error", Severity: SeverityError, Passed: false})
	require.False(t, r.Passed)
	require.Len(t, r.Failed(), 1)
	require.Equal(t, "a_failing_error", r.Failed()[0].Rule)

	// Once closed, a later passing check does not reopen it.
	r.AddCheck(QACheck{Rule: "later_ok", Severity: SeverityError, Passed: true})
	require.False(t, r.Passed)
}

// The field names are a project-wide contract (data-pipeline-qa, schema v1.0) that alerting and the
// escrow import's own report already depend on.
func TestQAReport_SerialisesTheV1Schema(t *testing.T) {
	r := NewQAReport("jisc-direct-import", "/tmp/in.json", map[string]string{"tld": "ac.uk"})
	r.Summary["domains"] = 3
	r.AddCheck(QACheck{Rule: "r", Description: "d", Severity: SeverityError, Passed: false, AffectedCount: 2, Message: "m", SampledItems: []string{"x"}})

	path := filepath.Join(t.TempDir(), "qa-report.json")
	require.NoError(t, r.WriteFile(path))

	raw, err := os.ReadFile(path)
	require.NoError(t, err)
	var got map[string]any
	require.NoError(t, json.Unmarshal(raw, &got))
	for _, k := range []string{"version", "timestamp", "pipeline", "context", "sourceKey", "passed", "summary", "checks"} {
		require.Contains(t, got, k)
	}
	require.Equal(t, "1.0", got["version"])
	require.Equal(t, false, got["passed"])

	check := got["checks"].([]any)[0].(map[string]any)
	for _, k := range []string{"rule", "description", "severity", "passed", "affectedCount", "message", "sampledItems"} {
		require.Contains(t, check, k)
	}
}
