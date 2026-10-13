package memstore

import (
	"context"
	"slices"
	"sync"
	"time"

	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
)

// EPPTransactions is an in-memory repositories.EPPTransactionRepository.
type EPPTransactions struct {
	mu  sync.Mutex
	txs map[string]entities.EPPTransaction
	err error
}

// NewEPPTransactions returns an empty audit log.
func NewEPPTransactions() *EPPTransactions {
	return &EPPTransactions{txs: map[string]entities.EPPTransaction{}}
}

// Fail makes every write return err, to simulate an outage; nil ends it.
func (r *EPPTransactions) Fail(err error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.err = err
}

// Save stores the transactions, keeping any already stored by svTRID.
func (r *EPPTransactions) Save(_ context.Context, txs ...*entities.EPPTransaction) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.err != nil {
		return r.err
	}
	for _, tx := range txs {
		if _, ok := r.txs[tx.SvTRID]; !ok {
			r.txs[tx.SvTRID] = *tx
		}
	}
	return nil
}

// DeleteReceivedBefore deletes up to limit transactions received before
// cutoff, oldest first.
func (r *EPPTransactions) DeleteReceivedBefore(_ context.Context, cutoff time.Time, limit int) (int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.err != nil {
		return 0, r.err
	}
	var old []entities.EPPTransaction
	for _, tx := range r.txs {
		if tx.ReceivedAt.Before(cutoff) {
			old = append(old, tx)
		}
	}
	slices.SortFunc(old, func(a, b entities.EPPTransaction) int { return a.ReceivedAt.Compare(b.ReceivedAt) })
	if len(old) > limit {
		old = old[:limit]
	}
	for _, tx := range old {
		delete(r.txs, tx.SvTRID)
	}
	return int64(len(old)), nil
}

// All returns the stored transactions, oldest first.
func (r *EPPTransactions) All() []entities.EPPTransaction {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]entities.EPPTransaction, 0, len(r.txs))
	for _, tx := range r.txs {
		out = append(out, tx)
	}
	slices.SortFunc(out, func(a, b entities.EPPTransaction) int {
		if c := a.ReceivedAt.Compare(b.ReceivedAt); c != 0 {
			return c
		}
		if a.SvTRID < b.SvTRID {
			return -1
		}
		return 1
	})
	return out
}
