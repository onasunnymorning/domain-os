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
	// Info returns a domain the registrar sponsors; or, for another
	// registrar that gives the domain's transfer code (authInfo, nil if none
	// was given), its limited view (RFC 9154 §5.3).
	Info(ctx context.Context, scope entities.RegistrarClID, name string, authInfo *string) (*queries.EPPDomainInfo, error)
	// Create registers a domain for the registrar.
	Create(ctx context.Context, scope entities.RegistrarClID, cmd commands.EPPCreateDomainCommand) (*entities.Domain, error)
	// Update changes a domain the registrar sponsors.
	Update(ctx context.Context, scope entities.RegistrarClID, cmd commands.EPPUpdateDomainCommand) (*entities.Domain, error)
	// Renew renews a domain the registrar sponsors.
	Renew(ctx context.Context, scope entities.RegistrarClID, cmd commands.EPPRenewDomainCommand) (*entities.Domain, error)
	// Delete starts the end of life of a domain the registrar sponsors,
	// and returns it as it is then.
	Delete(ctx context.Context, scope entities.RegistrarClID, name string) (*entities.Domain, error)
	// TransferRequest asks for a domain to be transferred to the
	// registrar, and returns the transfer, pending.
	TransferRequest(ctx context.Context, scope entities.RegistrarClID, cmd commands.EPPTransferDomainCommand) (*entities.DomainTransfer, error)
	// TransferApprove and TransferReject answer the pending transfer of a
	// domain the registrar sponsors.
	TransferApprove(ctx context.Context, scope entities.RegistrarClID, name string) (*entities.DomainTransfer, error)
	TransferReject(ctx context.Context, scope entities.RegistrarClID, name string) (*entities.DomainTransfer, error)
	// TransferCancel cancels a pending transfer the registrar requested.
	TransferCancel(ctx context.Context, scope entities.RegistrarClID, name string) (*entities.DomainTransfer, error)
	// TransferQuery returns a domain's latest transfer, to a registrar
	// party to it or that gives the domain's transfer code (authInfo, nil
	// if none was given).
	TransferQuery(ctx context.Context, scope entities.RegistrarClID, name string, authInfo *string) (*entities.DomainTransfer, error)
}

// EPPPollService is what the EPP server asks of the registry for <poll>
// (RFC 5730 §2.9.2.3): the registrar's own message queue.
type EPPPollService interface {
	// Request returns the registrar's oldest message and how many it has;
	// nil and 0 when it has none.
	Request(ctx context.Context, scope entities.RegistrarClID) (*entities.PollMessage, int64, error)
	// Ack removes one of the registrar's messages and returns how many
	// remain.
	Ack(ctx context.Context, scope entities.RegistrarClID, msgID string) (int64, error)
}

// EPPHostService is what the EPP server asks of the registry for host
// commands (RFC 5732). Every method is scoped to the session's registrar:
// host objects belong to a registrar (ADR 0006).
type EPPHostService interface {
	// Check reports, for each name in order, whether the registrar can
	// create a host by that name. An error means the registry could not
	// answer.
	Check(ctx context.Context, scope entities.RegistrarClID, names []string) ([]*queries.HostCheckResult, error)
	// Info returns the registrar's host object by that name.
	Info(ctx context.Context, scope entities.RegistrarClID, name string) (*entities.Host, error)
	// Create creates a host object for the registrar.
	Create(ctx context.Context, scope entities.RegistrarClID, cmd commands.EPPCreateHostCommand) (*entities.Host, error)
	// Update changes the registrar's host object.
	Update(ctx context.Context, scope entities.RegistrarClID, cmd commands.EPPUpdateHostCommand) (*entities.Host, error)
	// Delete deletes the registrar's host object.
	Delete(ctx context.Context, scope entities.RegistrarClID, name string) error
}

// EPPContactService is what the EPP server asks of the registry for
// contact commands (RFC 5733). A contact ID is unique in the registry, so
// check is the same for every registrar; the other commands are scoped to
// the session's registrar (ADR 0006).
type EPPContactService interface {
	// Check reports, for each ID in order, whether a contact can be created
	// with it. An error means the registry could not answer.
	Check(ctx context.Context, ids []string) ([]*queries.ContactCheckResult, error)
	// Info returns the registrar's contact.
	Info(ctx context.Context, scope entities.RegistrarClID, id string) (*entities.Contact, error)
	// Create creates a contact for the registrar.
	Create(ctx context.Context, scope entities.RegistrarClID, cmd commands.EPPCreateContactCommand) (*entities.Contact, error)
	// Update changes the registrar's contact.
	Update(ctx context.Context, scope entities.RegistrarClID, cmd commands.EPPUpdateContactCommand) (*entities.Contact, error)
	// Delete deletes the registrar's contact.
	Delete(ctx context.Context, scope entities.RegistrarClID, id string) error
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
