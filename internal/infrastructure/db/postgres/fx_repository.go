package postgres

import (
	"context"

	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
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

// UpdateAll replaces all exchange rates for the base currency of the given
// rates in a single transaction. Running delete + insert atomically ensures a
// failure can never leave the base currency without any rates (which would
// make every quote in a non-base currency fail until the next sync).
func (r *FXRepository) UpdateAll(ctx context.Context, fxs []*entities.FX) error {
	if len(fxs) == 0 {
		return nil
	}
	dbFXs := make([]*FX, len(fxs))
	for i, fx := range fxs {
		dbFXs[i] = &FX{}
		dbFXs[i].FromEntity(fx)
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Drop all records from the fx table for the given base currency
		if err := tx.Where("base = ?", fxs[0].BaseCurrency).Delete(&FX{}).Error; err != nil {
			return err
		}
		// Insert the new records
		return tx.Create(&dbFXs).Error
	})
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

// GetByBaseAndTargetCurrency gets the most recent exchange rate for a base and
// target currency. Ordering by date DESC matters: the primary key starts with
// date, so a bare First() would return the OLDEST rate if rates for multiple
// dates ever coexist.
func (r *FXRepository) GetByBaseAndTargetCurrency(ctx context.Context, baseCurrency, targetCurrency string) (*entities.FX, error) {
	var fx FX
	err := r.db.WithContext(ctx).Where("base = ? AND target = ?", baseCurrency, targetCurrency).Order("date DESC").First(&fx).Error
	if err != nil {
		return nil, err
	}

	return fx.ToEntity(), nil
}
