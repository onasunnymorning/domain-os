package commands

// CreateContactCommand is the command to create a Contact
type CreateContactCommand struct {
	ID            string `json:"id"`
	RoID          string `json:"roid"`
	Email         string `json:"email"`
	AuthInfo      string `json:"authInfo"`
	RegistrarCLID string `json:"registrarCLID"`
}
