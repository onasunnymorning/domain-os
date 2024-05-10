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
	assert.Equal(t, TransferLockPeriod, phasePolicy.TransferLockPeriod)
	assert.Equal(t, MaxHorizon, phasePolicy.MaxHorizon)
	assert.Equal(t, AllowAutoRenew, phasePolicy.AllowAutoRenew)
	assert.Equal(t, RequiresValidation, phasePolicy.RequiresValidation)
	assert.Equal(t, BaseCurrency, phasePolicy.BaseCurrency)
}
func TestPhasePolicy_LabelIsAllowed(t *testing.T) {
	phasePolicy := NewPhasePolicy()

	// Test case: label length is within the allowed range
	label := "example"
	expected := true
	actual := phasePolicy.LabelIsAllowed(label)
	assert.Equal(t, expected, actual)

	// Test case: label length is equal than the minimum allowed length
	label = "a"
	expected = true
	actual = phasePolicy.LabelIsAllowed(label)
	assert.Equal(t, expected, actual)

	// Test case: label length is less than the minimum allowed length
	label = ""
	expected = false
	actual = phasePolicy.LabelIsAllowed(label)
	assert.Equal(t, expected, actual)

	// Test case: label length is greater than the maximum allowed length
	label = "thisisaverylonglabelthatexceedsthemaximumallowedlengthofsixtythreecharacters"
	expected = false
	actual = phasePolicy.LabelIsAllowed(label)
	assert.Equal(t, expected, actual)

	// Test case: label length is less than the minimum allowed length
	phasePolicy.MinLabelLength = 2
	label = "a"
	expected = false
	actual = phasePolicy.LabelIsAllowed(label)
	assert.Equal(t, expected, actual)
}
