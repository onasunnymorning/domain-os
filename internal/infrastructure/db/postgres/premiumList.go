package postgres

import (
	"github.com/onasunnymorning/domain-os/internal/domain/entities"
)

// PremiumList represents a premium list in our repository
type PremiumList struct {
	Name      string `gorm:"primaryKey"`
	CreatedAt string
	UpdatedAt string
}

// TableName returns the table name for the PremiumList model
func (pl *PremiumList) TableName() string {
	return "premium_lists"
}

// ToEntity converts a PremiumList to a domain entity
func (pl *PremiumList) ToEntity() *entities.PremiumList {
	return &entities.PremiumList{
		Name:      pl.Name,
		CreatedAt: pl.CreatedAt,
		UpdatedAt: pl.UpdatedAt,
	}
}

// FromEntity converts a domain entity to a PremiumList
func (pl *PremiumList) FromEntity(premiumList *entities.PremiumList) {
	pl.Name = premiumList.Name
	pl.CreatedAt = premiumList.CreatedAt
	pl.UpdatedAt = premiumList.UpdatedAt
}
