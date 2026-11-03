package entities

import (
	"errors"
	"fmt"
)

const (
	// Default values for a TLD phase
	MinLabelLength     = 1
	MaxLabelLength     = 63
	RequiresValidation = false
	BaseCurrency       = "USD"
)

// ErrInvalidPhasePolicy is a phase policy that fails Validate.
var ErrInvalidPhasePolicy = errors.New("invalid phase policy")

// PhasePolicy value object consists of the commercial and launch settings of
// a TLD that can be changed in a phase roll: label lengths, validation,
// the base currency and the contact data policy. The lifecycle timing
// (grace periods, locks, redemption, the restore report, the horizon) is
// the phase's named LifecyclePolicy (ADR 0023), not part of this object.
type PhasePolicy struct {
	MinLabelLength     int    `json:"minLabelLength,omitempty" example:"2"`
	MaxLabelLength     int    `json:"maxLabelLength,omitempty" example:"63"`
	RequiresValidation *bool  `json:"requiresValidation,omitempty" example:"false"`
	BaseCurrency       string `json:"baseCurrency,omitempty" example:"USD"`
	ContactDataPolicy
}

// PhasePolicy factory. This returns a new PhasePolicy object with default values
func NewPhasePolicy() PhasePolicy {
	rv := RequiresValidation
	return PhasePolicy{
		MinLabelLength:     MinLabelLength,
		MaxLabelLength:     MaxLabelLength,
		RequiresValidation: &rv,
		BaseCurrency:       BaseCurrency,
		ContactDataPolicy:  NewContactDataPolicy(),
	}
}

// DomainIsAllowed checks if a domain is allowed in the current phase.
func (p *PhasePolicy) LabelIsAllowed(label string) bool {
	return len(label) >= p.MinLabelLength && len(label) <= p.MaxLabelLength
}

// Validate checks the label lengths: each 1 to 63, the minimum not above
// the maximum (ADR 0023 decision 4).
func (p *PhasePolicy) Validate() error {
	switch {
	case p.MinLabelLength < 1 || p.MinLabelLength > 63:
		return fmt.Errorf("%w: minLabelLength must be 1 to 63", ErrInvalidPhasePolicy)
	case p.MaxLabelLength < 1 || p.MaxLabelLength > 63:
		return fmt.Errorf("%w: maxLabelLength must be 1 to 63", ErrInvalidPhasePolicy)
	case p.MinLabelLength > p.MaxLabelLength:
		return fmt.Errorf("%w: minLabelLength (%d) is above maxLabelLength (%d)", ErrInvalidPhasePolicy, p.MinLabelLength, p.MaxLabelLength)
	}
	return nil
}

// UpdatePolicy updates the policy with the values from the passed in policy. It will keep the default values for any fields that are not set in the passed in policy.
func (p *PhasePolicy) UpdatePolicy(newPolicy *PhasePolicy) {
	if newPolicy.MinLabelLength != 0 {
		p.MinLabelLength = newPolicy.MinLabelLength
	}
	if newPolicy.MaxLabelLength != 0 {
		p.MaxLabelLength = newPolicy.MaxLabelLength
	}
	if newPolicy.RequiresValidation != nil {
		p.RequiresValidation = newPolicy.RequiresValidation
	}
	if newPolicy.BaseCurrency != "" {
		p.BaseCurrency = newPolicy.BaseCurrency
	}
	if newPolicy.ContactDataPolicy.RegistrantContactDataPolicy != "" {
		p.ContactDataPolicy.RegistrantContactDataPolicy = newPolicy.ContactDataPolicy.RegistrantContactDataPolicy
	}
	if newPolicy.ContactDataPolicy.TechContactDataPolicy != "" {
		p.ContactDataPolicy.TechContactDataPolicy = newPolicy.ContactDataPolicy.TechContactDataPolicy
	}
	if newPolicy.ContactDataPolicy.AdminContactDataPolicy != "" {
		p.ContactDataPolicy.AdminContactDataPolicy = newPolicy.ContactDataPolicy.AdminContactDataPolicy
	}
	if newPolicy.ContactDataPolicy.BillingContactDataPolicy != "" {
		p.ContactDataPolicy.BillingContactDataPolicy = newPolicy.ContactDataPolicy.BillingContactDataPolicy
	}
}
