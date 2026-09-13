package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
	"github.com/onasunnymorning/domain-os/pkg/domain/queries"
	"gorm.io/gorm"
)

// ---------------------------------------------------------------------------
// GORM models
//
// Operator tenant isolation is enforced here, in SQL: every scoped query
// carries `tenant_id = ?` bound to the entities.OperatorID passed in
// (ADR-0006). Deposits are immutable by absence of any update path; a run is
// finalised by exactly one conditional UPDATE.
// ---------------------------------------------------------------------------

// EscrowDepositRecord is the GORM model for the escrow_deposits table.
type EscrowDepositRecord struct {
	ID          uuid.UUID `gorm:"type:uuid;primaryKey"`
	TenantID    string    `gorm:"not null;index"`
	TLD         string    `gorm:"not null;index"`
	Profile     string    `gorm:"not null;default:'ryde+sig'"`
	ReceivedAt  time.Time `gorm:"not null"`
	SubmittedBy string    `gorm:"not null"`
	IntakeRef   string
	// Artifact* is the deposit itself; Signature* is the detached signature and
	// is empty for unsigned profiles (issue #415).
	ArtifactObjectKey  string `gorm:"not null"`
	ArtifactSHA256     string `gorm:"not null;size:64"`
	ArtifactBytes      int64  `gorm:"not null"`
	SignatureObjectKey string
	SignatureSHA256    string `gorm:"size:64"`
	SignatureBytes     int64
	CreatedAt          time.Time
}

// TableName specifies the table name for EscrowDepositRecord.
func (EscrowDepositRecord) TableName() string { return "escrow_deposits" }

// EscrowValidationRunRecord is the GORM model for the escrow_validation_runs table.
type EscrowValidationRunRecord struct {
	ID                       uuid.UUID `gorm:"type:uuid;primaryKey"`
	DepositID                uuid.UUID `gorm:"type:uuid;not null;index"`
	TenantID                 string    `gorm:"not null;index"`
	TLD                      string    `gorm:"not null;index"`
	WorkflowID               string    `gorm:"not null;index"`
	RunID                    string
	Profile                  string `gorm:"not null"`
	Outcome                  string `gorm:"not null;index"`
	StageReached             string
	Findings                 []byte `gorm:"type:jsonb"`
	FindingTally             []byte `gorm:"type:jsonb"`
	SigningKeyFingerprint    string
	DecryptionKeyFingerprint string
	// KeyEvidence is entities.EscrowRunKeyEvidence; the two version ids are
	// also columns so "runs that used this key version" is an indexed query.
	KeyEvidence            []byte     `gorm:"type:jsonb"`
	SigningKeyVersionID    *uuid.UUID `gorm:"type:uuid;index"`
	DecryptionKeyVersionID *uuid.UUID `gorm:"type:uuid;index"`
	PlaintextSHA256        string
	RDEDepositID           string
	RDEKind                string
	RDEResend              int
	RDEWatermark           *time.Time
	SummaryObjectKey       string
	ReportObjectKey        string
	NotificationObjectKey  string
	NotificationStatus     string
	StartedAt              time.Time `gorm:"not null;index"`
	CompletedAt            *time.Time
}

// TableName specifies the table name for EscrowValidationRunRecord.
func (EscrowValidationRunRecord) TableName() string { return "escrow_validation_runs" }

// ---------- mappers ----------

func toDBEscrowDeposit(d *entities.EscrowDeposit) *EscrowDepositRecord {
	return &EscrowDepositRecord{
		ID: d.ID, TenantID: d.TenantID.String(), TLD: d.TLD, Profile: d.Profile, ReceivedAt: d.ReceivedAt,
		SubmittedBy: d.SubmittedBy, IntakeRef: d.IntakeRef,
		ArtifactObjectKey: d.ArtifactObjectKey, ArtifactSHA256: d.ArtifactSHA256, ArtifactBytes: d.ArtifactBytes,
		SignatureObjectKey: d.SignatureObjectKey, SignatureSHA256: d.SignatureSHA256, SignatureBytes: d.SignatureBytes,
		CreatedAt: d.CreatedAt,
	}
}

func fromDBEscrowDeposit(r *EscrowDepositRecord) *entities.EscrowDeposit {
	return &entities.EscrowDeposit{
		ID: r.ID, TenantID: entities.OperatorID(r.TenantID), TLD: r.TLD, Profile: r.Profile, ReceivedAt: r.ReceivedAt,
		SubmittedBy: r.SubmittedBy, IntakeRef: r.IntakeRef,
		ArtifactObjectKey: r.ArtifactObjectKey, ArtifactSHA256: r.ArtifactSHA256, ArtifactBytes: r.ArtifactBytes,
		SignatureObjectKey: r.SignatureObjectKey, SignatureSHA256: r.SignatureSHA256, SignatureBytes: r.SignatureBytes,
		CreatedAt: r.CreatedAt,
	}
}

func toDBEscrowValidationRun(r *entities.EscrowValidationRun) (*EscrowValidationRunRecord, error) {
	findings := r.Findings
	if findings == nil {
		findings = []entities.EscrowFinding{}
	}
	raw, err := json.Marshal(findings)
	if err != nil {
		return nil, fmt.Errorf("encode findings: %w", err)
	}
	tally := r.FindingTally
	if tally == nil {
		tally = []entities.EscrowFindingTally{}
	}
	rawTally, err := json.Marshal(tally)
	if err != nil {
		return nil, fmt.Errorf("encode finding tally: %w", err)
	}
	keyEvidence, err := json.Marshal(r.Keys)
	if err != nil {
		return nil, fmt.Errorf("encode key evidence: %w", err)
	}
	return &EscrowValidationRunRecord{
		KeyEvidence: keyEvidence, SigningKeyVersionID: r.Keys.SigningKeyVersionID, DecryptionKeyVersionID: r.Keys.DecryptionKeyVersionID,
		ID: r.ID, DepositID: r.DepositID, TenantID: r.TenantID.String(), TLD: r.TLD,
		WorkflowID: r.WorkflowID, RunID: r.RunID, Profile: r.Profile,
		Outcome: string(r.Outcome), StageReached: r.StageReached, Findings: raw, FindingTally: rawTally,
		SigningKeyFingerprint: r.SigningKeyFingerprint, DecryptionKeyFingerprint: r.DecryptionKeyFingerprint,
		PlaintextSHA256: r.PlaintextSHA256,
		RDEDepositID:    r.RDEDepositID, RDEKind: r.RDEKind, RDEResend: r.RDEResend, RDEWatermark: r.RDEWatermark,
		SummaryObjectKey: r.SummaryObjectKey, ReportObjectKey: r.ReportObjectKey, NotificationObjectKey: r.NotificationObjectKey,
		NotificationStatus: string(r.NotificationStatus),
		StartedAt:          r.StartedAt, CompletedAt: r.CompletedAt,
	}, nil
}

func fromDBEscrowValidationRun(r *EscrowValidationRunRecord) (*entities.EscrowValidationRun, error) {
	findings := []entities.EscrowFinding{}
	if len(r.Findings) > 0 {
		if err := json.Unmarshal(r.Findings, &findings); err != nil {
			return nil, fmt.Errorf("decode findings: %w", err)
		}
	}
	tally := []entities.EscrowFindingTally{}
	if len(r.FindingTally) > 0 {
		if err := json.Unmarshal(r.FindingTally, &tally); err != nil {
			return nil, fmt.Errorf("decode finding tally: %w", err)
		}
	}
	var keys entities.EscrowRunKeyEvidence
	if len(r.KeyEvidence) > 0 {
		if err := json.Unmarshal(r.KeyEvidence, &keys); err != nil {
			return nil, fmt.Errorf("decode key evidence: %w", err)
		}
	}
	return &entities.EscrowValidationRun{
		Keys: keys,
		ID:   r.ID, DepositID: r.DepositID, TenantID: entities.OperatorID(r.TenantID), TLD: r.TLD,
		WorkflowID: r.WorkflowID, RunID: r.RunID, Profile: r.Profile,
		Outcome: entities.EscrowValidationOutcome(r.Outcome), StageReached: r.StageReached, Findings: findings, FindingTally: tally,
		SigningKeyFingerprint: r.SigningKeyFingerprint, DecryptionKeyFingerprint: r.DecryptionKeyFingerprint,
		PlaintextSHA256: r.PlaintextSHA256,
		RDEDepositID:    r.RDEDepositID, RDEKind: r.RDEKind, RDEResend: r.RDEResend, RDEWatermark: r.RDEWatermark,
		SummaryObjectKey: r.SummaryObjectKey, ReportObjectKey: r.ReportObjectKey, NotificationObjectKey: r.NotificationObjectKey,
		NotificationStatus: entities.EscrowNotificationStatus(r.NotificationStatus),
		StartedAt:          r.StartedAt, CompletedAt: r.CompletedAt,
	}, nil
}

// ---------------------------------------------------------------------------
// Deposits
// ---------------------------------------------------------------------------

// GormEscrowDepositRepository implements repositories.EscrowDepositRepository.
type GormEscrowDepositRepository struct{ db *gorm.DB }

// NewEscrowDepositRepository creates a new GormEscrowDepositRepository.
func NewEscrowDepositRepository(db *gorm.DB) *GormEscrowDepositRepository {
	return &GormEscrowDepositRepository{db: db}
}

// Create persists a deposit. The (tenant, tld, digests) unique index makes a
// concurrent duplicate bind fail loudly instead of producing two records.
func (r *GormEscrowDepositRepository) Create(ctx context.Context, d *entities.EscrowDeposit) error {
	if err := r.db.WithContext(ctx).Create(toDBEscrowDeposit(d)).Error; err != nil {
		return fmt.Errorf("EscrowDeposit.Create(id=%s): %w", d.ID, err)
	}
	return nil
}

// GetByID retrieves a deposit by tenant and ID.
func (r *GormEscrowDepositRepository) GetByID(ctx context.Context, scope entities.OperatorID, id uuid.UUID) (*entities.EscrowDeposit, error) {
	var rec EscrowDepositRecord
	err := r.db.WithContext(ctx).Where("id = ? AND tenant_id = ?", id, scope.String()).First(&rec).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, entities.ErrEscrowDepositNotFound
		}
		return nil, fmt.Errorf("EscrowDeposit.GetByID(id=%s): %w", id, err)
	}
	return fromDBEscrowDeposit(&rec), nil
}

// FindByDigests returns the deposit bound to this exact artifact set.
func (r *GormEscrowDepositRepository) FindByDigests(ctx context.Context, scope entities.OperatorID, tld, profile, artifactSHA256, signatureSHA256 string) (*entities.EscrowDeposit, error) {
	var rec EscrowDepositRecord
	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND tld = ? AND profile = ? AND artifact_sha256 = ? AND signature_sha256 = ?",
			scope.String(), tld, profile, artifactSHA256, signatureSHA256).
		First(&rec).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, entities.ErrEscrowDepositNotFound
		}
		return nil, fmt.Errorf("EscrowDeposit.FindByDigests(tld=%s): %w", tld, err)
	}
	return fromDBEscrowDeposit(&rec), nil
}

// List returns deposits for a tenant, newest first, cursor-paginated (INV-11).
func (r *GormEscrowDepositRepository) List(ctx context.Context, scope entities.OperatorID, q queries.ListItemsQuery) ([]*entities.EscrowDeposit, string, error) {
	pageSize := clampPageSize(q.PageSize)
	query := r.db.WithContext(ctx).Where("tenant_id = ?", scope.String())
	if f, ok := q.Filter.(queries.ListEscrowDepositsFilter); ok && f.TLDEquals != "" {
		query = query.Where("tld = ?", f.TLDEquals)
	}
	if q.PageCursor != "" {
		t, id, err := decodeObsCursor(q.PageCursor)
		if err != nil {
			return nil, "", fmt.Errorf("EscrowDeposit.List(cursor): %w", err)
		}
		query = query.Where("(received_at < ? OR (received_at = ? AND id < ?))", t, t, id)
	}
	var records []EscrowDepositRecord
	if err := query.Order("received_at DESC, id DESC").Limit(pageSize + 1).Find(&records).Error; err != nil {
		return nil, "", fmt.Errorf("EscrowDeposit.List: %w", err)
	}
	var next string
	if len(records) > pageSize {
		last := records[pageSize-1]
		next = encodeObsCursor(last.ReceivedAt, last.ID)
		records = records[:pageSize]
	}
	out := make([]*entities.EscrowDeposit, len(records))
	for i := range records {
		out[i] = fromDBEscrowDeposit(&records[i])
	}
	return out, next, nil
}

// ---------------------------------------------------------------------------
// Validation runs
// ---------------------------------------------------------------------------

// GormEscrowValidationRunRepository implements repositories.EscrowValidationRunRepository.
type GormEscrowValidationRunRepository struct{ db *gorm.DB }

// NewEscrowValidationRunRepository creates a new GormEscrowValidationRunRepository.
func NewEscrowValidationRunRepository(db *gorm.DB) *GormEscrowValidationRunRepository {
	return &GormEscrowValidationRunRepository{db: db}
}

// Create persists a RUNNING run.
func (r *GormEscrowValidationRunRepository) Create(ctx context.Context, run *entities.EscrowValidationRun) error {
	rec, err := toDBEscrowValidationRun(run)
	if err != nil {
		return fmt.Errorf("EscrowValidationRun.Create(id=%s): %w", run.ID, err)
	}
	if err := r.db.WithContext(ctx).Create(rec).Error; err != nil {
		return fmt.Errorf("EscrowValidationRun.Create(id=%s): %w", run.ID, err)
	}
	return nil
}

// Finalize writes the terminal state with a single conditional UPDATE that
// only matches a RUNNING row for this tenant. Zero rows affected means the
// run was already final (or is not this tenant's), and the earlier outcome is
// left untouched.
func (r *GormEscrowValidationRunRepository) Finalize(ctx context.Context, scope entities.OperatorID, run *entities.EscrowValidationRun) error {
	if !run.IsFinal() {
		return fmt.Errorf("EscrowValidationRun.Finalize(id=%s): %w", run.ID, entities.ErrInvalidEscrowValidationRun)
	}
	rec, err := toDBEscrowValidationRun(run)
	if err != nil {
		return fmt.Errorf("EscrowValidationRun.Finalize(id=%s): %w", run.ID, err)
	}
	res := r.db.WithContext(ctx).Model(&EscrowValidationRunRecord{}).
		Where("id = ? AND tenant_id = ? AND outcome = ?", run.ID, scope.String(), string(entities.EscrowValidationRunning)).
		Updates(map[string]interface{}{
			"outcome":                    rec.Outcome,
			"stage_reached":              rec.StageReached,
			"findings":                   rec.Findings,
			"finding_tally":              rec.FindingTally,
			"signing_key_fingerprint":    rec.SigningKeyFingerprint,
			"decryption_key_fingerprint": rec.DecryptionKeyFingerprint,
			"key_evidence":               rec.KeyEvidence,
			"signing_key_version_id":     rec.SigningKeyVersionID,
			"decryption_key_version_id":  rec.DecryptionKeyVersionID,
			"plaintext_sha256":           rec.PlaintextSHA256,
			"rde_deposit_id":             rec.RDEDepositID,
			"rde_kind":                   rec.RDEKind,
			"rde_resend":                 rec.RDEResend,
			"rde_watermark":              rec.RDEWatermark,
			"summary_object_key":         rec.SummaryObjectKey,
			"report_object_key":          rec.ReportObjectKey,
			"notification_object_key":    rec.NotificationObjectKey,
			"notification_status":        rec.NotificationStatus,
			"completed_at":               rec.CompletedAt,
		})
	if res.Error != nil {
		return fmt.Errorf("EscrowValidationRun.Finalize(id=%s): %w", run.ID, res.Error)
	}
	if res.RowsAffected == 0 {
		return entities.ErrEscrowValidationRunAlreadyFinal
	}
	return nil
}

// GetByID retrieves a run by tenant and ID.
func (r *GormEscrowValidationRunRepository) GetByID(ctx context.Context, scope entities.OperatorID, id uuid.UUID) (*entities.EscrowValidationRun, error) {
	var rec EscrowValidationRunRecord
	err := r.db.WithContext(ctx).Where("id = ? AND tenant_id = ?", id, scope.String()).First(&rec).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, entities.ErrEscrowValidationRunNotFound
		}
		return nil, fmt.Errorf("EscrowValidationRun.GetByID(id=%s): %w", id, err)
	}
	return fromDBEscrowValidationRun(&rec)
}

// ListByDeposit returns every run for a deposit, newest first.
func (r *GormEscrowValidationRunRepository) ListByDeposit(ctx context.Context, scope entities.OperatorID, depositID uuid.UUID) ([]*entities.EscrowValidationRun, error) {
	var records []EscrowValidationRunRecord
	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND deposit_id = ?", scope.String(), depositID).
		Order("started_at DESC, id DESC").
		Find(&records).Error
	if err != nil {
		return nil, fmt.Errorf("EscrowValidationRun.ListByDeposit(depositID=%s): %w", depositID, err)
	}
	return runsFromRecords(records)
}

// List returns runs for a tenant, newest first, cursor-paginated (INV-11).
func (r *GormEscrowValidationRunRepository) List(ctx context.Context, scope entities.OperatorID, q queries.ListItemsQuery) ([]*entities.EscrowValidationRun, string, error) {
	pageSize := clampPageSize(q.PageSize)
	query := r.db.WithContext(ctx).Where("tenant_id = ?", scope.String())
	if f, ok := q.Filter.(queries.ListEscrowValidationRunsFilter); ok {
		if f.TLDEquals != "" {
			query = query.Where("tld = ?", f.TLDEquals)
		}
		if f.OutcomeEquals != "" {
			query = query.Where("outcome = ?", f.OutcomeEquals)
		}
		if f.KeyVersionIDEquals != "" {
			id, err := uuid.Parse(f.KeyVersionIDEquals)
			if err != nil {
				return nil, "", fmt.Errorf("EscrowValidationRun.List(keyVersionId): %w", err)
			}
			query = query.Where("(signing_key_version_id = ? OR decryption_key_version_id = ?)", id, id)
		}
	}
	if q.PageCursor != "" {
		t, id, err := decodeObsCursor(q.PageCursor)
		if err != nil {
			return nil, "", fmt.Errorf("EscrowValidationRun.List(cursor): %w", err)
		}
		query = query.Where("(started_at < ? OR (started_at = ? AND id < ?))", t, t, id)
	}
	var records []EscrowValidationRunRecord
	if err := query.Order("started_at DESC, id DESC").Limit(pageSize + 1).Find(&records).Error; err != nil {
		return nil, "", fmt.Errorf("EscrowValidationRun.List: %w", err)
	}
	var next string
	if len(records) > pageSize {
		last := records[pageSize-1]
		next = encodeObsCursor(last.StartedAt, last.ID)
		records = records[:pageSize]
	}
	out, err := runsFromRecords(records)
	return out, next, err
}

func runsFromRecords(records []EscrowValidationRunRecord) ([]*entities.EscrowValidationRun, error) {
	out := make([]*entities.EscrowValidationRun, len(records))
	for i := range records {
		run, err := fromDBEscrowValidationRun(&records[i])
		if err != nil {
			return nil, err
		}
		out[i] = run
	}
	return out, nil
}

func clampPageSize(n int) int {
	if n <= 0 {
		return 50
	}
	if n > 200 {
		return 200
	}
	return n
}
