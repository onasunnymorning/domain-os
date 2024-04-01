package postgres

import (
	"context"

	"github.com/onasunnymorning/domain-os/internal/domain/entities"
	"gorm.io/gorm"
)

// PhaseFeeRepository is the GORM implementation of the PhaseFeeRepository
type PhaseFeeRepository struct {
	DB *gorm.DB
}

// NewPhaseFeeRepository creates a new PhaseFeeRepository instance
func NewFeeRepository(db *gorm.DB) *PhaseFeeRepository {
	return &PhaseFeeRepository{
		DB: db,
	}
}

// CreateFee creates a new phase fee in the database
func (r *PhaseFeeRepository) CreateFee(ctx context.Context, fee *entities.Fee) (*entities.Fee, error) {
	gormFee := &Fee{}
	gormFee.FromEntity(fee)
	err := r.DB.WithContext(ctx).Create(gormFee).Error
	if err != nil {
		return nil, err
	}

	return gormFee.ToEntity(), nil
}

// GetFee retrieves a phase fee from the database
func (r *PhaseFeeRepository) GetFee(ctx context.Context, phase, name, currency string) (*entities.Fee, error) {
	var gormFee Fee
	err := r.DB.WithContext(ctx).Where("phase_id = ? AND name = ? AND currency = ?", phase, name, currency).First(&gormFee).Error
	if err != nil {
		return nil, err
	}

	return gormFee.ToEntity(), nil
}

// DeleteFee deletes a phase fee from the database
func (r *PhaseFeeRepository) DeleteFee(ctx context.Context, phase, name, currency string) error {
	err := r.DB.WithContext(ctx).Where("phase_id = ? AND name = ? AND currency = ?", phase, name, currency).Delete(&Fee{}).Error
	if err != nil {
		return err
	}

	return nil
}
