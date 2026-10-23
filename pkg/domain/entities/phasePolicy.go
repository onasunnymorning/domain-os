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
	// PendingTransferPeriod is how many days the losing registrar has to
	// approve or reject a transfer before the registry approves it.
	PendingTransferPeriod = 5
	MaxHorizon            = 10
	AllowAutoRenew        = true
	RequiresValidation    = false
	BaseCurrency          = "USD"
)

// PhasePolicy value object consists of all the settings of a TLD that can be changed in a phase roll.
//
// The grace periods (RegistrationGP, RenewalGP, AutoRenewalGP, TransferGP)
// are the days after a transaction in which a delete earns a refund, and in
// which <rgp:infData> shows addPeriod, renewPeriod, autoRenewPeriod or
// transferPeriod (RFC 3915). Transfers have two more settings:
// PendingTransferPeriod, the days the losing registrar has to answer a
// transfer request, and TransferLockPeriod, the days after a registration or
// a transfer in which the domain cannot be transferred.
type PhasePolicy struct {
	MinLabelLength int `json:"minLabelLength,omitempty" example:"2"`
	MaxLabelLength int `json:"maxLabelLength,omitempty" example:"63"`
	RegistrationGP int `json:"registrationGP,omitempty" example:"5"`
	RenewalGP      int `json:"renewalGP,omitempty" example:"5"`
	AutoRenewalGP  int `json:"autorenewalGP,omitempty" example:"45"`
	TransferGP     int `json:"transferGP,omitempty" example:"5"`
	// PendingTransferPeriod: see the type's comment. 0 is not a setting;
	// the default applies.
	PendingTransferPeriod int `json:"pendingTransferPeriod,omitempty" example:"5"`
	RedemptionGP          int `json:"redemptionGP,omitempty" example:"30"`
	PendingDeleteGP       int `json:"pendingdeleteGP,omitempty" example:"5"`
	// TransferLockPeriod: see the type's comment. A pointer so that 0, no
	// lock (an RST or OT&E TLD), can be set; nil is the default. Read it
	// with TransferLockDays.
	TransferLockPeriod *int   `json:"transferLockPeriod,omitempty" example:"60"`
	MaxHorizon         int    `json:"maxHorizon,omitempty" example:"10"`
	AllowAutoRenew     *bool  `json:"allowAutorenew,omitempty" example:"true"`
	RequiresValidation *bool  `json:"requiresValidation,omitempty" example:"false"`
	BaseCurrency       string `json:"baseCurrency,omitempty" example:"USD"`
	ContactDataPolicy
}

// PhasePolicy factory. This returns a new PhasePolicy object with default values
func NewPhasePolicy() PhasePolicy {
	ar := AllowAutoRenew
	rv := RequiresValidation
	lock := TransferLockPeriod
	return PhasePolicy{
		MinLabelLength:        MinLabelLength,
		MaxLabelLength:        MaxLabelLength,
		RegistrationGP:        RegistrationGP,
		RenewalGP:             RenewalGP,
		AutoRenewalGP:         AutoRenewalGP,
		TransferGP:            TransferGP,
		PendingTransferPeriod: PendingTransferPeriod,
		RedemptionGP:          RedemptionGP,
		PendingDeleteGP:       PendingDeleteGP,
		TransferLockPeriod:    &lock,
		MaxHorizon:            MaxHorizon,
		AllowAutoRenew:        &ar,
		RequiresValidation:    &rv,
		BaseCurrency:          BaseCurrency,
		ContactDataPolicy:     NewContactDataPolicy(),
	}
}

// TransferLockDays is the transfer lock period in days: the setting, or
// the default when none is set.
func (p *PhasePolicy) TransferLockDays() int {
	if p.TransferLockPeriod == nil {
		return TransferLockPeriod
	}
	return *p.TransferLockPeriod
}

// TransferGraceDays is the transfer grace period in days: the setting, or
// the default when none is set. Phases created before the default was set
// (it was missing from NewPhasePolicy) store 0, which UpdatePolicy cannot
// set, so 0 is read as not set.
func (p *PhasePolicy) TransferGraceDays() int {
	if p.TransferGP <= 0 {
		return TransferGP
	}
	return p.TransferGP
}

// PendingTransferDays is the pending transfer period in days: the setting,
// or the default when none is set.
func (p *PhasePolicy) PendingTransferDays() int {
	if p.PendingTransferPeriod <= 0 {
		return PendingTransferPeriod
	}
	return p.PendingTransferPeriod
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
	if newPolicy.PendingTransferPeriod != 0 {
		p.PendingTransferPeriod = newPolicy.PendingTransferPeriod
	}
	if newPolicy.RedemptionGP != 0 {
		p.RedemptionGP = newPolicy.RedemptionGP
	}
	if newPolicy.PendingDeleteGP != 0 {
		p.PendingDeleteGP = newPolicy.PendingDeleteGP
	}
	// Given, even as 0, the lock is set: 0 is no lock.
	if newPolicy.TransferLockPeriod != nil {
		lock := *newPolicy.TransferLockPeriod
		p.TransferLockPeriod = &lock
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
