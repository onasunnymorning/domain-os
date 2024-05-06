package commands

// CreateRegistryOperatorCommand is the command for creating a registry operator
type CreateRegistryOperatorCommand struct {
	RyID  string `json:"RyID" binding:"required"`
	Name  string `json:"Name" binding:"required"`
	URL   string `json:"URL"`
	Email string `json:"Email" binding:"required"`
	Voice string `json:"Voice"`
	Fax   string `json:"Fax"`
}
