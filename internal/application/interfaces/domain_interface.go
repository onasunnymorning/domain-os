package interfaces

import (
	"context"

	"github.com/onasunnymorning/domain-os/internal/application/commands"
	"github.com/onasunnymorning/domain-os/internal/domain/entities"
)

type DomainService interface {
	GetDomainByName(ctx context.Context, name string) (*entities.Domain, error)
	CreateDomain(ctx context.Context, cmd *commands.CreateDomainCommand) (*entities.Domain, error)
	DeleteDomainByName(ctx context.Context, name string) error
	ListDomains(ctx context.Context, pageSize int, cursor string) ([]*entities.Domain, error)
}
