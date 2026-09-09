package rdevalidate

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/ProtonMail/go-crypto/openpgp"
)

// Input is everything one run needs. Nothing here touches disk: the
// ciphertext is opened as a stream (twice — see Run) and the plaintext is
// consumed as it is produced.
type Input struct {
	// OpenRyde opens the immutable .ryde artifact for reading. It is called
	// once to verify the detached signature and once more to decrypt.
	OpenRyde func(ctx context.Context) (io.ReadCloser, error)
	// Sig is the detached signature (binary or armored). Signatures are small.
	Sig []byte
	// ExpectedRydeSHA256 / ExpectedSigSHA256, when set, are re-checked against
	// what is actually read so a swapped artifact cannot be validated under
	// another pair's record.
	ExpectedRydeSHA256 string
	ExpectedSigSHA256  string

	TrustedKeys []string           // armored public keys active for the tenant/TLD
	ServiceKeys openpgp.EntityList // unlocked private keys, most recent first

	BoundTLD  string
	Limits    Limits
	Now       func() time.Time
	Heartbeat func(stage Stage, detail string)
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
	res = Result{Profile: ProfileRydeSig, StartedAt: now(), Findings: []Finding{}}
	defer func() {
		if p := recover(); p != nil {
			res.Add(Finding{Code: CodeInternal, Severity: SeverityError, Stage: res.StageReached, Message: "validation pipeline panicked", At: now()})
		}
		res.finish(now())
	}()
	heartbeat := func(stage Stage, detail string) {
		if in.Heartbeat != nil {
			in.Heartbeat(stage, detail)
		}
	}
	internal := func(stage Stage, msg string) {
		res.Add(Finding{Code: CodeInternal, Severity: SeverityError, Stage: stage, Message: msg, At: now()})
	}
	if err := in.Limits.Validate(); err != nil {
		res.StageReached = StageIntake
		internal(StageIntake, "resource limits are misconfigured")
		return res
	}

	// ---- intake: digest of the signature we were handed ----
	res.StageReached = StageIntake
	sigSum := sha256.Sum256(in.Sig)
	res.Digests.SigSHA256 = hex.EncodeToString(sigSum[:])
	if in.ExpectedSigSHA256 != "" && in.ExpectedSigSHA256 != res.Digests.SigSHA256 {
		internal(StageIntake, "signature artifact digest differs from the digest recorded at intake")
		return res
	}

	// ---- signature: pass 1 over the ciphertext ----
	res.StageReached = StageSignature
	heartbeat(StageSignature, "verifying detached signature")
	ring, parseFindings := ParseTrustedKeyring(in.TrustedKeys, now())
	for _, f := range parseFindings {
		res.Add(f)
	}
	rc, err := in.OpenRyde(ctx)
	if err != nil {
		internal(StageSignature, "deposit artifact could not be opened for signature verification")
		return res
	}
	rydeHash := sha256.New()
	counter := &countingReader{r: io.TeeReader(io.LimitReader(rc, in.Limits.MaxCompressedBytes+1), rydeHash)}
	sigInfo, f := VerifyDetached(ring, counter, in.Sig, now())
	_ = rc.Close()
	if counter.n > in.Limits.MaxCompressedBytes {
		res.Add(Finding{Code: CodeIntakeLimitCompressedSize, Severity: SeverityError, Stage: StageIntake, ObjectType: "deposit",
			Message: "deposit exceeds the compressed size limit (" + i64toa(in.Limits.MaxCompressedBytes) + " bytes)", At: now()})
		return res
	}
	if ctx.Err() != nil {
		res.Add(timeoutFinding(StageSignature, now()))
		return res
	}
	res.Digests.RydeSHA256 = hex.EncodeToString(rydeHash.Sum(nil))
	if in.ExpectedRydeSHA256 != "" && in.ExpectedRydeSHA256 != res.Digests.RydeSHA256 {
		internal(StageSignature, "deposit artifact digest differs from the digest recorded at intake")
		return res
	}
	if f != nil {
		res.Add(*f)
		return res
	}
	res.Signature = sigInfo

	// ---- decrypt + unpack + validate: pass 2, one chained stream ----
	res.StageReached = StageDecrypt
	heartbeat(StageDecrypt, "decrypting deposit")
	if len(in.ServiceKeys) == 0 {
		res.Add(Finding{Code: CodeDecryptKeyUnavailable, Severity: SeverityError, Stage: StageDecrypt, Message: "no service decryption key is available", At: now()})
		return res
	}
	rc2, err := in.OpenRyde(ctx)
	if err != nil {
		internal(StageDecrypt, "deposit artifact could not be opened for decryption")
		return res
	}
	defer rc2.Close()
	md, decInfo, f := Decrypt(in.ServiceKeys, io.LimitReader(rc2, in.Limits.MaxCompressedBytes), now())
	if f != nil {
		res.Add(*f)
		return res
	}
	res.Decryption = decInfo
	if decInfo.InnerSigned {
		res.Add(Finding{Code: CodeInfoInnerSigned, Severity: SeverityInfo, Stage: StageDecrypt, ObjectType: "deposit",
			Message: "the encrypted message carries an inner signature; trust is established by the detached signature only", At: now()})
	}

	res.StageReached = StageUnpack
	heartbeat(StageUnpack, "unpacking payload")
	bud := &budget{remaining: in.Limits.MaxUnpackedBytes}
	plainHash := sha256.New()
	plaintext := io.TeeReader(md.UnverifiedBody, plainHash)
	entry, f := SafeUnpack(plaintext, in.Limits, bud, now())
	if f != nil {
		res.Add(*f)
		return res
	}
	res.Layout = entry.Layout

	res.StageReached = StageXML
	heartbeat(StageXML, "validating deposit XML")
	v := &XMLValidator{BoundTLD: in.BoundTLD, Now: now, Heartbeat: heartbeat}
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

	// Drain the rest of the archive (second payload / unsafe trailing entry)
	// and the rest of the OpenPGP body so the integrity check (MDC) runs.
	if f := entry.Drain(); f != nil && !res.Has(f.Code) {
		res.Add(*f)
	}
	if _, err := io.Copy(io.Discard, &budgetReader{r: plaintext, b: bud}); err != nil {
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
		return res
	}
	res.Digests.PlaintextSHA256 = hex.EncodeToString(plainHash.Sum(nil))
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
