package repositories

import (
	"context"

	"github.com/onasunnymorning/domain-os/internal/domain/entities"
)

// HostRepository is the interface for the HostRepository
type HostRepository interface {
	CreateHost(ctx context.Context, h *entities.Host) (*entities.Host, error)
	GetHostByRoid(ctx context.Context, roid int64) (*entities.Host, error)
	UpdateHost(ctx context.Context, h *entities.Host) (*entities.Host, error)
	DeleteHostByRoid(ctx context.Context, roid int64) error
	ListHosts(ctx context.Context, pageSize int, cursor string) ([]*entities.Host, error)
}
