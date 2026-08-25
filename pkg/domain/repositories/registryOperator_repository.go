package repositories

import (
	"context"

	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
	"github.com/onasunnymorning/domain-os/pkg/domain/queries"
)

// RegistryOperatorRepository is an interface for the RegistryOperatorRepository
type RegistryOperatorRepository interface {
	Create(ctx context.Context, ro *entities.RegistryOperator) (*entities.RegistryOperator, error)
	GetByRyID(ctx context.Context, ryID string) (*entities.RegistryOperator, error)
	Update(ctx context.Context, ro *entities.RegistryOperator) (*entities.RegistryOperator, error)
	DeleteByRyID(ctx context.Context, ryID string) error
	List(ctx context.Context, params queries.ListItemsQuery) ([]*entities.RegistryOperator, string, error)
	Count(ctx context.Context, filter queries.ListRegistryOperatorsFilter) (int64, error)
}
