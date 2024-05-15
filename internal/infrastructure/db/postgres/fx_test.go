package postgres

import (
	"testing"
	"time"

	"github.com/onasunnymorning/domain-os/internal/domain/entities"
	"github.com/tj/assert"
)

func TestFX_Tablename(t *testing.T) {
	fx := FX{}
	assert.Equal(t, "fx", fx.TableName())
}

func TestFX_ToEntity(t *testing.T) {
	testTimeString := "2021-01-01T00:00:00Z"
	testTime, _ := time.Parse(time.RFC3339, testTimeString)
	fx := FX{
		Date:   testTime,
		Base:   "USD",
		Target: "EUR",
		Rate:   1.5,
	}
	entity := fx.ToEntity()
	assert.Equal(t, testTimeString, entity.Date.Format(time.RFC3339))
	assert.Equal(t, "USD", entity.BaseCurrency)
	assert.Equal(t, "EUR", entity.TargetCurrency)
	assert.Equal(t, 1.5, entity.Rate)
}

func TestFX_FromEntity(t *testing.T) {
	testTimeString := "2021-01-01T00:00:00Z"
	testTime, _ := time.Parse(time.RFC3339, testTimeString)
	entity := &entities.FX{
		Date:           testTime,
		BaseCurrency:   "USD",
		TargetCurrency: "EUR",
		Rate:           1.5,
	}
	fx := FX{}
	fx.FromEntity(entity)
	assert.Equal(t, testTime, fx.Date)
	assert.Equal(t, "USD", fx.Base)
	assert.Equal(t, "EUR", fx.Target)
	assert.Equal(t, 1.5, fx.Rate)
}
