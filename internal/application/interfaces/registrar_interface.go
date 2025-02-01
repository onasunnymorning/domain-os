package interfaces

import (
	"context"

	"github.com/onasunnymorning/domain-os/internal/application/commands"
	"github.com/onasunnymorning/domain-os/internal/domain/entities"
)

// RegistrarService is the interface for the registrar service
type RegistrarService interface {
	GetByClID(ctx context.Context, clid string, preloadTLDs bool) (*entities.Registrar, error)
	GetByGurID(ctx context.Context, gurID int) (*entities.Registrar, error)
	Create(ctx context.Context, rar *commands.CreateRegistrarCommand) (*entities.Registrar, error)
	BulkCreate(ctx context.Context, rars []*commands.CreateRegistrarCommand) error
	Update(ctx context.Context, rar *entities.Registrar) (*entities.Registrar, error)
	Delete(ctx context.Context, clid string) error
	List(ctx context.Context, pagesize int, pagecursor string) ([]*entities.RegistrarListItem, error)
	Count(ctx context.Context) (int64, error)
	SetStatus(ctx context.Context, clid string, status entities.RegistrarStatus) error
}
