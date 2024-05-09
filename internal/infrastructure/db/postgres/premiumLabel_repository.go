package postgres

import (
	"context"
	"errors"

	"github.com/onasunnymorning/domain-os/internal/domain/entities"
	"gorm.io/gorm"
)

// PremiumListRepository implements the PremiumListRepository interface
type PremiumLabelRepository struct {
	db *gorm.DB
}

// NewPremiumListRepository creates a new PremiumListRepository instance
func NewGORMPremiumLabelRepository(db *gorm.DB) *PremiumLabelRepository {
	return &PremiumLabelRepository{
		db: db,
	}
}

// Create creates a new premium list in the database
func (plr *PremiumLabelRepository) Create(ctx context.Context, premiumLabel *entities.PremiumLabel) (*entities.PremiumLabel, error) {
	pl := FromEntity(premiumLabel)
	err := plr.db.WithContext(ctx).Create(pl).Error
	if err != nil {
		return nil, err
	}
	return pl.ToEntity(), nil
}

// GetByLabelListAndCurrency retrieves a premium label by label, list, and currency
func (plr *PremiumLabelRepository) GetByLabelListAndCurrency(ctx context.Context, label, list, currency string) (*entities.PremiumLabel, error) {
	pl := &PremiumLabel{}
	if err := plr.db.WithContext(ctx).Where("label = ? AND premium_list_name = ? AND currency = ?", label, list, currency).First(pl).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, entities.ErrPremiumLabelNotFound
		}
		return nil, err
	}
	return pl.ToEntity(), nil
}

// DeleteByLabelListAndCurrency deletes a premium label by label, list, and currency
func (plr *PremiumLabelRepository) DeleteByLabelListAndCurrency(ctx context.Context, label, list, currency string) error {
	return plr.db.WithContext(ctx).Where("label = ? AND premium_list_name = ? AND currency = ?", label, list, currency).Delete(&PremiumLabel{}).Error
}

// List retrieves a list of premium labels
func (plr *PremiumLabelRepository) List(ctx context.Context, pagesize int, cursor, listName, currency, label string) ([]*entities.PremiumLabel, error) {
	dbpls := []*PremiumLabel{}

	err := plr.db.WithContext(ctx).Where(&PremiumLabel{PremiumListName: listName, Currency: currency, Label: label}).Order("label ASC").Limit(pagesize).Find(&dbpls, "label > ?", cursor).Error
	if err != nil {
		return nil, err
	}

	pls := make([]*entities.PremiumLabel, len(dbpls))
	for i, dbpl := range dbpls {
		pls[i] = dbpl.ToEntity()
	}

	return pls, nil
}
