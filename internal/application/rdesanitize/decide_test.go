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
	assert.Len(t, r.Findings, MaxFindings+1, "the cap plus one truncation notice")
	assert.True(t, r.Has(CodeFindingsTruncated))
	assert.Equal(t, OutcomeQuarantined, Decide(r.Findings))
}
