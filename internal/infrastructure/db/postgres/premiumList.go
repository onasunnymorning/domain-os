package postgres

import (
	"time"

	"github.com/onasunnymorning/domain-os/internal/domain/entities"
)

// PremiumList represents a premium list in our repository
type PremiumList struct {
	Name          string `gorm:"primaryKey"`
	RyID          string
	CreatedAt     time.Time
	UpdatedAt     time.Time
	PremiumLabels []PremiumLabel `gorm:"foreignKey:PremiumListName;constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
}

// TableName returns the table name for the PremiumList model
func (pl *PremiumList) TableName() string {
	return "premium_lists"
}

// ToEntity converts a PremiumList to a domain entity
func (pl *PremiumList) ToEntity() *entities.PremiumList {
	return &entities.PremiumList{
		Name:      pl.Name,
		CreatedAt: pl.CreatedAt.UTC(),
		UpdatedAt: pl.UpdatedAt.UTC(),
		RyID:      entities.ClIDType(pl.RyID),
	}
}

// FromEntity converts a domain entity to a PremiumList
func (pl *PremiumList) FromEntity(premiumList *entities.PremiumList) {
	pl.Name = premiumList.Name
	pl.CreatedAt = premiumList.CreatedAt.UTC()
	pl.UpdatedAt = premiumList.UpdatedAt.UTC()
	pl.RyID = premiumList.RyID.String()
}
