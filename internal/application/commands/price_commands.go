package commands

// CreatePriceCommand is the command for creating a price. Amounts are to specify in the smallest currency unit (e.g. cents in case of USD). The currency will be saved in uppercase regardless of the case in the request.
type CreatePriceCommand struct {
	PhaseName          string `json:"-"`
	TLDName            string `json:"-"`
	Currency           string `json:"currency"  binding:"required" example:"USD"`
	RegistrationAmount uint64 `json:"registrationAmount"  binding:"required" example:"1000"`
	RenewalAmount      uint64 `json:"renewalAmount"  binding:"required" example:"1000"`
	TransferAmount     uint64 `json:"transferAmount"  binding:"required" example:"1000"`
	RestoreAmount      uint64 `json:"restoreAmount"  binding:"required" example:"1000"`
}
