package commands

// CreatePremiumListCommand represents the command to create a premium list
type CreatePremiumLabelCommand struct {
	Label              string `json:"Label" binding:"required"`
	PremiumListName    string `json:"PremiumListName" binding:"required"`
	RegistrationAmount uint64 `json:"RegistrationAmount"`
	RenewalAmount      uint64 `json:"RenewalAmount"`
	TransferAmount     uint64 `json:"TransferAmount"`
	RestoreAmount      uint64 `json:"RestoreAmount"`
	Currency           string `json:"Currency" binding:"required"`
	Class              string `json:"Class" binding:"required"`
}
