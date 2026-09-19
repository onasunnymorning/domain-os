// Package dataqa is the QA report every staged-data pipeline produces before it writes to the live
// system (.agents/skills/data-pipeline-qa, INV-05). The report's Passed field is the gate: a pipeline
// reads it and halts on false.
//
// It lives below both internal/application/activities and internal/application/services so that a
// pipeline in either can produce the same schema. It is pure: no database, no object storage, no
// network, so a QA step built on it is safe to re-run at any time.
package dataqa

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
)

// SchemaVersion is the report schema version (QA Report Schema v1.0).
const SchemaVersion = "1.0"

// MaxSampledItems caps QACheck.SampledItems: the report is for triage, not replay.
const MaxSampledItems = 50

// Severity levels. Only an error-severity failure flips the gate.
const (
	SeverityError   = "error"
	SeverityWarning = "warning"
	SeverityInfo    = "info"
)

// QACheck represents a single quality check result.
type QACheck struct {
	Rule          string      `json:"rule"`
	Description   string      `json:"description"`
	Severity      string      `json:"severity"` // "error", "warning", "info"
	Passed        bool        `json:"passed"`
	AffectedCount int         `json:"affectedCount"`
	Message       string      `json:"message"`
	Detail        interface{} `json:"detail,omitempty"`
	SampledItems  interface{} `json:"sampledItems,omitempty"`
}

// QAReport is the structured QA report for a staged dataset.
type QAReport struct {
	Version   string            `json:"version"`
	Timestamp time.Time         `json:"timestamp"`
	Pipeline  string            `json:"pipeline"`
	Context   map[string]string `json:"context"`
	SourceKey string            `json:"sourceKey"`
	Passed    bool              `json:"passed"`
	Summary   map[string]int64  `json:"summary"`
	Checks    []QACheck         `json:"checks"`
}

// NewQAReport creates a report with Passed defaulting to true. It flips to false when an
// error-severity check fails.
func NewQAReport(pipeline, sourceKey string, ctx map[string]string) *QAReport {
	return &QAReport{
		Version:   SchemaVersion,
		Timestamp: time.Now().UTC(),
		Pipeline:  pipeline,
		Context:   ctx,
		SourceKey: sourceKey,
		Passed:    true,
		Summary:   make(map[string]int64),
		Checks:    []QACheck{},
	}
}

// AddCheck adds a check to the report and flips the gate on an error-severity failure.
func (r *QAReport) AddCheck(check QACheck) {
	r.Checks = append(r.Checks, check)
	if !check.Passed && check.Severity == SeverityError {
		r.Passed = false
	}
}

// Failed returns the error-severity checks that did not pass, which are the reasons the gate is closed.
func (r *QAReport) Failed() []QACheck {
	var failed []QACheck
	for _, c := range r.Checks {
		if !c.Passed && c.Severity == SeverityError {
			failed = append(failed, c)
		}
	}
	return failed
}

// WriteFile writes the report as indented JSON to path.
func (r *QAReport) WriteFile(path string) error {
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal qa report: %w", err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("write qa report: %w", err)
	}
	return nil
}
