package services

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
	"github.com/onasunnymorning/domain-os/pkg/domain/repositories"
)

// EPPCreateOutcomeRecorder counts each registrar's domain creates per UTC
// day, created and refused because the name is registered, and writes the
// counts in batches: every create on a dropcatch run is one, so one write
// each would be a write per create. Flush writes what is pending; counts
// not written are kept for the next flush.
type EPPCreateOutcomeRecorder struct {
	repo    repositories.EPPCreateOutcomeRepository
	now     func() time.Time
	mu      sync.Mutex
	pending map[createOutcomeKey]*entities.EPPCreateOutcomes
}

type createOutcomeKey struct {
	clid entities.ClIDType
	day  string
}

// NewEPPCreateOutcomeRecorder returns a recorder writing to repo.
func NewEPPCreateOutcomeRecorder(repo repositories.EPPCreateOutcomeRepository) *EPPCreateOutcomeRecorder {
	return &EPPCreateOutcomeRecorder{repo: repo, now: time.Now, pending: map[createOutcomeKey]*entities.EPPCreateOutcomes{}}
}

// Add counts one create of clid's, now.
func (r *EPPCreateOutcomeRecorder) Add(clid entities.ClIDType, failed bool) {
	now := r.now().UTC()
	y, m, d := now.Date()
	day := time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
	r.mu.Lock()
	defer r.mu.Unlock()
	k := createOutcomeKey{clid: clid, day: day.Format(time.DateOnly)}
	e := r.pending[k]
	if e == nil {
		e = &entities.EPPCreateOutcomes{ClID: clid, Day: day}
		r.pending[k] = e
	}
	if failed {
		e.Failed++
	} else {
		e.Succeeded++
	}
}

// Flush writes the pending counts. If the write fails they are kept, added
// to whatever is counted meanwhile.
func (r *EPPCreateOutcomeRecorder) Flush(ctx context.Context) error {
	r.mu.Lock()
	pending := r.pending
	r.pending = map[createOutcomeKey]*entities.EPPCreateOutcomes{}
	r.mu.Unlock()
	if len(pending) == 0 {
		return nil
	}
	counts := make([]entities.EPPCreateOutcomes, 0, len(pending))
	for _, e := range pending {
		counts = append(counts, *e)
	}
	if err := r.repo.AddCreateOutcomes(ctx, counts); err != nil {
		r.mu.Lock()
		for k, e := range pending {
			if cur := r.pending[k]; cur != nil {
				cur.Succeeded += e.Succeeded
				cur.Failed += e.Failed
			} else {
				r.pending[k] = e
			}
		}
		r.mu.Unlock()
		return fmt.Errorf("write EPP create outcomes: %w", err)
	}
	return nil
}
