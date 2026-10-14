package interfaces

import (
	"context"

	"github.com/onasunnymorning/domain-os/internal/application/commands"
	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
	"github.com/onasunnymorning/domain-os/pkg/domain/queries"
)

// EPPDomainService is what the EPP server asks of the registry for domain
// commands (docs/epp/ARCHITECTURE.md §4).
type EPPDomainService interface {
	// Check reports, for each name in order, whether it is available. An
	// unavailable name is a result with a reason; an error means the
	// registry could not answer.
	Check(ctx context.Context, names []string) ([]*queries.DomainCheckResult, error)
	// Info returns a domain the registrar sponsors.
	Info(ctx context.Context, scope entities.RegistrarClID, name string) (*queries.EPPDomainInfo, error)
	// Create registers a domain for the registrar.
	Create(ctx context.Context, scope entities.RegistrarClID, cmd commands.EPPCreateDomainCommand) (*entities.Domain, error)
}

// EPPAuditLog records EPP commands and responses
// (docs/epp/ARCHITECTURE.md §6).
type EPPAuditLog interface {
	// Record writes tx before returning: transforms are logged before the
	// registrar sees the response.
	Record(ctx context.Context, tx *entities.EPPTransaction) error
	// Enqueue queues tx to be written in a batch, or writes it at once if
	// the queue is full.
	Enqueue(ctx context.Context, tx *entities.EPPTransaction) error
}
