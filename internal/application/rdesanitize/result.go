package rdesanitize

import (
	"strconv"
	"time"

	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
)

// MaxFindings bounds the findings list so a source that is wrong in a million
// places cannot bloat the run record or the workflow payload. Counts stay exact.
const MaxFindings = 1000

// Outcome is the decision for one sanitisation run.
type Outcome string

const (
	OutcomePass        Outcome = Outcome(entities.EscrowSanitizationPass)
	OutcomeQuarantined Outcome = Outcome(entities.EscrowSanitizationQuarantined)
	OutcomeError       Outcome = Outcome(entities.EscrowSanitizationError)
)

// Finding is one reason code with its position. Message and Locator must be
// built from constant templates and numbers only — never from element names,
// attribute values or payload text. A finding is copied verbatim into the run
// record and into logs, so anything placed here is published.
type Finding struct {
	Code     Code      `json:"code"`
	Severity Severity  `json:"severity"`
	Stage    Stage     `json:"stage"`
	Locator  string    `json:"locator,omitempty"`
	Message  string    `json:"message"`
	At       time.Time `json:"at"`
}

// Result is the complete, serialisable outcome of one rewrite.
type Result struct {
	Outcome      Outcome                           `json:"outcome"`
	StageReached Stage                             `json:"stageReached"`
	Findings     []Finding                         `json:"findings"`
	Counts       entities.EscrowSanitizationCounts `json:"counts"`
	// SourceElements is the number of elements read from the source, kept for
	// the manifest even when the run is quarantined.
	SourceElements int64     `json:"sourceElements"`
	StartedAt      time.Time `json:"startedAt"`
	CompletedAt    time.Time `json:"completedAt"`

	suppressed int
}

// Add appends a finding, capping the list and recording the overflow once.
func (r *Result) Add(f Finding) {
	if len(r.Findings) >= MaxFindings {
		if r.suppressed == 0 {
			r.Findings = append(r.Findings, Finding{
				Code: CodeFindingsTruncated, Severity: SeverityInfo, Stage: f.Stage,
				Message: "further findings suppressed after " + strconv.Itoa(MaxFindings), At: f.At,
			})
		}
		r.suppressed++
		return
	}
	r.Findings = append(r.Findings, f)
}

// Has reports whether the result carries a finding with this code.
func (r *Result) Has(c Code) bool {
	for _, f := range r.Findings {
		if f.Code == c {
			return true
		}
	}
	return false
}

// Codes returns the distinct codes in order of first appearance.
func (r *Result) Codes() []Code {
	seen := map[Code]bool{}
	out := []Code{}
	for _, f := range r.Findings {
		if !seen[f.Code] {
			seen[f.Code] = true
			out = append(out, f.Code)
		}
	}
	return out
}

// Decide is the pure decision rule. A service-side failure is ERROR; anything
// the source did that we refuse to transform is QUARANTINED; otherwise PASS.
//
// There is no "partial" outcome on purpose: a derivative that dropped an
// unclassified field would be exactly the silent pass-through the profile
// exists to prevent.
func Decide(findings []Finding) Outcome {
	quarantine := false
	for _, f := range findings {
		if f.Code.IsErrorClass() {
			return OutcomeError
		}
		if f.Severity == SeverityError {
			quarantine = true
		}
	}
	if quarantine {
		return OutcomeQuarantined
	}
	return OutcomePass
}

// finish stamps the completion time and decides the outcome.
func (r *Result) finish(now time.Time) {
	r.CompletedAt = now
	r.Outcome = Decide(r.Findings)
}

// ToEntityFindings converts findings to their persisted form.
func ToEntityFindings(fs []Finding) []entities.EscrowFinding {
	out := make([]entities.EscrowFinding, 0, len(fs))
	for _, f := range fs {
		out = append(out, entities.EscrowFinding{
			Code: string(f.Code), Severity: string(f.Severity), Stage: string(f.Stage),
			Locator: f.Locator, Message: f.Message, At: f.At,
		})
	}
	return out
}
