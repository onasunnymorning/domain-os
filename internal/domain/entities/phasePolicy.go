package entities

const (
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
	AllowAutoRenew     bool   `json:"allowAutorenew,omitempty" example:"true"`
	RequiresValidation bool   `json:"requiresValidation,omitempty" example:"false"`
	BaseCurrency       string `json:"baseCurrency,omitempty" example:"USD"`
}

// PhasePolicy factory. This returns a new PhasePolicy object with default values
func NewPhasePolicy() PhasePolicy {
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
		AllowAutoRenew:     AllowAutoRenew,
		RequiresValidation: RequiresValidation,
		BaseCurrency:       BaseCurrency,
	}
}
