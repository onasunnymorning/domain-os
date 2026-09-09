package rdevalidate

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"hash"
	"io"
	"time"

	"github.com/ProtonMail/go-crypto/openpgp"
)

// Input is everything one run needs. Nothing here touches disk: the artifact
// is opened as a stream (twice — see openDeposit) and the payload is consumed
// as it is produced.
type Input struct {
	// Profile selects the artifact shape. Empty means ProfileRydeSig.
	Profile string
	// OpenArtifact opens the immutable deposit artifact for reading. It is
	// called once to digest it (and, for a signed profile, verify the detached
	// signature) and once more to process it.
	OpenArtifact func(ctx context.Context) (io.ReadCloser, error)
	// Sig is the detached signature (binary or armored). Signatures are small.
	// It must be empty for an unsigned profile.
	Sig []byte
	// ExpectedArtifactSHA256 / ExpectedSignatureSHA256, when set, are re-checked
	// against what is actually read so a swapped artifact cannot be validated
	// under another set's record.
	ExpectedArtifactSHA256  string
	ExpectedSignatureSHA256 string

	TrustedKeys []string           // armored public keys active for the tenant/TLD
	ServiceKeys openpgp.EntityList // unlocked private keys, most recent first

	BoundTLD  string
	Limits    Limits
	Now       func() time.Time
	Heartbeat func(stage Stage, detail string)
}

func (in Input) profile() string {
	if in.Profile == "" {
		return ProfileRydeSig
	}
	return in.Profile
}

// openState holds what a caller must drain after it has finished reading the
// XML entry: the payload stream, the shared byte budget and the payload digest.
type openState struct {
	payload io.Reader // the (tee'd) payload stream feeding the unpacker
	budget  *budget
	hash    hash.Hash
	closers []io.Closer
}

// Close releases the artifact stream. Callers must call it once they have
// finished with the XML entry.
func (s *openState) Close() {
	for _, c := range s.closers {
		_ = c.Close()
	}
}

// finish drains whatever the XML reader did not consume — the rest of the
// archive (a second payload or an unsafe trailing entry) and the rest of the
// OpenPGP body, so the integrity check (MDC) actually runs — and records the
// payload digest.
func (s *openState) finish(res *Result, entry *XMLEntry, now func() time.Time) {
	if f := entry.Drain(); f != nil && !res.Has(f.Code) {
		res.Add(*f)
	}
	if _, err := io.Copy(io.Discard, &budgetReader{r: s.payload, b: s.budget}); err != nil {
		switch {
		case errors.Is(err, errBudgetExceeded):
			if !res.Has(CodeArchiveLimitUnpackedSize) {
				res.Add(Finding{Code: CodeArchiveLimitUnpackedSize, Severity: SeverityError, Stage: StageUnpack, ObjectType: "archive", Message: "unpacked size limit exceeded", At: now()})
			}
		case IsIntegrityError(err):
			res.Add(Finding{Code: CodeDecryptFailed, Severity: SeverityError, Stage: StageDecrypt, ObjectType: "deposit", Message: "deposit integrity check failed: ciphertext was altered", At: now()})
		default:
			res.Add(Finding{Code: CodeDecryptFailed, Severity: SeverityError, Stage: StageDecrypt, ObjectType: "deposit", Message: "deposit could not be read to the end", At: now()})
		}
		return
	}
	res.Digests.PlaintextSHA256 = hex.EncodeToString(s.hash.Sum(nil))
}

// OpenDeposit runs the intake, signature, decrypt and unpack stages and returns
// the single XML entry the deposit carries. Findings are recorded on res; ok is
// false when a stage failed and the caller must stop.
//
// It is the one place that knows how an artifact is opened, so validation
// (Run) and derivative production (internal/application/rdesanitize) cannot
// drift apart on what "the deposit XML" means.
func OpenDeposit(ctx context.Context, in Input, res *Result, now func() time.Time) (entry *XMLEntry, st *openState, ok bool) {
	heartbeat := func(stage Stage, detail string) {
		if in.Heartbeat != nil {
			in.Heartbeat(stage, detail)
		}
	}
	internal := func(stage Stage, msg string) {
		res.Add(Finding{Code: CodeInternal, Severity: SeverityError, Stage: stage, Message: msg, At: now()})
	}
	signed := in.profile() == ProfileRydeSig

	// ---- intake: digest of the signature we were handed ----
	res.StageReached = StageIntake
	if signed {
		sigSum := sha256.Sum256(in.Sig)
		res.Digests.SignatureSHA256 = hex.EncodeToString(sigSum[:])
		if in.ExpectedSignatureSHA256 != "" && in.ExpectedSignatureSHA256 != res.Digests.SignatureSHA256 {
			internal(StageIntake, "signature artifact digest differs from the digest recorded at intake")
			return nil, nil, false
		}
	}

	// ---- pass 1: digest the artifact, and verify the signature when signed ----
	var ring openpgp.EntityList
	if signed {
		res.StageReached = StageSignature
		heartbeat(StageSignature, "verifying detached signature")
		var parseFindings []Finding
		ring, parseFindings = ParseTrustedKeyring(in.TrustedKeys, now())
		for _, f := range parseFindings {
			res.Add(f)
		}
	} else {
		heartbeat(StageIntake, "digesting deposit artifact")
	}
	rc, err := in.OpenArtifact(ctx)
	if err != nil {
		internal(res.StageReached, "deposit artifact could not be opened")
		return nil, nil, false
	}
	artifactHash := sha256.New()
	counter := &countingReader{r: io.TeeReader(io.LimitReader(rc, in.Limits.MaxCompressedBytes+1), artifactHash)}
	var sigFinding *Finding
	if signed {
		var sigInfo SignatureInfo
		sigInfo, sigFinding = VerifyDetached(ring, counter, in.Sig, now())
		res.Signature = sigInfo
	} else {
		// Nothing to verify, but the artifact must still be read to the end to
		// be digested and to be measured against the compressed-size limit.
		_, err = io.Copy(io.Discard, counter)
	}
	_ = rc.Close()
	if counter.n > in.Limits.MaxCompressedBytes {
		res.Add(Finding{Code: CodeIntakeLimitCompressedSize, Severity: SeverityError, Stage: StageIntake, ObjectType: "deposit",
			Message: "deposit exceeds the compressed size limit (" + i64toa(in.Limits.MaxCompressedBytes) + " bytes)", At: now()})
		return nil, nil, false
	}
	if err != nil {
		internal(res.StageReached, "deposit artifact could not be read")
		return nil, nil, false
	}
	if ctx.Err() != nil {
		res.Add(timeoutFinding(res.StageReached, now()))
		return nil, nil, false
	}
	res.Digests.ArtifactSHA256 = hex.EncodeToString(artifactHash.Sum(nil))
	if in.ExpectedArtifactSHA256 != "" && in.ExpectedArtifactSHA256 != res.Digests.ArtifactSHA256 {
		internal(res.StageReached, "deposit artifact digest differs from the digest recorded at intake")
		return nil, nil, false
	}
	if sigFinding != nil {
		res.Add(*sigFinding)
		return nil, nil, false
	}

	// ---- pass 2: (decrypt ->) unpack, as one chained stream ----
	st = &openState{budget: &budget{remaining: in.Limits.MaxUnpackedBytes}, hash: sha256.New()}
	rc2, err := in.OpenArtifact(ctx)
	if err != nil {
		internal(res.StageReached, "deposit artifact could not be reopened")
		return nil, nil, false
	}
	st.closers = append(st.closers, rc2)
	source := io.Reader(io.LimitReader(rc2, in.Limits.MaxCompressedBytes))

	if signed {
		res.StageReached = StageDecrypt
		heartbeat(StageDecrypt, "decrypting deposit")
		if len(in.ServiceKeys) == 0 {
			res.Add(Finding{Code: CodeDecryptKeyUnavailable, Severity: SeverityError, Stage: StageDecrypt, Message: "no service decryption key is available", At: now()})
			st.Close()
			return nil, nil, false
		}
		md, decInfo, f := Decrypt(in.ServiceKeys, source, now())
		if f != nil {
			res.Add(*f)
			st.Close()
			return nil, nil, false
		}
		res.Decryption = decInfo
		if decInfo.InnerSigned {
			res.Add(Finding{Code: CodeInfoInnerSigned, Severity: SeverityInfo, Stage: StageDecrypt, ObjectType: "deposit",
				Message: "the encrypted message carries an inner signature; trust is established by the detached signature only", At: now()})
		}
		source = md.UnverifiedBody
	}

	res.StageReached = StageUnpack
	heartbeat(StageUnpack, "unpacking payload")
	st.payload = io.TeeReader(source, st.hash)
	entry, f := SafeUnpack(st.payload, in.Limits, st.budget, now())
	if f != nil {
		res.Add(*f)
		st.Close()
		return nil, nil, false
	}
	res.Layout = entry.Layout
	return entry, st, true
}

// Run executes the pipeline and never panics: every failure becomes a
// Finding and the outcome is decided by Decide. Stages run in order and stop
// at the first ERROR-severity finding — in particular a deposit whose
// signature does not verify is never decrypted (fail closed).
func Run(ctx context.Context, in Input) (res Result) {
	now := in.Now
	if now == nil {
		now = time.Now
	}
	res = Result{Profile: in.profile(), StartedAt: now(), Findings: []Finding{}}
	defer func() {
		if p := recover(); p != nil {
			res.Add(Finding{Code: CodeInternal, Severity: SeverityError, Stage: res.StageReached, Message: "validation pipeline panicked", At: now()})
		}
		res.finish(now())
	}()
	if err := in.Limits.Validate(); err != nil {
		res.StageReached = StageIntake
		res.Add(Finding{Code: CodeInternal, Severity: SeverityError, Stage: StageIntake, Message: "resource limits are misconfigured", At: now()})
		return res
	}

	entry, st, ok := OpenDeposit(ctx, in, &res, now)
	if !ok {
		return res
	}
	defer st.Close()

	res.StageReached = StageXML
	if in.Heartbeat != nil {
		in.Heartbeat(StageXML, "validating deposit XML")
	}
	v := &XMLValidator{BoundTLD: in.BoundTLD, Now: now, Heartbeat: in.Heartbeat}
	summary, findings := v.Validate(ctx, entry.Reader)
	res.Deposit = summary
	for _, f := range findings {
		res.Add(f)
	}
	res.StageReached = StageRDE
	if ctx.Err() != nil && !res.Has(CodeValidationTimeout) {
		res.Add(timeoutFinding(StageRDE, now()))
		return res
	}

	st.finish(&res, entry, now)
	return res
}

func timeoutFinding(stage Stage, at time.Time) Finding {
	return Finding{Code: CodeValidationTimeout, Severity: SeverityError, Stage: stage, Message: "validation deadline reached", At: at}
}

type countingReader struct {
	r io.Reader
	n int64
}

func (c *countingReader) Read(p []byte) (int, error) {
	n, err := c.r.Read(p)
	c.n += int64(n)
	return n, err
}

// String renders a compact operator-safe summary (codes only, no payload data).
func (r Result) String() string {
	return fmt.Sprintf("profile=%s outcome=%s stage=%s codes=%v", r.Profile, r.Outcome, r.StageReached, r.Codes())
}
