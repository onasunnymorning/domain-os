package rdevalidate

import (
	"sort"
	"time"

	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
)

// Profiles this package can validate. Only ProfileRydeSig can yield a
// cryptographically verified pass (constraint 5); ProfilePlaintextXML accepts
// an unsigned `.xml` or `.xml.gz` deposit and establishes nothing about its
// origin (issue #415).
const (
	ProfileRydeSig      = entities.EscrowProfileRydeSig
	ProfilePlaintextXML = entities.EscrowProfilePlaintextXML
)

// MaxFindings bounds the findings list so a pathologically broken deposit
// cannot bloat the persisted record or the workflow payload. Counts stay exact.
const MaxFindings = 1000

// MaxFindingsPerRule bounds how many worked examples of one kind of finding
// the list keeps. Without it the cap is spent first-come: a deposit whose
// every domain trips the same warning fills the list with a thousand copies of
// it, and the one ERROR that decides the run — emitted after the per-object
// findings — is not in the record at all. Keeping a few of each kind instead
// costs nothing and is what makes the list worth reading.
const MaxFindingsPerRule = 50

// Outcome is the decision for a run.
type Outcome string

const (
	OutcomePass  Outcome = "PASS"
	OutcomeFail  Outcome = "FAIL"
	OutcomeError Outcome = "ERROR"
)

// Finding is one check result. Message, Locator and Rule must be built from
// constant templates and numbers only (offsets, indices, counts, line
// numbers) — never from object names, file names or payload text. Object is
// the single, deliberate exception; see its doc comment.
type Finding struct {
	Code     Code     `json:"code"`
	Severity Severity `json:"severity"`
	Stage    Stage    `json:"stage"`
	// Rule names the specific check that produced the finding, where the code
	// alone is too coarse to act on. RDE_OBJECT_ENTITY_REJECTED says an object
	// was refused; the rule says which rule refused it, so an operator can fix
	// the deposit instead of guessing. It is part of the tally key, so a run
	// reports how many objects each individual rule rejected.
	Rule       string `json:"rule,omitempty"`
	ObjectType string `json:"objectType,omitempty"`
	// Object is the identifier of the object the finding is about, as the
	// deposit wrote it: a domain or host name, a contact or registrar id. It
	// is the only field built from deposit content, and it is here because
	// Locator is reproducible but not actionable — "domain#2374
	// offset=2820739" identifies the object exactly and tells an operator
	// nothing they can go and fix.
	//
	// Because it is untrusted payload, its blast radius is fenced rather than
	// trusted: it reaches the run record and the operational summary, both
	// tenant-scoped and read behind the API's authorisation, and nothing
	// else. It is deliberately absent from the structured log line (see
	// activities/escrow_validation.go) and from the DVFN sent to ICANN (see
	// rdereport.resultsFromFindings), each of which builds its text from
	// Message and Locator alone. Tests hold both fences.
	Object  string    `json:"object,omitempty"`
	Locator string    `json:"locator,omitempty"`
	Message string    `json:"message"`
	At      time.Time `json:"at"`
}

// FindingTally counts every finding of one kind the run produced, including
// the ones the MaxFindings cap kept out of Findings. It is the only exact
// record of a deposit that is wrong in more than MaxFindings places, and it is
// what the outcome is decided from — see finish.
type FindingTally struct {
	Code       Code     `json:"code"`
	Severity   Severity `json:"severity"`
	Stage      Stage    `json:"stage"`
	ObjectType string   `json:"objectType,omitempty"`
	Rule       string   `json:"rule,omitempty"`
	Count      int      `json:"count"`
}

// tallyKey identifies one kind of finding for counting purposes. The rule and
// the object type are part of it: "56926 objects were rejected" is not
// something an operator can act on, and "43313 hosts, 10613 contacts and 3000
// domains were rejected because their roid is not in this registry's format"
// is the whole answer.
type tallyKey struct {
	code       Code
	severity   Severity
	stage      Stage
	objectType string
	rule       string
}

// MaxTallyRows bounds how many distinct kinds the tally distinguishes. The
// rule of a missing-element finding names the combination of elements that
// were missing, which is combinatorial, so without a ceiling a deliberately
// varied deposit could make the tally the largest thing in the run record and
// in the workflow payload. Past the ceiling a new kind is still counted, under
// its code alone.
const MaxTallyRows = 500

// SignatureInfo describes the verified detached signature.
type SignatureInfo struct {
	KeyFingerprint string    `json:"keyFingerprint"` // primary key fingerprint of the trusted signer, upper hex
	KeyID          string    `json:"keyId"`          // issuer key id as reported by the signature packet, upper hex
	SignedAt       time.Time `json:"signedAt"`
}

// DecryptionInfo describes how the ciphertext was opened.
type DecryptionInfo struct {
	KeyFingerprint    string    `json:"keyFingerprint"` // primary key fingerprint of the service key that decrypted
	EncryptedToKeyIDs []string  `json:"encryptedToKeyIds"`
	InnerSigned       bool      `json:"innerSigned"` // the OpenPGP message carried its own signature (recorded, not trusted)
	LiteralTime       time.Time `json:"literalTime"` // modification time carried by the OpenPGP literal-data packet (zero if absent)
}

// DepositSummary is what the report needs from the deposit itself.
type DepositSummary struct {
	ID          string             `json:"id"`
	PrevID      string             `json:"prevId,omitempty"`
	Kind        string             `json:"kind"` // FULL | DIFF | INCR
	Resend      int                `json:"resend"`
	Watermark   time.Time          `json:"watermark"`
	Header      entities.RDEHeader `json:"header"`
	HeaderFound bool               `json:"headerFound"`
	Observed    map[string]int     `json:"observed"` // object namespace URI -> count actually seen
}

// Digests are the content digests recorded against the run.
type Digests struct {
	ArtifactSHA256  string `json:"artifactSha256"`
	SignatureSHA256 string `json:"signatureSha256,omitempty"`
	// PlaintextSHA256 digests the decrypted payload. For an unsigned profile
	// there is nothing to decrypt, so it equals ArtifactSHA256.
	PlaintextSHA256 string `json:"plaintextSha256,omitempty"`
}

// Result is the complete, serialisable outcome of one pipeline run.
type Result struct {
	Profile      string         `json:"profile"`
	Outcome      Outcome        `json:"outcome"`
	StageReached Stage          `json:"stageReached"`
	Findings     []Finding      `json:"findings"`
	Signature    SignatureInfo  `json:"signature"`
	Decryption   DecryptionInfo `json:"decryption"`
	Deposit      DepositSummary `json:"deposit"`
	Digests      Digests        `json:"digests"`
	Layout       Layout         `json:"layout,omitempty"`
	StartedAt    time.Time      `json:"startedAt"`
	CompletedAt  time.Time      `json:"completedAt"`

	// Tally counts every finding the run produced, including those Findings
	// was too small to hold. Populated by finish, ordered deterministically,
	// and bounded by the number of distinct code/severity/stage triples rather
	// than by the size of the deposit.
	Tally []FindingTally `json:"tally,omitempty"`
	// Suppressed is how many findings the MaxFindings cap kept out of Findings.
	Suppressed int `json:"suppressed,omitempty"`

	// tally accumulates Tally while the run is in progress, and retained
	// counts how many worked examples of each kind Findings already holds.
	// Both are unexported because they do not survive the activity boundary;
	// Tally and Suppressed do.
	tally    map[tallyKey]int
	retained map[tallyKey]int
}

// Add records a finding. Every finding is counted in the tally; Findings keeps
// worked examples only, so an enormous deposit cannot bloat the run record or
// the workflow payload.
//
// Retention is per kind rather than first-come. A kind gets up to
// MaxFindingsPerRule examples until the list reaches MaxFindings, and past
// that a kind seen for the first time still gets one — otherwise a deposit
// whose every object trips the same warning would crowd out every other
// finding it produced, including the ERROR the run is decided on.
func (r *Result) Add(f Finding) {
	if r.tally == nil {
		r.tally = make(map[tallyKey]int)
		r.retained = make(map[tallyKey]int)
	}
	k := tallyKey{f.Code, f.Severity, f.Stage, f.ObjectType, f.Rule}
	if _, known := r.tally[k]; !known && len(r.tally) >= MaxTallyRows {
		k = tallyKey{code: f.Code, severity: f.Severity, stage: f.Stage}
	}
	r.tally[k]++

	quota := MaxFindingsPerRule
	if len(r.Findings) >= MaxFindings {
		// The list is full. Only a kind with nothing to show still gets in,
		// which bounds the overshoot by the number of distinct kinds.
		quota = 1
	}
	if r.retained[k] >= quota {
		r.Suppressed++
		return
	}
	r.retained[k]++
	r.Findings = append(r.Findings, f)
}

// Has reports whether a finding with the code is present. It consults the
// tally first so that a code is still found after MaxFindings has been
// reached: Findings alone would answer no for a suppressed finding.
func (r *Result) Has(code Code) bool {
	for k, n := range r.tally {
		if k.code == code && n > 0 {
			return true
		}
	}
	for _, e := range r.Tally {
		if e.Code == code && e.Count > 0 {
			return true
		}
	}
	// A Result assembled literally, without Add, has neither tally.
	for _, f := range r.Findings {
		if f.Code == code {
			return true
		}
	}
	return false
}

// Codes returns the distinct finding codes. Once the tally exists it is the
// source, so a code that appears only among the suppressed findings is still
// reported; the order is then the tally's rather than first appearance.
func (r *Result) Codes() []Code {
	seen := map[Code]bool{}
	var out []Code
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

// TotalFindings is how many findings the run produced, suppressed ones
// included. It is exact where len(Findings) is capped.
func (r Result) TotalFindings() int {
	n := 0
	for _, e := range r.Tally {
		n += e.Count
	}
	return n
}

// materialiseTally turns the in-progress map into the ordered exported slice.
// The order is stage, then severity, then code, so goldens and diffs are
// stable across runs of the same deposit.
func (r *Result) materialiseTally() []FindingTally {
	out := make([]FindingTally, 0, len(r.tally))
	for k, n := range r.tally {
		out = append(out, FindingTally{
			Code: k.code, Severity: k.severity, Stage: k.stage,
			ObjectType: k.objectType, Rule: k.rule, Count: n,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.Stage != b.Stage {
			return a.Stage < b.Stage
		}
		if a.Severity != b.Severity {
			return a.Severity < b.Severity
		}
		if a.Code != b.Code {
			return a.Code < b.Code
		}
		if a.Rule != b.Rule {
			return a.Rule < b.Rule
		}
		return a.ObjectType < b.ObjectType
	})
	return out
}

// Verified reports whether the run is a cryptographically verified RDE pass.
func (r Result) Verified() bool {
	return r.Outcome == OutcomePass && r.Profile == ProfileRydeSig
}

// finish stamps the completion time, materialises the tally, decides the
// outcome and appends the truncation notice if any findings were suppressed.
//
// The outcome is decided from the tally, never from Findings. Findings stops
// at MaxFindings, and the XML validator appends its ERROR-severity count
// checks after every per-object finding, so a deposit with a thousand warnings
// followed by a count mismatch would otherwise be decided PASS on the strength
// of an error that had been dropped.
func (r *Result) finish(now time.Time) {
	r.Tally = r.materialiseTally()
	r.Outcome = DecideTally(r.Tally)
	if r.Suppressed > 0 {
		r.Findings = append(r.Findings, Finding{
			Code: CodeFindingsTruncated, Severity: SeverityInfo, Stage: r.StageReached,
			Message: "findings list keeps at most " + itoa(MaxFindingsPerRule) + " examples of each finding and " +
				itoa(MaxFindings) + " in total; " + itoa(r.Suppressed) + " further findings were counted in the tally but not listed",
			At: now,
		})
	}
	r.CompletedAt = now
}

// Decide is the pure decision rule (table-tested from testdata/decisions.yaml):
//
//   - any finding whose code is error-class (service could not decide) -> ERROR
//   - otherwise any ERROR-severity finding                              -> FAIL
//   - otherwise                                                          -> PASS
//
// ERROR takes precedence over FAIL so a timeout part-way through a broken
// deposit is not reported as a verified failure of that deposit.
func Decide(findings []Finding) Outcome {
	fail := false
	for _, f := range findings {
		if f.Code.IsErrorClass() {
			return OutcomeError
		}
		if f.Severity == SeverityError {
			fail = true
		}
	}
	if fail {
		return OutcomeFail
	}
	return OutcomePass
}

// DecideTally applies the same rule to a tally. It is what a run is actually
// decided by, because the tally counts findings that Findings could not hold.
func DecideTally(tally []FindingTally) Outcome {
	fail := false
	for _, e := range tally {
		if e.Count == 0 {
			continue
		}
		if e.Code.IsErrorClass() {
			return OutcomeError
		}
		if e.Severity == SeverityError {
			fail = true
		}
	}
	if fail {
		return OutcomeFail
	}
	return OutcomePass
}

// NotificationStatus maps an outcome to the rdeNotification status this
// service emits. ERROR yields none: fail closed, but claim nothing.
func NotificationStatus(o Outcome) entities.EscrowNotificationStatus {
	switch o {
	case OutcomePass:
		return entities.EscrowNotificationDVPN
	case OutcomeFail:
		return entities.EscrowNotificationDVFN
	default:
		return entities.EscrowNotificationNone
	}
}

// ToEntityFindings converts findings to their persisted domain form.
func ToEntityFindings(fs []Finding) []entities.EscrowFinding {
	out := make([]entities.EscrowFinding, len(fs))
	for i, f := range fs {
		out[i] = entities.EscrowFinding{
			Code: string(f.Code), Severity: string(f.Severity), Stage: string(f.Stage), Rule: f.Rule,
			ObjectType: f.ObjectType, Object: f.Object, Locator: f.Locator, Message: f.Message, At: f.At,
		}
	}
	return out
}

// ToEntityTally converts the tally to its persisted domain form.
func ToEntityTally(ts []FindingTally) []entities.EscrowFindingTally {
	out := make([]entities.EscrowFindingTally, len(ts))
	for i, t := range ts {
		out[i] = entities.EscrowFindingTally{
			Code: string(t.Code), Severity: string(t.Severity), Stage: string(t.Stage),
			ObjectType: t.ObjectType, Rule: t.Rule, Count: t.Count,
		}
	}
	return out
}
