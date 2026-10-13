package services

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onasunnymorning/domain-os/internal/infrastructure/memstore"
	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
)

func auditTx(id string, at time.Time) *entities.EPPTransaction {
	return &entities.EPPTransaction{SvTRID: id, Command: "domain:check", ReceivedAt: at}
}

func TestEPPAuditRecordAndBatches(t *testing.T) {
	store := memstore.NewEPPTransactions()
	audit := NewEPPAuditService(store, 10)

	require.NoError(t, audit.Record(context.Background(), auditTx("T1", time.Now())))
	assert.Len(t, store.All(), 1, "Record writes before it returns")

	require.NoError(t, audit.Enqueue(context.Background(), auditTx("Q1", time.Now())))
	assert.Len(t, store.All(), 1, "Enqueue only queues")
	assert.Equal(t, 1, audit.Queued())

	ctx, stop := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { defer close(done); audit.Run(ctx, time.Second, nil) }()
	require.Eventually(t, func() bool { return len(store.All()) == 2 }, 3*time.Second, 10*time.Millisecond, "the queue is written within a flush interval")

	require.NoError(t, audit.Enqueue(context.Background(), auditTx("Q2", time.Now())))
	stop()
	<-done
	assert.Len(t, store.All(), 3, "what is queued at shutdown is written before Run returns")
}

func TestEPPAuditFullQueueWritesInline(t *testing.T) {
	store := memstore.NewEPPTransactions()
	audit := NewEPPAuditService(store, 1)
	require.NoError(t, audit.Enqueue(context.Background(), auditTx("Q1", time.Now())))
	require.NoError(t, audit.Enqueue(context.Background(), auditTx("Q2", time.Now())))
	assert.Len(t, store.All(), 1, "the record that did not fit was written, not dropped")

	store.Fail(errors.New("database down"))
	err := audit.Enqueue(context.Background(), auditTx("Q3", time.Now()))
	require.ErrorIs(t, err, ErrEPPAuditUnavailable, "a record that can be neither queued nor written is reported")
}

func TestEPPAuditRetriesAFailedBatch(t *testing.T) {
	store := memstore.NewEPPTransactions()
	store.Fail(errors.New("database down"))
	audit := NewEPPAuditService(store, 10)
	require.NoError(t, audit.Enqueue(context.Background(), auditTx("Q1", time.Now())))

	var mu sync.Mutex
	var failures int
	ctx, stop := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		audit.Run(ctx, time.Second, func(error) { mu.Lock(); failures++; mu.Unlock() })
	}()
	require.Eventually(t, func() bool { mu.Lock(); defer mu.Unlock(); return failures > 0 }, 3*time.Second, 10*time.Millisecond)
	store.Fail(nil)
	require.Eventually(t, func() bool { return len(store.All()) == 1 }, 3*time.Second, 10*time.Millisecond, "the failed batch is retried, not dropped")
	stop()
	<-done
}

func TestEPPAuditPrune(t *testing.T) {
	store := memstore.NewEPPTransactions()
	audit := NewEPPAuditService(store, 10)
	now := time.Now()
	audit.now = func() time.Time { return now }
	for i, age := range []time.Duration{48 * time.Hour, 25 * time.Hour, time.Hour} {
		require.NoError(t, audit.Record(context.Background(), auditTx(string(rune('A'+i)), now.Add(-age))))
	}
	n, err := audit.Prune(context.Background(), 24*time.Hour)
	require.NoError(t, err)
	assert.Equal(t, int64(2), n)
	left := store.All()
	require.Len(t, left, 1)
	assert.Equal(t, "C", left[0].SvTRID)
}

func TestEPPAuditShutdownRetriesThenReportsLoss(t *testing.T) {
	run := func(t *testing.T, store *memstore.EPPTransactions, flushTimeout time.Duration) []error {
		t.Helper()
		audit := NewEPPAuditService(store, 10)
		for _, id := range []string{"Q1", "Q2", "Q3"} {
			require.NoError(t, audit.Enqueue(context.Background(), auditTx(id, time.Now())))
		}
		var mu sync.Mutex
		var errs []error
		ctx, stop := context.WithCancel(context.Background())
		stop() // shut down at once: everything goes through the final flush
		audit.Run(ctx, flushTimeout, func(err error) { mu.Lock(); errs = append(errs, err); mu.Unlock() })
		return errs
	}

	t.Run("a database that recovers within the timeout gets every record", func(t *testing.T) {
		store := memstore.NewEPPTransactions()
		store.Fail(errors.New("database restarting"))
		time.AfterFunc(250*time.Millisecond, func() { store.Fail(nil) })
		errs := run(t, store, 2*time.Second)
		assert.Empty(t, errs)
		assert.Len(t, store.All(), 3)
	})

	t.Run("records still unwritten at the timeout are reported lost, with their count", func(t *testing.T) {
		store := memstore.NewEPPTransactions()
		store.Fail(errors.New("database down"))
		errs := run(t, store, 300*time.Millisecond)
		require.Len(t, errs, 1)
		var lost *EPPAuditLostError
		require.ErrorAs(t, errs[0], &lost)
		assert.Equal(t, 3, lost.Records)
		assert.ErrorIs(t, errs[0], ErrEPPAuditUnavailable)
	})
}
