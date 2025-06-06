package postgres

import (
	"context"

	"github.com/onasunnymorning/domain-os/internal/domain/entities"
	"gorm.io/gorm"
)

// FXRepository is the GORM implementation of the FXRepository
type FXRepository struct {
	db *gorm.DB
}

// NewFXRepository creates a new FXRepository instance
func NewFXRepository(db *gorm.DB) *FXRepository {
	return &FXRepository{
		db: db,
	}
}

// UpdateAll updates all exchange rates in the database
func (r *FXRepository) UpdateAll(ctx context.Context, fxs []*FX) error {
	// Drop all records from the fx table for a given currency
	err := r.db.WithContext(ctx).Where("base = ?", fxs[0].Base).Delete(&FX{}).Error
	if err != nil {
		return err
	}

	// Insert all records into the fx table
	return r.db.Create(&fxs).Error
}

// ListByBaseCurrency lists all exchange rates by base currency
func (r *FXRepository) ListByBaseCurrency(ctx context.Context, baseCurrency string) ([]*entities.FX, error) {
	var fxs []*FX
	err := r.db.WithContext(ctx).Where("base = ?", baseCurrency).Find(&fxs).Error
	if err != nil {
		return nil, err
	}

	result := make([]*entities.FX, len(fxs))
	for i, fx := range fxs {
		result[i] = fx.ToEntity()
	}

	return result, nil
}

// GetByBaseAndTargetCurrency gets the exchange rate for a base and target currency
func (r *FXRepository) GetByBaseAndTargetCurrency(ctx context.Context, baseCurrency, targetCurrency string) (*entities.FX, error) {
	var fx FX
	err := r.db.WithContext(ctx).Where("base = ? AND target = ?", baseCurrency, targetCurrency).First(&fx).Error
	if err != nil {
		return nil, err
	}

	return fx.ToEntity(), nil
}
