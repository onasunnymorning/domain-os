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
		Starts:          time.Now(),
		Ends:            nil,
		PremiumListName: "PremiumList",
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
		TLDName:         "tld.com",
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
		Starts:          time.Now(),
		Ends:            nil,
		PremiumListName: "PremiumList",
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
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
}
