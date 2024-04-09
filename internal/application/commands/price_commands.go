package commands

// CreatePriceCommand is the command for creating a price. Amounts are to specify in the smallest currency unit (e.g. cents in case of USD). The currency will be saved in uppercase regardless of the case in the request.
type CreatePriceCommand struct {
	PhaseName          string `json:"-"`
	TLDName            string `json:"-"`
	Currency           string `json:"currency"  binding:"required" example:"USD"`
	RegistrationAmount int64  `json:"registrationAmount"  binding:"required" example:"1000"`
	RenewalAmount      int64  `json:"renewalAmount"  binding:"required" example:"1000"`
	TransferAmount     int64  `json:"transferAmount"  binding:"required" example:"1000"`
	RestoreAmount      int64  `json:"restoreAmount"  binding:"required" example:"1000"`
}
