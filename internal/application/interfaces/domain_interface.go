package interfaces

import (
	"context"

	"github.com/onasunnymorning/domain-os/internal/application/commands"
	"github.com/onasunnymorning/domain-os/internal/application/queries"
	"github.com/onasunnymorning/domain-os/internal/domain/entities"
)

type DomainService interface {
	// These are ADMIN services
	GetDomainByName(ctx context.Context, name string, preloadHosts bool) (*entities.Domain, error)
	CreateDomain(ctx context.Context, cmd *commands.CreateDomainCommand) (*entities.Domain, error)
	DeleteDomainByName(ctx context.Context, name string) error
	ListDomains(ctx context.Context, pageSize int, cursor string) ([]*entities.Domain, error)
	UpdateDomain(ctx context.Context, name string, cmd *commands.UpdateDomainCommand) (*entities.Domain, error)
	AddHostToDomain(ctx context.Context, name string, hostRoID string) error
	RemoveHostFromDomain(ctx context.Context, name string, hostRoID string) error

	// These are Registrar services
	// CheckDomain checks if a domain is available and supports the fee extension
	CheckDomain(ctx context.Context, q *queries.DomainCheckQuery) (*queries.DomainCheckResult, error)
	// RegisterDomain registers a domain as a registrar and supports the fee extension
	RegisterDomain(ctx context.Context, cmd *commands.RegisterDomainCommand) (*entities.Domain, error)
	// RenewDomain renews a domain as a registrar and supports the fee extension
	RenewDomain(ctx context.Context, cmd *commands.RenewDomainCommand) (*entities.Domain, error)
	// MarkDomainForDelete marks a domain for deletion as a registrar
	MarkDomainForDeletion(ctx context.Context, domainName string) (*entities.Domain, error)
}
