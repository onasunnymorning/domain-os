package postgres

import "github.com/onasunnymorning/domain-os/internal/domain/entities"

// PremiumLabel represents a premium label in our repository
type PremiumLabel struct {
	Label              string `gorm:"primary_key"`
	PremiumListName    string `gorm:"primary_key"`
	RegistrationAmount uint64
	RenewalAmount      uint64
	TransferAmount     uint64
	RestoreAmount      uint64
	Currency           string `gorm:"primary_key"`
	Class              string
}

// TableName returns the table name for the PremiumLabel model
func (PremiumLabel) TableName() string {
	return "premium_labels"
}

// ToEntity converts the PremiumLabel to a domain entity
func (pl *PremiumLabel) ToEntity() *entities.PremiumLabel {
	return &entities.PremiumLabel{
		Label:              entities.Label(pl.Label),
		PremiumListName:    pl.PremiumListName,
		RegistrationAmount: pl.RegistrationAmount,
		RenewalAmount:      pl.RenewalAmount,
		TransferAmount:     pl.TransferAmount,
		RestoreAmount:      pl.RestoreAmount,
		Currency:           pl.Currency,
		Class:              pl.Class,
	}
}

// FromEntity converts the domain entity to a PremiumLabel
func FromEntity(pl *entities.PremiumLabel) *PremiumLabel {
	return &PremiumLabel{
		Label:              pl.Label.String(),
		PremiumListName:    pl.PremiumListName,
		RegistrationAmount: pl.RegistrationAmount,
		RenewalAmount:      pl.RenewalAmount,
		TransferAmount:     pl.TransferAmount,
		RestoreAmount:      pl.RestoreAmount,
		Currency:           pl.Currency,
		Class:              pl.Class,
	}
}
