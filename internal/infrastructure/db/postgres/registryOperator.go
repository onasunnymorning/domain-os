package postgres

import (
	"time"

	"github.com/onasunnymorning/domain-os/internal/domain/entities"
)

// RegistryOperator represents a registry Operator Entity in our repository
type RegistryOperator struct {
	RyID      string `gorm:"primary_key"`
	Name      string `gorm:"unique;not null"`
	URL       string
	Email     string `gorm:"not null"`
	Voice     string
	Fax       string
	CreatedAt time.Time
	UpdatedAt time.Time

	PremiumLists []*PremiumList `gorm:"foreignKey:RyID"`
}

// TableName returns the table name for the RegistryOperator entity
func (RegistryOperator) TableName() string {
	return "registry_operators"
}

// ToEntity converts the RegistryOperator to an entity
func (ro *RegistryOperator) ToEntity() *entities.RegistryOperator {
	pls := make([]*entities.PremiumList, len(ro.PremiumLists))
	for i, pl := range ro.PremiumLists {
		pls[i] = pl.ToEntity()
	}
	return &entities.RegistryOperator{
		RyID:         entities.ClIDType(ro.RyID),
		Name:         ro.Name,
		URL:          entities.URL(ro.URL),
		Email:        ro.Email,
		Voice:        entities.E164Type(ro.Voice),
		Fax:          entities.E164Type(ro.Fax),
		CreatedAt:    ro.CreatedAt.UTC(),
		UpdatedAt:    ro.UpdatedAt.UTC(),
		PremiumLists: pls,
	}
}

// FromEntity converts the entity to a RegistryOperator
func (ro *RegistryOperator) FromEntity(e *entities.RegistryOperator) {
	ro.RyID = e.RyID.String()
	ro.Name = e.Name
	ro.URL = e.URL.String()
	ro.Email = e.Email
	ro.Voice = e.Voice.String()
	ro.Fax = e.Fax.String()
	ro.CreatedAt = e.CreatedAt.UTC()
	ro.UpdatedAt = e.UpdatedAt.UTC()

	pls := make([]*PremiumList, len(e.PremiumLists))
	for i, pl := range e.PremiumLists {
		pls[i] = &PremiumList{}
		pls[i].FromEntity(pl)
	}
	ro.PremiumLists = pls
}
