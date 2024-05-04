package repositories

import (
	"context"

	"github.com/onasunnymorning/domain-os/internal/domain/entities"
)

// RegistrarRepository is the interface for the registrar repository
type RegistrarRepository interface {
	GetByClID(ctx context.Context, clid string, preloadTLDs bool) (*entities.Registrar, error)
	GetByGurID(ctx context.Context, gurID int) (*entities.Registrar, error)
	Create(ctx context.Context, rar *entities.Registrar) (*entities.Registrar, error)
	Update(ctx context.Context, rar *entities.Registrar) (*entities.Registrar, error)
	Delete(ctx context.Context, clid string) error
	List(ctx context.Context, pagesize int, pagecursor string) ([]*entities.Registrar, error)
}
