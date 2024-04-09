package commands

// CreateFeeCommand is the command for creating a fee
type CreateFeeCommand struct {
	PhaseName  string `json:"-"`
	TLDName    string `json:"-"`
	Name       string `json:"name" binding:"required" example:"appliction_fee"`
	Currency   string `json:"currency" binding:"required" example:"USD"`
	Amount     int64  `json:"amount" binding:"required" example:"1000"`
	Refundable bool   `json:"refundable" binding:"required" example:"true"`
}
