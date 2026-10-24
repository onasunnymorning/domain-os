package memstore

import (
	"context"
	"slices"
	"sync"
	"time"

	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
)

// EPPCreateOutcomes is an in-memory repositories.EPPCreateOutcomeRepository
// for tests.
type EPPCreateOutcomes struct {
	mu     sync.Mutex
	counts map[entities.ClIDType]map[time.Time]*entities.EPPCreateOutcomes
}

// NewEPPCreateOutcomes returns an empty repository.
func NewEPPCreateOutcomes() *EPPCreateOutcomes {
	return &EPPCreateOutcomes{counts: map[entities.ClIDType]map[time.Time]*entities.EPPCreateOutcomes{}}
}

func utcDay(t time.Time) time.Time {
	y, m, d := t.UTC().Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

// AddCreateOutcomes implements repositories.EPPCreateOutcomeRepository.
func (r *EPPCreateOutcomes) AddCreateOutcomes(_ context.Context, counts []entities.EPPCreateOutcomes) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, c := range counts {
		days := r.counts[c.ClID]
		if days == nil {
			days = map[time.Time]*entities.EPPCreateOutcomes{}
			r.counts[c.ClID] = days
		}
		day := utcDay(c.Day)
		e := days[day]
		if e == nil {
			e = &entities.EPPCreateOutcomes{ClID: c.ClID, Day: day}
			days[day] = e
		}
		e.Succeeded += c.Succeeded
		e.Failed += c.Failed
	}
	return nil
}

// ListCreateOutcomes implements repositories.EPPCreateOutcomeRepository.
func (r *EPPCreateOutcomes) ListCreateOutcomes(_ context.Context, clid entities.ClIDType, from, to time.Time) ([]entities.EPPCreateOutcomes, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []entities.EPPCreateOutcomes
	for day, e := range r.counts[clid] {
		if !day.Before(utcDay(from)) && !day.After(utcDay(to)) {
			out = append(out, *e)
		}
	}
	slices.SortFunc(out, func(a, b entities.EPPCreateOutcomes) int { return a.Day.Compare(b.Day) })
	return out, nil
}
