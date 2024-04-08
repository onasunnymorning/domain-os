package commands

// CreatePriceCommand is the command for creating a price
type CreatePriceCommand struct {
	PhaseID            int64
	Currency           string `json:"currency"  binding:"required" example:"USD"`
	RegistrationAmount int64  `json:"registrationAmount"  binding:"required" example:"1000"`
	RenewalAmount      int64  `json:"renewalAmount"  binding:"required" example:"1000"`
	TransferAmount     int64  `json:"transferAmount"  binding:"required" example:"1000"`
	RestoreAmount      int64  `json:"restoreAmount"  binding:"required" example:"1000"`
}
