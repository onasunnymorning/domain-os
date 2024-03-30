package entities

import (
	"testing"

	assert "github.com/stretchr/testify/assert"
)

func TestNewPhasePolicy(t *testing.T) {
	phasePolicy := NewPhasePolicy()
	assert.Equal(t, MinLabelLength, phasePolicy.MinLabelLength)
	assert.Equal(t, MaxLabelLength, phasePolicy.MaxLabelLength)
	assert.Equal(t, RegistrationGP, phasePolicy.RegistrationGP)
	assert.Equal(t, RenewalGP, phasePolicy.RenewalGP)
	assert.Equal(t, AutoRenewalGP, phasePolicy.AutoRenewalGP)
	assert.Equal(t, RedemptionGP, phasePolicy.RedemptionGP)
	assert.Equal(t, PendingDeleteGP, phasePolicy.PendingDeleteGP)
	assert.Equal(t, TrLockPeriod, phasePolicy.TrLockPeriod)
	assert.Equal(t, MaxHorizon, phasePolicy.MaxHorizon)
	assert.Equal(t, AllowAutoRenew, phasePolicy.AllowAutoRenew)
	assert.Equal(t, RequiresValidation, phasePolicy.RequiresValidation)
	assert.Equal(t, BaseCurrency, phasePolicy.BaseCurrency)
}
