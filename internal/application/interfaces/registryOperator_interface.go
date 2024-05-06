package interfaces

import (
	"context"

	"github.com/onasunnymorning/domain-os/internal/application/commands"
	"github.com/onasunnymorning/domain-os/internal/domain/entities"
)

// RegistryOperatorService is the interface for the RegistryOperatorService
type RegistryOperatorService interface {
	Create(ctx context.Context, cmd *commands.CreateRegistryOperatorCommand) (*entities.RegistryOperator, error)
	GetByRyID(ctx context.Context, ryid string) (*entities.RegistryOperator, error)
	Update(ctx context.Context, ry *entities.RegistryOperator) (*entities.RegistryOperator, error)
	DeleteByRyID(ctx context.Context, ryid string) error
}
