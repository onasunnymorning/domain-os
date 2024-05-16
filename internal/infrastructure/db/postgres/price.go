package postgres

import "github.com/onasunnymorning/domain-os/internal/domain/entities"

// Price is the GORM model for the phase_price table. We use a composite primary key to ensure that a price with the same currency is not inserted twice in the same phase
type Price struct {
	Currency           string `gorm:"primaryKey"`
	RegistrationAmount uint64
	RenewalAmount      uint64
	TransferAmount     uint64
	RestoreAmount      uint64
	PhaseID            int64 `gorm:"primaryKey"`
	Phase              Phase
}

// TableName returns the table name for the PhasePrice model
func (Price) TableName() string {
	return "phase_prices"
}

// FromEntitye converst an entities.PhasePrice to a postgres.PhasePrice
func (pp *Price) FromEntity(ppEntity *entities.Price) {
	pp.Currency = ppEntity.Currency
	pp.RegistrationAmount = ppEntity.RegistrationAmount
	pp.RenewalAmount = ppEntity.RenewalAmount
	pp.TransferAmount = ppEntity.TransferAmount
	pp.RestoreAmount = ppEntity.RestoreAmount
	pp.PhaseID = ppEntity.PhaseID
}

// ToEntity converts a postgres.PhasePrice to an entities.PhasePrice
func (pp *Price) ToEntity() *entities.Price {
	return &entities.Price{
		Currency:           pp.Currency,
		RegistrationAmount: pp.RegistrationAmount,
		RenewalAmount:      pp.RenewalAmount,
		TransferAmount:     pp.TransferAmount,
		RestoreAmount:      pp.RestoreAmount,
		PhaseID:            pp.PhaseID,
	}
}
