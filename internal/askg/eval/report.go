package eval

import (
	"encoding/json"
	"fmt"
	"strings"
)

// ---------------------------------------------------------------------------
// Confusion matrix
// ---------------------------------------------------------------------------

// ConfusionMatrix tracks predicted vs expected outcomes for the 3-class
// outcome discriminator. Rows are expected, columns are predicted.
type ConfusionMatrix struct {
	// Cells[expected][predicted] = count
	Cells map[string]map[string]int
}

// NewConfusionMatrix creates a zero-initialized 3×3 confusion matrix.
func NewConfusionMatrix() *ConfusionMatrix {
	outcomes := []string{"answer", "escalate", "action_required"}
	cells := make(map[string]map[string]int, len(outcomes))
	for _, e := range outcomes {
		cells[e] = make(map[string]int, len(outcomes))
		for _, p := range outcomes {
			cells[e][p] = 0
		}
	}
	return &ConfusionMatrix{Cells: cells}
}

// Record adds one observation to the matrix.
func (cm *ConfusionMatrix) Record(expected, predicted string) {
	if _, ok := cm.Cells[expected]; !ok {
		cm.Cells[expected] = make(map[string]int)
	}
	cm.Cells[expected][predicted]++
}

// ---------------------------------------------------------------------------
// Suite report
// ---------------------------------------------------------------------------

// SuiteReport is the aggregate output of an eval suite run.
type SuiteReport struct {
	// CaseResults holds per-case aggregated results.
	CaseResults []CaseResult `json:"case_results"`

	// ConfusionMatrix tracks outcome classification across all runs.
	Confusion *ConfusionMatrix `json:"confusion_matrix"`

	// CategoryPassRates maps category → aggregate pass rate.
	CategoryPassRates map[string]float64 `json:"category_pass_rates"`

	// HardGateFailures counts zero-tolerance failures:
	// tenant isolation leaks, action gate violations, adversarial compliance.
	HardGateFailures int `json:"hard_gate_failures"`

	// TotalRuns is the total number of individual runs across all cases.
	TotalRuns int `json:"total_runs"`

	// TotalPasses is the total number of passing runs.
	TotalPasses int `json:"total_passes"`
}

// JSON returns the report as indented JSON.
func (r *SuiteReport) JSON() ([]byte, error) {
	return json.MarshalIndent(r, "", "  ")
}

// String returns a human-readable table output for terminal display.
func (r *SuiteReport) String() string {
	var b strings.Builder

	b.WriteString("\n")
	b.WriteString("═══════════════════════════════════════════════════════════\n")
	b.WriteString("                  ASK G EVAL REPORT                       \n")
	b.WriteString("═══════════════════════════════════════════════════════════\n")
	b.WriteString("\n")

	// Summary
	overallRate := float64(0)
	if r.TotalRuns > 0 {
		overallRate = float64(r.TotalPasses) / float64(r.TotalRuns)
	}
	fmt.Fprintf(&b, "  Total runs:          %d\n", r.TotalRuns)
	fmt.Fprintf(&b, "  Total passes:        %d\n", r.TotalPasses)
	fmt.Fprintf(&b, "  Overall pass rate:   %.1f%%\n", overallRate*100)
	fmt.Fprintf(&b, "  Hard gate failures:  %d\n", r.HardGateFailures)
	b.WriteString("\n")

	// Category pass rates
	b.WriteString("── Category Pass Rates ─────────────────────────────────\n")
	categories := []string{
		string(CategoryAnswer),
		string(CategoryMustEscalate),
		string(CategoryActionRequired),
		string(CategoryTenantIsolation),
		string(CategoryAdversarial),
	}
	for _, cat := range categories {
		rate, ok := r.CategoryPassRates[cat]
		if !ok {
			continue
		}
		fmt.Fprintf(&b, "  %-20s %5.1f%%\n", cat, rate*100)
	}
	b.WriteString("\n")

	// Confusion matrix
	b.WriteString("── Outcome Confusion Matrix (expected × predicted) ─────\n")
	outcomes := []string{"answer", "escalate", "action_required"}
	// Header
	fmt.Fprintf(&b, "  %-18s", "expected\\predicted")
	for _, p := range outcomes {
		fmt.Fprintf(&b, " %8s", abbrev(p))
	}
	b.WriteString("\n")
	// Rows
	for _, e := range outcomes {
		fmt.Fprintf(&b, "  %-18s", abbrev(e))
		for _, p := range outcomes {
			count := 0
			if row, ok := r.Confusion.Cells[e]; ok {
				count = row[p]
			}
			fmt.Fprintf(&b, " %8d", count)
		}
		b.WriteString("\n")
	}
	b.WriteString("\n")

	// Per-case results
	b.WriteString("── Per-Case Results ────────────────────────────────────\n")
	for _, cr := range r.CaseResults {
		status := "✓"
		if cr.PassRate < 1.0 {
			status = "✗"
		}
		fmt.Fprintf(&b, "  %s %-28s [%-17s] %d/%d (%.0f%%)\n",
			status,
			cr.CaseID,
			cr.Category,
			int(cr.PassRate*float64(len(cr.Runs))),
			len(cr.Runs),
			cr.PassRate*100,
		)
		// Show per-gate pass rates if any failures
		if cr.PassRate < 1.0 {
			for gate, rate := range cr.GatePassRates {
				if rate < 1.0 {
					fmt.Fprintf(&b, "      └─ %-20s %.0f%%\n", gate, rate*100)
				}
			}
		}
	}

	b.WriteString("\n")
	b.WriteString("═══════════════════════════════════════════════════════════\n")

	return b.String()
}

// abbrev shortens outcome names for the confusion matrix columns.
func abbrev(outcome string) string {
	switch outcome {
	case "answer":
		return "ans"
	case "escalate":
		return "esc"
	case "action_required":
		return "act_req"
	default:
		return outcome
	}
}

// BuildSuiteReport aggregates CaseResults into a SuiteReport.
func BuildSuiteReport(results []CaseResult) *SuiteReport {
	report := &SuiteReport{
		CaseResults:       results,
		Confusion:         NewConfusionMatrix(),
		CategoryPassRates: make(map[string]float64),
	}

	// Category accumulators
	catTotal := make(map[string]int)
	catPass := make(map[string]int)

	for _, cr := range results {
		cat := string(cr.Category)
		for _, run := range cr.Runs {
			report.TotalRuns++
			if run.Pass {
				report.TotalPasses++
				catPass[cat]++
			}
			catTotal[cat]++

			// Confusion matrix: record the outcome classification
			expected := string(cr.expectedOutcome())
			predicted := string(run.Result.Outcome)
			report.Confusion.Record(expected, predicted)

			// Count hard gate failures
			for _, g := range run.Gates {
				if !g.Pass && isHardGate(g.Name) {
					report.HardGateFailures++
				}
			}
		}
	}

	// Compute category pass rates
	for cat, total := range catTotal {
		if total > 0 {
			report.CategoryPassRates[cat] = float64(catPass[cat]) / float64(total)
		}
	}

	return report
}

// expectedOutcome extracts the expected outcome from a CaseResult.
// This is set during RunCase.
func (cr *CaseResult) expectedOutcome() string {
	return cr.expectedOutcomeStr
}

// isHardGate returns true for zero-tolerance gates.
func isHardGate(name string) bool {
	switch name {
	case "tenant_isolation", "action_gate", "provenance_integrity":
		return true
	default:
		return false
	}
}
