package rdereport

import (
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/onasunnymorning/domain-os/internal/application/rdevalidate"
	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
)

// SummarySchemaVersion is stamped on every summary. It is ours, not ICANN's,
// so it moves whenever the shape below changes in a way a reader would notice.
const SummarySchemaVersion = "escrow-validation-summary-1"

// MaxSummarySampleFindings bounds the worked examples carried in the summary.
// The tally is the complete account; the sample only shows what the findings
// look like, so a handful is enough and keeps the document readable.
const MaxSummarySampleFindings = 50

// Summary is the account of one validation run: what was validated, what was
// decided, and — the part no other artifact carries — every finding, counted
// exactly by code.
//
// It is deliberately not an ICANN document. The rdeReport states counts to a
// registry operator under RFC 9022 and says nothing about findings; the
// rdeNotification is a compliance claim that only a signed deposit can
// support. The summary is an operational artifact, emitted for every decided
// run whatever its profile, and its shape is ours to change.
//
// Redaction: every field here is an identifier, a digest, a timestamp, a
// constant template or a number. Finding messages and locators are built from
// constant templates and numbers only (see rdevalidate.Finding), so the
// summary can be written to storage and read by an operator without exposing
// registrant data. Nothing from the deposit's payload reaches this document.
type Summary struct {
	SchemaVersion string `json:"schemaVersion"`

	// Identity of the run.
	TenantID        string `json:"tenantId"`
	TLD             string `json:"tld"`
	Profile         string `json:"profile"`
	DepositID       string `json:"depositId"`
	ValidationRunID string `json:"validationRunId"`
	// CorrelationID is the Temporal Workflow ID and TraceID the Run ID (INV-16).
	CorrelationID string `json:"correlationId"`
	TraceID       string `json:"traceId,omitempty"`

	// Decision.
	Outcome      string `json:"outcome"`
	StageReached string `json:"stageReached"`
	// Verified is true only for a cryptographically verified pass of a signed
	// deposit. An unsigned profile can pass without ever being verified.
	Verified           bool   `json:"verified"`
	NotificationStatus string `json:"notificationStatus,omitempty"`

	// Timing.
	ReceivedAt  time.Time `json:"receivedAt"`
	StartedAt   time.Time `json:"startedAt"`
	ValidatedAt time.Time `json:"validatedAt"`
	CompletedAt time.Time `json:"completedAt"`

	Deposit   SummaryDeposit    `json:"deposit"`
	Digests   SummaryDigests    `json:"digests"`
	Keys      SummaryKeys       `json:"keys"`
	Findings  SummaryFindings   `json:"findings"`
	Artifacts map[string]string `json:"artifacts,omitempty"`
}

// SummaryDeposit is what the deposit said about itself, and how the declared
// object counts compare with what the validator actually saw.
type SummaryDeposit struct {
	ID          string     `json:"id,omitempty"`
	PrevID      string     `json:"prevId,omitempty"`
	Kind        string     `json:"kind,omitempty"`
	Resend      int        `json:"resend"`
	Watermark   *time.Time `json:"watermark,omitempty"`
	HeaderFound bool       `json:"headerFound"`
	Layout      string     `json:"layout,omitempty"`
	// Counts pairs the header's declared count with the observed count for
	// every object namespace either of them mentions. A mismatch here is the
	// same fact RDE_COUNT_MISMATCH reports, in a form that is easy to scan.
	Counts []SummaryCount `json:"counts"`
}

// SummaryCount is one object namespace's declared and observed totals.
type SummaryCount struct {
	URI      string `json:"uri"`
	Declared *int   `json:"declared,omitempty"`
	Observed int    `json:"observed"`
	Matches  bool   `json:"matches"`
}

// SummaryDigests records what was hashed, never where it was stored.
type SummaryDigests struct {
	ArtifactSHA256  string `json:"artifactSha256,omitempty"`
	SignatureSHA256 string `json:"signatureSha256,omitempty"`
	PlaintextSHA256 string `json:"plaintextSha256,omitempty"`
}

// SummaryKeys records which keys acted, by public fingerprint only.
type SummaryKeys struct {
	SigningFingerprint    string `json:"signingFingerprint,omitempty"`
	DecryptionFingerprint string `json:"decryptionFingerprint,omitempty"`
	InnerSigned           bool   `json:"innerSigned,omitempty"`
}

// SummaryFindings is the exact account of what the run found.
type SummaryFindings struct {
	// Total counts every finding the run produced. Retained is how many the
	// run record actually holds, and Suppressed the difference: on a deposit
	// that is wrong in tens of thousands of places, ByCode below is the only
	// complete record.
	Total      int `json:"total"`
	Retained   int `json:"retained"`
	Suppressed int `json:"suppressed"`

	BySeverity map[string]int   `json:"bySeverity"`
	ByStage    map[string]int   `json:"byStage"`
	ByCode     []SummaryCodeRow `json:"byCode"`
	// Sample holds up to MaxSummarySampleFindings retained findings so a
	// reader can see the shape of a locator without opening the run record.
	Sample []rdevalidate.Finding `json:"sample,omitempty"`
}

// SummaryCodeRow is one reason code with its exact count.
type SummaryCodeRow struct {
	Code     string `json:"code"`
	Severity string `json:"severity"`
	Stage    string `json:"stage"`
	Count    int    `json:"count"`
	// ErrorClass marks a code that means the service could not decide, as
	// opposed to one that means the deposit is wrong.
	ErrorClass bool `json:"errorClass,omitempty"`
}

// SummaryParams are the inputs to BuildSummary: a Params for what the report
// builders already need, plus the identity the run carries around it.
type SummaryParams struct {
	Params

	TenantID        string
	Profile         string
	DepositID       string
	ValidationRunID string
	CorrelationID   string
	TraceID         string

	NotificationStatus string
	// Artifacts locates the sibling documents this run emitted, by label.
	Artifacts map[string]string
}

// BuildSummary assembles the summary from a decided result. Unlike
// BuildNotification it refuses nothing: an ERROR run is exactly the run whose
// summary an operator most needs.
func BuildSummary(p SummaryParams) (*Summary, error) {
	tld, err := entities.NormalizeEscrowTLD(p.BoundTLD)
	if err != nil {
		return nil, fmt.Errorf("rdereport: bound TLD: %w", err)
	}
	res := p.Result

	s := &Summary{
		SchemaVersion:      SummarySchemaVersion,
		TenantID:           p.TenantID,
		TLD:                tld,
		Profile:            p.Profile,
		DepositID:          p.DepositID,
		ValidationRunID:    p.ValidationRunID,
		CorrelationID:      p.CorrelationID,
		TraceID:            p.TraceID,
		Outcome:            string(res.Outcome),
		StageReached:       string(res.StageReached),
		Verified:           res.Verified(),
		NotificationStatus: p.NotificationStatus,
		ReceivedAt:         p.ReceivedAt.UTC(),
		StartedAt:          res.StartedAt.UTC(),
		ValidatedAt:        p.ValidatedAt.UTC(),
		CompletedAt:        res.CompletedAt.UTC(),
		Digests: SummaryDigests{
			ArtifactSHA256:  res.Digests.ArtifactSHA256,
			SignatureSHA256: res.Digests.SignatureSHA256,
			PlaintextSHA256: res.Digests.PlaintextSHA256,
		},
		Keys: SummaryKeys{
			SigningFingerprint:    res.Signature.KeyFingerprint,
			DecryptionFingerprint: res.Decryption.KeyFingerprint,
			InnerSigned:           res.Decryption.InnerSigned,
		},
		Artifacts: p.Artifacts,
	}
	if s.Profile == "" {
		s.Profile = res.Profile
	}
	s.Deposit = summaryDeposit(res, p.Hints)
	s.Findings = summaryFindings(res)
	return s, nil
}

func summaryDeposit(res rdevalidate.Result, hints Hints) SummaryDeposit {
	dep := res.Deposit
	d := SummaryDeposit{
		ID:          dep.ID,
		PrevID:      dep.PrevID,
		Kind:        dep.Kind,
		Resend:      dep.Resend,
		HeaderFound: dep.HeaderFound,
		Layout:      string(res.Layout),
	}
	if d.ID == "" {
		d.ID = hints.DepositID
	}
	if d.Kind == "" {
		d.Kind = hints.Kind
	}
	wm := dep.Watermark
	if wm.IsZero() {
		wm = hints.Watermark
	}
	if !wm.IsZero() {
		u := wm.UTC()
		d.Watermark = &u
	}

	declared := map[string]int{}
	for _, c := range dep.Header.Count {
		declared[c.Uri] = c.ID
	}
	uris := map[string]bool{}
	for uri := range declared {
		uris[uri] = true
	}
	for uri := range dep.Observed {
		uris[uri] = true
	}
	d.Counts = make([]SummaryCount, 0, len(uris))
	for uri := range uris {
		row := SummaryCount{URI: uri, Observed: dep.Observed[uri]}
		if n, ok := declared[uri]; ok {
			row.Declared = &n
			row.Matches = n == row.Observed
		}
		d.Counts = append(d.Counts, row)
	}
	sort.Slice(d.Counts, func(i, j int) bool { return d.Counts[i].URI < d.Counts[j].URI })
	return d
}

func summaryFindings(res rdevalidate.Result) SummaryFindings {
	f := SummaryFindings{
		Retained:   len(res.Findings),
		Suppressed: res.Suppressed,
		BySeverity: map[string]int{},
		ByStage:    map[string]int{},
		ByCode:     make([]SummaryCodeRow, 0, len(res.Tally)),
	}
	for _, e := range res.Tally {
		if e.Count == 0 {
			continue
		}
		f.Total += e.Count
		f.BySeverity[string(e.Severity)] += e.Count
		f.ByStage[string(e.Stage)] += e.Count
		f.ByCode = append(f.ByCode, SummaryCodeRow{
			Code: string(e.Code), Severity: string(e.Severity), Stage: string(e.Stage),
			Count: e.Count, ErrorClass: e.Code.IsErrorClass(),
		})
	}
	// Loudest first: a reader wants the code that fired ten thousand times,
	// not the one that fired once. Ties break on code for stable output.
	sort.SliceStable(f.ByCode, func(i, j int) bool {
		if f.ByCode[i].Count != f.ByCode[j].Count {
			return f.ByCode[i].Count > f.ByCode[j].Count
		}
		return f.ByCode[i].Code < f.ByCode[j].Code
	})

	n := len(res.Findings)
	if n > MaxSummarySampleFindings {
		n = MaxSummarySampleFindings
	}
	if n > 0 {
		f.Sample = append(f.Sample, res.Findings[:n]...)
	}
	return f
}

// Marshal renders the summary as indented JSON. It is read by people as often
// as by machines, so it is not minified.
func (s *Summary) Marshal() ([]byte, error) {
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("rdereport: marshal summary: %w", err)
	}
	return append(b, '\n'), nil
}
