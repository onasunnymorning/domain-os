package postgres

import (
	"testing"
	"time"

	"github.com/onasunnymorning/domain-os/internal/domain/entities"
	"github.com/tj/assert"
)

func TestPhase_ToEntity(t *testing.T) {
	phase := &Phase{
		ID:              1,
		Name:            "TestPhase",
		Type:            "GA",
		Starts:          time.Now().UTC(),
		Ends:            nil,
		PremiumListName: "PremiumList",
		CreatedAt:       time.Now().UTC(),
		UpdatedAt:       time.Now().UTC(),
		TLDName:         "tld.com",
		PhasePolicy:     entities.NewPhasePolicy(),
	}
	expected := &entities.Phase{
		ID:              1,
		Name:            entities.ClIDType("TestPhase"),
		Type:            entities.PhaseType("GA"),
		Starts:          phase.Starts,
		Ends:            phase.Ends,
		PremiumListName: "PremiumList",
		CreatedAt:       phase.CreatedAt,
		UpdatedAt:       phase.UpdatedAt,
		TLDName:         entities.DomainName("tld.com"),
		Policy:          entities.NewPhasePolicy(),
	}
	actual := phase.ToEntity()
	assert.Equal(t, expected, actual)
}
func TestPhase_ToEntityWithFeeAndPrice(t *testing.T) {
	phase := &Phase{
		ID:              1,
		Name:            "TestPhase",
		Type:            "GA",
		Starts:          time.Now().UTC(),
		Ends:            nil,
		PremiumListName: "PremiumList",
		CreatedAt:       time.Now().UTC(),
		UpdatedAt:       time.Now().UTC(),
		TLDName:         "tld.com",
		PhasePolicy:     entities.NewPhasePolicy(),
		Fees:            []Fee{{Name: "TestFee", Amount: 100}},
		Prices:          []Price{{Currency: "PEN", RegistrationAmount: 100}},
	}
	expected := &entities.Phase{
		ID:              1,
		Name:            entities.ClIDType("TestPhase"),
		Type:            entities.PhaseType("GA"),
		Starts:          phase.Starts,
		Ends:            phase.Ends,
		PremiumListName: "PremiumList",
		CreatedAt:       phase.CreatedAt,
		UpdatedAt:       phase.UpdatedAt,
		TLDName:         entities.DomainName("tld.com"),
		Policy:          entities.NewPhasePolicy(),
		Fees:            []entities.Fee{{Name: "TestFee", Amount: 100}},
		Prices:          []entities.Price{{Currency: "PEN", RegistrationAmount: 100}},
	}
	actual := phase.ToEntity()
	assert.Equal(t, expected, actual)
}

func TestPhase_FromEntity(t *testing.T) {
	phase := &Phase{}
	expected := &entities.Phase{
		ID:              1,
		Name:            entities.ClIDType("TestPhase"),
		Type:            entities.PhaseType("GA"),
		Starts:          time.Now().UTC(),
		Ends:            nil,
		PremiumListName: "PremiumList",
		CreatedAt:       time.Now().UTC(),
		UpdatedAt:       time.Now().UTC(),
		TLDName:         entities.DomainName("tld.com"),
	}
	phase.FromEntity(expected)
	assert.Equal(t, expected.ID, phase.ID)
	assert.Equal(t, string(expected.Name), phase.Name)
	assert.Equal(t, string(expected.Type), phase.Type)
	assert.Equal(t, expected.Starts, phase.Starts)
	assert.Equal(t, expected.Ends, phase.Ends)
	assert.Equal(t, expected.PremiumListName, phase.PremiumListName)
	assert.Equal(t, expected.CreatedAt, phase.CreatedAt)
	assert.Equal(t, expected.UpdatedAt, phase.UpdatedAt)
	assert.Equal(t, string(expected.TLDName), phase.TLDName)
	assert.Equal(t, expected.Policy, phase.PhasePolicy)
}

func TestPhase_FromEntityWithFeeAndPrice(t *testing.T) {
	phase := &Phase{
		Fees:   []Fee{{Name: "TestFee", Amount: 100}},
		Prices: []Price{{Currency: "PEN", RegistrationAmount: 100}},
	}
	expected := &entities.Phase{
		ID:              1,
		Name:            entities.ClIDType("TestPhase"),
		Type:            entities.PhaseType("GA"),
		Starts:          time.Now().UTC(),
		Ends:            nil,
		PremiumListName: "PremiumList",
		CreatedAt:       time.Now().UTC(),
		UpdatedAt:       time.Now().UTC(),
		TLDName:         entities.DomainName("tld.com"),
		Fees:            []entities.Fee{{Name: "TestFee", Amount: 100}},
		Prices:          []entities.Price{{Currency: "PEN", RegistrationAmount: 100}},
	}
	phase.FromEntity(expected)
	assert.Equal(t, expected.ID, phase.ID)
	assert.Equal(t, string(expected.Name), phase.Name)
	assert.Equal(t, string(expected.Type), phase.Type)
	assert.Equal(t, expected.Starts, phase.Starts)
	assert.Equal(t, expected.Ends, phase.Ends)
	assert.Equal(t, expected.PremiumListName, phase.PremiumListName)
	assert.Equal(t, expected.CreatedAt, phase.CreatedAt)
	assert.Equal(t, expected.UpdatedAt, phase.UpdatedAt)
	assert.Equal(t, string(expected.TLDName), phase.TLDName)
	assert.Equal(t, expected.Policy, phase.PhasePolicy)
	assert.Equal(t, 1, len(phase.Fees))
	assert.Equal(t, expected.Fees[0].Name, phase.Fees[0].Name)
	assert.Equal(t, expected.Fees[0].Amount, phase.Fees[0].Amount)
	assert.Equal(t, 1, len(phase.Prices))
	assert.Equal(t, expected.Prices[0].Currency, phase.Prices[0].Currency)
	assert.Equal(t, expected.Prices[0].RegistrationAmount, phase.Prices[0].RegistrationAmount)
}
