package postgres

import (
	"context"

	"github.com/onasunnymorning/domain-os/internal/domain/entities"
	"gorm.io/gorm"
)

// FeeRepository is the GORM implementation of the FeeRepository
type FeeRepository struct {
	DB *gorm.DB
}

// NewPhaseFeeRepository creates a new PhaseFeeRepository instance
func NewFeeRepository(db *gorm.DB) *FeeRepository {
	return &FeeRepository{
		DB: db,
	}
}

// CreateFee creates a new phase fee in the database
func (r *FeeRepository) CreateFee(ctx context.Context, fee *entities.Fee) (*entities.Fee, error) {
	gormFee := &Fee{}
	gormFee.FromEntity(fee)
	err := r.DB.WithContext(ctx).Create(gormFee).Error
	if err != nil {
		return nil, err
	}

	return gormFee.ToEntity(), nil
}

// GetFee retrieves a phase fee from the database
func (r *FeeRepository) GetFee(ctx context.Context, phaseID int64, name, currency string) (*entities.Fee, error) {
	var gormFee Fee
	err := r.DB.WithContext(ctx).Where("phase_id = ? AND name = ? AND currency = ?", phaseID, name, currency).First(&gormFee).Error
	if err != nil {
		return nil, err
	}

	return gormFee.ToEntity(), nil
}

// DeleteFee deletes a phase fee from the database
func (r *FeeRepository) DeleteFee(ctx context.Context, phaseID int64, name, currency string) error {
	return r.DB.WithContext(ctx).Where("phase_id = ? AND name = ? AND currency = ?", phaseID, name, currency).Delete(&Fee{}).Error
}
