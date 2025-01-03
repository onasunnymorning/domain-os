package entities

import (
	"testing"

	assert "github.com/stretchr/testify/assert"
)

func TestNewPhasePolicy(t *testing.T) {
	ar := true
	rv := false
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
	assert.Equal(t, &ar, phasePolicy.AllowAutoRenew)
	assert.Equal(t, &rv, phasePolicy.RequiresValidation)
	assert.Equal(t, BaseCurrency, phasePolicy.BaseCurrency)
	assert.NotNil(t, phasePolicy.ContactDataPolicy)
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
func TestPhasePolicy_UpdatePolicy(t *testing.T) {
	phasePolicy := NewPhasePolicy()

	// Test case: update all fields
	ar := false
	rv := true
	newPolicy := &PhasePolicy{
		MinLabelLength:     2,
		MaxLabelLength:     64,
		RegistrationGP:     6,
		RenewalGP:          6,
		AutoRenewalGP:      46,
		TransferGP:         6,
		RedemptionGP:       31,
		PendingDeleteGP:    6,
		TransferLockPeriod: 61,
		MaxHorizon:         11,
		AllowAutoRenew:     &ar,
		RequiresValidation: &rv,
		BaseCurrency:       "EUR",
	}
	phasePolicy.UpdatePolicy(newPolicy)
	assert.Equal(t, newPolicy.MinLabelLength, phasePolicy.MinLabelLength)
	assert.Equal(t, newPolicy.MaxLabelLength, phasePolicy.MaxLabelLength)
	assert.Equal(t, newPolicy.RegistrationGP, phasePolicy.RegistrationGP)
	assert.Equal(t, newPolicy.RenewalGP, phasePolicy.RenewalGP)
	assert.Equal(t, newPolicy.AutoRenewalGP, phasePolicy.AutoRenewalGP)
	assert.Equal(t, newPolicy.TransferGP, phasePolicy.TransferGP)
	assert.Equal(t, newPolicy.RedemptionGP, phasePolicy.RedemptionGP)
	assert.Equal(t, newPolicy.PendingDeleteGP, phasePolicy.PendingDeleteGP)
	assert.Equal(t, newPolicy.TransferLockPeriod, phasePolicy.TransferLockPeriod)
	assert.Equal(t, newPolicy.MaxHorizon, phasePolicy.MaxHorizon)
	assert.Equal(t, newPolicy.AllowAutoRenew, phasePolicy.AllowAutoRenew)
	assert.Equal(t, newPolicy.RequiresValidation, phasePolicy.RequiresValidation)
	assert.Equal(t, newPolicy.BaseCurrency, phasePolicy.BaseCurrency)

	// Test case: update some fields
	phasePolicy = NewPhasePolicy()
	newPolicy = &PhasePolicy{
		MinLabelLength: 3,
		MaxLabelLength: 65,
		RegistrationGP: 7,
	}
	phasePolicy.UpdatePolicy(newPolicy)
	assert.Equal(t, newPolicy.MinLabelLength, phasePolicy.MinLabelLength)
	assert.Equal(t, newPolicy.MaxLabelLength, phasePolicy.MaxLabelLength)
	assert.Equal(t, newPolicy.RegistrationGP, phasePolicy.RegistrationGP)
	assert.Equal(t, phasePolicy.RenewalGP, phasePolicy.RenewalGP)
	assert.Equal(t, phasePolicy.AutoRenewalGP, phasePolicy.AutoRenewalGP)
	assert.Equal(t, phasePolicy.TransferGP, phasePolicy.TransferGP)
	assert.Equal(t, phasePolicy.RedemptionGP, phasePolicy.RedemptionGP)
	assert.Equal(t, phasePolicy.PendingDeleteGP, phasePolicy.PendingDeleteGP)
	assert.Equal(t, phasePolicy.TransferLockPeriod, phasePolicy.TransferLockPeriod)
	assert.Equal(t, phasePolicy.MaxHorizon, phasePolicy.MaxHorizon)
	assert.Equal(t, phasePolicy.AllowAutoRenew, phasePolicy.AllowAutoRenew)
	assert.Equal(t, phasePolicy.RequiresValidation, phasePolicy.RequiresValidation)
	assert.Equal(t, phasePolicy.BaseCurrency, phasePolicy.BaseCurrency)

	// Test case: update no fields
	phasePolicy = NewPhasePolicy()
	newPolicy = &PhasePolicy{}
	phasePolicy.UpdatePolicy(newPolicy)
	assert.Equal(t, phasePolicy.MinLabelLength, phasePolicy.MinLabelLength)
	assert.Equal(t, phasePolicy.MaxLabelLength, phasePolicy.MaxLabelLength)
	assert.Equal(t, phasePolicy.RegistrationGP, phasePolicy.RegistrationGP)
	assert.Equal(t, phasePolicy.RenewalGP, phasePolicy.RenewalGP)
	assert.Equal(t, phasePolicy.AutoRenewalGP, phasePolicy.AutoRenewalGP)
	assert.Equal(t, phasePolicy.TransferGP, phasePolicy.TransferGP)
	assert.Equal(t, phasePolicy.RedemptionGP, phasePolicy.RedemptionGP)
	assert.Equal(t, phasePolicy.PendingDeleteGP, phasePolicy.PendingDeleteGP)
	assert.Equal(t, phasePolicy.TransferLockPeriod, phasePolicy.TransferLockPeriod)
	assert.Equal(t, phasePolicy.MaxHorizon, phasePolicy.MaxHorizon)
	assert.Equal(t, phasePolicy.AllowAutoRenew, phasePolicy.AllowAutoRenew)
	assert.Equal(t, phasePolicy.RequiresValidation, phasePolicy.RequiresValidation)
	assert.Equal(t, phasePolicy.BaseCurrency, phasePolicy.BaseCurrency)
}
