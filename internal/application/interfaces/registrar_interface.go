package interfaces

import (
	"context"

	"github.com/onasunnymorning/domain-os/internal/application/commands"
	"github.com/onasunnymorning/domain-os/internal/domain/entities"
)

// RegistrarService is the interface for the registrar service
type RegistrarService interface {
	GetByClID(ctx context.Context, clid string) (*entities.Registrar, error)
	Create(ctx context.Context, rar *commands.CreateRegistrarCommand) (*commands.CreateRegistrarCommandResult, error)
	Update(ctx context.Context, rar *entities.Registrar) (*entities.Registrar, error)
	Delete(ctx context.Context, clid string) error
	List(ctx context.Context, pagesize int, pagecursor string) ([]*entities.Registrar, error)
}
