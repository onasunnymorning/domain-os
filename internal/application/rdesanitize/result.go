package rdesanitize

import (
	"sort"
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

// FindingTally counts every finding of one kind the run produced, including
// the ones the MaxFindings cap kept out of Findings, and is what the outcome
// is decided from — see finish.
type FindingTally struct {
	Code     Code     `json:"code"`
	Severity Severity `json:"severity"`
	Stage    Stage    `json:"stage"`
	Count    int      `json:"count"`
}

type tallyKey struct {
	code     Code
	severity Severity
	stage    Stage
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

	// Tally counts every finding by kind, including those Findings was too
	// small to hold, and survives the activity boundary where the in-progress
	// map below does not.
	Tally      []FindingTally `json:"tally,omitempty"`
	Suppressed int            `json:"suppressed,omitempty"`

	tally map[tallyKey]int
}

// Add records a finding, capping the retained list and recording the overflow
// once. Every finding is counted in the tally whether or not it is retained.
func (r *Result) Add(f Finding) {
	if r.tally == nil {
		r.tally = make(map[tallyKey]int)
	}
	r.tally[tallyKey{f.Code, f.Severity, f.Stage}]++
	if len(r.Findings) >= MaxFindings {
		if r.Suppressed == 0 {
			r.Findings = append(r.Findings, Finding{
				Code: CodeFindingsTruncated, Severity: SeverityInfo, Stage: f.Stage,
				Message: "further findings suppressed after " + strconv.Itoa(MaxFindings), At: f.At,
			})
		}
		r.Suppressed++
		return
	}
	r.Findings = append(r.Findings, f)
}

// Has reports whether the result carries a finding with this code. The tally
// is consulted first so a suppressed finding is still found.
func (r *Result) Has(c Code) bool {
	for k, n := range r.tally {
		if k.code == c && n > 0 {
			return true
		}
	}
	for _, e := range r.Tally {
		if e.Code == c && e.Count > 0 {
			return true
		}
	}
	for _, f := range r.Findings {
		if f.Code == c {
			return true
		}
	}
	return false
}

// Codes returns the distinct codes, including any that appear only among the
// suppressed findings.
func (r *Result) Codes() []Code {
	seen := map[Code]bool{}
	out := []Code{}
	add := func(c Code) {
		if !seen[c] {
			seen[c] = true
			out = append(out, c)
		}
	}
	for _, f := range r.Findings {
		add(f.Code)
	}
	for _, e := range r.Tally {
		if e.Count > 0 {
			add(e.Code)
		}
	}
	for k, n := range r.tally {
		if n > 0 {
			add(k.code)
		}
	}
	return out
}

// materialiseTally orders the tally by stage, severity then code so goldens
// and diffs are stable.
func (r *Result) materialiseTally() []FindingTally {
	out := make([]FindingTally, 0, len(r.tally))
	for k, n := range r.tally {
		out = append(out, FindingTally{Code: k.code, Severity: k.severity, Stage: k.stage, Count: n})
	}
	sort.Slice(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.Stage != b.Stage {
			return a.Stage < b.Stage
		}
		if a.Severity != b.Severity {
			return a.Severity < b.Severity
		}
		return a.Code < b.Code
	})
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

// finish stamps the completion time and decides the outcome from the tally.
// Deciding from Findings would let an error-class code that arrived after
// MaxFindings be lost, downgrading an ERROR to a QUARANTINED.
func (r *Result) finish(now time.Time) {
	r.CompletedAt = now
	r.Decide()
}

// Decide materialises the tally and sets Outcome from it. Call it on any
// Result assembled outside the rewriter, so that such a Result reaches its
// outcome by the same rule and carries the same tally as a real run.
func (r *Result) Decide() {
	r.Tally = r.materialiseTally()
	r.Outcome = DecideTally(r.Tally)
}

// DecideTally applies Decide's rule to a tally, which counts the findings
// Findings could not hold.
func DecideTally(tally []FindingTally) Outcome {
	quarantine := false
	for _, e := range tally {
		if e.Count == 0 {
			continue
		}
		if e.Code.IsErrorClass() {
			return OutcomeError
		}
		if e.Severity == SeverityError {
			quarantine = true
		}
	}
	if quarantine {
		return OutcomeQuarantined
	}
	return OutcomePass
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
