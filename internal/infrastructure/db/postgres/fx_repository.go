package postgres

import (
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
func (r *FXRepository) UpdateAll(fxs []*FX) error {
	// Drop all records from the fx table
	err := r.db.Exec("DELETE FROM fx").Error
	if err != nil {
		return err
	}

	// Insert all records into the fx table
	return r.db.Create(&fxs).Error
}

// ListByBaseCurrency lists all exchange rates by base currency
func (r *FXRepository) ListByBaseCurrency(baseCurrency string) ([]*entities.FX, error) {
	var fxs []*FX
	err := r.db.Where("base = ?", baseCurrency).Find(&fxs).Error
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
func (r *FXRepository) GetByBaseAndTargetCurrency(baseCurrency, targetCurrency string) (*entities.FX, error) {
	var fx FX
	err := r.db.Where("base = ? AND target = ?", baseCurrency, targetCurrency).First(&fx).Error
	if err != nil {
		return nil, err
	}

	return fx.ToEntity(), nil
}
