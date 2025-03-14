package repositories

import (
	"context"

	"github.com/onasunnymorning/domain-os/internal/application/queries"
	"github.com/onasunnymorning/domain-os/internal/domain/entities"
)

type TLDRepository interface {
	Create(ctx context.Context, tld *entities.TLD) error
	GetByName(ctx context.Context, name string, preloadAll bool) (*entities.TLD, error)
	List(ctx context.Context, params queries.ListTldsQuery) ([]*entities.TLD, error)
	Update(ctx context.Context, tld *entities.TLD) error
	DeleteByName(ctx context.Context, name string) error
	Count(ctx context.Context) (int64, error)
}
