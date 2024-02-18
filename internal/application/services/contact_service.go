package services

import (
	"context"
	"github.com/onasunnymorning/domain-os/internal/application/commands"
	"github.com/onasunnymorning/domain-os/internal/domain/entities"
	"github.com/onasunnymorning/domain-os/internal/domain/repositories"
)

// ContactService implements the ContactService interface
type ContactService struct {
	ContactRepository repositories.ContactRepository
}

// NewContactService returns a new ContactService
func NewContactService(ContactRepo repositories.ContactRepository) *ContactService {
	return &ContactService{
		ContactRepository: ContactRepo,
	}
}

func (s *ContactService) CreateContact(ctx context.Context, c *commands.CreateContactCommand) (*entities.Contact, error) {
	contact, err := entities.NewContact(c.ID, c.RoID, c.Email, c.AuthInfo)
	if err != nil {
		return nil, err
	}
	contact, err = s.ContactRepository.CreateContact(ctx, contact)
	if err != nil {
		return nil, err
	}
	return contact, nil
}

func (s *ContactService) GetContactByID(ctx context.Context, id string) (*entities.Contact, error) {
	return s.ContactRepository.GetContactByID(ctx, id)
}

func (s *ContactService) UpdateContact(ctx context.Context, c *entities.Contact) (*entities.Contact, error) {
	return s.ContactRepository.UpdateContact(ctx, c)
}

func (s *ContactService) DeleteContactByID(ctx context.Context, id string) error {
	return s.ContactRepository.DeleteContactByID(ctx, id)
}
