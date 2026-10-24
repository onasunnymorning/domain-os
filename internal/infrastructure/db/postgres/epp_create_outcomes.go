package postgres

import (
	"context"
	"fmt"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
)

// EPPCreateOutcomeRecord is a registrar's domain creates for one UTC day
// (epp_create_outcomes). It has no foreign key to the registrar: the
// counts are a billing record, kept if the registrar is deleted.
type EPPCreateOutcomeRecord struct {
	ClID      string    `gorm:"primaryKey"`
	Day       time.Time `gorm:"primaryKey;type:date"`
	Succeeded int64     `gorm:"not null;default:0"`
	Failed    int64     `gorm:"not null;default:0"`
}

// TableName pins the table name.
func (EPPCreateOutcomeRecord) TableName() string { return "epp_create_outcomes" }

// EPPCreateOutcomeRepository is the Postgres
// repositories.EPPCreateOutcomeRepository.
type EPPCreateOutcomeRepository struct{ db *gorm.DB }

// NewEPPCreateOutcomeRepository returns the repository.
func NewEPPCreateOutcomeRepository(db *gorm.DB) *EPPCreateOutcomeRepository {
	return &EPPCreateOutcomeRepository{db: db}
}

func utcDay(t time.Time) time.Time {
	y, m, d := t.UTC().Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

// AddCreateOutcomes implements repositories.EPPCreateOutcomeRepository,
// adding to the stored counts in one statement per batch.
func (r *EPPCreateOutcomeRepository) AddCreateOutcomes(ctx context.Context, counts []entities.EPPCreateOutcomes) error {
	if len(counts) == 0 {
		return nil
	}
	recs := make([]EPPCreateOutcomeRecord, len(counts))
	for i, c := range counts {
		recs[i] = EPPCreateOutcomeRecord{ClID: c.ClID.String(), Day: utcDay(c.Day), Succeeded: c.Succeeded, Failed: c.Failed}
	}
	err := r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "cl_id"}, {Name: "day"}},
		DoUpdates: clause.Assignments(map[string]any{
			"succeeded": gorm.Expr("epp_create_outcomes.succeeded + excluded.succeeded"),
			"failed":    gorm.Expr("epp_create_outcomes.failed + excluded.failed"),
		}),
	}).Create(&recs).Error
	if err != nil {
		return fmt.Errorf("add EPP create outcomes: %w", err)
	}
	return nil
}

// ListCreateOutcomes implements repositories.EPPCreateOutcomeRepository.
func (r *EPPCreateOutcomeRepository) ListCreateOutcomes(ctx context.Context, clid entities.ClIDType, from, to time.Time) ([]entities.EPPCreateOutcomes, error) {
	var recs []EPPCreateOutcomeRecord
	err := r.db.WithContext(ctx).Where("cl_id = ? AND day >= ? AND day <= ?", clid.String(), utcDay(from), utcDay(to)).
		Order("day").Find(&recs).Error
	if err != nil {
		return nil, fmt.Errorf("list EPP create outcomes: %w", err)
	}
	out := make([]entities.EPPCreateOutcomes, len(recs))
	for i, rec := range recs {
		out[i] = entities.EPPCreateOutcomes{ClID: entities.ClIDType(rec.ClID), Day: utcDay(rec.Day), Succeeded: rec.Succeeded, Failed: rec.Failed}
	}
	return out, nil
}
