package services

import (
	"context"
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
	// auditFlushRetry is how often the final write at shutdown is retried.
	auditFlushRetry = 100 * time.Millisecond
	// auditPruneBatch is how many expired rows one delete removes, so
	// pruning a large backlog never holds a long lock.
	auditPruneBatch = 5000
)

// EPPAuditLostError reports queued records that were never written: at
// shutdown, the database did not take them before the flush timeout.
type EPPAuditLostError struct {
	Records int
	Err     error
}

func (e *EPPAuditLostError) Error() string {
	return fmt.Sprintf("%d EPP audit records lost at shutdown: %v", e.Records, e.Err)
}

func (e *EPPAuditLostError) Unwrap() error { return e.Err }

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
		return fmt.Errorf("%w: %w", entities.ErrEPPAuditUnavailable, err)
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

// Run writes queued records until ctx is done, then writes what is left,
// retrying until flushTimeout. A batch that fails is retried every
// interval; while it is, the queue fills and Enqueue falls back to writing
// records itself. errs receives every failed batch write, and at shutdown an
// *EPPAuditLostError for any records still unwritten; it may be nil.
func (s *EPPAuditService) Run(ctx context.Context, flushTimeout time.Duration, errs func(error)) {
	ticker := time.NewTicker(auditFlushInterval)
	defer ticker.Stop()
	batch := make([]*entities.EPPTransaction, 0, auditBatchSize)
	report := func(err error) {
		if errs != nil {
			errs(err)
		}
	}
	write := func(ctx context.Context) error {
		if len(batch) == 0 {
			return nil
		}
		if err := s.repo.Save(ctx, batch...); err != nil {
			return fmt.Errorf("%w: %w", entities.ErrEPPAuditUnavailable, err)
		}
		batch = batch[:0]
		return nil
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
				if err := write(ctx); err != nil {
					report(err)
				}
			}
		case <-ticker.C:
			if err := write(ctx); err != nil {
				report(err)
			}
		case <-ctx.Done():
			for drained := false; !drained; {
				select {
				case tx := <-s.queue:
					batch = append(batch, tx)
				default:
					drained = true
				}
			}
			s.flush(flushTimeout, write, report, func() int { return len(batch) })
			return
		}
	}
}

// flush writes the final batch at shutdown, retrying until timeout, and
// reports what is still unwritten then as lost.
func (s *EPPAuditService) flush(timeout time.Duration, write func(context.Context) error, report func(error), pending func() int) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	retry := time.NewTicker(auditFlushRetry)
	defer retry.Stop()
	for {
		err := write(ctx)
		if err == nil {
			return
		}
		select {
		case <-retry.C:
		case <-ctx.Done():
			report(&EPPAuditLostError{Records: pending(), Err: err})
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
			return total, fmt.Errorf("%w: %w", entities.ErrEPPAuditUnavailable, err)
		}
		if n < auditPruneBatch {
			return total, nil
		}
	}
}
