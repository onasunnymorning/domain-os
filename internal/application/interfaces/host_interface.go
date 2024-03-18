package interfaces

import (
	"context"

	"github.com/onasunnymorning/domain-os/internal/application/commands"
	"github.com/onasunnymorning/domain-os/internal/domain/entities"
)

// HostService is the interface for the HostService
type HostService interface {
	CreateHost(ctx context.Context, h *commands.CreateHostCommand) (*entities.Host, error)
	GetHostByRoID(ctx context.Context, roidString string) (*entities.Host, error)
	DeleteHostByRoID(ctx context.Context, roidString string) error
	ListHosts(ctx context.Context, pageSize int, cursor string) ([]*entities.Host, error)
	AddHostAddress(ctx context.Context, roidString, ip string) (*entities.Host, error)
	RemoveHostAddress(ctx context.Context, roidString, ip string) (*entities.Host, error)
}
