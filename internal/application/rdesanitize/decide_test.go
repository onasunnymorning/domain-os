package rdesanitize

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDecide(t *testing.T) {
	f := func(c Code, s Severity) Finding { return Finding{Code: c, Severity: s} }

	cases := []struct {
		name     string
		findings []Finding
		want     Outcome
	}{
		{"nothing wrong", nil, OutcomePass},
		{"informational only", []Finding{f(CodeFindingsTruncated, SeverityInfo)}, OutcomePass},
		{"an unclassified element", []Finding{f(CodePolicyUnknownElement, SeverityError)}, OutcomeQuarantined},
		{"a DTD", []Finding{f(CodeXMLDTDPresent, SeverityError)}, OutcomeQuarantined},
		{"our key store is down", []Finding{f(CodeTokenKeyUnavailable, SeverityError)}, OutcomeError},
		{"a service failure outranks a source failure", []Finding{
			f(CodePolicyUnknownElement, SeverityError), f(CodeInternal, SeverityError),
		}, OutcomeError},
		{"a timeout is ours, not the deposit's", []Finding{f(CodeSanitizeTimeout, SeverityError)}, OutcomeError},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, Decide(tc.findings))
		})
	}
}

func TestResult_FindingsAreCapped(t *testing.T) {
	var r Result
	for i := 0; i < MaxFindings+50; i++ {
		r.Add(Finding{Code: CodePolicyUnknownElement, Severity: SeverityError, Stage: StageRewrite})
	}
	assert.Len(t, r.Findings, MaxFindingsPerKind+1, "examples of the one kind, plus the truncation notice")
	assert.Equal(t, MaxFindings+50-MaxFindingsPerKind, r.Suppressed)
	assert.True(t, r.Has(CodeFindingsTruncated))
	assert.Equal(t, OutcomeQuarantined, Decide(r.Findings))
}

// TestResult_RetainsAnExampleOfEveryKind is rdevalidate's guarantee in this
// package: a source that trips one policy finding on every element must not
// crowd the others out of the record it is quarantined on.
func TestResult_RetainsAnExampleOfEveryKind(t *testing.T) {
	var r Result
	for i := 0; i < 40000; i++ {
		r.Add(Finding{Code: CodePolicyUnknownElement, Severity: SeverityError, Stage: StageRewrite})
	}
	r.Add(Finding{Code: CodeDerivativePIIDetected, Severity: SeverityError, Stage: StageVerify})
	r.Decide()

	kinds := map[Code]int{}
	for _, f := range r.Findings {
		kinds[f.Code]++
	}
	assert.Equal(t, MaxFindingsPerKind, kinds[CodePolicyUnknownElement])
	assert.Equal(t, 1, kinds[CodeDerivativePIIDetected], "the last finding is still in the record")
}
