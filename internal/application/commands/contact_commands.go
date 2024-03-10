package commands

import "github.com/onasunnymorning/domain-os/internal/domain/entities"

// CreateContactCommand is the command to create a Contact
type CreateContactCommand struct {
	ID         string                         `json:"ID" binding:"required"`
	RoID       string                         `json:"RoID"` // If not provided, one will be generated. Allowing it to be specified allows import of escrow contacts without changing the ID
	Email      string                         `json:"Email" binding:"required"`
	AuthInfo   string                         `json:"AuthInfo" binding:"required"`
	ClID       string                         `json:"ClID" binding:"required"`
	CrRr       string                         `json:"CrRr"`
	UpRr       string                         `json:"UpRr"`
	PostalInfo [2]*entities.ContactPostalInfo `json:"PostalInfo" binding:"required"`
	Voice      string                         `json:"Voice"`
	Fax        string                         `json:"Fax"`
	Status     entities.ContactStatus         `json:"Status"`
	Disclose   entities.ContactDisclose       `json:"Disclose"`
}
