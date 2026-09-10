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

// EscrowSanitizationRunRecord is the GORM model for escrow_sanitization_runs
// (issue #415). Findings, the finding tally and counts are jsonb: each is
// small, bounded and only ever read as a whole.
type EscrowSanitizationRunRecord struct {
	ID       uuid.UUID `gorm:"type:uuid;primaryKey"`
	TenantID string    `gorm:"not null;index"`
	TLD      string    `gorm:"not null;index"`

	SourceValidationRunID uuid.UUID `gorm:"type:uuid;not null;index"`
	SourceDepositID       uuid.UUID `gorm:"type:uuid;not null;index"`
	SourceArtifactSHA256  string    `gorm:"not null;size:64"`

	PolicyVersion   string `gorm:"not null"`
	WorkflowVersion string `gorm:"not null"`
	SyntheticSuffix string `gorm:"not null"`

	WorkflowID string `gorm:"not null;index"`
	RunID      string

	Outcome      string `gorm:"not null;index"`
	StageReached string
	Findings     []byte `gorm:"type:jsonb"`
	FindingTally []byte `gorm:"type:jsonb"`

	DerivativeObjectKey string
	DerivativeSHA256    string `gorm:"size:64"`
	DerivativeBytes     int64
	ManifestObjectKey   string
	Counts              []byte `gorm:"type:jsonb"`

	StartedAt   time.Time `gorm:"not null;index"`
	CompletedAt *time.Time
}

// TableName specifies the table name for EscrowSanitizationRunRecord.
func (EscrowSanitizationRunRecord) TableName() string { return "escrow_sanitization_runs" }

// ---------- mappers ----------

func toDBEscrowSanitizationRun(r *entities.EscrowSanitizationRun) (*EscrowSanitizationRunRecord, error) {
	findings := r.Findings
	if findings == nil {
		findings = []entities.EscrowFinding{}
	}
	rawFindings, err := json.Marshal(findings)
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
	rawCounts, err := json.Marshal(r.Counts)
	if err != nil {
		return nil, fmt.Errorf("encode counts: %w", err)
	}
	return &EscrowSanitizationRunRecord{
		ID: r.ID, TenantID: r.TenantID.String(), TLD: r.TLD,
		SourceValidationRunID: r.SourceValidationRunID, SourceDepositID: r.SourceDepositID,
		SourceArtifactSHA256: r.SourceArtifactSHA256,
		PolicyVersion:        r.PolicyVersion, WorkflowVersion: r.WorkflowVersion, SyntheticSuffix: r.SyntheticSuffix,
		WorkflowID: r.WorkflowID, RunID: r.RunID,
		Outcome: string(r.Outcome), StageReached: r.StageReached, Findings: rawFindings, FindingTally: rawTally,
		DerivativeObjectKey: r.DerivativeObjectKey, DerivativeSHA256: r.DerivativeSHA256,
		DerivativeBytes: r.DerivativeBytes, ManifestObjectKey: r.ManifestObjectKey, Counts: rawCounts,
		StartedAt: r.StartedAt, CompletedAt: r.CompletedAt,
	}, nil
}

func fromDBEscrowSanitizationRun(rec *EscrowSanitizationRunRecord) (*entities.EscrowSanitizationRun, error) {
	var findings []entities.EscrowFinding
	if len(rec.Findings) > 0 {
		if err := json.Unmarshal(rec.Findings, &findings); err != nil {
			return nil, fmt.Errorf("decode findings: %w", err)
		}
	}
	tally := []entities.EscrowFindingTally{}
	if len(rec.FindingTally) > 0 {
		if err := json.Unmarshal(rec.FindingTally, &tally); err != nil {
			return nil, fmt.Errorf("decode finding tally: %w", err)
		}
	}
	var counts entities.EscrowSanitizationCounts
	if len(rec.Counts) > 0 {
		if err := json.Unmarshal(rec.Counts, &counts); err != nil {
			return nil, fmt.Errorf("decode counts: %w", err)
		}
	}
	return &entities.EscrowSanitizationRun{
		ID: rec.ID, TenantID: entities.OperatorID(rec.TenantID), TLD: rec.TLD,
		SourceValidationRunID: rec.SourceValidationRunID, SourceDepositID: rec.SourceDepositID,
		SourceArtifactSHA256: rec.SourceArtifactSHA256,
		PolicyVersion:        rec.PolicyVersion, WorkflowVersion: rec.WorkflowVersion, SyntheticSuffix: rec.SyntheticSuffix,
		WorkflowID: rec.WorkflowID, RunID: rec.RunID,
		Outcome: entities.EscrowSanitizationOutcome(rec.Outcome), StageReached: rec.StageReached, Findings: findings, FindingTally: tally,
		DerivativeObjectKey: rec.DerivativeObjectKey, DerivativeSHA256: rec.DerivativeSHA256,
		DerivativeBytes: rec.DerivativeBytes, ManifestObjectKey: rec.ManifestObjectKey, Counts: counts,
		StartedAt: rec.StartedAt, CompletedAt: rec.CompletedAt,
	}, nil
}

// ---------------------------------------------------------------------------
// Sanitization runs
// ---------------------------------------------------------------------------

// GormEscrowSanitizationRunRepository implements
// repositories.EscrowSanitizationRunRepository.
type GormEscrowSanitizationRunRepository struct{ db *gorm.DB }

// NewEscrowSanitizationRunRepository creates a new GormEscrowSanitizationRunRepository.
func NewEscrowSanitizationRunRepository(db *gorm.DB) *GormEscrowSanitizationRunRepository {
	return &GormEscrowSanitizationRunRepository{db: db}
}

// Create persists a RUNNING run. The unique index on
// (tenant_id, source_validation_run_id, policy_version) is what stops a second
// derivative of the same source under the same policy.
func (r *GormEscrowSanitizationRunRepository) Create(ctx context.Context, run *entities.EscrowSanitizationRun) error {
	rec, err := toDBEscrowSanitizationRun(run)
	if err != nil {
		return fmt.Errorf("EscrowSanitizationRun.Create(id=%s): %w", run.ID, err)
	}
	if err := r.db.WithContext(ctx).Create(rec).Error; err != nil {
		return fmt.Errorf("EscrowSanitizationRun.Create(id=%s): %w", run.ID, err)
	}
	return nil
}

// Finalize writes the terminal state with a single conditional UPDATE that only
// matches a RUNNING row for this tenant. Zero rows affected means the run was
// already final (or is not this tenant's) and the earlier outcome stands.
func (r *GormEscrowSanitizationRunRepository) Finalize(ctx context.Context, scope entities.OperatorID, run *entities.EscrowSanitizationRun) error {
	if !run.IsFinal() {
		return fmt.Errorf("EscrowSanitizationRun.Finalize(id=%s): %w", run.ID, entities.ErrInvalidEscrowSanitizationRun)
	}
	rec, err := toDBEscrowSanitizationRun(run)
	if err != nil {
		return fmt.Errorf("EscrowSanitizationRun.Finalize(id=%s): %w", run.ID, err)
	}
	res := r.db.WithContext(ctx).Model(&EscrowSanitizationRunRecord{}).
		Where("id = ? AND tenant_id = ? AND outcome = ?", run.ID, scope.String(), string(entities.EscrowSanitizationRunning)).
		Updates(map[string]interface{}{
			"outcome":               rec.Outcome,
			"stage_reached":         rec.StageReached,
			"findings":              rec.Findings,
			"finding_tally":         rec.FindingTally,
			"derivative_object_key": rec.DerivativeObjectKey,
			"derivative_sha256":     rec.DerivativeSHA256,
			"derivative_bytes":      rec.DerivativeBytes,
			"manifest_object_key":   rec.ManifestObjectKey,
			"counts":                rec.Counts,
			"completed_at":          rec.CompletedAt,
		})
	if res.Error != nil {
		return fmt.Errorf("EscrowSanitizationRun.Finalize(id=%s): %w", run.ID, res.Error)
	}
	if res.RowsAffected == 0 {
		return entities.ErrEscrowSanitizationRunAlreadyFinal
	}
	return nil
}

// GetByID retrieves a run by tenant and ID.
func (r *GormEscrowSanitizationRunRepository) GetByID(ctx context.Context, scope entities.OperatorID, id uuid.UUID) (*entities.EscrowSanitizationRun, error) {
	var rec EscrowSanitizationRunRecord
	err := r.db.WithContext(ctx).Where("id = ? AND tenant_id = ?", id, scope.String()).First(&rec).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, entities.ErrEscrowSanitizationRunNotFound
		}
		return nil, fmt.Errorf("EscrowSanitizationRun.GetByID(id=%s): %w", id, err)
	}
	return fromDBEscrowSanitizationRun(&rec)
}

// FindBySourceAndPolicy returns the existing derivative run for this source and
// policy version, if any.
func (r *GormEscrowSanitizationRunRepository) FindBySourceAndPolicy(ctx context.Context, scope entities.OperatorID, sourceValidationRunID uuid.UUID, policyVersion string) (*entities.EscrowSanitizationRun, error) {
	var rec EscrowSanitizationRunRecord
	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND source_validation_run_id = ? AND policy_version = ?", scope.String(), sourceValidationRunID, policyVersion).
		First(&rec).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, entities.ErrEscrowSanitizationRunNotFound
		}
		return nil, fmt.Errorf("EscrowSanitizationRun.FindBySourceAndPolicy(source=%s): %w", sourceValidationRunID, err)
	}
	return fromDBEscrowSanitizationRun(&rec)
}

// List returns sanitisation runs for a tenant, newest first, cursor-paginated
// (INV-11).
func (r *GormEscrowSanitizationRunRepository) List(ctx context.Context, scope entities.OperatorID, q queries.ListItemsQuery) ([]*entities.EscrowSanitizationRun, string, error) {
	pageSize := clampPageSize(q.PageSize)
	query := r.db.WithContext(ctx).Where("tenant_id = ?", scope.String())
	if f, ok := q.Filter.(queries.ListEscrowSanitizationRunsFilter); ok {
		if f.TLDEquals != "" {
			query = query.Where("tld = ?", f.TLDEquals)
		}
		if f.OutcomeEquals != "" {
			query = query.Where("outcome = ?", f.OutcomeEquals)
		}
		if f.SourceValidationRunIDEquals != "" {
			id, err := uuid.Parse(f.SourceValidationRunIDEquals)
			if err != nil {
				return nil, "", fmt.Errorf("EscrowSanitizationRun.List(sourceValidationRunId): %w", err)
			}
			query = query.Where("source_validation_run_id = ?", id)
		}
	}
	if q.PageCursor != "" {
		t, id, err := decodeObsCursor(q.PageCursor)
		if err != nil {
			return nil, "", fmt.Errorf("EscrowSanitizationRun.List(cursor): %w", err)
		}
		query = query.Where("(started_at < ? OR (started_at = ? AND id < ?))", t, t, id)
	}
	var records []EscrowSanitizationRunRecord
	if err := query.Order("started_at DESC, id DESC").Limit(pageSize + 1).Find(&records).Error; err != nil {
		return nil, "", fmt.Errorf("EscrowSanitizationRun.List: %w", err)
	}
	var next string
	if len(records) > pageSize {
		last := records[pageSize-1]
		next = encodeObsCursor(last.StartedAt, last.ID)
		records = records[:pageSize]
	}
	out := make([]*entities.EscrowSanitizationRun, len(records))
	for i := range records {
		run, err := fromDBEscrowSanitizationRun(&records[i])
		if err != nil {
			return nil, "", fmt.Errorf("EscrowSanitizationRun.List: %w", err)
		}
		out[i] = run
	}
	return out, next, nil
}
