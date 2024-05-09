package repositories

import (
	"context"

	"github.com/onasunnymorning/domain-os/internal/domain/entities"
)

// RegistryOperatorRepository is an interface for the RegistryOperatorRepository
type RegistryOperatorRepository interface {
	Create(ctx context.Context, ro *entities.RegistryOperator) (*entities.RegistryOperator, error)
	GetByRyID(ctx context.Context, ryID string) (*entities.RegistryOperator, error)
	Update(ctx context.Context, ro *entities.RegistryOperator) (*entities.RegistryOperator, error)
	DeleteByRyID(ctx context.Context, ryID string) error
	List(ctx context.Context, pagesize int, pagecursor string) ([]*entities.RegistryOperator, error)
}
