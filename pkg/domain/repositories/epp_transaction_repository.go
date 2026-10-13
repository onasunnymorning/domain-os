package repositories

import (
	"context"
	"time"

	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
)

// EPPTransactionRepository is the EPP audit log (docs/epp/ARCHITECTURE.md
// §6): every command and response, kept for the configured retention.
type EPPTransactionRepository interface {
	// Save appends transactions. A transaction already saved, by svTRID, is
	// left as it is, so a retried batch does not fail on its own earlier
	// half.
	Save(ctx context.Context, txs ...*entities.EPPTransaction) error
	// DeleteReceivedBefore removes up to limit transactions received before
	// cutoff, oldest first, and returns how many it removed.
	DeleteReceivedBefore(ctx context.Context, cutoff time.Time, limit int) (int64, error)
}
