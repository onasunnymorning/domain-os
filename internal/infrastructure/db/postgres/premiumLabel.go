package postgres

import "github.com/onasunnymorning/domain-os/internal/domain/entities"

// PremiumLabel represents a premium label in our repository
type PremiumLabel struct {
	ID                 int64  `gorm:"primaryKey"`
	Label              string `gorm:"uniqueIndex:idx_uniq_label_list_cur;not null"`
	PremiumListName    string `gorm:"uniqueIndex:idx_uniq_label_list_cur;not null"`
	RegistrationAmount uint64
	RenewalAmount      uint64
	TransferAmount     uint64
	RestoreAmount      uint64
	Currency           string `gorm:"uniqueIndex:idx_uniq_label_list_cur;not null"`
	Class              string
}

// TableName returns the table name for the PremiumLabel model
func (PremiumLabel) TableName() string {
	return "premium_labels"
}

// ToEntity converts the PremiumLabel to a domain entity
func (pl *PremiumLabel) ToEntity() *entities.PremiumLabel {
	return &entities.PremiumLabel{
		ID:                 pl.ID,
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
		ID:                 pl.ID,
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
