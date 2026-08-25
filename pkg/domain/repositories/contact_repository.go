package repositories

import (
	"context"

	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
	"github.com/onasunnymorning/domain-os/pkg/domain/queries"
)

// ContactRepository is the interface for the ContactRepository
type ContactRepository interface {
	CreateContact(ctx context.Context, c *entities.Contact) (*entities.Contact, error)
	GetContactByID(ctx context.Context, id string) (*entities.Contact, error)
	UpdateContact(ctx context.Context, c *entities.Contact) (*entities.Contact, error)
	DeleteContactByID(ctx context.Context, id string) error
	ListContacts(ctx context.Context, params queries.ListItemsQuery) ([]*entities.Contact, string, error)
	Count(ctx context.Context, filter queries.ListContactsFilter) (int64, error)
	BulkCreate(ctx context.Context, contacts []*entities.Contact) error
}
