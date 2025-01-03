package entities

const (
	// Default values for a TLD phase
	MinLabelLength     = 1
	MaxLabelLength     = 63
	RegistrationGP     = 5
	RenewalGP          = 5
	AutoRenewalGP      = 45
	TransferGP         = 5
	RedemptionGP       = 30
	PendingDeleteGP    = 5
	TransferLockPeriod = 60
	MaxHorizon         = 10
	AllowAutoRenew     = true
	RequiresValidation = false
	BaseCurrency       = "USD"
)

// PhasePolicy value object consists of all the settings of a TLD that can be changed in a phase roll
type PhasePolicy struct {
	MinLabelLength     int    `json:"minLabelLenght,omitempty" example:"2"`
	MaxLabelLength     int    `json:"maxLabelLenght,omitempty" example:"63"`
	RegistrationGP     int    `json:"registrationGP,omitempty" example:"5"`
	RenewalGP          int    `json:"renewalGP,omitempty" example:"5"`
	AutoRenewalGP      int    `json:"autorenewalGP,omitempty" example:"45"`
	TransferGP         int    `json:"transferGP,omitempty" example:"5"`
	RedemptionGP       int    `json:"redemptionGP,omitempty" example:"30"`
	PendingDeleteGP    int    `json:"pendingdeleteGP,omitempty" example:"5"`
	TransferLockPeriod int    `json:"transferLockPeriod,omitempty" example:"60"`
	MaxHorizon         int    `json:"maxHorizon,omitempty" example:"10"`
	AllowAutoRenew     *bool  `json:"allowAutorenew,omitempty" example:"true"`
	RequiresValidation *bool  `json:"requiresValidation,omitempty" example:"false"`
	BaseCurrency       string `json:"baseCurrency,omitempty" example:"USD"`
	ContactPolicy
}

// PhasePolicy factory. This returns a new PhasePolicy object with default values
func NewPhasePolicy() PhasePolicy {
	ar := AllowAutoRenew
	rv := RequiresValidation
	return PhasePolicy{
		MinLabelLength:     MinLabelLength,
		MaxLabelLength:     MaxLabelLength,
		RegistrationGP:     RegistrationGP,
		RenewalGP:          RenewalGP,
		AutoRenewalGP:      AutoRenewalGP,
		RedemptionGP:       RedemptionGP,
		PendingDeleteGP:    PendingDeleteGP,
		TransferLockPeriod: TransferLockPeriod,
		MaxHorizon:         MaxHorizon,
		AllowAutoRenew:     &ar,
		RequiresValidation: &rv,
		BaseCurrency:       BaseCurrency,
		ContactPolicy:      NewContactPolicy(),
	}
}

// DomainIsAllowed checks if a domain is allowed in the current phase.
func (p *PhasePolicy) LabelIsAllowed(label string) bool {
	return len(label) >= p.MinLabelLength && len(label) <= p.MaxLabelLength
}

// UpdatePolicy updates the policy with the values from the passed in policy. It will keep the default values for any fields that are not set in the passed in policy.
func (p *PhasePolicy) UpdatePolicy(newPolicy *PhasePolicy) {
	if newPolicy.MinLabelLength != 0 {
		p.MinLabelLength = newPolicy.MinLabelLength
	}
	if newPolicy.MaxLabelLength != 0 {
		p.MaxLabelLength = newPolicy.MaxLabelLength
	}
	if newPolicy.RegistrationGP != 0 {
		p.RegistrationGP = newPolicy.RegistrationGP
	}
	if newPolicy.RenewalGP != 0 {
		p.RenewalGP = newPolicy.RenewalGP
	}
	if newPolicy.AutoRenewalGP != 0 {
		p.AutoRenewalGP = newPolicy.AutoRenewalGP
	}
	if newPolicy.TransferGP != 0 {
		p.TransferGP = newPolicy.TransferGP
	}
	if newPolicy.RedemptionGP != 0 {
		p.RedemptionGP = newPolicy.RedemptionGP
	}
	if newPolicy.PendingDeleteGP != 0 {
		p.PendingDeleteGP = newPolicy.PendingDeleteGP
	}
	if newPolicy.TransferLockPeriod != 0 {
		p.TransferLockPeriod = newPolicy.TransferLockPeriod
	}
	if newPolicy.MaxHorizon != 0 {
		p.MaxHorizon = newPolicy.MaxHorizon
	}
	if newPolicy.AllowAutoRenew != nil {
		p.AllowAutoRenew = newPolicy.AllowAutoRenew
	}
	if newPolicy.RequiresValidation != nil {
		p.RequiresValidation = newPolicy.RequiresValidation
	}
	if newPolicy.BaseCurrency != "" {
		p.BaseCurrency = newPolicy.BaseCurrency
	}
}
