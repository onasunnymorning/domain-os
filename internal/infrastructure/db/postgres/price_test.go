package postgres

import (
	"testing"

	"github.com/onasunnymorning/domain-os/internal/domain/entities"
	"github.com/stretchr/testify/require"
)

func TestPrice_TableName(t *testing.T) {
	price := &Price{}
	require.Equal(t, "phase_prices", price.TableName())
}

func TestPrice_FromEntity_(t *testing.T) {
	entity := &entities.Price{
		Currency:           "USD",
		RegistrationAmount: 1000,
		RenewalAmount:      1000,
		TransferAmount:     1000,
		RestoreAmount:      1000,
		PhaseID:            1,
	}

	price := &Price{}
	price.FromEntity(entity)

	require.Equal(t, entity.Currency, price.Currency)
	require.Equal(t, entity.RegistrationAmount, price.RegistrationAmount)
	require.Equal(t, entity.RenewalAmount, price.RenewalAmount)
	require.Equal(t, entity.TransferAmount, price.TransferAmount)
	require.Equal(t, entity.RestoreAmount, price.RestoreAmount)
	require.Equal(t, entity.PhaseID, price.PhaseID)

}

func TestPrice_ToEntity(t *testing.T) {
	price := &Price{
		Currency:           "USD",
		RegistrationAmount: 1000,
		RenewalAmount:      1000,
		TransferAmount:     1000,
		RestoreAmount:      1000,
		PhaseID:            1,
	}

	entity := price.ToEntity()

	require.Equal(t, price.Currency, entity.Currency)
	require.Equal(t, price.RegistrationAmount, entity.RegistrationAmount)
	require.Equal(t, price.RenewalAmount, entity.RenewalAmount)
	require.Equal(t, price.TransferAmount, entity.TransferAmount)
	require.Equal(t, price.RestoreAmount, entity.RestoreAmount)
	require.Equal(t, price.PhaseID, entity.PhaseID)
}
