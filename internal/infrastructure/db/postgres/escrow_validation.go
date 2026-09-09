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
// (ADR-0006). Deposits and trusted keys are immutable by absence of any
// update path; a run is finalised by exactly one conditional UPDATE.
// ---------------------------------------------------------------------------

// EscrowDepositRecord is the GORM model for the escrow_deposits table.
type EscrowDepositRecord struct {
	ID            uuid.UUID `gorm:"type:uuid;primaryKey"`
	TenantID      string    `gorm:"not null;index"`
	TLD           string    `gorm:"not null;index"`
	ReceivedAt    time.Time `gorm:"not null"`
	SubmittedBy   string    `gorm:"not null"`
	IntakeRef     string
	RydeObjectKey string `gorm:"not null"`
	SigObjectKey  string `gorm:"not null"`
	RydeSHA256    string `gorm:"not null;size:64"`
	SigSHA256     string `gorm:"not null;size:64"`
	RydeBytes     int64  `gorm:"not null"`
	SigBytes      int64  `gorm:"not null"`
	CreatedAt     time.Time
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
	SigningKeyFingerprint    string
	DecryptionKeyFingerprint string
	PlaintextSHA256          string
	RDEDepositID             string
	RDEKind                  string
	RDEResend                int
	RDEWatermark             *time.Time
	ReportObjectKey          string
	NotificationObjectKey    string
	NotificationStatus       string
	StartedAt                time.Time `gorm:"not null;index"`
	CompletedAt              *time.Time
}

// TableName specifies the table name for EscrowValidationRunRecord.
func (EscrowValidationRunRecord) TableName() string { return "escrow_validation_runs" }

// EscrowTrustedKeyRecord is the GORM model for the escrow_trusted_keys table.
type EscrowTrustedKeyRecord struct {
	ID               uuid.UUID `gorm:"type:uuid;primaryKey"`
	TenantID         string    `gorm:"not null;index"`
	TLD              string    `gorm:"not null;index"`
	Fingerprint      string    `gorm:"not null;size:64"`
	ArmoredPublicKey string    `gorm:"not null;type:text"`
	Label            string
	ValidFrom        time.Time `gorm:"not null"`
	ValidTo          *time.Time
	RetiredAt        *time.Time
	CreatedAt        time.Time
}

// TableName specifies the table name for EscrowTrustedKeyRecord.
func (EscrowTrustedKeyRecord) TableName() string { return "escrow_trusted_keys" }

// ---------- mappers ----------

func toDBEscrowDeposit(d *entities.EscrowDeposit) *EscrowDepositRecord {
	return &EscrowDepositRecord{
		ID: d.ID, TenantID: d.TenantID.String(), TLD: d.TLD, ReceivedAt: d.ReceivedAt,
		SubmittedBy: d.SubmittedBy, IntakeRef: d.IntakeRef,
		RydeObjectKey: d.RydeObjectKey, SigObjectKey: d.SigObjectKey,
		RydeSHA256: d.RydeSHA256, SigSHA256: d.SigSHA256, RydeBytes: d.RydeBytes, SigBytes: d.SigBytes,
		CreatedAt: d.CreatedAt,
	}
}

func fromDBEscrowDeposit(r *EscrowDepositRecord) *entities.EscrowDeposit {
	return &entities.EscrowDeposit{
		ID: r.ID, TenantID: entities.OperatorID(r.TenantID), TLD: r.TLD, ReceivedAt: r.ReceivedAt,
		SubmittedBy: r.SubmittedBy, IntakeRef: r.IntakeRef,
		RydeObjectKey: r.RydeObjectKey, SigObjectKey: r.SigObjectKey,
		RydeSHA256: r.RydeSHA256, SigSHA256: r.SigSHA256, RydeBytes: r.RydeBytes, SigBytes: r.SigBytes,
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
	return &EscrowValidationRunRecord{
		ID: r.ID, DepositID: r.DepositID, TenantID: r.TenantID.String(), TLD: r.TLD,
		WorkflowID: r.WorkflowID, RunID: r.RunID, Profile: r.Profile,
		Outcome: string(r.Outcome), StageReached: r.StageReached, Findings: raw,
		SigningKeyFingerprint: r.SigningKeyFingerprint, DecryptionKeyFingerprint: r.DecryptionKeyFingerprint,
		PlaintextSHA256: r.PlaintextSHA256,
		RDEDepositID:    r.RDEDepositID, RDEKind: r.RDEKind, RDEResend: r.RDEResend, RDEWatermark: r.RDEWatermark,
		ReportObjectKey: r.ReportObjectKey, NotificationObjectKey: r.NotificationObjectKey,
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
	return &entities.EscrowValidationRun{
		ID: r.ID, DepositID: r.DepositID, TenantID: entities.OperatorID(r.TenantID), TLD: r.TLD,
		WorkflowID: r.WorkflowID, RunID: r.RunID, Profile: r.Profile,
		Outcome: entities.EscrowValidationOutcome(r.Outcome), StageReached: r.StageReached, Findings: findings,
		SigningKeyFingerprint: r.SigningKeyFingerprint, DecryptionKeyFingerprint: r.DecryptionKeyFingerprint,
		PlaintextSHA256: r.PlaintextSHA256,
		RDEDepositID:    r.RDEDepositID, RDEKind: r.RDEKind, RDEResend: r.RDEResend, RDEWatermark: r.RDEWatermark,
		ReportObjectKey: r.ReportObjectKey, NotificationObjectKey: r.NotificationObjectKey,
		NotificationStatus: entities.EscrowNotificationStatus(r.NotificationStatus),
		StartedAt:          r.StartedAt, CompletedAt: r.CompletedAt,
	}, nil
}

func toDBEscrowTrustedKey(k *entities.EscrowTrustedKey) *EscrowTrustedKeyRecord {
	return &EscrowTrustedKeyRecord{
		ID: k.ID, TenantID: k.TenantID.String(), TLD: k.TLD, Fingerprint: k.Fingerprint,
		ArmoredPublicKey: k.ArmoredPublicKey, Label: k.Label,
		ValidFrom: k.ValidFrom, ValidTo: k.ValidTo, RetiredAt: k.RetiredAt, CreatedAt: k.CreatedAt,
	}
}

func fromDBEscrowTrustedKey(r *EscrowTrustedKeyRecord) *entities.EscrowTrustedKey {
	return &entities.EscrowTrustedKey{
		ID: r.ID, TenantID: entities.OperatorID(r.TenantID), TLD: r.TLD, Fingerprint: r.Fingerprint,
		ArmoredPublicKey: r.ArmoredPublicKey, Label: r.Label,
		ValidFrom: r.ValidFrom, ValidTo: r.ValidTo, RetiredAt: r.RetiredAt, CreatedAt: r.CreatedAt,
	}
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

// FindByDigests returns the deposit bound to this exact artifact pair.
func (r *GormEscrowDepositRepository) FindByDigests(ctx context.Context, scope entities.OperatorID, tld, rydeSHA256, sigSHA256 string) (*entities.EscrowDeposit, error) {
	var rec EscrowDepositRecord
	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND tld = ? AND ryde_sha256 = ? AND sig_sha256 = ?", scope.String(), tld, rydeSHA256, sigSHA256).
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
			"signing_key_fingerprint":    rec.SigningKeyFingerprint,
			"decryption_key_fingerprint": rec.DecryptionKeyFingerprint,
			"plaintext_sha256":           rec.PlaintextSHA256,
			"rde_deposit_id":             rec.RDEDepositID,
			"rde_kind":                   rec.RDEKind,
			"rde_resend":                 rec.RDEResend,
			"rde_watermark":              rec.RDEWatermark,
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

// ---------------------------------------------------------------------------
// Trusted keys
// ---------------------------------------------------------------------------

// GormEscrowTrustedKeyRepository implements repositories.EscrowTrustedKeyRepository.
type GormEscrowTrustedKeyRepository struct{ db *gorm.DB }

// NewEscrowTrustedKeyRepository creates a new GormEscrowTrustedKeyRepository.
func NewEscrowTrustedKeyRepository(db *gorm.DB) *GormEscrowTrustedKeyRepository {
	return &GormEscrowTrustedKeyRepository{db: db}
}

// Create persists a trusted key registration.
func (r *GormEscrowTrustedKeyRepository) Create(ctx context.Context, k *entities.EscrowTrustedKey) error {
	if err := r.db.WithContext(ctx).Create(toDBEscrowTrustedKey(k)).Error; err != nil {
		return fmt.Errorf("EscrowTrustedKey.Create(id=%s): %w", k.ID, err)
	}
	return nil
}

// Retire stamps retired_at on a key that is not yet retired.
func (r *GormEscrowTrustedKeyRepository) Retire(ctx context.Context, scope entities.OperatorID, id uuid.UUID, at time.Time) error {
	res := r.db.WithContext(ctx).Model(&EscrowTrustedKeyRecord{}).
		Where("id = ? AND tenant_id = ? AND retired_at IS NULL", id, scope.String()).
		Update("retired_at", at.UTC())
	if res.Error != nil {
		return fmt.Errorf("EscrowTrustedKey.Retire(id=%s): %w", id, res.Error)
	}
	if res.RowsAffected == 0 {
		// Either not this tenant's key or already retired; distinguish for the caller.
		var rec EscrowTrustedKeyRecord
		if err := r.db.WithContext(ctx).Where("id = ? AND tenant_id = ?", id, scope.String()).First(&rec).Error; err != nil {
			return entities.ErrEscrowTrustedKeyNotFound
		}
		return entities.ErrEscrowTrustedKeyAlreadyRetired
	}
	return nil
}

// GetByID retrieves a trusted key by tenant and ID.
func (r *GormEscrowTrustedKeyRepository) GetByID(ctx context.Context, scope entities.OperatorID, id uuid.UUID) (*entities.EscrowTrustedKey, error) {
	var rec EscrowTrustedKeyRecord
	err := r.db.WithContext(ctx).Where("id = ? AND tenant_id = ?", id, scope.String()).First(&rec).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, entities.ErrEscrowTrustedKeyNotFound
		}
		return nil, fmt.Errorf("EscrowTrustedKey.GetByID(id=%s): %w", id, err)
	}
	return fromDBEscrowTrustedKey(&rec), nil
}

// ListActive returns the keys usable to verify a signature at the instant.
func (r *GormEscrowTrustedKeyRepository) ListActive(ctx context.Context, scope entities.OperatorID, tld string, at time.Time) ([]*entities.EscrowTrustedKey, error) {
	at = at.UTC()
	var records []EscrowTrustedKeyRecord
	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND tld = ? AND valid_from <= ? AND (valid_to IS NULL OR valid_to > ?) AND (retired_at IS NULL OR retired_at > ?)",
			scope.String(), tld, at, at, at).
		Order("valid_from DESC, id DESC").
		Find(&records).Error
	if err != nil {
		return nil, fmt.Errorf("EscrowTrustedKey.ListActive(tld=%s): %w", tld, err)
	}
	return keysFromRecords(records), nil
}

// List returns every key registered for a tenant/TLD, newest first.
func (r *GormEscrowTrustedKeyRepository) List(ctx context.Context, scope entities.OperatorID, tld string) ([]*entities.EscrowTrustedKey, error) {
	var records []EscrowTrustedKeyRecord
	query := r.db.WithContext(ctx).Where("tenant_id = ?", scope.String())
	if tld != "" {
		query = query.Where("tld = ?", tld)
	}
	if err := query.Order("valid_from DESC, id DESC").Find(&records).Error; err != nil {
		return nil, fmt.Errorf("EscrowTrustedKey.List(tld=%s): %w", tld, err)
	}
	return keysFromRecords(records), nil
}

func keysFromRecords(records []EscrowTrustedKeyRecord) []*entities.EscrowTrustedKey {
	out := make([]*entities.EscrowTrustedKey, len(records))
	for i := range records {
		out[i] = fromDBEscrowTrustedKey(&records[i])
	}
	return out
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
