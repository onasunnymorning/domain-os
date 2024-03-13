package commands

import (
	"time"

	"github.com/onasunnymorning/domain-os/internal/domain/entities"
)

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

// UpdateContactCommand is the command to update a Contact. It is basically an entities. Command wihout the ID and RoID which are assigned at create time.
type UpdateContactCommand struct {
	PostalInfo               [2]*entities.ContactPostalInfo `json:"PostalInfo"` // 1 required, 2 maximum. I prefer the approach to have an array of two optional items over a map because it makes manipulating (updating) easier
	Voice                    entities.E164Type              `json:"Voice" example:"+1.9567345623"`
	Fax                      entities.E164Type              `json:"Fax" example:"+1.9567345623"`
	Email                    string                         `json:"Email" example:"solutions@apex.domains"` // Required
	ClID                     entities.ClIDType              `json:"ClID" example:"sh8013"`                  // Required
	CrRr                     entities.ClIDType              `json:"CrRr" example:"sh8013"`
	CreatedAt                time.Time                      `json:"CrDate" example:"2023-04-03T22:00:00.0Z"`
	UpRr                     entities.ClIDType              `json:"UpRr" example:"sh8013"`
	UpdatedAt                time.Time                      `json:"UpDate" example:"2023-04-03T22:00:00.0Z"`
	AuthInfo                 entities.AuthInfoType          `json:"AuthInfo" example:"sTr0N5p@zzWqRD"` // Required
	entities.ContactStatus   `json:"Status"`
	entities.ContactDisclose `json:"Disclose"`
}
