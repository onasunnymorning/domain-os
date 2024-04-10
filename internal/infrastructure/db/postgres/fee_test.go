package postgres

import (
	"testing"

	"github.com/onasunnymorning/domain-os/internal/domain/entities"
	"github.com/stretchr/testify/require"
)

func TestFee_TableName(t *testing.T) {
	fee := &Fee{}
	require.Equal(t, "phase_fees", fee.TableName())
}

func TestFee_FromEntity(t *testing.T) {
	b := true
	entity := &entities.Fee{
		Currency:   "USD",
		Name:       "registration",
		Amount:     1000,
		Refundable: &b,
		PhaseID:    1,
	}

	fee := &Fee{}
	fee.FromEntity(entity)

	require.Equal(t, entity.Currency, fee.Currency)
	require.Equal(t, entity.Name.String(), fee.Name)
	require.Equal(t, entity.Amount, fee.Amount)
	require.Equal(t, entity.Refundable, fee.Refundable)
	require.Equal(t, entity.PhaseID, fee.PhaseID)

}

func TestFee_ToEntity(t *testing.T) {
	b := true
	fee := &Fee{
		Currency:   "USD",
		Name:       "registration",
		Amount:     1000,
		Refundable: &b,
		PhaseID:    1,
	}

	entity := fee.ToEntity()

	require.Equal(t, fee.Currency, entity.Currency)
	require.Equal(t, fee.Name, entity.Name.String())
	require.Equal(t, fee.Amount, entity.Amount)
	require.Equal(t, fee.Refundable, entity.Refundable)
	require.Equal(t, fee.PhaseID, entity.PhaseID)
}
