package postgres

import (
	"context"

	"github.com/onasunnymorning/domain-os/internal/domain/entities"
	"gorm.io/gorm"
)

// PremiumListRepository implements the PremiumListRepository interface
type PremiumListRepository struct {
	db *gorm.DB
}

// NewPremiumListRepository creates a new PremiumListRepository instance
func NewGORMPremiumListRepository(db *gorm.DB) *PremiumListRepository {
	return &PremiumListRepository{
		db: db,
	}
}

// Create creates a new premium list in the database
func (plr *PremiumListRepository) Create(ctx context.Context, premiumList *entities.PremiumList) (*entities.PremiumList, error) {
	pl := &PremiumList{}
	pl.FromEntity(premiumList)
	err := plr.db.WithContext(ctx).Create(pl).Error
	if err != nil {
		return nil, err
	}
	return pl.ToEntity(), nil
}

// GetByName retrieves a premium list by name
func (plr *PremiumListRepository) GetByName(ctx context.Context, name string) (*entities.PremiumList, error) {
	pl := &PremiumList{}
	if err := plr.db.WithContext(ctx).Where("name = ?", name).First(pl).Error; err != nil {
		return nil, err
	}
	return pl.ToEntity(), nil
}

// DeleteByName deletes a premium list by name
func (plr *PremiumListRepository) DeleteByName(ctx context.Context, name string) error {
	return plr.db.WithContext(ctx).Where("name = ?", name).Delete(&PremiumList{}).Error
}

// List retrieves premium lists
func (plr *PremiumListRepository) List(ctx context.Context, pagesize int, cursor string) ([]*entities.PremiumList, error) {
	dbpls := []*PremiumList{}

	err := plr.db.WithContext(ctx).Order("name ASC").Limit(pagesize).Find(&dbpls, "name > ?", cursor).Error
	if err != nil {
		return nil, err
	}

	pls := make([]*entities.PremiumList, len(dbpls))
	for i, dbpl := range dbpls {
		pls[i] = dbpl.ToEntity()
	}

	return pls, nil
}
