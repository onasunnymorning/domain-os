package repositories

import (
	"context"
	"time"

	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
)

// DomainTransferRepository stores domain transfers (entities.DomainTransfer)
// and makes the writes that start and end them. Each write is one
// transaction with the domain, its host links and the poll messages it
// queues, so none of them can disagree with another.
type DomainTransferRepository interface {
	// RequestTransfer writes d, made pendingTransfer by
	// entities.Domain.RequestTransfer, if the stored domain still has the
	// version and sponsor d was read with (else entities.ErrDomainChanged);
	// records t, setting its ID; and queues msgs. It returns d as stored.
	RequestTransfer(ctx context.Context, d *entities.Domain, t *entities.DomainTransfer, msgs []*entities.PollMessage) (*entities.Domain, error)
	// ResolveTransfer writes the end of a pending transfer, as r describes
	// (entities.TransferResolution).
	// It returns the domain as stored.
	ResolveTransfer(ctx context.Context, r *entities.TransferResolution) (*entities.Domain, error)
	// LatestTransfer returns the latest transfer of the domain with ROID
	// roid, pending or not, or entities.ErrTransferNotPending if it has
	// none.
	LatestTransfer(ctx context.Context, roid entities.RoidType) (*entities.DomainTransfer, error)
	// ListDueTransfers returns up to limit pending transfers of domains that
	// still exist, whose ActionDate is at or before dueBy, in (ActionDate,
	// ID) order after the cursor (afterDate, afterID; zero for the start):
	// those the registry approves because the losing registrar did not
	// answer in time. A sweep pages with the cursor, so a transfer that
	// cannot be approved is passed over, not listed again, until the next
	// sweep.
	ListDueTransfers(ctx context.Context, dueBy, afterDate time.Time, afterID int64, limit int) ([]*entities.DomainTransfer, error)
}

// PollMessageRepository is the registrars' message queues (RFC 5730
// §2.9.2.3). Messages are queued by the writes that cause them, such as
// DomainTransferRepository's.
type PollMessageRepository interface {
	// OldestPollMessage returns the registrar's oldest message and how many
	// messages it has; nil and 0 when it has none.
	OldestPollMessage(ctx context.Context, clid entities.ClIDType) (*entities.PollMessage, int64, error)
	// AckPollMessage removes message id from the registrar's queue and
	// returns how many messages remain. A message not in the queue,
	// including another registrar's, is entities.ErrPollMessageNotFound.
	AckPollMessage(ctx context.Context, clid entities.ClIDType, id int64) (int64, error)
}
