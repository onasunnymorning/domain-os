package rdevalidate

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestDecide_Table(t *testing.T) {
	raw, err := os.ReadFile("testdata/decisions.yaml")
	require.NoError(t, err)
	var doc struct {
		Cases []struct {
			Name     string `yaml:"name"`
			Findings []struct {
				Code     string `yaml:"code"`
				Severity string `yaml:"severity"`
			} `yaml:"findings"`
			Outcome string `yaml:"outcome"`
		} `yaml:"cases"`
	}
	require.NoError(t, yaml.Unmarshal(raw, &doc))
	require.NotEmpty(t, doc.Cases)
	for _, c := range doc.Cases {
		t.Run(c.Name, func(t *testing.T) {
			var fs []Finding
			for _, f := range c.Findings {
				fs = append(fs, Finding{Code: Code(f.Code), Severity: Severity(f.Severity)})
			}
			require.Equal(t, Outcome(c.Outcome), Decide(fs))
		})
	}
}

func TestResult_AddCapsFindings(t *testing.T) {
	var r Result
	for i := 0; i < MaxFindings+5; i++ {
		r.Add(Finding{Code: CodeRDECountMismatch, Severity: SeverityError})
	}
	r.finish(r.StartedAt)
	// One kind of finding gets MaxFindingsPerRule worked examples however many
	// times it fires, plus the truncation notice.
	require.Len(t, r.Findings, MaxFindingsPerRule+1)
	require.Equal(t, CodeFindingsTruncated, r.Findings[MaxFindingsPerRule].Code)
	require.Equal(t, MaxFindings+5-MaxFindingsPerRule, r.Suppressed)
	require.Equal(t, MaxFindings+5, r.TotalFindings())
	require.Equal(t, OutcomeFail, r.Outcome)
	require.True(t, r.Has(CodeFindingsTruncated))
}

// TestResult_RetainsAnExampleOfEveryKind pins the property that makes the
// findings list worth reading: a deposit that trips one warning on every one
// of its objects must not crowd the other findings out of the record. Retaining
// the first MaxFindings did exactly that.
func TestResult_RetainsAnExampleOfEveryKind(t *testing.T) {
	var r Result
	r.StageReached = StageRDE
	// Far more noise than the whole list can hold, under two rules of the
	// same code — the shape a real deposit takes.
	for i := 0; i < 30000; i++ {
		r.Add(Finding{Code: CodeRDEObjectEntityRejected, Severity: SeverityWarning, Stage: StageRDE, Rule: "roid"})
	}
	for i := 0; i < 30000; i++ {
		r.Add(Finding{Code: CodeRDEObjectEntityRejected, Severity: SeverityWarning, Stage: StageRDE, Rule: "email"})
	}
	// Then the quiet ERROR the run is actually decided on.
	r.Add(Finding{Code: CodeRDECountMismatch, Severity: SeverityError, Stage: StageRDE, Rule: "domain"})
	r.finish(r.StartedAt)

	require.Equal(t, OutcomeFail, r.Outcome)
	byRule := map[string]int{}
	for _, f := range r.Findings {
		byRule[string(f.Code)+"/"+f.Rule]++
	}
	require.Equal(t, MaxFindingsPerRule, byRule["RDE_OBJECT_ENTITY_REJECTED/roid"])
	require.Equal(t, MaxFindingsPerRule, byRule["RDE_OBJECT_ENTITY_REJECTED/email"])
	require.Equal(t, 1, byRule["RDE_COUNT_MISMATCH/domain"], "the deciding finding must be in the record, not only in the tally")

	// The two rules are counted apart, so the report can say which one rejected
	// what: a single RDE_OBJECT_ENTITY_REJECTED row of 60000 would not.
	counts := map[string]int{}
	for _, e := range r.Tally {
		counts[string(e.Code)+"/"+e.Rule] = e.Count
	}
	require.Equal(t, 30000, counts["RDE_OBJECT_ENTITY_REJECTED/roid"])
	require.Equal(t, 30000, counts["RDE_OBJECT_ENTITY_REJECTED/email"])
	require.Equal(t, 1, counts["RDE_COUNT_MISMATCH/domain"])
}

// TestResult_DecidesFromTallyNotTruncatedFindings pins the shape of a real
// deposit: the XML validator emits one warning per rejected object and appends
// its ERROR-severity count checks last, so on a deposit with more than
// MaxFindings rejected objects the deciding finding is the one that gets
// suppressed. Deciding from Findings reported PASS on such a deposit.
func TestResult_DecidesFromTallyNotTruncatedFindings(t *testing.T) {
	var r Result
	r.StageReached = StageRDE
	for i := 0; i < MaxFindings+50; i++ {
		r.Add(Finding{Code: CodeRDEObjectEntityRejected, Severity: SeverityWarning, Stage: StageRDE})
	}
	r.Add(Finding{Code: CodeRDECountMismatch, Severity: SeverityError, Stage: StageRDE})
	r.finish(r.StartedAt)

	require.Equal(t, OutcomeFail, r.Outcome, "the count mismatch must decide the run")
	require.Len(t, r.Findings, MaxFindingsPerRule+2, "examples of each kind, plus the truncation notice")
	require.Equal(t, MaxFindings+50-MaxFindingsPerRule, r.Suppressed)
	require.Equal(t, MaxFindings+51, r.TotalFindings())

	// The tally carries the exact counts the truncated list cannot.
	counts := map[Code]int{}
	for _, e := range r.Tally {
		counts[e.Code] = e.Count
	}
	require.Equal(t, MaxFindings+50, counts[CodeRDEObjectEntityRejected])
	require.Equal(t, 1, counts[CodeRDECountMismatch])

	// Both codes are still discoverable even though one was never retained.
	require.True(t, r.Has(CodeRDECountMismatch))
	require.Contains(t, r.Codes(), CodeRDECountMismatch)
}

// TestResult_TallySurvivesTheActivityBoundary guards the field being made
// exported: the tally is computed in one activity and decided on, reported and
// rendered in others, so it has to round-trip as JSON.
func TestResult_TallySurvivesTheActivityBoundary(t *testing.T) {
	var r Result
	r.StageReached = StageRDE
	for i := 0; i < MaxFindings+3; i++ {
		r.Add(Finding{Code: CodeRDEObjectEntityRejected, Severity: SeverityWarning, Stage: StageRDE})
	}
	r.finish(r.StartedAt)

	b, err := json.Marshal(r)
	require.NoError(t, err)
	var got Result
	require.NoError(t, json.Unmarshal(b, &got))

	require.Equal(t, r.Tally, got.Tally)
	require.Equal(t, r.Suppressed, got.Suppressed)
	require.Equal(t, r.TotalFindings(), got.TotalFindings())
	require.True(t, got.Has(CodeRDEObjectEntityRejected))
	require.Equal(t, r.Outcome, DecideTally(got.Tally))
}
