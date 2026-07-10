package eval

import (
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testdataDir returns the absolute path to the testdata directory
// relative to this test file.
func testdataDir() string {
	_, thisFile, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(thisFile), "testdata")
}

// TestEvalFixtureCases_Deterministic validates structural integrity of all
// eval cases in cases.yaml without requiring a live model. It checks:
//   - YAML loads without error
//   - Required fields are present (id, category, question, expect.outcome)
//   - All fixture tool names are recognized by FixtureToolExecutor.Tools()
//   - All expect.tool_calls reference valid tool names
//   - Case IDs are unique
func TestEvalFixtureCases_Deterministic(t *testing.T) {
	casesPath := filepath.Join(testdataDir(), "cases.yaml")
	cases, err := LoadCases(casesPath)
	require.NoError(t, err, "failed to load eval cases from %s", casesPath)
	require.NotEmpty(t, cases, "eval cases file should contain at least one case")

	// Build the set of valid tool names from the fixture executor.
	exec := NewFixtureToolExecutor(nil)
	validTools := make(map[string]bool)
	for _, td := range exec.Tools() {
		validTools[td.Name] = true
	}

	// Track IDs for uniqueness.
	seenIDs := make(map[string]bool, len(cases))

	for _, tc := range cases {
		t.Run(tc.ID, func(t *testing.T) {
			// Required fields.
			assert.NotEmpty(t, tc.ID, "case must have an id")
			assert.NotEmpty(t, tc.Category, "case %s must have a category", tc.ID)
			assert.NotEmpty(t, tc.Question, "case %s must have a question", tc.ID)
			assert.NotEmpty(t, tc.Expect.Outcome, "case %s must have expect.outcome", tc.ID)

			// Unique ID.
			assert.False(t, seenIDs[tc.ID], "duplicate case id: %s", tc.ID)
			seenIDs[tc.ID] = true

			// Valid category.
			validCategories := map[Category]bool{
				CategoryAnswer:          true,
				CategoryMustEscalate:    true,
				CategoryActionRequired:  true,
				CategoryTenantIsolation: true,
				CategoryAdversarial:     true,
			}
			assert.True(t, validCategories[tc.Category],
				"case %s has unrecognized category %q", tc.ID, tc.Category)

			// All fixture tool names must be recognized.
			for i, f := range tc.Fixtures {
				assert.True(t, validTools[f.Tool],
					"case %s: fixture[%d] references unrecognized tool %q (valid: %v)",
					tc.ID, i, f.Tool, validTools)
			}

			// All expected tool_calls must reference valid tool names.
			for i, exp := range tc.Expect.ToolCalls {
				assert.True(t, validTools[exp.Tool],
					"case %s: expect.tool_calls[%d] references unrecognized tool %q (valid: %v)",
					tc.ID, i, exp.Tool, validTools)
			}
		})
	}

	t.Logf("validated %d eval cases across cases.yaml", len(cases))
}
