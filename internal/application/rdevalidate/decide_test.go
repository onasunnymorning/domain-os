package rdevalidate

import (
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
	require.Len(t, r.Findings, MaxFindings+1)
	require.Equal(t, CodeFindingsTruncated, r.Findings[MaxFindings].Code)
	require.Equal(t, OutcomeFail, r.Outcome)
	require.True(t, r.Has(CodeFindingsTruncated))
}
