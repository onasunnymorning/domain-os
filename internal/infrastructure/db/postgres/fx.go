package postgres

import (
	"time"

	"github.com/onasunnymorning/domain-os/internal/domain/entities"
)

// FX represents a foreign exchange rate in the database
type FX struct {
	Date      time.Time `gorm:"primaryKey"`
	Ticker    string    `gorm:"primaryKey"`
	Rate      float64
	CreatedAt time.Time
	UpdatedAt time.Time
}

// TableName returns the table name for the FX model
func (FX) TableName() string {
	return "fx"
}

// TickerFrom returns the 'from' currency of the FX model (e.g. USDPEN would return USD)
func (fx *FX) TickerFrom() string {
	return fx.Ticker[:3]
}

// TickerTo returns the 'to' currency of the FX model (e.g. USDPEN would return PEN)
func (fx *FX) TickerTo() string {
	return fx.Ticker[3:]
}

// ToEntity converts the FX model to an FX entity
func (fx *FX) ToEntity() *entities.FX {
	return &entities.FX{
		Date: fx.Date,
		From: fx.TickerFrom(),
		To:   fx.TickerTo(),
		Rate: fx.Rate,
	}
}

// FromEntity converts an FX entity to an FX model
func (fx *FX) FromEntity(entity *entities.FX) {
	fx.Date = entity.Date
	fx.Ticker = entity.From + entity.To
	fx.Rate = entity.Rate
}
