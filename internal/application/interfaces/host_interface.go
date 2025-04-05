package interfaces

import (
	"context"

	"github.com/onasunnymorning/domain-os/internal/application/commands"
	"github.com/onasunnymorning/domain-os/internal/application/queries"
	"github.com/onasunnymorning/domain-os/internal/domain/entities"
)

// HostService is the interface for the HostService
type HostService interface {
	// CreateHost creates a new host including its optional addresses
	CreateHost(ctx context.Context, h *commands.CreateHostCommand) (*entities.Host, error)
	GetHostByRoID(ctx context.Context, roidString string) (*entities.Host, error)
	// GetHostByNameAndClID gets a host by its name and clid
	GetHostByNameAndClID(ctx context.Context, name, clid string) (*entities.Host, error)
	DeleteHostByRoID(ctx context.Context, roidString string) error
	ListHosts(ctx context.Context, params queries.ListItemsQuery) ([]*entities.Host, string, error)
	AddHostAddress(ctx context.Context, roidString, ip string) (*entities.Host, error)
	RemoveHostAddress(ctx context.Context, roidString, ip string) (*entities.Host, error)
	// BulkCreate creates multiple hosts in a single transaction. If addresses are provided, they will be created as well
	// Should one of the hosts fail to be created, the operation fails and no hosts are created, the error will be returned
	BulkCreate(ctx context.Context, cmds []*commands.CreateHostCommand) error
}
