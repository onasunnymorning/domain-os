package services

import (
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
			return nil, err
		}
	} else {
		roid = entities.RoidType(cmd.RoID)
		err := roid.Validate()
		if err != nil {
			return nil, err
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
				return nil, err
			}
		}
	}

	// Add the optional elements
	if cmd.Voice != "" {
		v, err := entities.NewE164Type(cmd.Voice)
		if err != nil {
			return nil, err
		}
		c.Voice = *v
	}
	if cmd.Fax != "" {
		f, err := entities.NewE164Type(cmd.Fax)
		if err != nil {
			return nil, err
		}
		c.Fax = *f
	}

	// TODO: Set the disclose flags

	// Save the contact
	newContact, err := s.contactRepository.CreateContact(ctx, c)
	if err != nil {
		return nil, err
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
