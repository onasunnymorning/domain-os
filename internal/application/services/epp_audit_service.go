package services

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
	"github.com/onasunnymorning/domain-os/pkg/domain/repositories"
)

// Audit log batching. Queries are queued and written in batches of up to
// auditBatchSize, at least every auditFlushInterval.
const (
	auditBatchSize     = 200
	auditFlushInterval = time.Second
	// auditPruneBatch is how many expired rows one delete removes, so
	// pruning a large backlog never holds a long lock.
	auditPruneBatch = 5000
)

// ErrEPPAuditUnavailable wraps a failure to write the EPP audit log.
var ErrEPPAuditUnavailable = errors.New("EPP audit log unavailable")

// EPPAuditService writes the EPP audit log (docs/epp/ARCHITECTURE.md §6).
// Transforms are written before their response is sent (Record); queries go
// through a bounded queue and are written in batches (Enqueue, drained by
// Run). A full queue never drops a record: Enqueue then writes it itself,
// which slows that session rather than losing its trail.
type EPPAuditService struct {
	repo  repositories.EPPTransactionRepository
	queue chan *entities.EPPTransaction
	now   func() time.Time
}

// NewEPPAuditService returns an audit log over repo whose queue holds up to
// buffer queries.
func NewEPPAuditService(repo repositories.EPPTransactionRepository, buffer int) *EPPAuditService {
	if buffer < 1 {
		buffer = 1
	}
	return &EPPAuditService{repo: repo, queue: make(chan *entities.EPPTransaction, buffer), now: time.Now}
}

// Record writes tx now.
func (s *EPPAuditService) Record(ctx context.Context, tx *entities.EPPTransaction) error {
	if err := s.repo.Save(ctx, tx); err != nil {
		return fmt.Errorf("%w: %w", ErrEPPAuditUnavailable, err)
	}
	return nil
}

// Enqueue queues tx for the next batch, or writes it now if the queue is
// full.
func (s *EPPAuditService) Enqueue(ctx context.Context, tx *entities.EPPTransaction) error {
	select {
	case s.queue <- tx:
		return nil
	default:
		return s.Record(ctx, tx)
	}
}

// Queued is how many records are waiting to be written.
func (s *EPPAuditService) Queued() int { return len(s.queue) }

// Run writes queued records until ctx is done, then writes what is left
// using flushTimeout. A batch that fails is retried every interval; while it
// is, the queue fills and Enqueue falls back to writing records itself.
// errs receives every failed batch write; it may be nil.
func (s *EPPAuditService) Run(ctx context.Context, flushTimeout time.Duration, errs func(error)) {
	ticker := time.NewTicker(auditFlushInterval)
	defer ticker.Stop()
	batch := make([]*entities.EPPTransaction, 0, auditBatchSize)
	write := func(ctx context.Context) {
		if len(batch) == 0 {
			return
		}
		if err := s.repo.Save(ctx, batch...); err != nil {
			if errs != nil {
				errs(fmt.Errorf("%w: %w", ErrEPPAuditUnavailable, err))
			}
			return
		}
		batch = batch[:0]
	}
	for {
		// A full batch is one being retried: leave further records in the
		// queue until it is written.
		var in <-chan *entities.EPPTransaction
		if len(batch) < auditBatchSize {
			in = s.queue
		}
		select {
		case tx := <-in:
			batch = append(batch, tx)
			if len(batch) == auditBatchSize {
				write(ctx)
			}
		case <-ticker.C:
			write(ctx)
		case <-ctx.Done():
			for drained := false; !drained; {
				select {
				case tx := <-s.queue:
					batch = append(batch, tx)
				default:
					drained = true
				}
			}
			final, cancel := context.WithTimeout(context.Background(), flushTimeout)
			write(final)
			cancel()
			return
		}
	}
}

// Prune deletes records received more than retention ago, and returns how
// many it deleted.
func (s *EPPAuditService) Prune(ctx context.Context, retention time.Duration) (int64, error) {
	cutoff := s.now().Add(-retention)
	var total int64
	for {
		n, err := s.repo.DeleteReceivedBefore(ctx, cutoff, auditPruneBatch)
		total += n
		if err != nil {
			return total, fmt.Errorf("%w: %w", ErrEPPAuditUnavailable, err)
		}
		if n < auditPruneBatch {
			return total, nil
		}
	}
}
