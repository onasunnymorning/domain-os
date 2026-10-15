package commands

import (
	"errors"
	"time"

	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
)

var (
	ErrMissingCreateContactCommandFields = errors.New("id, email, authinfo, and clid are required")
)

// CreateContactCommand is the command to create a Contact
type CreateContactCommand struct {
	ID         string                         `json:"ID" binding:"required"`
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
	// The deposit's ROID is never kept: every object gets a new ROID of this
	// registry's when it is created (ADR 0017). A placeholder lets ToEntity
	// validate the rest, without touching the caller's RDEContact.
	r := *rdeContact
	r.RoID = "1_CONT-RDE"

	// Create the contact entity from our RDEContact, this will validate the contact
	contact, err := r.ToEntity()
	if err != nil {
		return err
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

	// The contact has no ROID yet: one is generated when it is created
	// (ADR 0017). A placeholder lets the rest be validated now.
	contact.RoID = "1_CONT-NEW"
	if _, err := contact.IsValid(); err != nil {
		return nil, err
	}
	contact.RoID = ""
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

// ChunkCreateContactCommand returns a channel that yields slices of size chunkSize.
func ChunkCreateContactCommand(cmds []CreateContactCommand, chunkSize int) <-chan []CreateContactCommand {
	ch := make(chan []CreateContactCommand)

	go func() {
		defer close(ch)

		if chunkSize <= 0 {
			// Fallback to 1 if invalid chunkSize
			chunkSize = 1
		}

		for i := 0; i < len(cmds); i += chunkSize {
			end := i + chunkSize
			if end > len(cmds) {
				end = len(cmds)
			}
			// Send the chunk to the channel
			ch <- cmds[i:end]
		}
	}()

	return ch
}
