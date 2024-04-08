package commands

// CreateFeeCommand is the command for creating a fee
type CreateFeeCommand struct {
	PhaseName  string
	TLDName    string
	Name       string `json:"name" binding:"required"`
	Currency   string `json:"currency" binding:"required"`
	Amount     int64  `json:"amount" binding:"required"`
	Refundable bool   `json:"refundable" binding:"required"`
}
