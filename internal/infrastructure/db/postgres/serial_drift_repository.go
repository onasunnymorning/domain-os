package postgres

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
	"gorm.io/gorm"
)

// GormSerialDriftRepository implements repositories.SerialDriftRepository.
//
// Operator tenant isolation is enforced here, in SQL: every scoped query
// carries `tenant_id = ?` bound to the entities.OperatorID passed in. Per
// ADR-0006 the repository is the enforcement layer — services above it are not
// trusted to filter, and no query may drop the predicate.
type GormSerialDriftRepository struct {
	db *gorm.DB
}

// NewSerialDriftRepository creates a new GormSerialDriftRepository.
func NewSerialDriftRepository(db *gorm.DB) *GormSerialDriftRepository {
	return &GormSerialDriftRepository{db: db}
}

// ---------- ZoneSlaving CRUD ----------

// CreateSlaving persists a new ZoneSlaving record.
func (r *GormSerialDriftRepository) CreateSlaving(ctx context.Context, s *entities.ZoneSlaving) error {
	rec := toDBZoneSlaving(s)
	if err := r.db.WithContext(ctx).Create(rec).Error; err != nil {
		return fmt.Errorf("CreateSlaving(id=%s, zone=%s): %w", s.ID, s.Zone, err)
	}
	return nil
}

// GetSlaving retrieves a ZoneSlaving by tenant and ID.
func (r *GormSerialDriftRepository) GetSlaving(ctx context.Context, scope entities.OperatorID, id uuid.UUID) (*entities.ZoneSlaving, error) {
	var rec ZoneSlavingRecord
	err := r.db.WithContext(ctx).Where("id = ? AND tenant_id = ?", id, scope).First(&rec).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, entities.ErrZoneSlavingNotFound
		}
		return nil, fmt.Errorf("GetSlaving(id=%s): %w", id, err)
	}
	return fromDBZoneSlaving(&rec), nil
}

// UpdateSlavingStatus sets the status of a ZoneSlaving record.
func (r *GormSerialDriftRepository) UpdateSlavingStatus(ctx context.Context, scope entities.OperatorID, id uuid.UUID, status entities.ZoneSlavingStatus) error {
	result := r.db.WithContext(ctx).
		Model(&ZoneSlavingRecord{}).
		Where("id = ? AND tenant_id = ?", id, scope).
		Updates(map[string]interface{}{
			"status":     string(status),
			"updated_at": time.Now().UTC(),
		})
	if result.Error != nil {
		return fmt.Errorf("UpdateSlavingStatus(id=%s, status=%s): %w", id, status, result.Error)
	}
	if result.RowsAffected == 0 {
		return entities.ErrZoneSlavingNotFound
	}
	return nil
}

// ListActiveSlavings returns all active ZoneSlaving records for a tenant.
func (r *GormSerialDriftRepository) ListActiveSlavings(ctx context.Context, scope entities.OperatorID) ([]*entities.ZoneSlaving, error) {
	var records []ZoneSlavingRecord
	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND status = ?", scope, string(entities.ZoneSlavingStatusActive)).
		Order("created_at ASC").
		Find(&records).Error
	if err != nil {
		return nil, fmt.Errorf("ListActiveSlavings(tenant=%s): %w", scope, err)
	}
	result := make([]*entities.ZoneSlaving, len(records))
	for i := range records {
		result[i] = fromDBZoneSlaving(&records[i])
	}
	return result, nil
}

// ---------- Observation writes ----------

// CreateRun persists a SerialCheckRun record.
func (r *GormSerialDriftRepository) CreateRun(ctx context.Context, run *entities.SerialCheckRun) error {
	rec := toDBSerialCheckRun(run)
	if err := r.db.WithContext(ctx).Create(rec).Error; err != nil {
		return fmt.Errorf("CreateRun(id=%s, slavingID=%s): %w", run.ID, run.SlavingID, err)
	}
	return nil
}

// CreateObservations batch-inserts SerialObservation records.
func (r *GormSerialDriftRepository) CreateObservations(ctx context.Context, obs []*entities.SerialObservation) error {
	if len(obs) == 0 {
		return nil
	}
	records := make([]SerialObservationRecord, len(obs))
	for i, o := range obs {
		records[i] = *toDBSerialObservation(o)
	}
	if err := r.db.WithContext(ctx).Create(&records).Error; err != nil {
		return fmt.Errorf("CreateObservations(count=%d): %w", len(obs), err)
	}
	return nil
}

// ---------- Observation reads ----------

// encodeObsCursor encodes an observedAt timestamp and observation ID into an opaque cursor.
func encodeObsCursor(observedAt time.Time, id uuid.UUID) string {
	raw := observedAt.Format(time.RFC3339Nano) + "|" + id.String()
	return base64.StdEncoding.EncodeToString([]byte(raw))
}

// decodeObsCursor decodes an opaque cursor into an observedAt timestamp and observation ID.
func decodeObsCursor(cursor string) (time.Time, uuid.UUID, error) {
	decoded, err := base64.StdEncoding.DecodeString(cursor)
	if err != nil {
		return time.Time{}, uuid.Nil, fmt.Errorf("decodeObsCursor(base64): %w", err)
	}
	parts := strings.SplitN(string(decoded), "|", 2)
	if len(parts) != 2 {
		return time.Time{}, uuid.Nil, fmt.Errorf("decodeObsCursor: invalid format, expected 'timestamp|id'")
	}
	t, err := time.Parse(time.RFC3339Nano, parts[0])
	if err != nil {
		return time.Time{}, uuid.Nil, fmt.Errorf("decodeObsCursor(time.Parse): %w", err)
	}
	id, err := uuid.Parse(parts[1])
	if err != nil {
		return time.Time{}, uuid.Nil, fmt.Errorf("decodeObsCursor(uuid.Parse): %w", err)
	}
	return t, id, nil
}

// ListObservations returns observations with cursor-based pagination (newest first).
func (r *GormSerialDriftRepository) ListObservations(ctx context.Context, scope entities.OperatorID, slavingID uuid.UUID, pageSize int, cursor string) ([]*entities.SerialObservation, string, error) {
	if pageSize <= 0 {
		pageSize = 50
	}
	if pageSize > 200 {
		pageSize = 200
	}

	query := r.db.WithContext(ctx).
		Where("tenant_id = ? AND slaving_id = ?", scope, slavingID)

	if cursor != "" {
		cursorTime, cursorID, err := decodeObsCursor(cursor)
		if err != nil {
			return nil, "", fmt.Errorf("ListObservations(cursor): %w", err)
		}
		query = query.Where("(observed_at < ? OR (observed_at = ? AND id < ?))", cursorTime, cursorTime, cursorID)
	}

	var records []SerialObservationRecord
	err := query.Order("observed_at DESC, id DESC").Limit(pageSize + 1).Find(&records).Error
	if err != nil {
		return nil, "", fmt.Errorf("ListObservations(slavingID=%s): %w", slavingID, err)
	}

	var nextCursor string
	if len(records) > pageSize {
		lastRecord := records[pageSize-1]
		nextCursor = encodeObsCursor(lastRecord.ObservedAt, lastRecord.ID)
		records = records[:pageSize]
	}

	result := make([]*entities.SerialObservation, len(records))
	for i := range records {
		result[i] = fromDBSerialObservation(&records[i])
	}
	return result, nextCursor, nil
}

// GetRecentObservations returns the most recent observations for a slaving monitor.
func (r *GormSerialDriftRepository) GetRecentObservations(ctx context.Context, scope entities.OperatorID, slavingID uuid.UUID, limit int) ([]*entities.SerialObservation, error) {
	if limit <= 0 {
		limit = 50
	}
	var records []SerialObservationRecord
	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND slaving_id = ?", scope, slavingID).
		Order("observed_at DESC, id DESC").
		Limit(limit).
		Find(&records).Error
	if err != nil {
		return nil, fmt.Errorf("GetRecentObservations(slavingID=%s): %w", slavingID, err)
	}
	result := make([]*entities.SerialObservation, len(records))
	for i := range records {
		result[i] = fromDBSerialObservation(&records[i])
	}
	return result, nil
}

// GetRecentRuns returns the most recent check runs for a slaving monitor.
func (r *GormSerialDriftRepository) GetRecentRuns(ctx context.Context, scope entities.OperatorID, slavingID uuid.UUID, limit int) ([]*entities.SerialCheckRun, error) {
	if limit <= 0 {
		limit = 50
	}
	var records []SerialCheckRunRecord
	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND slaving_id = ?", scope, slavingID).
		Order("started_at DESC").
		Limit(limit).
		Find(&records).Error
	if err != nil {
		return nil, fmt.Errorf("GetRecentRuns(slavingID=%s): %w", slavingID, err)
	}
	result := make([]*entities.SerialCheckRun, len(records))
	for i := range records {
		result[i] = fromDBSerialCheckRun(&records[i])
	}
	return result, nil
}

// ---------- Confidence rollup ----------

// GetConfidenceRollup computes the convergence confidence state from observation history.
func (r *GormSerialDriftRepository) GetConfidenceRollup(ctx context.Context, scope entities.OperatorID, slavingID uuid.UUID) (*entities.SlavingConfidenceRollup, error) {
	// 1. Get the ZoneSlaving record for ConfidenceN
	var slavingRec ZoneSlavingRecord
	err := r.db.WithContext(ctx).
		Where("id = ? AND tenant_id = ?", slavingID, scope).
		First(&slavingRec).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, entities.ErrZoneSlavingNotFound
		}
		return nil, fmt.Errorf("GetConfidenceRollup(slavingID=%s): fetch slaving: %w", slavingID, err)
	}
	confidenceN := slavingRec.ConfidenceN

	// 2. Get total run count and latest run
	var totalRuns int64
	r.db.WithContext(ctx).
		Model(&SerialCheckRunRecord{}).
		Where("tenant_id = ? AND slaving_id = ?", scope, slavingID).
		Count(&totalRuns)

	var latestRun SerialCheckRunRecord
	err = r.db.WithContext(ctx).
		Where("tenant_id = ? AND slaving_id = ?", scope, slavingID).
		Order("started_at DESC").
		First(&latestRun).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// No runs yet — return empty rollup
			return &entities.SlavingConfidenceRollup{
				SlavingID: slavingID,
				Zone:      slavingRec.Zone,
			}, nil
		}
		return nil, fmt.Errorf("GetConfidenceRollup(slavingID=%s): fetch latest run: %w", slavingID, err)
	}

	// 3. Count distinct master serials across all runs (increments_tracked)
	var distinctMasterSerials int64
	r.db.WithContext(ctx).
		Model(&SerialCheckRunRecord{}).
		Where("tenant_id = ? AND slaving_id = ?", scope, slavingID).
		Distinct("master_serial").
		Count(&distinctMasterSerials)

	// 4. Get unique slave nameservers from observations
	var slaveNameservers []string
	r.db.WithContext(ctx).
		Model(&SerialObservationRecord{}).
		Where("tenant_id = ? AND slaving_id = ? AND is_master = ?", scope, slavingID, false).
		Distinct("nameserver").
		Pluck("nameserver", &slaveNameservers)

	// 5. For each slave, compute confidence
	fetchLimit := confidenceN + 5
	slaves := make([]entities.SlaveConfidence, 0, len(slaveNameservers))
	allReady := true

	for _, ns := range slaveNameservers {
		var obsRecords []SerialObservationRecord
		r.db.WithContext(ctx).
			Where("tenant_id = ? AND slaving_id = ? AND nameserver = ? AND is_master = ?", scope, slavingID, ns, false).
			Order("observed_at DESC").
			Limit(fetchLimit).
			Find(&obsRecords)

		if len(obsRecords) == 0 {
			slaves = append(slaves, entities.SlaveConfidence{
				Nameserver: ns,
			})
			allReady = false
			continue
		}

		// Count consecutive converged streak from most recent
		consecutiveConverged := 0
		for _, obs := range obsRecords {
			if obs.Status == string(entities.SlaveStatusConverged) {
				consecutiveConverged++
			} else {
				break
			}
		}

		latest := obsRecords[0]
		converged := latest.Status == string(entities.SlaveStatusConverged)
		confidenceReady := int64(distinctMasterSerials) >= 1 && consecutiveConverged >= confidenceN

		sc := entities.SlaveConfidence{
			Nameserver:           ns,
			LatestSerial:         latest.Serial,
			Converged:            converged,
			IncrementsTracked:    int(distinctMasterSerials),
			ConsecutiveConverged: consecutiveConverged,
			ConfidenceReady:      confidenceReady,
			LatestStatus:         entities.SlaveStatus(latest.Status),
			LatestDriftTier:      entities.DriftTier(latest.DriftTier),
		}
		slaves = append(slaves, sc)

		if !confidenceReady {
			allReady = false
		}
	}

	// If no slaves were found, allReady should be false
	if len(slaveNameservers) == 0 {
		allReady = false
	}

	return &entities.SlavingConfidenceRollup{
		SlavingID:          slavingID,
		Zone:               slavingRec.Zone,
		MasterSerial:       latestRun.MasterSerial,
		Slaves:             slaves,
		AllConfidenceReady: allReady,
		TotalRuns:          int(totalRuns),
		LastRunAt:          latestRun.StartedAt,
	}, nil
}
