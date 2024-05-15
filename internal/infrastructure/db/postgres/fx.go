package postgres

import (
	"time"

	"github.com/onasunnymorning/domain-os/internal/domain/entities"
)

// FX represents a foreign exchange rate in the database
type FX struct {
	Date      time.Time `gorm:"primaryKey"`
	Base      string    `gorm:"primaryKey"`
	Target    string    `gorm:"primaryKey"`
	Rate      float64
	CreatedAt time.Time
	UpdatedAt time.Time
}

// TableName returns the table name for the FX model
func (FX) TableName() string {
	return "fx"
}

// ToEntity converts the FX struct to an entities.FX struct
func (fx *FX) ToEntity() *entities.FX {
	return &entities.FX{
		Date:           fx.Date,
		BaseCurrency:   fx.Base,
		TargetCurrency: fx.Target,
		Rate:           fx.Rate,
	}
}

// FromEntity converts an entities.FX struct to an FX struct
func (fx *FX) FromEntity(entity *entities.FX) {
	fx.Date = entity.Date
	fx.Base = entity.BaseCurrency
	fx.Target = entity.TargetCurrency
	fx.Rate = entity.Rate
}
