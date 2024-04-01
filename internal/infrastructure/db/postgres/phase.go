package postgres

import (
	"time"

	"github.com/onasunnymorning/domain-os/internal/domain/entities"
)

// Phase GORM entity
type Phase struct {
	ID     int64  `gorm:"primaryKey"`
	Name   string `gorm:"uniqueIndex"`
	Type   string `gorm:"index"`
	Starts time.Time
	Ends   *time.Time
	// Prices          []Price
	// Fees            []Fee
	PremiumListName      string
	CreatedAt            time.Time
	UpdatedAt            time.Time
	TLDName              string `gorm:"not null;foreignKey"`
	TLD                  TLD
	entities.PhasePolicy `gorm:"embedded"`
}

// TableName returns the table name for the Phase model
func (Phase) TableName() string {
	return "phases"
}

// ToEntity converts a Phase to a domain model *entities.Phase
func (p *Phase) ToEntity() *entities.Phase {
	phase := &entities.Phase{
		ID:              p.ID,
		Name:            entities.ClIDType(p.Name),
		Type:            entities.PhaseType(p.Type),
		Starts:          p.Starts,
		Ends:            p.Ends,
		PremiumListName: p.PremiumListName,
		CreatedAt:       p.CreatedAt,
		UpdatedAt:       p.UpdatedAt,
		TLDName:         entities.DomainName(p.TLDName),
		PhasePolicy:     p.PhasePolicy,
	}
	return phase
}

// FromEntity converts a domain model *entities.Phase to a Phase
func (p *Phase) FromEntity(phase *entities.Phase) {
	p.ID = phase.ID
	p.Name = string(phase.Name)
	p.Type = string(phase.Type)
	p.Starts = phase.Starts
	p.Ends = phase.Ends
	p.PremiumListName = phase.PremiumListName
	p.CreatedAt = phase.CreatedAt
	p.UpdatedAt = phase.UpdatedAt
	p.TLDName = string(phase.TLDName)
	p.PhasePolicy = phase.PhasePolicy
}
