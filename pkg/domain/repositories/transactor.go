package repositories

import "context"

// Transactor runs work in one database transaction. Repositories that
// support it (the domain, host, transfer, NNDN and tombstone repositories,
// and the event publisher) do their reads and writes in the transaction
// the context carries, so a change and the outbox event recording it
// commit together or not at all. A transaction begun while one is open
// joins it: only the outermost commits.
type Transactor interface {
	// Begin opens a transaction carried by the returned context, or joins
	// the one ctx carries. finish ends it: it rolls back if err is not nil
	// and returns err, or commits and returns any commit error. A joined
	// transaction's finish only returns err; the outermost decides.
	Begin(ctx context.Context) (txCtx context.Context, finish func(err error) error)
}
