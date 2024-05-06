package commands

// CreatePremiumListCommand represents the command to create a premium list
type CreatePremiumListCommand struct {
	Name string `json:"name" binding:"required"`
}
