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
func FromRdeContact(rdeContact *entities.RDEContact) (*CreateContactCommand, error) {
	postalInfos := [2]*entities.ContactPostalInfo{}
	for i, postal := range rdeContact.PostalInfo {
		postalInfo, err := postal.ToEntity()
		if err != nil {
			return nil, err
		}
		postalInfos[i] = postalInfo
	}
	// TODO: implement Disclose

	// Since the Escrow specification (RFC 9022) does not specify the authInfo field, we will generate a random one to import the data
	aInfo, err := entities.NewAuthInfoType("escr0W1mP*rt")
	if err != nil {
		return nil, err // Untestable, just catching the error incase our AuthInfoType is validation changes
	}
	// Create a new create contact command object
	cmd, err := NewCreateContactCommand(rdeContact.ID, rdeContact.Email, aInfo.String(), rdeContact.ClID)
	if err != nil {
		return nil, err
	}
	// Add the postal info and disclose to the contact
	cmd.PostalInfo = postalInfos
	// Set the optional fields
	if rdeContact.Voice != "" {
		cmd.Voice = rdeContact.Voice
	}
	if rdeContact.Fax != "" {
		cmd.Fax = rdeContact.Fax
	}
	if rdeContact.CrDate != "" {
		crd, err := time.Parse(time.RFC3339, rdeContact.CrDate)
		if err != nil {
			return nil, errors.Join(entities.ErrInvalidTimeFormat, err)
		}
		cmd.CreatedAt = crd
	}
	if rdeContact.UpDate != "" {
		upd, err := time.Parse(time.RFC3339, rdeContact.UpDate)
		if err != nil {
			return nil, errors.Join(entities.ErrInvalidTimeFormat, err)
		}
		cmd.UpdatedAt = upd
	}
	if rdeContact.CrRr != "" {
		cmd.CrRr = rdeContact.CrRr
	}
	if rdeContact.UpRr != "" {
		cmd.UpRr = rdeContact.UpRr
	}
	// Set the status
	cs, err := entities.GetContactStatusFromRDEContactStatus(rdeContact.Status)
	if err != nil {
		return nil, err
	}
	// Validate the status
	if !cs.IsValidContactStatus() {
		return nil, entities.ErrInvalidContactStatusCombination
	}
	cmd.Status = cs

	// Make sure it is a valid command

	return cmd, nil

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
		if errors.Is(entities.ErrInvalidRoid, err) || cmd.RoID == "" {
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
