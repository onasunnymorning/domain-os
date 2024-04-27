package commands

import (
	"errors"
	"time"

	"github.com/onasunnymorning/domain-os/internal/domain/entities"
)

var (
	ErrMissingCreateContactCommandFields = errors.New("id, email, authinfo, and clid are required")
)

// CreateContactCommand is the command to create a Contact
type CreateContactCommand struct {
	ID         string                         `json:"ID" binding:"required"`
	RoID       string                         `json:"RoID"` // If not provided, one will be generated. Allowing it to be specified allows import of escrow contacts without changing the ID
	Email      string                         `json:"Email" binding:"required"`
	AuthInfo   string                         `json:"AuthInfo" binding:"required"`
	ClID       string                         `json:"ClID" binding:"required"`
	CrRr       string                         `json:"CrRr"`
	CreatedAt  time.Time                      `json:"CrDate"`
	UpdatedAt  time.Time                      `json:"UpDate"`
	UpRr       string                         `json:"UpRr"`
	PostalInfo [2]*entities.ContactPostalInfo `json:"PostalInfo" binding:"required"`
	Voice      string                         `json:"Voice"`
	Fax        string                         `json:"Fax"`
	Status     entities.ContactStatus         `json:"Status"`
	Disclose   entities.ContactDisclose       `json:"Disclose"`
}

// NewCreateContactCommand creates a new CreateContactCommand
func NewCreateContactCommand(id, email, authinfo, clid string) (*CreateContactCommand, error) {
	if id == "" || email == "" || authinfo == "" || clid == "" {
		return nil, ErrMissingCreateContactCommandFields
	}
	return &CreateContactCommand{
		ID:       id,
		Email:    email,
		AuthInfo: authinfo,
		ClID:     clid,
	}, nil
}

// FromRdeContact creates a new CreateContactCommand from an RDEContact
func (cmd *CreateContactCommand) FromRdeContact(rdeContact *entities.RDEContact) error {
	// Check if we have a valid RoID (this will only be the case if we are importing our own escrows).
	// If the Roid is invalid, use a valid one to pass through contact validation and unset it in the final command to have one generated.
	roid := entities.RoidType(rdeContact.RoID)
	if roid.Validate() != nil || roid.ObjectIdentifier() != "CONT" {
		// set a dummy valid RoID to pass through domain validation
		rdeContact.RoID = "1_CONT-APEX"
	}

	// Create the contact entity from our RDEContact, this will validate the contact
	contact, err := rdeContact.ToEntity()
	if err != nil {
		return err
	}

	// Now that we have a a valid contact, convert it to a command
	// Only set the RoID if it is not the dummy RoID
	if contact.RoID.String() != "1_CONT-APEX" {
		cmd.RoID = contact.RoID.String()
	}
	cmd.ID = contact.ID.String()
	cmd.Email = contact.Email
	cmd.AuthInfo = contact.AuthInfo.String()
	cmd.ClID = contact.ClID.String()
	cmd.CrRr = contact.CrRr.String()
	cmd.CreatedAt = contact.CreatedAt
	cmd.UpdatedAt = contact.UpdatedAt
	cmd.UpRr = contact.UpRr.String()
	cmd.PostalInfo = contact.PostalInfo
	cmd.Voice = contact.Voice.String()
	cmd.Fax = contact.Fax.String()
	cmd.Status = contact.Status
	cmd.Disclose = contact.Disclose

	return err

}

// ToContact creates a new Contact from a CreateContactCommand
func (cmd *CreateContactCommand) ToContact() (*entities.Contact, error) {
	contact := &entities.Contact{
		ID:         entities.ClIDType(cmd.ID),
		RoID:       entities.RoidType(cmd.RoID),
		Email:      cmd.Email,
		AuthInfo:   entities.AuthInfoType(cmd.AuthInfo),
		ClID:       entities.ClIDType(cmd.ClID),
		CrRr:       entities.ClIDType(cmd.CrRr),
		CreatedAt:  cmd.CreatedAt,
		UpRr:       entities.ClIDType(cmd.UpRr),
		UpdatedAt:  cmd.UpdatedAt,
		Voice:      entities.E164Type(cmd.Voice),
		Fax:        entities.E164Type(cmd.Fax),
		Status:     cmd.Status,
		Disclose:   cmd.Disclose,
		PostalInfo: cmd.PostalInfo,
	}

	if _, err := contact.IsValid(); err != nil {
		// If the command doesn't have the optional RoID set, exclude it from the validation as a RoID will be generated on object creation
		if errors.Is(err, entities.ErrInvalidRoid) || cmd.RoID == "" {
			return contact, nil
		}
		return nil, err
	}

	return contact, nil

}

// UpdateContactCommand is the command to update a Contact. It is basically an entities. Command wihout the ID and RoID which are assigned at create time.
type UpdateContactCommand struct {
	PostalInfo [2]*entities.ContactPostalInfo `json:"PostalInfo"` // 1 required, 2 maximum. I prefer the approach to have an array of two optional items over a map because it makes manipulating (updating) easier
	Voice      entities.E164Type              `json:"Voice" example:"+1.9567345623"`
	Fax        entities.E164Type              `json:"Fax" example:"+1.9567345623"`
	Email      string                         `json:"Email" example:"solutions@apex.domains"`
	ClID       entities.ClIDType              `json:"ClID" example:"sh8013"`
	CrRr       entities.ClIDType              `json:"CrRr" example:"sh8013"`
	CreatedAt  time.Time                      `json:"CrDate" example:"2023-04-03T22:00:00.0Z"`
	UpRr       entities.ClIDType              `json:"UpRr" example:"sh8013"`
	UpdatedAt  time.Time                      `json:"UpDate" example:"2023-04-03T22:00:00.0Z"`
	AuthInfo   entities.AuthInfoType          `json:"AuthInfo" example:"sTr0N5p@zzWqRD"`
	Status     entities.ContactStatus         `json:"Status"`
	Disclose   entities.ContactDisclose       `json:"Disclose"`
}
