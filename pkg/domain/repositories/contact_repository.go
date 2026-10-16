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
	// UpdateContactIfUnchanged saves the contact if the stored contact is
	// still as it was read (c.UpdatedAt): otherwise it is
	// entities.ErrContactChanged, and if the contact is gone,
	// entities.ErrContactNotFound.
	UpdateContactIfUnchanged(ctx context.Context, c *entities.Contact) (*entities.Contact, error)
	// CountContactLinks counts the domains that use the contact, as
	// registrant or as admin, tech or billing contact.
	CountContactLinks(ctx context.Context, id string) (int64, error)
}
