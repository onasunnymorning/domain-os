package postgres

import (
	"context"

	"github.com/onasunnymorning/domain-os/internal/domain/entities"
	"gorm.io/gorm"
)

// PriceRepository is the GORM implementation of the PriceRepository
type PriceRepository struct {
	db *gorm.DB
}

// NewGormPriceRepository creates a new GormPriceRepository
func NewGormPriceRepository(db *gorm.DB) *PriceRepository {
	return &PriceRepository{db: db}
}

// CreatePrice creates a new price in the database
func (r *PriceRepository) CreatePrice(ctx context.Context, price *entities.Price) (*entities.Price, error) {
	gormPrice := &Price{}
	gormPrice.FromEntity(price)

	if err := r.db.WithContext(ctx).Create(gormPrice).Error; err != nil {
		return nil, err
	}

	return gormPrice.ToEntity(), nil
}

// GetPrice retrieves a price from the database
func (r *PriceRepository) GetPrice(ctx context.Context, phaseID int64, currency string) (*entities.Price, error) {
	var gormPrice Price
	err := r.db.WithContext(ctx).Where("phase_id = ? AND currency = ?", phaseID, currency).First(&gormPrice).Error
	if err != nil {
		return nil, err
	}

	return gormPrice.ToEntity(), nil
}

// DeletePrice deletes a price from the database
func (r *PriceRepository) DeletePrice(ctx context.Context, phaseID int64, currency string) error {
	return r.db.WithContext(ctx).Where("phase_id = ? AND currency = ?", phaseID, currency).Delete(&Price{}).Error
}
