package postgres

import "github.com/onasunnymorning/domain-os/internal/domain/entities"

// Fee is the GORM model for the phase_fee table. Uses a composite primary key to ensure that a fee with the same currency and name is not inserted twice in a phase
type Fee struct {
	Currency   string `gorm:"primaryKey"`
	Name       string `gorm:"primaryKey"`
	Amount     int64
	Refundable *bool
	PhaseID    int64 `gorm:"primaryKey"`
}

// TableName returns the table name for the PhaseFee model
func (Fee) TableName() string {
	return "phase_fees"
}

// FromEntity converts a Fee entity to a Fee model
func (f *Fee) FromEntity(entity *entities.Fee) {
	f.Currency = entity.Currency
	f.Name = entity.Name
	f.Amount = entity.Amount
	f.Refundable = entity.Refundable
	f.PhaseID = entity.PhaseID
}

// ToEntity converts a Fee model to a Fee entity
func (f *Fee) ToEntity() *entities.Fee {
	return &entities.Fee{
		Currency:   f.Currency,
		Name:       f.Name,
		Amount:     f.Amount,
		Refundable: f.Refundable,
		PhaseID:    f.PhaseID,
	}
}
