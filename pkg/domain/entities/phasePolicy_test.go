package entities

import (
	"testing"

	assert "github.com/stretchr/testify/assert"
)

func TestNewPhasePolicy(t *testing.T) {
	rv := false
	phasePolicy := NewPhasePolicy()
	assert.Equal(t, MinLabelLength, phasePolicy.MinLabelLength)
	assert.Equal(t, MaxLabelLength, phasePolicy.MaxLabelLength)
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
	rv := true
	ContactDataPolicy := ContactDataPolicy{
		RegistrantContactDataPolicy: ContactDataPolicyTypeMandatory,
		TechContactDataPolicy:       ContactDataPolicyTypeMandatory,
		AdminContactDataPolicy:      ContactDataPolicyTypeOptional,
		BillingContactDataPolicy:    ContactDataPolicyTypeOptional,
	}
	newPolicy := &PhasePolicy{
		MinLabelLength:     2,
		MaxLabelLength:     64,
		RequiresValidation: &rv,
		BaseCurrency:       "EUR",
		ContactDataPolicy:  ContactDataPolicy,
	}
	phasePolicy.UpdatePolicy(newPolicy)
	assert.Equal(t, newPolicy.MinLabelLength, phasePolicy.MinLabelLength)
	assert.Equal(t, newPolicy.MaxLabelLength, phasePolicy.MaxLabelLength)
	assert.Equal(t, newPolicy.RequiresValidation, phasePolicy.RequiresValidation)
	assert.Equal(t, newPolicy.BaseCurrency, phasePolicy.BaseCurrency)
	assert.Equal(t, newPolicy.ContactDataPolicy, phasePolicy.ContactDataPolicy)

	// Test case: update some fields
	phasePolicy = NewPhasePolicy()
	newPolicy = &PhasePolicy{MinLabelLength: 3}
	phasePolicy.UpdatePolicy(newPolicy)
	assert.Equal(t, 3, phasePolicy.MinLabelLength)
	assert.Equal(t, MaxLabelLength, phasePolicy.MaxLabelLength)
	assert.Equal(t, BaseCurrency, phasePolicy.BaseCurrency)

	// Test case: update no fields
	phasePolicy = NewPhasePolicy()
	phasePolicy.UpdatePolicy(&PhasePolicy{})
	assert.Equal(t, NewPhasePolicy(), phasePolicy)
}

func TestPhasePolicy_Validate(t *testing.T) {
	p := NewPhasePolicy()
	assert.NoError(t, p.Validate())
	p.MinLabelLength, p.MaxLabelLength = 10, 5
	assert.ErrorIs(t, p.Validate(), ErrInvalidPhasePolicy, "min above max")
	p.MinLabelLength, p.MaxLabelLength = 0, 5
	assert.ErrorIs(t, p.Validate(), ErrInvalidPhasePolicy, "min below 1")
	p.MinLabelLength, p.MaxLabelLength = 1, 64
	assert.ErrorIs(t, p.Validate(), ErrInvalidPhasePolicy, "max above 63")
}
