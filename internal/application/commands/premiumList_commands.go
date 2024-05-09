package commands

// CreatePremiumListCommand represents the command to create a premium list
type CreatePremiumListCommand struct {
	Name string `json:"Name" binding:"required"`
	RyID string `json:"RyID" binding:"required"`
}
