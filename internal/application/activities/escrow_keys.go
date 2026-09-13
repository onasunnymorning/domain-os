package activities

import (
	"bytes"
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/ProtonMail/go-crypto/openpgp"
	"github.com/google/uuid"
	"github.com/onasunnymorning/domain-os/internal/application/interfaces"
	"github.com/onasunnymorning/domain-os/internal/application/rdesanitize"
	"github.com/onasunnymorning/domain-os/internal/application/rdevalidate"
	"github.com/onasunnymorning/domain-os/internal/infrastructure/db/postgres"
	"github.com/onasunnymorning/domain-os/internal/infrastructure/secrets"
	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
	"github.com/onasunnymorning/domain-os/pkg/domain/repositories"
	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/temporal"
	"gorm.io/gorm"
)

// ---------------------------------------------------------------------------
// Escrow key registry activities — issue #429, ADR-0009.
//
// The probe proves, on a worker, that a key version's material can be fetched
// from the key store and actually used: it is what makes "activate" mean
// "the workers that will run validations hold a working key", not "someone
// uploaded something". Logs carry version ids, fingerprints and constant probe
// codes only — never material, a secret name or a backend message.
// ---------------------------------------------------------------------------

// Probe result codes. They are constant vocabulary: recorded as the audit
// reason and returned to the caller.
const (
	EscrowKeyProbeOK                  = "PROBE_OK"
	EscrowKeyProbeStoreNotConfigured  = "KEY_STORE_NOT_CONFIGURED"
	EscrowKeyProbeStoreUnavailable    = "KEY_STORE_UNAVAILABLE"
	EscrowKeyProbeSecretNotFound      = "SECRET_NOT_FOUND"
	EscrowKeyProbeMaterialUnreadable  = "MATERIAL_UNREADABLE"
	EscrowKeyProbeFingerprintMismatch = "FINGERPRINT_MISMATCH"
	EscrowKeyProbeRoundTripFailed     = "ROUND_TRIP_FAILED"

	escrowKeyProbeRejectedErrorType = "ESCROW_KEY_PROBE_REJECTED"
	// EscrowKeyStoreUnavailableErrorType marks the one retryable probe failure
	// the workflow records as a verdict once retries are exhausted.
	EscrowKeyStoreUnavailableErrorType = "ESCROW_KEY_STORE_UNAVAILABLE"
)

// EscrowKeyActivities holds the dependencies of the key registry workflows.
type EscrowKeyActivities struct {
	versions repositories.EscrowKeyVersionRepository
	loader   interfaces.EscrowKeyMaterialLoader
}

// NewEscrowKeyActivities builds the activities from the environment. A worker
// without a key store still registers them: public keys can still be probed,
// and a private one records KEY_STORE_NOT_CONFIGURED instead of the workflow
// failing on an unregistered activity.
func NewEscrowKeyActivities() (*EscrowKeyActivities, error) {
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
		return nil, fmt.Errorf("escrow key activities: database: %w", err)
	}
	store, err := secrets.NewEscrowSecretStoreFromEnv(context.Background())
	if err != nil && !errors.Is(err, entities.ErrEscrowKeyStoreNotConfigured) {
		return nil, fmt.Errorf("escrow key activities: %w", err)
	}
	return NewEscrowKeyActivitiesWithDeps(postgres.NewEscrowKeyVersionRepository(db), secrets.NewEscrowKeyLoader(store, 0)), nil
}

// NewEscrowKeyActivitiesWithDeps builds the activities from explicit dependencies.
func NewEscrowKeyActivitiesWithDeps(versions repositories.EscrowKeyVersionRepository, loader interfaces.EscrowKeyMaterialLoader) *EscrowKeyActivities {
	return &EscrowKeyActivities{versions: versions, loader: loader}
}

// EscrowKeyOwnerRef names the owner a workflow acts for. The launch surface
// has already authorized the caller for that owner (an Auth0 scope for the
// platform, the operator scope for an operator), exactly as the escrow
// validation workflow carries its operator scope; the activity turns it back
// into a typed scope so every read is still scoped in SQL.
type EscrowKeyOwnerRef struct {
	Kind     string `json:"kind"`               // "platform" | "operator"
	Operator string `json:"operator,omitempty"` // RyID when Kind is operator
}

// Scope converts the reference into a typed key-registry scope.
func (o EscrowKeyOwnerRef) Scope() (entities.EscrowKeyScope, error) {
	switch entities.EscrowKeyOwnerKind(o.Kind) {
	case entities.EscrowKeyOwnerPlatform:
		if o.Operator != "" {
			return entities.EscrowKeyScope{}, entities.ErrInvalidEscrowKeyOwner
		}
		return entities.PlatformEscrowKeyScope(entities.NewPlatformScope()), nil
	case entities.EscrowKeyOwnerOperator:
		op, err := entities.NewOperatorID(o.Operator)
		if err != nil {
			return entities.EscrowKeyScope{}, err
		}
		return entities.OperatorEscrowKeyScope(op), nil
	default:
		return entities.EscrowKeyScope{}, entities.ErrInvalidEscrowKeyOwner
	}
}

// OwnerRefFor returns the reference for an owner.
func OwnerRefFor(o entities.EscrowKeyOwner) EscrowKeyOwnerRef {
	return EscrowKeyOwnerRef{Kind: string(o.Kind), Operator: o.Operator.String()}
}

// ProbeEscrowKeyVersionInput names the version to probe.
type ProbeEscrowKeyVersionInput struct {
	Owner      EscrowKeyOwnerRef `json:"owner"`
	VersionID  string            `json:"versionId"`
	WorkflowID string            `json:"workflowId"`
}

// ProbeEscrowKeyVersionOutput is the probe verdict.
type ProbeEscrowKeyVersionOutput struct {
	OK          bool   `json:"ok"`
	Code        string `json:"code"`
	Fingerprint string `json:"fingerprint"`
}

// ProbeEscrowKeyVersion fetches and exercises a version's material. It writes
// nothing. A store outage is returned as an error so Temporal retries it;
// every other failure is a verdict about the material and is returned as a
// result, because retrying cannot change it.
func (a *EscrowKeyActivities) ProbeEscrowKeyVersion(ctx context.Context, in ProbeEscrowKeyVersionInput) (ProbeEscrowKeyVersionOutput, error) {
	logger := activity.GetLogger(ctx)
	v, err := a.loadProbeable(ctx, in.Owner, in.VersionID)
	if err != nil {
		return ProbeEscrowKeyVersionOutput{}, err
	}
	out := ProbeEscrowKeyVersionOutput{Fingerprint: v.Fingerprint}
	out.Code = a.probe(ctx, v)
	if out.Code == EscrowKeyProbeStoreUnavailable {
		return out, temporal.NewApplicationError("escrow key probe: the key store is unavailable", EscrowKeyStoreUnavailableErrorType)
	}
	out.OK = out.Code == EscrowKeyProbeOK
	logger.Info("escrow key probe: finished", "correlation_id", in.WorkflowID, "key_version_id", v.ID.String(),
		"purpose", string(v.Purpose), "fingerprint", v.Fingerprint, "code", out.Code)
	return out, nil
}

func (a *EscrowKeyActivities) probe(ctx context.Context, v *entities.EscrowKeyVersion) string {
	ref := v.Ref()
	switch v.Policy().Material {
	case entities.EscrowKeyMaterialOpenPGPPublic:
		e, err := rdevalidate.ParseArmoredPublicKey(v.ArmoredPublicKey)
		if err != nil {
			return EscrowKeyProbeMaterialUnreadable
		}
		if rdevalidate.FingerprintHex(e) != v.Fingerprint {
			return EscrowKeyProbeFingerprintMismatch
		}
		return EscrowKeyProbeOK
	case entities.EscrowKeyMaterialOpenPGPPrivate:
		e, err := a.loader.OpenPGPPrivateKey(ctx, ref)
		if err != nil {
			return probeCodeFor(err)
		}
		if !openPGPRoundTrip(e) {
			return EscrowKeyProbeRoundTripFailed
		}
		return EscrowKeyProbeOK
	case entities.EscrowKeyMaterialSymmetric:
		key, err := a.loader.SymmetricKey(ctx, ref)
		if err != nil {
			return probeCodeFor(err)
		}
		// Deriving a tokenizer is exactly what a sanitization run will do.
		if _, err := rdesanitize.NewTokenizer(key, "probe", rdesanitize.PolicyVersion); err != nil {
			return EscrowKeyProbeMaterialUnreadable
		}
		return EscrowKeyProbeOK
	default:
		return EscrowKeyProbeMaterialUnreadable
	}
}

func probeCodeFor(err error) string {
	switch {
	case errors.Is(err, entities.ErrEscrowKeyStoreNotConfigured):
		return EscrowKeyProbeStoreNotConfigured
	case errors.Is(err, entities.ErrEscrowKeyStoreUnavailable):
		return EscrowKeyProbeStoreUnavailable
	case errors.Is(err, entities.ErrEscrowSecretNotFound):
		return EscrowKeyProbeSecretNotFound
	case errors.Is(err, entities.ErrEscrowKeyFingerprintMismatch):
		return EscrowKeyProbeFingerprintMismatch
	default:
		return EscrowKeyProbeMaterialUnreadable
	}
}

// openPGPRoundTrip encrypts a random payload to the key and decrypts it with
// the same key. It proves the key has a usable encryption subkey and that the
// private half unlocked, which is what decrypting a deposit needs.
func openPGPRoundTrip(e *openpgp.Entity) bool {
	nonce := make([]byte, 32)
	if _, err := rand.Read(nonce); err != nil {
		return false
	}
	var buf bytes.Buffer
	w, err := openpgp.Encrypt(&buf, openpgp.EntityList{e}, nil, nil, nil)
	if err != nil {
		return false
	}
	if _, err := w.Write(nonce); err != nil {
		return false
	}
	if err := w.Close(); err != nil {
		return false
	}
	md, err := openpgp.ReadMessage(&buf, openpgp.EntityList{e}, nil, nil)
	if err != nil {
		return false
	}
	got, err := io.ReadAll(md.UnverifiedBody)
	return err == nil && bytes.Equal(got, nonce)
}

// RecordEscrowKeyProbeInput carries a probe verdict to persist.
type RecordEscrowKeyProbeInput struct {
	Owner         EscrowKeyOwnerRef `json:"owner"`
	VersionID     string            `json:"versionId"`
	OK            bool              `json:"ok"`
	Code          string            `json:"code"`
	Actor         string            `json:"actor"`
	WorkflowID    string            `json:"workflowId"`
	TemporalRunID string            `json:"temporalRunId"`
	At            time.Time         `json:"at"`
}

// RecordEscrowKeyProbe stores the verdict on the version with its audit entry
// and outbox event. Every probe is audited, passed or failed: a probe is a
// worker reading secret material, which is exactly what custody audit is for.
func (a *EscrowKeyActivities) RecordEscrowKeyProbe(ctx context.Context, in RecordEscrowKeyProbeInput) error {
	v, err := a.loadProbeable(ctx, in.Owner, in.VersionID)
	if err != nil {
		return err
	}
	before := v.Clone()
	if err := v.RecordProbe(in.OK, in.At); err != nil {
		return temporal.NewNonRetryableApplicationError("escrow key probe cannot be recorded", escrowKeyProbeRejectedErrorType, err)
	}
	action := entities.EscrowAuditVersionProbeFailed
	if in.OK {
		action = entities.EscrowAuditVersionProbePassed
	}
	ev, err := entities.NewEscrowKeyVersionAuditEvent(entities.EscrowKeyAuditContext{
		Actor: in.Actor, TraceID: in.TemporalRunID, CorrelationID: in.WorkflowID, At: in.At,
	}, before, v, action)
	if err != nil {
		return temporal.NewNonRetryableApplicationError("escrow key probe audit rejected", escrowKeyProbeRejectedErrorType, err)
	}
	ev.Reason = in.Code
	// A concurrent change (say, a revocation) makes this a conflict; the retry
	// reloads the version and either records against the new state or stops.
	if err := a.versions.ApplyTransitions(ctx, []entities.EscrowKeyVersionTransition{{Version: v, ExpectedState: before.State}}, []*entities.EscrowKeyAuditEvent{ev}); err != nil {
		return fmt.Errorf("RecordEscrowKeyProbe(version=%s): %w", in.VersionID, err)
	}
	return nil
}

func (a *EscrowKeyActivities) loadProbeable(ctx context.Context, owner EscrowKeyOwnerRef, rawID string) (*entities.EscrowKeyVersion, error) {
	id, err := uuid.Parse(rawID)
	if err != nil {
		return nil, temporal.NewNonRetryableApplicationError("invalid key version id", escrowKeyProbeRejectedErrorType, err)
	}
	scope, err := owner.Scope()
	if err != nil {
		return nil, temporal.NewNonRetryableApplicationError("invalid key owner", escrowKeyProbeRejectedErrorType, err)
	}
	v, err := a.versions.GetByID(ctx, scope, id)
	if errors.Is(err, entities.ErrEscrowKeyVersionNotFound) {
		return nil, temporal.NewNonRetryableApplicationError("key version not found", escrowKeyProbeRejectedErrorType, err)
	}
	if err != nil {
		return nil, fmt.Errorf("escrow key probe: load version %s: %w", id, err)
	}
	if !scope.CanManage(v.Owner) {
		// An operator scope can see platform versions but must not probe them.
		return nil, temporal.NewNonRetryableApplicationError("key version not found", escrowKeyProbeRejectedErrorType, entities.ErrEscrowKeyVersionNotFound)
	}
	if !v.UsableForRecordedRun() {
		return nil, temporal.NewNonRetryableApplicationError("key version is "+string(v.State)+" and cannot be probed", escrowKeyProbeRejectedErrorType, entities.ErrEscrowKeyInvalidTransition)
	}
	return v, nil
}
