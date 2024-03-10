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
	var roid entities.RoidType
	var err error
	if cmd.RoID == "" {
		roid, err = s.roidService.GenerateRoid("contact")
		if err != nil {
			return nil, errors.Join(entities.ErrInvalidContact, err)
		}
	} else {
		roid = entities.RoidType(cmd.RoID)
		err := roid.Validate()
		if err != nil {
			return nil, errors.Join(entities.ErrInvalidContact, err)
		}
	}
	c, err := entities.NewContact(cmd.ID, roid.String(), cmd.Email, cmd.AuthInfo, cmd.ClID)
	if err != nil {
		return nil, errors.Join(entities.ErrInvalidContact, err)
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
	c.ContactDisclose = cmd.Disclose
	// If it's present in the command, check if the status is valid
	if !cmd.Status.IsNil() && !cmd.Status.IsValidContactStatus() {
		return nil, errors.Join(entities.ErrInvalidContact, entities.ErrInvalidContactStatusCombination)
	}
	// Set the status
	c.ContactStatus = cmd.Status
	c.SetOKStatusIfNeeded()
	c.UnSetOKStatusIfNeeded()

	// Check if this results in a valid state
	_, err = c.IsValid()
	if err != nil {
		return nil, errors.Join(entities.ErrInvalidContact, err)
	}

	// Save the contact
	newContact, err := s.contactRepository.CreateContact(ctx, c)
	if err != nil {
		return nil, errors.Join(entities.ErrInvalidContact, err)
	}

	// Map to the response if successful
	// resp := mappers.ContactCreateResultFromContact(newContact)

	return newContact, nil
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
