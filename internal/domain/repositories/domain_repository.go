package repositories

import (
	"context"

	"github.com/onasunnymorning/domain-os/internal/domain/entities"
)

// DomainRepository is the interface for the DomainRepository
type DomainRepository interface {
	CreateDomain(ctx context.Context, d *entities.Domain) (*entities.Domain, error)
	GetDomainByName(ctx context.Context, name string) (*entities.Domain, error)
	UpdateDomain(ctx context.Context, d *entities.Domain) (*entities.Domain, error)
	DeleteDomainByName(ctx context.Context, name string) error
	ListDomains(ctx context.Context, pageSize int, cursor string) ([]*entities.Domain, error)
	AddHostToDomain(ctx context.Context, domRoid int64, hostRoid int64) error
	RemoveHostFromDomain(ctx context.Context, domRoid int64, hostRoid int64) error
}
