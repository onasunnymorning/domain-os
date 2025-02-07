package services

import (
	"errors"

	"github.com/onasunnymorning/domain-os/internal/application/commands"
	"github.com/onasunnymorning/domain-os/internal/domain/entities"
	"github.com/onasunnymorning/domain-os/internal/domain/repositories"
	"golang.org/x/net/context"
)

// ContactService implements the ContactService interface
type ContactService struct {
	contactRepository repositories.ContactRepository
	roidService       RoidService
}

// NewContactService returns a new ContactService
func NewContactService(contactRepo repositories.ContactRepository, roidService RoidService) *ContactService {
	return &ContactService{
		contactRepository: contactRepo,
		roidService:       roidService,
	}
}

// CreateContact creates a new contact
func (s *ContactService) CreateContact(ctx context.Context, cmd *commands.CreateContactCommand) (*entities.Contact, error) {

	// Create a new contact from the command
	c, err := s.contactFromCreateContactCommand(cmd)
	if err != nil {
		return nil, err
	}

	// Save the contact
	newContact, err := s.contactRepository.CreateContact(ctx, c)
	if err != nil {
		return nil, err
	}

	return newContact, nil
}

// BulkCreateContacts creates multiple contacts.
// It creates contacts out of the commands and saves them in the repository
// If any of the contacts is invalid, it returns an error and does not save any of the contacts
func (s *ContactService) BulkCreate(ctx context.Context, cmds []*commands.CreateContactCommand) error {
	// Create contacts out of the commands
	contacts := make([]*entities.Contact, 0, len(cmds))
	for _, cmd := range cmds {
		c, err := s.contactFromCreateContactCommand(cmd)
		if err != nil {
			return err
		}
		contacts = append(contacts, c)
	}

	// save in the repository
	err := s.contactRepository.BulkCreate(ctx, contacts)
	if err != nil {
		return errors.Join(entities.ErrInvalidContact, err)
	}

	return nil
}

func (s *ContactService) GetContactByID(ctx context.Context, id string) (*entities.Contact, error) {
	return s.contactRepository.GetContactByID(ctx, id)
}

func (s *ContactService) UpdateContact(ctx context.Context, c *entities.Contact) (*entities.Contact, error) {
	return s.contactRepository.UpdateContact(ctx, c)
}

func (s *ContactService) DeleteContactByID(ctx context.Context, id string) error {
	return s.contactRepository.DeleteContactByID(ctx, id)
}

func (s *ContactService) ListContacts(ctx context.Context, pageSize int, cursor string) ([]*entities.Contact, error) {
	return s.contactRepository.ListContacts(ctx, pageSize, cursor)
}

// contactFromCreateContactCommand creates a new contact from a CreateContactCommand and validates if it results in a valid contact
func (s *ContactService) contactFromCreateContactCommand(cmd *commands.CreateContactCommand) (*entities.Contact, error) {
	var roid entities.RoidType
	var err error
	if cmd.RoID == "" {
		roid, err = s.roidService.GenerateRoid("contact")
		if err != nil {
			return nil, errors.Join(entities.ErrInvalidContact, err)
		}
	} else {
		roid = entities.RoidType(cmd.RoID)
		// check if it is a valid Roid
		err := roid.Validate()
		if err != nil {
			return nil, errors.Join(entities.ErrInvalidContact, err)
		}
		// Check if it is a Contact Roid
		if roid.ObjectIdentifier() != entities.CONTACT_ROID_ID {
			return nil, errors.Join(entities.ErrInvalidContact, entities.ErrInvalidRoid)
		}
	}
	c, err := entities.NewContact(cmd.ID, roid.String(), cmd.Email, cmd.AuthInfo, cmd.ClID)
	if err != nil {
		return nil, err
	}

	for _, pi := range cmd.PostalInfo {
		if pi != nil {
			err = c.AddPostalInfo(pi)
			if err != nil {
				return nil, errors.Join(entities.ErrInvalidContact, err)
			}
		}
	}

	// Add the optional elements
	if cmd.Voice != "" {
		v, err := entities.NewE164Type(cmd.Voice)
		if err != nil {
			return nil, errors.Join(entities.ErrInvalidContact, err)
		}
		c.Voice = *v
	}
	if cmd.Fax != "" {
		f, err := entities.NewE164Type(cmd.Fax)
		if err != nil {
			return nil, errors.Join(entities.ErrInvalidContact, err)
		}
		c.Fax = *f
	}
	if cmd.CrRr != "" {
		r, err := entities.NewClIDType(cmd.CrRr)
		if err != nil {
			return nil, errors.Join(entities.ErrInvalidContact, err)
		}
		c.CrRr = r
	}
	if cmd.UpRr != "" {
		r, err := entities.NewClIDType(cmd.UpRr)
		if err != nil {
			return nil, errors.Join(entities.ErrInvalidContact, err)
		}
		c.UpRr = r
	}

	// Set the disclose flags
	c.Disclose = cmd.Disclose
	// Set the status
	err = c.SetFullStatus(cmd.Status)
	if err != nil {
		return nil, errors.Join(entities.ErrInvalidContact, err)
	}

	// Check if this results in a valid contact
	_, err = c.IsValid()
	if err != nil {
		return nil, errors.Join(entities.ErrInvalidContact, err)
	}

	return c, nil
}
