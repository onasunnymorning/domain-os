package commands

import "github.com/onasunnymorning/domain-os/internal/domain/entities"

// CreateContactCommand is the command to create a Contact
type CreateContactCommand struct {
	ID            string                         `json:"id"`
	RoID          string                         `json:"roid"`
	Email         string                         `json:"email"`
	AuthInfo      string                         `json:"authInfo"`
	RegistrarCLID string                         `json:"registrarCLID"`
	PostalInfo    [2]*entities.ContactPostalInfo `json:"postalInfo"`
}
