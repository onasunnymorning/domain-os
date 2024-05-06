package postgres

import (
	"github.com/onasunnymorning/domain-os/internal/domain/entities"
	"gorm.io/gorm"
)

// PremiumListRepository implements the PremiumListRepository interface
type PremiumListRepository struct {
	db *gorm.DB
}

// NewPremiumListRepository creates a new PremiumListRepository instance
func NewPremiumListRepository(db *gorm.DB) *PremiumListRepository {
	return &PremiumListRepository{
		db: db,
	}
}

// Create creates a new premium list in the database
func (plr *PremiumListRepository) Create(premiumList *entities.PremiumList) error {
	pl := &PremiumList{}
	pl.FromEntity(premiumList)
	return plr.db.Create(pl).Error
}
