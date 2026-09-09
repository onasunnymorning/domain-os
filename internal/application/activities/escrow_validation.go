package activities

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/ProtonMail/go-crypto/openpgp"
	"io"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/onasunnymorning/domain-os/internal/application/interfaces"
	"github.com/onasunnymorning/domain-os/internal/application/rdereport"
	"github.com/onasunnymorning/domain-os/internal/application/rdevalidate"
	postgres "github.com/onasunnymorning/domain-os/internal/infrastructure/db/postgres"
	"github.com/onasunnymorning/domain-os/internal/infrastructure/secrets"
	"github.com/onasunnymorning/domain-os/internal/infrastructure/storage"
	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
	"github.com/onasunnymorning/domain-os/pkg/domain/repositories"
	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/temporal"
	"gorm.io/gorm"
)

// ---------------------------------------------------------------------------
// Escrow validation (EVE) activities — issue #412.
//
// These activities wire the pure rdevalidate/rdereport packages to object
// storage, Postgres and the key boundary. Design constraint 1 (validation is
// not import) is structural: the struct holds only the escrow repositories —
// never *gorm.DB — and nothing here imports the import services or the
// EscrowImportActivities. escrow_validation_isolation_test.go in the
// workflows package asserts that with go/ast.
//
// Logging (constraint 6): every line carries exactly correlation_id,
// deposit_id, run_id, tld, stage, outcome and codes — identifiers and
// constant vocabulary only, never a file name, object name or payload text.
// ---------------------------------------------------------------------------

const (
	// escrowValidationPrefix is the immutable artifact prefix in the escrow bucket
	// and the report prefix in the reports bucket: escrow-validation/{tenant}/{tld}/{depositID}/...
	escrowValidationPrefix = "escrow-validation"
	// maxSignatureBytes bounds the detached signature read; real signatures are a few hundred bytes.
	maxSignatureBytes         = 1 << 20
	escrowValidationErrorType = "ESCROW_VALIDATION"
)

// EscrowValidationActivities holds the dependencies of the validation workflow.
type EscrowValidationActivities struct {
	tlds        repositories.TLDRepository
	deposits    repositories.EscrowDepositRepository
	runs        repositories.EscrowValidationRunRepository
	keys        repositories.EscrowTrustedKeyRepository
	keyProvider interfaces.EscrowDecryptionKeyProvider
	escrowStore interfaces.ObjectStore
	reportStore interfaces.ObjectStore
	limits      rdevalidate.Limits
	deaName     string
	now         func() time.Time
}

// NewEscrowValidationActivities builds the activities from the environment,
// following the DB idiom of NewSerialDriftActivities. It fails when the
// keyring is not configured so the worker can decline to register the
// activities instead of running a decryptor with no key.
func NewEscrowValidationActivities() (*EscrowValidationActivities, error) {
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
		return nil, fmt.Errorf("escrow validation activities: database: %w", err)
	}
	escrowStore, err := storage.NewS3ClientFromEnv()
	if err != nil {
		return nil, fmt.Errorf("escrow validation activities: escrow bucket: %w", err)
	}
	reportStore, err := storage.NewReportsS3Client()
	if err != nil {
		return nil, fmt.Errorf("escrow validation activities: reports bucket: %w", err)
	}
	keyProvider, err := secrets.NewEnvEscrowKeyProviderFromEnv()
	if err != nil {
		return nil, fmt.Errorf("escrow validation activities: %w", err)
	}
	limits, err := loadEscrowValidationLimits()
	if err != nil {
		return nil, fmt.Errorf("escrow validation activities: limits: %w", err)
	}
	return NewEscrowValidationActivitiesWithDeps(
		postgres.NewGormTLDRepo(db),
		postgres.NewEscrowDepositRepository(db),
		postgres.NewEscrowValidationRunRepository(db),
		postgres.NewEscrowTrustedKeyRepository(db),
		keyProvider, escrowStore, reportStore, limits, escrowDEAName(),
	), nil
}

// NewEscrowValidationActivitiesWithDeps builds the activities from explicit
// dependencies (tests, alternative composition roots).
func NewEscrowValidationActivitiesWithDeps(
	tlds repositories.TLDRepository,
	deposits repositories.EscrowDepositRepository,
	runs repositories.EscrowValidationRunRepository,
	keys repositories.EscrowTrustedKeyRepository,
	keyProvider interfaces.EscrowDecryptionKeyProvider,
	escrowStore, reportStore interfaces.ObjectStore,
	limits rdevalidate.Limits,
	deaName string,
) *EscrowValidationActivities {
	return &EscrowValidationActivities{
		tlds: tlds, deposits: deposits, runs: runs, keys: keys,
		keyProvider: keyProvider, escrowStore: escrowStore, reportStore: reportStore,
		limits: limits, deaName: deaName, now: func() time.Time { return time.Now().UTC() },
	}
}

// ---------------------------------------------------------------------------
// 1. BindDeposit
// ---------------------------------------------------------------------------

// BindDepositInput binds an artifact set to the authenticated tenant/TLD.
// Scope, TLD and profile come from the launch context, never from the artifact.
type BindDepositInput struct {
	Scope             string `json:"scope"`
	TLD               string `json:"tld"`
	Profile           string `json:"profile"`
	ArtifactObjectKey string `json:"artifactObjectKey"`
	// SignatureObjectKey is required for a signed profile and must be empty
	// otherwise.
	SignatureObjectKey string    `json:"signatureObjectKey"`
	SubmittedBy        string    `json:"submittedBy"`
	IntakeRef          string    `json:"intakeRef"`
	ReceivedAt         time.Time `json:"receivedAt"`
	WorkflowID         string    `json:"workflowId"`
	RunID              string    `json:"runId"`
}

// BindDepositOutput identifies the (possibly pre-existing) deposit record and
// the new RUNNING run.
type BindDepositOutput struct {
	DepositID       uuid.UUID       `json:"depositId"`
	ValidationRunID uuid.UUID       `json:"validationRunId"`
	Replay          bool            `json:"replay"` // the same set was already bound
	Profile         string          `json:"profile"`
	ArtifactKey     string          `json:"artifactKey"`
	SignatureKey    string          `json:"signatureKey"`
	ArtifactSHA256  string          `json:"artifactSha256"`
	SignatureSHA256 string          `json:"signatureSha256"`
	ArtifactBytes   int64           `json:"artifactBytes"`
	SignatureBytes  int64           `json:"signatureBytes"`
	Hints           rdereport.Hints `json:"hints"`
}

// BindDeposit verifies TLD ownership, digests the artifacts, binds them to
// an immutable deposit record (reusing an existing one on replay), copies
// them to the immutable prefix once, and opens a RUNNING run.
func (a *EscrowValidationActivities) BindDeposit(ctx context.Context, in BindDepositInput) (BindDepositOutput, error) {
	logger := activity.GetLogger(ctx)
	scope, err := entities.NewOperatorID(in.Scope)
	if err != nil {
		return BindDepositOutput{}, nonRetryable("invalid operator scope", err)
	}
	tld, err := entities.NormalizeEscrowTLD(in.TLD)
	if err != nil {
		return BindDepositOutput{}, nonRetryable("invalid tld", err)
	}
	profile := in.Profile
	if profile == "" {
		profile = entities.EscrowProfileRydeSig
	}
	if !entities.IsEscrowProfile(profile) {
		return BindDepositOutput{}, nonRetryable("unknown validation profile", entities.ErrUnknownEscrowProfile)
	}
	signed := entities.EscrowProfileIsSigned(profile)
	// R3 defence in depth: the launch surface already checked ownership, but a
	// forgetful launch path must not be able to create a record.
	if _, err := a.tlds.GetByNameForOperator(ctx, scope, tld); err != nil {
		if errors.Is(err, entities.ErrTLDNotFound) {
			return BindDepositOutput{}, nonRetryable("tld is not operated by the caller's scope", err)
		}
		return BindDepositOutput{}, fmt.Errorf("BindDeposit: tld lookup: %w", err)
	}
	if strings.TrimSpace(in.ArtifactObjectKey) == "" {
		return BindDepositOutput{}, nonRetryable("the artifact key is required", nil)
	}
	keys := []string{in.ArtifactObjectKey}
	switch {
	case signed && strings.TrimSpace(in.SignatureObjectKey) == "":
		return BindDepositOutput{}, nonRetryable("a signed profile requires a signature key", nil)
	case signed:
		keys = append(keys, in.SignatureObjectKey)
	case strings.TrimSpace(in.SignatureObjectKey) != "":
		return BindDepositOutput{}, nonRetryable("an unsigned profile must not carry a signature key", nil)
	}
	for _, key := range keys {
		ok, err := a.escrowStore.Exists(ctx, key)
		if err != nil {
			return BindDepositOutput{}, fmt.Errorf("BindDeposit: artifact lookup: %w", err)
		}
		if !ok {
			return BindDepositOutput{}, nonRetryable(string(rdevalidate.CodeIntakeArtifactMissing)+": an artifact of the set is not in storage", nil)
		}
	}

	activity.RecordHeartbeat(ctx, "digesting artifacts")
	artifactSHA, artifactBytes, err := a.digest(ctx, in.ArtifactObjectKey, 0)
	if err != nil {
		return BindDepositOutput{}, fmt.Errorf("BindDeposit: digest deposit: %w", err)
	}
	var sigSHA string
	var sigBytes int64
	if signed {
		sigSHA, sigBytes, err = a.digest(ctx, in.SignatureObjectKey, maxSignatureBytes)
		if err != nil {
			if errors.Is(err, errArtifactTooLarge) {
				return BindDepositOutput{}, nonRetryable("signature artifact exceeds the accepted size", nil)
			}
			return BindDepositOutput{}, fmt.Errorf("BindDeposit: digest signature: %w", err)
		}
	}

	replay := true
	deposit, err := a.deposits.FindByDigests(ctx, scope, tld, profile, artifactSHA, sigSHA)
	if errors.Is(err, entities.ErrEscrowDepositNotFound) {
		replay = false
		// The archive prefix embeds the deposit id, so the id is chosen before the
		// entity is built and the constructor validates the final keys.
		depositID := uuid.New()
		prefix := fmt.Sprintf("%s/%s/%s/%s", escrowValidationPrefix, scope.String(), tld, depositID)
		artifactKey := prefix + "/" + artifactFileName(profile, in.ArtifactObjectKey)
		sigKey := ""
		if signed {
			sigKey = prefix + "/deposit.sig"
		}
		deposit, err = entities.NewEscrowDeposit(scope, tld, profile, in.ReceivedAt, in.SubmittedBy, in.IntakeRef,
			artifactKey, artifactSHA, artifactBytes, sigKey, sigSHA, sigBytes)
		if err != nil {
			return BindDepositOutput{}, nonRetryable("deposit record rejected", err)
		}
		deposit.ID = depositID
		activity.RecordHeartbeat(ctx, "archiving artifacts")
		if err := a.copyIfAbsent(ctx, in.ArtifactObjectKey, deposit.ArtifactObjectKey); err != nil {
			return BindDepositOutput{}, fmt.Errorf("BindDeposit: archive deposit: %w", err)
		}
		if signed {
			if err := a.copyIfAbsent(ctx, in.SignatureObjectKey, deposit.SignatureObjectKey); err != nil {
				return BindDepositOutput{}, fmt.Errorf("BindDeposit: archive signature: %w", err)
			}
		}
		if err := a.deposits.Create(ctx, deposit); err != nil {
			// A concurrent bind of the same set won the unique index; use its record.
			existing, ferr := a.deposits.FindByDigests(ctx, scope, tld, profile, artifactSHA, sigSHA)
			if ferr != nil {
				return BindDepositOutput{}, fmt.Errorf("BindDeposit: create deposit: %w", err)
			}
			deposit, replay = existing, true
		}
	} else if err != nil {
		return BindDepositOutput{}, fmt.Errorf("BindDeposit: digest lookup: %w", err)
	}

	run, err := entities.NewEscrowValidationRun(deposit.ID, scope, tld, in.WorkflowID, in.RunID, profile, a.now())
	if err != nil {
		return BindDepositOutput{}, nonRetryable("run record rejected", err)
	}
	if err := a.runs.Create(ctx, run); err != nil {
		return BindDepositOutput{}, fmt.Errorf("BindDeposit: create run: %w", err)
	}
	hints, _ := rdereport.ParseRydeFileName(in.ArtifactObjectKey)

	logger.Info("escrow validation: deposit bound",
		"correlation_id", in.WorkflowID, "deposit_id", deposit.ID.String(), "run_id", run.ID.String(),
		"tld", tld, "profile", profile, "stage", string(rdevalidate.StageIntake),
		"outcome", string(entities.EscrowValidationRunning), "replay", replay)
	return BindDepositOutput{
		DepositID: deposit.ID, ValidationRunID: run.ID, Replay: replay, Profile: profile,
		ArtifactKey: deposit.ArtifactObjectKey, SignatureKey: deposit.SignatureObjectKey,
		ArtifactSHA256: artifactSHA, SignatureSHA256: sigSHA,
		ArtifactBytes: artifactBytes, SignatureBytes: sigBytes, Hints: hints,
	}, nil
}

// artifactFileName keeps the archived name honest about what the artifact is,
// preserving a .xml.gz vs .xml distinction that the profile alone does not
// carry. It never uses the caller's file name verbatim.
func artifactFileName(profile, sourceKey string) string {
	if entities.EscrowProfileIsSigned(profile) {
		return "deposit.ryde"
	}
	if strings.HasSuffix(strings.ToLower(sourceKey), ".gz") {
		return "deposit.xml.gz"
	}
	return "deposit.xml"
}

var errArtifactTooLarge = errors.New("artifact too large")

func (a *EscrowValidationActivities) digest(ctx context.Context, key string, maxBytes int64) (string, int64, error) {
	rc, _, err := a.escrowStore.GetObjectStream(ctx, key)
	if err != nil {
		return "", 0, err
	}
	defer rc.Close()
	h := sha256.New()
	var r io.Reader = rc
	if maxBytes > 0 {
		r = io.LimitReader(rc, maxBytes+1)
	}
	n, err := io.Copy(h, r)
	if err != nil {
		return "", 0, err
	}
	if maxBytes > 0 && n > maxBytes {
		return "", 0, errArtifactTooLarge
	}
	return hex.EncodeToString(h.Sum(nil)), n, nil
}

func (a *EscrowValidationActivities) copyIfAbsent(ctx context.Context, src, dst string) error {
	ok, err := a.escrowStore.Exists(ctx, dst)
	if err != nil {
		return err
	}
	if ok {
		return nil // never overwrite an archived artifact
	}
	return a.escrowStore.CopyObject(ctx, src, dst)
}

// ---------------------------------------------------------------------------
// 2. ValidateArtifacts
// ---------------------------------------------------------------------------

// ValidateArtifactsInput points at the archived artifact set.
type ValidateArtifactsInput struct {
	Scope           string    `json:"scope"`
	TLD             string    `json:"tld"`
	Profile         string    `json:"profile"`
	DepositID       uuid.UUID `json:"depositId"`
	ValidationRunID uuid.UUID `json:"validationRunId"`
	WorkflowID      string    `json:"workflowId"`
	ArtifactKey     string    `json:"artifactKey"`
	SignatureKey    string    `json:"signatureKey"`
	ArtifactSHA256  string    `json:"artifactSha256"`
	SignatureSHA256 string    `json:"signatureSha256"`
}

// ValidateArtifacts runs the streaming pipeline. It returns an error only
// for infrastructure failures that Temporal should retry (key store or
// signature fetch unavailable); everything the deposit does wrong, and the
// service-side conditions the pipeline classifies as ERROR, come back inside
// the Result.
func (a *EscrowValidationActivities) ValidateArtifacts(ctx context.Context, in ValidateArtifactsInput) (rdevalidate.Result, error) {
	logger := activity.GetLogger(ctx)
	scope, err := entities.NewOperatorID(in.Scope)
	if err != nil {
		return rdevalidate.Result{}, nonRetryable("invalid operator scope", err)
	}
	profile := in.Profile
	if profile == "" {
		profile = entities.EscrowProfileRydeSig
	}
	// An unsigned profile establishes nothing about the depositor, so it needs
	// neither trusted signing keys nor the service decryption keyring.
	var armored []string
	var ring openpgp.EntityList
	var sig []byte
	if entities.EscrowProfileIsSigned(profile) {
		trusted, err := a.keys.ListActive(ctx, scope, in.TLD, a.now())
		if err != nil {
			return rdevalidate.Result{}, fmt.Errorf("ValidateArtifacts: trusted keys: %w", err)
		}
		armored = make([]string, len(trusted))
		for i, k := range trusted {
			armored[i] = k.ArmoredPublicKey
		}
		if ring, err = a.keyProvider.DecryptionKeyring(ctx); err != nil {
			// Reported through the pipeline as DECRYPT_KEY_UNAVAILABLE (ERROR class).
			ring = nil
		}
		if sig, err = a.readSmall(ctx, in.SignatureKey, maxSignatureBytes); err != nil {
			return rdevalidate.Result{}, fmt.Errorf("ValidateArtifacts: signature: %w", err)
		}
	}

	runCtx, cancel := context.WithTimeout(ctx, a.limits.Timeout)
	defer cancel()
	res := rdevalidate.Run(runCtx, rdevalidate.Input{
		Profile: profile,
		OpenArtifact: func(c context.Context) (io.ReadCloser, error) {
			rc, _, err := a.escrowStore.GetObjectStream(c, in.ArtifactKey)
			return rc, err
		},
		Sig:                     sig,
		ExpectedArtifactSHA256:  in.ArtifactSHA256,
		ExpectedSignatureSHA256: in.SignatureSHA256,
		TrustedKeys:             armored,
		ServiceKeys:             ring,
		BoundTLD:                in.TLD,
		Limits:                  a.limits,
		Now:                     a.now,
		Heartbeat: func(stage rdevalidate.Stage, detail string) {
			activity.RecordHeartbeat(ctx, string(stage)+": "+detail)
		},
	})

	logger.Info("escrow validation: pipeline finished",
		"correlation_id", in.WorkflowID, "deposit_id", in.DepositID.String(), "run_id", in.ValidationRunID.String(),
		"tld", in.TLD, "stage", string(res.StageReached), "outcome", string(res.Outcome), "codes", codeStrings(res.Codes()))
	return res, nil
}

func (a *EscrowValidationActivities) readSmall(ctx context.Context, key string, maxBytes int64) ([]byte, error) {
	rc, _, err := a.escrowStore.GetObjectStream(ctx, key)
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	b, err := io.ReadAll(io.LimitReader(rc, maxBytes+1))
	if err != nil {
		return nil, err
	}
	if int64(len(b)) > maxBytes {
		return nil, nonRetryable("signature artifact exceeds the accepted size", nil)
	}
	return b, nil
}

// ---------------------------------------------------------------------------
// 3. EmitReportAndNotification
// ---------------------------------------------------------------------------

// EmitReportInput carries a decided result (PASS or FAIL).
type EmitReportInput struct {
	Scope           string             `json:"scope"`
	TLD             string             `json:"tld"`
	DepositID       uuid.UUID          `json:"depositId"`
	ValidationRunID uuid.UUID          `json:"validationRunId"`
	WorkflowID      string             `json:"workflowId"`
	Result          rdevalidate.Result `json:"result"`
	ReceivedAt      time.Time          `json:"receivedAt"`
	ValidatedAt     time.Time          `json:"validatedAt"`
	Hints           rdereport.Hints    `json:"hints"`
}

// EmitReportOutput locates the emitted documents.
type EmitReportOutput struct {
	ReportKey          string `json:"reportKey"`
	NotificationKey    string `json:"notificationKey"`
	NotificationStatus string `json:"notificationStatus"`
}

// EmitReportAndNotification writes the rdeReport and the DVPN/DVFN to the
// reports bucket under the run's own prefix; nothing is ever overwritten
// because the run id is part of the key.
func (a *EscrowValidationActivities) EmitReportAndNotification(ctx context.Context, in EmitReportInput) (EmitReportOutput, error) {
	logger := activity.GetLogger(ctx)
	scope, err := entities.NewOperatorID(in.Scope)
	if err != nil {
		return EmitReportOutput{}, nonRetryable("invalid operator scope", err)
	}
	params := rdereport.Params{
		DEAName: a.deaName, Result: in.Result, BoundTLD: in.TLD,
		ReceivedAt: in.ReceivedAt, ValidatedAt: in.ValidatedAt, Hints: in.Hints,
	}
	notification, err := rdereport.BuildNotification(params)
	if err != nil {
		if errors.Is(err, rdereport.ErrNoNotificationForOutcome) {
			return EmitReportOutput{}, nonRetryable("no notification for this outcome", err)
		}
		return EmitReportOutput{}, nonRetryable("notification could not be built", err)
	}
	reportDoc, err := notification.Report.Marshal()
	if err != nil {
		return EmitReportOutput{}, nonRetryable("report could not be rendered", err)
	}
	notificationDoc, err := notification.Marshal()
	if err != nil {
		return EmitReportOutput{}, nonRetryable("notification could not be rendered", err)
	}
	prefix := fmt.Sprintf("%s/%s/%s/%s/%s", escrowValidationPrefix, scope.String(), in.TLD, in.DepositID, in.ValidationRunID)
	out := EmitReportOutput{
		ReportKey:          prefix + "/report.xml",
		NotificationKey:    prefix + "/notification.xml",
		NotificationStatus: notification.Status,
	}
	if err := a.reportStore.UploadStream(ctx, out.ReportKey, strings.NewReader(string(reportDoc)), "application/xml"); err != nil {
		return EmitReportOutput{}, fmt.Errorf("EmitReportAndNotification: upload report: %w", err)
	}
	if err := a.reportStore.UploadStream(ctx, out.NotificationKey, strings.NewReader(string(notificationDoc)), "application/xml"); err != nil {
		return EmitReportOutput{}, fmt.Errorf("EmitReportAndNotification: upload notification: %w", err)
	}
	logger.Info("escrow validation: notification emitted",
		"correlation_id", in.WorkflowID, "deposit_id", in.DepositID.String(), "run_id", in.ValidationRunID.String(),
		"tld", in.TLD, "stage", "report", "outcome", string(in.Result.Outcome), "notification", out.NotificationStatus)
	return out, nil
}

// ---------------------------------------------------------------------------
// 4. FinalizeValidationRun
// ---------------------------------------------------------------------------

// FinalizeRunInput closes the run exactly once.
type FinalizeRunInput struct {
	Scope              string             `json:"scope"`
	ValidationRunID    uuid.UUID          `json:"validationRunId"`
	WorkflowID         string             `json:"workflowId"`
	Result             rdevalidate.Result `json:"result"`
	ReportKey          string             `json:"reportKey"`
	NotificationKey    string             `json:"notificationKey"`
	NotificationStatus string             `json:"notificationStatus"`
	CompletedAt        time.Time          `json:"completedAt"`
	// Failure, when set, finalises the run as ERROR with a single INTERNAL_ERROR
	// finding (used when an activity failed before a Result existed).
	Failure string `json:"failure,omitempty"`
}

// FinalizeValidationRun persists the terminal state. A run that is already
// final is left untouched and the activity fails non-retryably.
func (a *EscrowValidationActivities) FinalizeValidationRun(ctx context.Context, in FinalizeRunInput) error {
	logger := activity.GetLogger(ctx)
	scope, err := entities.NewOperatorID(in.Scope)
	if err != nil {
		return nonRetryable("invalid operator scope", err)
	}
	run, err := a.runs.GetByID(ctx, scope, in.ValidationRunID)
	if err != nil {
		if errors.Is(err, entities.ErrEscrowValidationRunNotFound) {
			return nonRetryable("run not found for scope", err)
		}
		return fmt.Errorf("FinalizeValidationRun: load run: %w", err)
	}

	res := in.Result
	if in.Failure != "" {
		res = rdevalidate.Result{Profile: rdevalidate.ProfileRydeSig, Outcome: rdevalidate.OutcomeError, StageReached: res.StageReached}
		res.Findings = []rdevalidate.Finding{{
			Code: rdevalidate.CodeInternal, Severity: rdevalidate.SeverityError, Stage: res.StageReached,
			Message: "validation could not complete: " + in.Failure, At: in.CompletedAt,
		}}
	}
	var wm *time.Time
	if !res.Deposit.Watermark.IsZero() {
		t := res.Deposit.Watermark
		wm = &t
	}
	fin := entities.EscrowValidationFinalization{
		Outcome:                  entities.EscrowValidationOutcome(res.Outcome),
		StageReached:             string(res.StageReached),
		Findings:                 rdevalidate.ToEntityFindings(res.Findings),
		SigningKeyFingerprint:    res.Signature.KeyFingerprint,
		DecryptionKeyFingerprint: res.Decryption.KeyFingerprint,
		PlaintextSHA256:          res.Digests.PlaintextSHA256,
		RDEDepositID:             res.Deposit.ID,
		RDEKind:                  res.Deposit.Kind,
		RDEResend:                res.Deposit.Resend,
		RDEWatermark:             wm,
		ReportObjectKey:          in.ReportKey,
		NotificationObjectKey:    in.NotificationKey,
		NotificationStatus:       entities.EscrowNotificationStatus(in.NotificationStatus),
		CompletedAt:              in.CompletedAt,
	}
	if err := run.Finalize(fin); err != nil {
		if errors.Is(err, entities.ErrEscrowValidationRunAlreadyFinal) {
			return nonRetryable("run is already final", err)
		}
		return nonRetryable("finalisation rejected", err)
	}
	if err := a.runs.Finalize(ctx, scope, run); err != nil {
		if errors.Is(err, entities.ErrEscrowValidationRunAlreadyFinal) {
			return nonRetryable("run is already final", err)
		}
		return fmt.Errorf("FinalizeValidationRun: persist: %w", err)
	}
	logger.Info("escrow validation: run finalised",
		"correlation_id", in.WorkflowID, "deposit_id", run.DepositID.String(), "run_id", run.ID.String(),
		"tld", run.TLD, "stage", string(res.StageReached), "outcome", string(run.Outcome), "codes", run.FindingCodes())
	return nil
}

// ---------------------------------------------------------------------------

func nonRetryable(msg string, cause error) error {
	if cause != nil {
		msg = msg + ": " + cause.Error()
	}
	return temporal.NewNonRetryableApplicationError(msg, escrowValidationErrorType, cause)
}

func codeStrings(codes []rdevalidate.Code) []string {
	out := make([]string, len(codes))
	for i, c := range codes {
		out[i] = string(c)
	}
	return out
}
