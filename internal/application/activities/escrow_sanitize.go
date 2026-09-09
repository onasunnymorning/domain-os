package activities

import (
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/ProtonMail/go-crypto/openpgp"
	"github.com/google/uuid"
	"github.com/onasunnymorning/domain-os/internal/application/interfaces"
	"github.com/onasunnymorning/domain-os/internal/application/rdesanitize"
	"github.com/onasunnymorning/domain-os/internal/application/rdevalidate"
	"github.com/onasunnymorning/domain-os/internal/infrastructure/db/postgres"
	"github.com/onasunnymorning/domain-os/internal/infrastructure/secrets"
	"github.com/onasunnymorning/domain-os/internal/infrastructure/storage"
	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
	"github.com/onasunnymorning/domain-os/pkg/domain/repositories"
	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/temporal"
	"gorm.io/gorm"
)

const (
	// EscrowSanitizeWorkflowVersion is stamped on every run and manifest so a
	// derivative can be traced to the code that produced it, independently of
	// the policy version that decided its content.
	EscrowSanitizeWorkflowVersion = "escrow-sanitize/1"

	escrowSanitizeErrorType = "ESCROW_SANITIZE"
	// derivativeContentType is what the object store is told; the derivative is
	// always gzipped XML.
	derivativeContentType = "application/gzip"
)

// EscrowSanitizeActivities produce a sanitized-pseudonymized derivative of an
// already-validated deposit (issue #415).
//
// It holds the escrow repositories and object stores only — never *gorm.DB,
// never a service from internal/application/services, and nothing from the
// escrow import pipeline. The derivative path has no way to write registry
// data, and escrow_validation_isolation_test.go asserts it.
type EscrowSanitizeActivities struct {
	tlds          repositories.TLDRepository
	deposits      repositories.EscrowDepositRepository
	runs          repositories.EscrowValidationRunRepository
	sanitizations repositories.EscrowSanitizationRunRepository
	keys          repositories.EscrowTrustedKeyRepository
	decryptKeys   interfaces.EscrowDecryptionKeyProvider
	tokenKeys     interfaces.SanitizationTokenKeyProvider

	// escrowStore holds the source deposit, the pending/ staging object and the
	// published derivative. The custody boundary is the key prefix, not the
	// bucket: a derivative is written under pending/, verified there, and only
	// then server-side copied into sanitized/. That copy is what makes
	// "nothing is published unless it passed" true, and keeping both prefixes
	// in one bucket is what makes it a copy rather than a full re-upload.
	//
	// Co-locating the derivative with the deposit is a deliberate deviation
	// from the ticket's separate bucket and IAM role; see ADR-0008. A lifecycle
	// rule should expire the pending/ prefix.
	escrowStore interfaces.ObjectStore

	validateLimits rdevalidate.Limits
	limits         rdesanitize.Limits
	defaultSuffix  string
	now            func() time.Time
}

// NewEscrowSanitizeActivities wires the activities from the environment. It
// fails when the token key is absent, so a worker without one declines to
// register rather than quietly producing derivatives with a weak pseudonym.
func NewEscrowSanitizeActivities() (*EscrowSanitizeActivities, error) {
	var db *gorm.DB
	var err error
	if dbURL := os.Getenv("DATABASE_URL"); dbURL != "" {
		db, err = postgres.NewConnectionFromURL(dbURL, false)
	} else {
		db, err = postgres.NewConnection(postgres.Config{
			User:    os.Getenv("DB_USER"),
			Pass:    os.Getenv("DB_PASS"),
			Host:    os.Getenv("DB_HOST"),
			Port:    os.Getenv("DB_PORT"),
			DBName:  os.Getenv("DB_NAME"),
			SSLmode: os.Getenv("DB_SSLMODE"),
		})
	}
	if err != nil {
		return nil, fmt.Errorf("escrow sanitization activities: database: %w", err)
	}
	escrowStore, err := storage.NewS3ClientFromEnv()
	if err != nil {
		return nil, fmt.Errorf("NewEscrowSanitizeActivities: escrow storage: %w", err)
	}
	decryptKeys, err := secrets.NewEnvEscrowKeyProviderFromEnv()
	if err != nil {
		return nil, fmt.Errorf("NewEscrowSanitizeActivities: escrow decryption keys: %w", err)
	}
	tokenKeys, err := secrets.NewEnvSanitizeKeyProviderFromEnv()
	if err != nil {
		return nil, fmt.Errorf("NewEscrowSanitizeActivities: sanitization token key: %w", err)
	}
	validateLimits, err := loadEscrowValidationLimits()
	if err != nil {
		return nil, fmt.Errorf("NewEscrowSanitizeActivities: validation limits: %w", err)
	}
	limits, err := loadEscrowSanitizeLimits()
	if err != nil {
		return nil, fmt.Errorf("NewEscrowSanitizeActivities: sanitization limits: %w", err)
	}
	return NewEscrowSanitizeActivitiesWithDeps(
		postgres.NewGormTLDRepo(db),
		postgres.NewEscrowDepositRepository(db),
		postgres.NewEscrowValidationRunRepository(db),
		postgres.NewEscrowSanitizationRunRepository(db),
		postgres.NewEscrowTrustedKeyRepository(db),
		decryptKeys, tokenKeys, escrowStore,
		validateLimits, limits, escrowSanitizeSuffix(),
	), nil
}

// NewEscrowSanitizeActivitiesWithDeps builds the activities from explicit
// dependencies, for tests and for callers that already hold them.
func NewEscrowSanitizeActivitiesWithDeps(
	tlds repositories.TLDRepository,
	deposits repositories.EscrowDepositRepository,
	runs repositories.EscrowValidationRunRepository,
	sanitizations repositories.EscrowSanitizationRunRepository,
	keys repositories.EscrowTrustedKeyRepository,
	decryptKeys interfaces.EscrowDecryptionKeyProvider,
	tokenKeys interfaces.SanitizationTokenKeyProvider,
	escrowStore interfaces.ObjectStore,
	validateLimits rdevalidate.Limits,
	limits rdesanitize.Limits,
	defaultSuffix string,
) *EscrowSanitizeActivities {
	return &EscrowSanitizeActivities{
		tlds: tlds, deposits: deposits, runs: runs, sanitizations: sanitizations, keys: keys,
		decryptKeys: decryptKeys, tokenKeys: tokenKeys,
		escrowStore:    escrowStore,
		validateLimits: validateLimits, limits: limits, defaultSuffix: defaultSuffix,
		now: func() time.Time { return time.Now().UTC() },
	}
}

// ---------------------------------------------------------------------------
// 1. BindSanitizationSource
// ---------------------------------------------------------------------------

// BindSanitizationSourceInput names the accepted validation run to derive from.
type BindSanitizationSourceInput struct {
	Scope                 string    `json:"scope"`
	SourceValidationRunID uuid.UUID `json:"sourceValidationRunId"`
	// SyntheticSuffix overrides ESCROW_SANITIZE_SUFFIX for this run.
	SyntheticSuffix string `json:"syntheticSuffix,omitempty"`
	WorkflowID      string `json:"workflowId"`
	RunID           string `json:"runId"`
}

// BindSanitizationSourceOutput carries everything the later steps need, so they
// never re-read the source records and cannot disagree about what was bound.
type BindSanitizationSourceOutput struct {
	SanitizationRunID uuid.UUID `json:"sanitizationRunId"`
	Replay            bool      `json:"replay"`
	TLD               string    `json:"tld"`
	DepositID         uuid.UUID `json:"depositId"`
	SourceProfile     string    `json:"sourceProfile"`
	ArtifactKey       string    `json:"artifactKey"`
	SignatureKey      string    `json:"signatureKey"`
	ArtifactSHA256    string    `json:"artifactSha256"`
	SignatureSHA256   string    `json:"signatureSha256"`
	SyntheticSuffix   string    `json:"syntheticSuffix"`
	PolicyVersion     string    `json:"policyVersion"`
	StagingKey        string    `json:"stagingKey"`
	DerivativeKey     string    `json:"derivativeKey"`
	ManifestKey       string    `json:"manifestKey"`
	AlreadyFinal      bool      `json:"alreadyFinal"`
}

// BindSanitizationSource refuses anything but an accepted validation run, binds
// the derivative to it, and opens a RUNNING record.
//
// It never opens the source for writing. The raw deposit stays the
// authoritative custody artifact; this step only records which version of it a
// derivative was made from.
func (a *EscrowSanitizeActivities) BindSanitizationSource(ctx context.Context, in BindSanitizationSourceInput) (BindSanitizationSourceOutput, error) {
	logger := activity.GetLogger(ctx)
	scope, err := entities.NewOperatorID(in.Scope)
	if err != nil {
		return BindSanitizationSourceOutput{}, nonRetryableSanitize("invalid operator scope", err)
	}
	source, err := a.runs.GetByID(ctx, scope, in.SourceValidationRunID)
	if err != nil {
		if errors.Is(err, entities.ErrEscrowValidationRunNotFound) {
			return BindSanitizationSourceOutput{}, nonRetryableSanitize(string(rdesanitize.CodeSourceNotAccepted)+": source validation run not found for this tenant", err)
		}
		return BindSanitizationSourceOutput{}, fmt.Errorf("BindSanitizationSource: source run: %w", err)
	}
	// Transforming a deposit this service has not accepted would put
	// unvalidated content behind a label that says it was checked.
	if !entities.EscrowValidationRunIsSanitizableSource(source) {
		return BindSanitizationSourceOutput{}, nonRetryableSanitize(
			string(rdesanitize.CodeSourceNotAccepted)+": source validation run did not pass", entities.ErrEscrowSourceNotAccepted)
	}
	deposit, err := a.deposits.GetByID(ctx, scope, source.DepositID)
	if err != nil {
		return BindSanitizationSourceOutput{}, fmt.Errorf("BindSanitizationSource: source deposit: %w", err)
	}
	// Defence in depth, as in BindDeposit: a launch path that forgot the scoped
	// TLD lookup still cannot create a record for another operator's TLD.
	if _, err := a.tlds.GetByNameForOperator(ctx, scope, deposit.TLD); err != nil {
		if errors.Is(err, entities.ErrTLDNotFound) {
			return BindSanitizationSourceOutput{}, nonRetryableSanitize("tld is not operated by the caller's scope", err)
		}
		return BindSanitizationSourceOutput{}, fmt.Errorf("BindSanitizationSource: tld lookup: %w", err)
	}

	suffix := strings.TrimSpace(in.SyntheticSuffix)
	if suffix == "" {
		suffix = a.defaultSuffix
	}

	out := BindSanitizationSourceOutput{
		TLD: deposit.TLD, DepositID: deposit.ID, SourceProfile: deposit.Profile,
		ArtifactKey: deposit.ArtifactObjectKey, SignatureKey: deposit.SignatureObjectKey,
		ArtifactSHA256: deposit.ArtifactSHA256, SignatureSHA256: deposit.SignatureSHA256,
		SyntheticSuffix: suffix, PolicyVersion: rdesanitize.PolicyVersion,
	}

	// One derivative per source per policy version: a replay binds to the
	// existing record instead of writing a second object.
	existing, err := a.sanitizations.FindBySourceAndPolicy(ctx, scope, source.ID, rdesanitize.PolicyVersion)
	switch {
	case err == nil:
		out.SanitizationRunID, out.Replay, out.AlreadyFinal = existing.ID, true, existing.IsFinal()
		out.SyntheticSuffix = existing.SyntheticSuffix
	case errors.Is(err, entities.ErrEscrowSanitizationRunNotFound):
		run, nerr := entities.NewEscrowSanitizationRun(scope, deposit.TLD, source.ID, deposit.ID,
			deposit.ArtifactSHA256, rdesanitize.PolicyVersion, EscrowSanitizeWorkflowVersion, suffix,
			in.WorkflowID, in.RunID, a.now())
		if nerr != nil {
			return BindSanitizationSourceOutput{}, nonRetryableSanitize("sanitization run rejected", nerr)
		}
		if cerr := a.sanitizations.Create(ctx, run); cerr != nil {
			// A concurrent bind won the unique index; use its record.
			raced, ferr := a.sanitizations.FindBySourceAndPolicy(ctx, scope, source.ID, rdesanitize.PolicyVersion)
			if ferr != nil {
				return BindSanitizationSourceOutput{}, fmt.Errorf("BindSanitizationSource: create run: %w", cerr)
			}
			run, out.Replay, out.AlreadyFinal = raced, true, raced.IsFinal()
			out.SyntheticSuffix = raced.SyntheticSuffix
		}
		out.SanitizationRunID, out.SyntheticSuffix = run.ID, run.SyntheticSuffix
	default:
		return BindSanitizationSourceOutput{}, fmt.Errorf("BindSanitizationSource: existing derivative lookup: %w", err)
	}

	prefix := fmt.Sprintf("%s/%s/%s/%s/%s", escrowValidationPrefix, scope.String(), deposit.TLD, deposit.ID, out.SanitizationRunID)
	out.StagingKey = prefix + "/pending/deposit-" + rdesanitize.PolicyVersion + ".xml.gz"
	out.DerivativeKey = prefix + "/sanitized/deposit-" + rdesanitize.PolicyVersion + ".xml.gz"
	out.ManifestKey = prefix + "/sanitized/manifest-" + rdesanitize.PolicyVersion + ".json"

	logger.Info("escrow sanitization: source bound",
		"correlation_id", in.WorkflowID, "sanitization_run_id", out.SanitizationRunID.String(),
		"source_run_id", source.ID.String(), "tld", deposit.TLD, "stage", string(rdesanitize.StageSource),
		"policy_version", rdesanitize.PolicyVersion, "replay", out.Replay)
	return out, nil
}

// ---------------------------------------------------------------------------
// 2. ProduceDerivative
// ---------------------------------------------------------------------------

// ProduceDerivativeInput is the bound source plus where to stage the output.
type ProduceDerivativeInput struct {
	Scope             string    `json:"scope"`
	SanitizationRunID uuid.UUID `json:"sanitizationRunId"`
	WorkflowID        string    `json:"workflowId"`
	TLD               string    `json:"tld"`
	SourceProfile     string    `json:"sourceProfile"`
	ArtifactKey       string    `json:"artifactKey"`
	SignatureKey      string    `json:"signatureKey"`
	ArtifactSHA256    string    `json:"artifactSha256"`
	SignatureSHA256   string    `json:"signatureSha256"`
	SyntheticSuffix   string    `json:"syntheticSuffix"`
	StagingKey        string    `json:"stagingKey"`
}

// ProduceDerivativeOutput reports what was staged. A non-PASS result means
// nothing usable was staged and nothing must be published.
type ProduceDerivativeOutput struct {
	Result           rdesanitize.Result `json:"result"`
	DerivativeSHA256 string             `json:"derivativeSha256"`
	DerivativeBytes  int64              `json:"derivativeBytes"`
	TokenKeyID       string             `json:"tokenKeyId"`
}

// ProduceDerivative streams the source deposit through the profile and stages
// the derivative. Nothing touches disk and the plaintext is never persisted:
// for a signed source the artifact is re-verified and re-decrypted here, which
// costs a second pass and is the reason the plaintext never had to be kept.
func (a *EscrowSanitizeActivities) ProduceDerivative(ctx context.Context, in ProduceDerivativeInput) (ProduceDerivativeOutput, error) {
	logger := activity.GetLogger(ctx)
	scope, err := entities.NewOperatorID(in.Scope)
	if err != nil {
		return ProduceDerivativeOutput{}, nonRetryableSanitize("invalid operator scope", err)
	}
	now := a.now
	out := ProduceDerivativeOutput{}

	master, err := a.tokenKeys.TokenKey(ctx)
	if err != nil {
		// Our key store, not the deposit: ERROR, not a quarantine.
		out.Result = sanitizeServiceError(rdesanitize.CodeTokenKeyUnavailable, "the sanitization token key is unavailable", now())
		return out, nil
	}
	tokens, err := rdesanitize.NewTokenizer(master, scope.String(), rdesanitize.PolicyVersion)
	if err != nil {
		out.Result = sanitizeServiceError(rdesanitize.CodeTokenKeyUnavailable, "the sanitization token key is unusable", now())
		return out, nil
	}
	out.TokenKeyID = tokens.KeyFingerprint()

	suffix, err := rdesanitize.NewSuffixRewriter(in.TLD, in.SyntheticSuffix)
	if err != nil {
		return out, nonRetryableSanitize("the configured synthetic suffix is unusable", err)
	}

	runCtx, cancel := context.WithTimeout(ctx, a.limits.Timeout)
	defer cancel()

	openInput, err := a.sourceInput(runCtx, in)
	if err != nil {
		return out, err
	}
	var openRes rdevalidate.Result
	entry, st, ok := rdevalidate.OpenDeposit(runCtx, openInput, &openRes, now)
	if !ok {
		// The source no longer opens the way validation left it: the artifact
		// changed, or our decryption keyring is gone. Either way it is not a
		// statement about the derivative.
		out.Result = sanitizeSourceFailure(openRes, now())
		return out, nil
	}
	defer st.Close()

	rw := &rdesanitize.Rewriter{
		Profile: rdesanitize.BaselineProfile(), Tokens: tokens, Suffix: suffix,
		Limits: a.limits, Now: now,
		Heartbeat: func(_ rdesanitize.Stage, detail string) { activity.RecordHeartbeat(ctx, detail) },
	}

	res, sha, size, err := a.stage(runCtx, in.StagingKey, entry.Reader, rw)
	if err != nil {
		return out, fmt.Errorf("ProduceDerivative: stage derivative: %w", err)
	}
	out.Result, out.DerivativeSHA256, out.DerivativeBytes = res, sha, size

	logger.Info("escrow sanitization: derivative staged",
		"correlation_id", in.WorkflowID, "sanitization_run_id", in.SanitizationRunID.String(),
		"tld", in.TLD, "stage", string(res.StageReached), "outcome", string(res.Outcome),
		"codes", codeStringsSanitize(res.Codes()))
	return out, nil
}

// sourceInput rebuilds the validator input for the bound source, so the
// derivative is made from exactly what validation accepted.
func (a *EscrowSanitizeActivities) sourceInput(ctx context.Context, in ProduceDerivativeInput) (rdevalidate.Input, error) {
	input := rdevalidate.Input{
		Profile: in.SourceProfile,
		OpenArtifact: func(c context.Context) (io.ReadCloser, error) {
			rc, _, err := a.escrowStore.GetObjectStream(c, in.ArtifactKey)
			return rc, err
		},
		ExpectedArtifactSHA256:  in.ArtifactSHA256,
		ExpectedSignatureSHA256: in.SignatureSHA256,
		BoundTLD:                in.TLD,
		Limits:                  a.validateLimits,
		Now:                     a.now,
	}
	if !entities.EscrowProfileIsSigned(in.SourceProfile) {
		return input, nil
	}
	scope, err := entities.NewOperatorID(in.Scope)
	if err != nil {
		return input, nonRetryableSanitize("invalid operator scope", err)
	}
	trusted, err := a.keys.ListActive(ctx, scope, in.TLD, a.now())
	if err != nil {
		return input, fmt.Errorf("ProduceDerivative: trusted keys: %w", err)
	}
	input.TrustedKeys = make([]string, len(trusted))
	for i, k := range trusted {
		input.TrustedKeys[i] = k.ArmoredPublicKey
	}
	var ring openpgp.EntityList
	if ring, err = a.decryptKeys.DecryptionKeyring(ctx); err != nil {
		ring = nil // surfaces as DECRYPT_KEY_UNAVAILABLE, an ERROR-class code
	}
	input.ServiceKeys = ring
	sig, err := a.readSmallStore(ctx, a.escrowStore, in.SignatureKey, maxSignatureBytes)
	if err != nil {
		return input, fmt.Errorf("ProduceDerivative: signature: %w", err)
	}
	input.Sig = sig
	return input, nil
}

// stage rewrites src into the staging object, gzipping and digesting as it
// goes. When the rewrite does not pass, the upload is aborted rather than
// completed: a quarantined derivative is never stored anywhere.
func (a *EscrowSanitizeActivities) stage(ctx context.Context, key string, src io.Reader, rw *rdesanitize.Rewriter) (rdesanitize.Result, string, int64, error) {
	pr, pw := io.Pipe()
	h := sha256.New()
	counter := &countingWriter{}

	var res rdesanitize.Result
	go func() {
		gz := gzip.NewWriter(io.MultiWriter(pw, h, counter))
		res = rw.Rewrite(ctx, src, gz)
		if res.Outcome != rdesanitize.OutcomePass {
			// Abort the upload so nothing partial is stored.
			_ = gz.Close()
			_ = pw.CloseWithError(errSanitizeNotPassed)
			return
		}
		if err := gz.Close(); err != nil {
			_ = pw.CloseWithError(err)
			return
		}
		_ = pw.Close()
	}()

	err := a.escrowStore.UploadStream(ctx, key, pr, derivativeContentType)
	_ = pr.CloseWithError(err)
	switch {
	case err == nil:
		return res, hex.EncodeToString(h.Sum(nil)), counter.n, nil
	case errors.Is(err, errSanitizeNotPassed) || strings.Contains(err.Error(), errSanitizeNotPassed.Error()):
		// Expected: the rewrite refused the source and cancelled the upload.
		return res, "", 0, nil
	default:
		return res, "", 0, err
	}
}

var errSanitizeNotPassed = errors.New("sanitization did not pass; upload aborted")

type countingWriter struct{ n int64 }

func (c *countingWriter) Write(p []byte) (int, error) {
	c.n += int64(len(p))
	return len(p), nil
}

// ---------------------------------------------------------------------------
// 3. VerifyDerivative
// ---------------------------------------------------------------------------

// VerifyDerivativeInput points at the staged derivative.
type VerifyDerivativeInput struct {
	Scope                 string             `json:"scope"`
	SanitizationRunID     uuid.UUID          `json:"sanitizationRunId"`
	WorkflowID            string             `json:"workflowId"`
	RunID                 string             `json:"runId"`
	TLD                   string             `json:"tld"`
	DepositID             uuid.UUID          `json:"depositId"`
	SourceValidationRunID uuid.UUID          `json:"sourceValidationRunId"`
	SourceProfile         string             `json:"sourceProfile"`
	SourceArtifactSHA256  string             `json:"sourceArtifactSha256"`
	SyntheticSuffix       string             `json:"syntheticSuffix"`
	StagingKey            string             `json:"stagingKey"`
	DerivativeKey         string             `json:"derivativeKey"`
	ManifestKey           string             `json:"manifestKey"`
	DerivativeSHA256      string             `json:"derivativeSha256"`
	DerivativeBytes       int64              `json:"derivativeBytes"`
	TokenKeyID            string             `json:"tokenKeyId"`
	Produced              rdesanitize.Result `json:"produced"`
}

// VerifyDerivativeOutput reports the verified derivative, or why it was not
// published.
type VerifyDerivativeOutput struct {
	Result        rdesanitize.Result `json:"result"`
	DerivativeKey string             `json:"derivativeKey"`
	ManifestKey   string             `json:"manifestKey"`
}

// VerifyDerivative re-reads what was staged, checks it against the outcome
// rather than the policy, counts the objects it actually contains, and only
// then publishes it beside the source with a non-PII manifest.
func (a *EscrowSanitizeActivities) VerifyDerivative(ctx context.Context, in VerifyDerivativeInput) (VerifyDerivativeOutput, error) {
	logger := activity.GetLogger(ctx)
	scope, err := entities.NewOperatorID(in.Scope)
	if err != nil {
		return VerifyDerivativeOutput{}, nonRetryableSanitize("invalid operator scope", err)
	}
	now := a.now
	res := in.Produced
	res.StageReached = rdesanitize.StageVerify

	suffix, err := rdesanitize.NewSuffixRewriter(in.TLD, in.SyntheticSuffix)
	if err != nil {
		return VerifyDerivativeOutput{}, nonRetryableSanitize("the configured synthetic suffix is unusable", err)
	}

	runCtx, cancel := context.WithTimeout(ctx, a.limits.Timeout)
	defer cancel()

	// Pass 1: the PII regression scan.
	findings, err := a.overStagedDerivative(runCtx, in.StagingKey, func(r io.Reader) ([]rdesanitize.Finding, error) {
		return rdesanitize.Scan(runCtx, r, rdesanitize.ScanOptions{
			Profile: rdesanitize.BaselineProfile(), Suffix: suffix, Limits: a.limits, Now: now,
		}), nil
	})
	if err != nil {
		return VerifyDerivativeOutput{}, fmt.Errorf("VerifyDerivative: scan: %w", err)
	}
	for _, f := range findings {
		res.Add(f)
	}

	// Pass 2: structural re-validation, and the aggregate statistics. Counting
	// from the derivative rather than from the rewriter means the manifest
	// describes what was actually written.
	observed, structural, err := a.revalidate(runCtx, in.StagingKey, in.SyntheticSuffix)
	if err != nil {
		return VerifyDerivativeOutput{}, fmt.Errorf("VerifyDerivative: revalidate: %w", err)
	}
	for _, f := range structural {
		res.Add(f)
	}
	if len(observed) > 0 {
		res.Counts.ObjectsByType = observed
	}

	res.Outcome = rdesanitize.Decide(res.Findings)
	res.CompletedAt = now()
	out := VerifyDerivativeOutput{Result: res}
	if res.Outcome != rdesanitize.OutcomePass {
		logger.Info("escrow sanitization: derivative refused",
			"correlation_id", in.WorkflowID, "sanitization_run_id", in.SanitizationRunID.String(),
			"tld", in.TLD, "stage", string(rdesanitize.StageVerify), "outcome", string(res.Outcome),
			"codes", codeStringsSanitize(res.Codes()))
		return out, nil
	}

	// Publish: the staged object becomes the derivative, and the manifest is
	// written beside it.
	if err := a.escrowStore.CopyObject(ctx, in.StagingKey, in.DerivativeKey); err != nil {
		return VerifyDerivativeOutput{}, fmt.Errorf("VerifyDerivative: publish derivative: %w", err)
	}
	manifest := rdesanitize.Manifest{
		Label: entities.EscrowDerivativeLabel, TenantID: scope.String(), TLD: in.TLD,
		SyntheticSuffix:       in.SyntheticSuffix,
		SourceValidationRunID: in.SourceValidationRunID.String(), SourceDepositID: in.DepositID.String(),
		SourceProfile: in.SourceProfile, SourceArtifactSHA256: in.SourceArtifactSHA256,
		DerivativeObjectKey: in.DerivativeKey, DerivativeSHA256: in.DerivativeSHA256, DerivativeBytes: in.DerivativeBytes,
		PolicyVersion: rdesanitize.PolicyVersion, WorkflowVersion: EscrowSanitizeWorkflowVersion,
		TokenKeyFingerprint: in.TokenKeyID, WorkflowID: in.WorkflowID, RunID: in.RunID,
		Outcome: string(res.Outcome), ReasonCodes: codeStringsSanitize(res.Codes()),
		StartedAt: res.StartedAt, CompletedAt: res.CompletedAt,
		SourceElements: res.SourceElements, Counts: res.Counts,
	}
	raw, err := manifest.Marshal()
	if err != nil {
		return VerifyDerivativeOutput{}, fmt.Errorf("VerifyDerivative: manifest: %w", err)
	}
	if err := a.escrowStore.UploadStream(ctx, in.ManifestKey, strings.NewReader(string(raw)), "application/json"); err != nil {
		return VerifyDerivativeOutput{}, fmt.Errorf("VerifyDerivative: publish manifest: %w", err)
	}
	out.DerivativeKey, out.ManifestKey = in.DerivativeKey, in.ManifestKey

	logger.Info("escrow sanitization: derivative published",
		"correlation_id", in.WorkflowID, "sanitization_run_id", in.SanitizationRunID.String(),
		"tld", in.TLD, "stage", string(rdesanitize.StageVerify), "outcome", string(res.Outcome))
	return out, nil
}

// overStagedDerivative streams the staged object through gzip and hands the
// plaintext to fn.
func (a *EscrowSanitizeActivities) overStagedDerivative(ctx context.Context, key string, fn func(io.Reader) ([]rdesanitize.Finding, error)) ([]rdesanitize.Finding, error) {
	rc, _, err := a.escrowStore.GetObjectStream(ctx, key)
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	gz, err := gzip.NewReader(rc)
	if err != nil {
		return nil, err
	}
	defer gz.Close()
	return fn(gz)
}

// revalidate streams the derivative through the ordinary RDE validator, bound
// to the synthetic suffix. Only ERROR-severity findings count against it:
// registry-specific entity rules are not RDE rules, exactly as in validation.
func (a *EscrowSanitizeActivities) revalidate(ctx context.Context, key, syntheticSuffix string) (map[string]int64, []rdesanitize.Finding, error) {
	var observed map[string]int64
	var out []rdesanitize.Finding
	_, err := a.overStagedDerivative(ctx, key, func(r io.Reader) ([]rdesanitize.Finding, error) {
		v := &rdevalidate.XMLValidator{BoundTLD: syntheticSuffix, Now: a.now}
		summary, findings := v.Validate(ctx, r)
		observed = make(map[string]int64, len(summary.Observed))
		for uri, n := range summary.Observed {
			observed[uri] = int64(n)
		}
		for _, f := range findings {
			if f.Severity != rdevalidate.SeverityError {
				continue
			}
			out = append(out, rdesanitize.Finding{
				Code: rdesanitize.CodeDerivativeInvalid, Severity: rdesanitize.SeverityError,
				Stage: rdesanitize.StageVerify, Locator: string(f.Code),
				Message: "derivative failed RDE validation", At: a.now(),
			})
		}
		return nil, nil
	})
	return observed, out, err
}

// ---------------------------------------------------------------------------
// 4. FinalizeSanitizationRun
// ---------------------------------------------------------------------------

// FinalizeSanitizationRunInput closes out the immutable record.
type FinalizeSanitizationRunInput struct {
	Scope             string             `json:"scope"`
	SanitizationRunID uuid.UUID          `json:"sanitizationRunId"`
	WorkflowID        string             `json:"workflowId"`
	Result            rdesanitize.Result `json:"result"`
	DerivativeKey     string             `json:"derivativeKey"`
	ManifestKey       string             `json:"manifestKey"`
	DerivativeSHA256  string             `json:"derivativeSha256"`
	DerivativeBytes   int64              `json:"derivativeBytes"`
	CompletedAt       time.Time          `json:"completedAt"`
	// Failure, when set, records an infrastructure failure the pipeline never
	// got to classify.
	Failure string `json:"failure,omitempty"`
}

// FinalizeSanitizationRun writes the terminal state exactly once.
func (a *EscrowSanitizeActivities) FinalizeSanitizationRun(ctx context.Context, in FinalizeSanitizationRunInput) error {
	logger := activity.GetLogger(ctx)
	scope, err := entities.NewOperatorID(in.Scope)
	if err != nil {
		return nonRetryableSanitize("invalid operator scope", err)
	}
	run, err := a.sanitizations.GetByID(ctx, scope, in.SanitizationRunID)
	if err != nil {
		return fmt.Errorf("FinalizeSanitizationRun: load run: %w", err)
	}
	res := in.Result
	if in.Failure != "" {
		res = sanitizeServiceError(rdesanitize.CodeInternal, "the sanitization workflow failed before a decision was reached", a.now())
	}
	completed := in.CompletedAt
	if completed.IsZero() {
		completed = a.now()
	}
	f := entities.EscrowSanitizationFinalization{
		Outcome:      entities.EscrowSanitizationOutcome(res.Outcome),
		StageReached: string(res.StageReached),
		Findings:     rdesanitize.ToEntityFindings(res.Findings),
		Counts:       res.Counts,
		CompletedAt:  completed,
	}
	// Only a PASS may reference an object; the entity enforces it too.
	if res.Outcome == rdesanitize.OutcomePass {
		f.DerivativeObjectKey, f.ManifestObjectKey = in.DerivativeKey, in.ManifestKey
		f.DerivativeSHA256, f.DerivativeBytes = in.DerivativeSHA256, in.DerivativeBytes
	}
	if err := run.Finalize(f); err != nil {
		return nonRetryableSanitize("sanitization run rejected finalisation", err)
	}
	if err := a.sanitizations.Finalize(ctx, scope, run); err != nil {
		if errors.Is(err, entities.ErrEscrowSanitizationRunAlreadyFinal) {
			return nonRetryableSanitize("run is already final", err)
		}
		return fmt.Errorf("FinalizeSanitizationRun: %w", err)
	}
	logger.Info("escrow sanitization: run finalised",
		"correlation_id", in.WorkflowID, "sanitization_run_id", run.ID.String(), "tld", run.TLD,
		"stage", string(res.StageReached), "outcome", string(res.Outcome), "codes", codeStringsSanitize(res.Codes()))
	return nil
}

// ---------------------------------------------------------------------------
// shared
// ---------------------------------------------------------------------------

func nonRetryableSanitize(msg string, cause error) error {
	return temporal.NewNonRetryableApplicationError(msg, escrowSanitizeErrorType, cause)
}

// sanitizeServiceError builds a result for a condition that is ours, not the
// deposit's: it decides to ERROR and never to QUARANTINED.
func sanitizeServiceError(code rdesanitize.Code, msg string, at time.Time) rdesanitize.Result {
	res := rdesanitize.Result{StageReached: rdesanitize.StageSource, StartedAt: at, CompletedAt: at}
	res.Add(rdesanitize.Finding{Code: code, Severity: rdesanitize.SeverityError, Stage: rdesanitize.StageSource, Message: msg, At: at})
	res.Outcome = rdesanitize.Decide(res.Findings)
	return res
}

// sanitizeSourceFailure maps a failure to reopen the validated source. It is a
// service-side condition: the deposit passed validation, so anything that stops
// us reading it now is about us or about the artifact having changed.
func sanitizeSourceFailure(open rdevalidate.Result, at time.Time) rdesanitize.Result {
	code := rdesanitize.CodeSourceArtifactChanged
	msg := "the validated source could not be reopened as validation left it"
	for _, c := range open.Codes() {
		if c == rdevalidate.CodeDecryptKeyUnavailable {
			code, msg = rdesanitize.CodeTokenKeyUnavailable, "the escrow decryption keyring is unavailable"
			break
		}
	}
	res := rdesanitize.Result{StageReached: rdesanitize.StageSource, StartedAt: at, CompletedAt: at}
	res.Add(rdesanitize.Finding{Code: code, Severity: rdesanitize.SeverityError, Stage: rdesanitize.StageSource, Message: msg, At: at})
	res.Add(rdesanitize.Finding{Code: rdesanitize.CodeInternal, Severity: rdesanitize.SeverityError, Stage: rdesanitize.StageSource,
		Locator: strings.Join(codeStringsValidate(open.Codes()), ","), Message: "source could not be reopened", At: at})
	res.Outcome = rdesanitize.Decide(res.Findings)
	return res
}

func codeStringsSanitize(codes []rdesanitize.Code) []string {
	out := make([]string, len(codes))
	for i, c := range codes {
		out[i] = string(c)
	}
	return out
}

func codeStringsValidate(codes []rdevalidate.Code) []string {
	out := make([]string, len(codes))
	for i, c := range codes {
		out[i] = string(c)
	}
	return out
}

// readSmallStore reads a bounded object from a store.
func (a *EscrowSanitizeActivities) readSmallStore(ctx context.Context, store interfaces.ObjectStore, key string, max int64) ([]byte, error) {
	rc, _, err := store.GetObjectStream(ctx, key)
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	b, err := io.ReadAll(io.LimitReader(rc, max+1))
	if err != nil {
		return nil, err
	}
	if int64(len(b)) > max {
		return nil, errArtifactTooLarge
	}
	return b, nil
}
