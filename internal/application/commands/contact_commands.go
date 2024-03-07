package commands

// CreateContactCommand is the command to create a Contact
type CreateContactCommand struct {
	ID            string `json:"ID"`
	RoID          string `json:"RoID"`
	Email         string `json:"Email"`
	AuthInfo      string `json:"AuthInfo"`
	RegistrarClID string `json:"RegistrarClID"`
}
