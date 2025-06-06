package interfaces

import (
	"context"

	"github.com/onasunnymorning/domain-os/internal/application/commands"
	"github.com/onasunnymorning/domain-os/internal/application/queries"
	"github.com/onasunnymorning/domain-os/internal/domain/entities"
)

type ContactService interface {
	CreateContact(ctx context.Context, c *commands.CreateContactCommand) (*entities.Contact, error)
	GetContactByID(ctx context.Context, id string) (*entities.Contact, error)
	UpdateContact(ctx context.Context, c *entities.Contact) (*entities.Contact, error)
	DeleteContactByID(ctx context.Context, id string) error
	ListContacts(ctx context.Context, params queries.ListItemsQuery) ([]*entities.Contact, string, error)
	BulkCreate(ctx context.Context, cmds []*commands.CreateContactCommand) error
}
