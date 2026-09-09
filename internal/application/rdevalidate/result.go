package rdevalidate

import (
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

// Outcome is the decision for a run.
type Outcome string

const (
	OutcomePass  Outcome = "PASS"
	OutcomeFail  Outcome = "FAIL"
	OutcomeError Outcome = "ERROR"
)

// Finding is one check result. Message and Locator must be built from
// constant templates and numbers only (offsets, indices, counts, line
// numbers) — never from object names, file names or payload text.
type Finding struct {
	Code       Code      `json:"code"`
	Severity   Severity  `json:"severity"`
	Stage      Stage     `json:"stage"`
	ObjectType string    `json:"objectType,omitempty"`
	Locator    string    `json:"locator,omitempty"`
	Message    string    `json:"message"`
	At         time.Time `json:"at"`
}

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

	suppressed int
}

// Add appends a finding, honouring MaxFindings.
func (r *Result) Add(f Finding) {
	if len(r.Findings) >= MaxFindings {
		r.suppressed++
		return
	}
	r.Findings = append(r.Findings, f)
}

// Has reports whether a finding with the code is present.
func (r *Result) Has(code Code) bool {
	for _, f := range r.Findings {
		if f.Code == code {
			return true
		}
	}
	return false
}

// Codes returns the distinct finding codes in order of first appearance.
func (r *Result) Codes() []Code {
	seen := map[Code]bool{}
	var out []Code
	for _, f := range r.Findings {
		if !seen[f.Code] {
			seen[f.Code] = true
			out = append(out, f.Code)
		}
	}
	return out
}

// Verified reports whether the run is a cryptographically verified RDE pass.
func (r Result) Verified() bool {
	return r.Outcome == OutcomePass && r.Profile == ProfileRydeSig
}

// finish stamps the completion time, appends the truncation notice if any
// findings were suppressed, and decides the outcome.
func (r *Result) finish(now time.Time) {
	if r.suppressed > 0 {
		r.Findings = append(r.Findings, Finding{
			Code: CodeFindingsTruncated, Severity: SeverityInfo, Stage: r.StageReached,
			Message: "findings list truncated at " + itoa(MaxFindings) + "; " + itoa(r.suppressed) + " further findings suppressed",
			At:      now,
		})
	}
	r.CompletedAt = now
	r.Outcome = Decide(r.Findings)
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
			Code: string(f.Code), Severity: string(f.Severity), Stage: string(f.Stage),
			ObjectType: f.ObjectType, Locator: f.Locator, Message: f.Message, At: f.At,
		}
	}
	return out
}
