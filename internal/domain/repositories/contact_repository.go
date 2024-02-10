package repositories

import (
	"context"

	"github.com/onasunnymorning/domain-os/internal/domain/entities"
)

// ContactRepository is the interface for the ContactRepository
type ContactRepository interface {
	CreateContact(ctx context.Context, c *entities.Contact) (*entities.Contact, error)
	GetContactByID(ctx context.Context, id string) (*entities.Contact, error)
	UpdateContact(ctx context.Context, c *entities.Contact) (*entities.Contact, error)
	DeleteContactByID(ctx context.Context, id string) error
}
