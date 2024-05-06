package commands

// CreateRegistryOperatorCommand is the command for creating a registry operator
type CreateRegistryOperatorCommand struct {
	RyID  string `json:"RyID" binding:"required"`
	Name  string `json:"name" binding:"required"`
	URL   string `json:"url"`
	Email string `json:"email" binding:"required"`
	Voice string `json:"voice"`
	Fax   string `json:"fax"`
}
