package services

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onasunnymorning/domain-os/internal/infrastructure/memstore"
	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
)

type failingOutcomes struct{ *memstore.EPPCreateOutcomes }

func (failingOutcomes) AddCreateOutcomes(context.Context, []entities.EPPCreateOutcomes) error {
	return errors.New("database down")
}

func TestEPPCreateOutcomeRecorder(t *testing.T) {
	ctx := context.Background()
	repo := memstore.NewEPPCreateOutcomes()
	r := NewEPPCreateOutcomeRecorder(repo)
	now := time.Date(2026, 10, 23, 23, 59, 0, 0, time.UTC)
	r.now = func() time.Time { return now }

	r.Add("rar-a", true)
	r.Add("rar-a", true)
	r.Add("rar-a", false)
	now = now.Add(2 * time.Minute) // the next UTC day
	r.Add("rar-a", true)
	require.NoError(t, r.Flush(ctx))
	got, _ := repo.ListCreateOutcomes(ctx, "rar-a", now.AddDate(0, 0, -1), now)
	require.Len(t, got, 2)
	assert.Equal(t, entities.EPPCreateOutcomes{ClID: "rar-a", Day: time.Date(2026, 10, 23, 0, 0, 0, 0, time.UTC), Succeeded: 1, Failed: 2}, got[0])
	assert.Equal(t, int64(1), got[1].Failed)

	t.Run("a failed write keeps the counts for the next", func(t *testing.T) {
		r.Add("rar-b", true)
		r.repo = failingOutcomes{repo}
		require.Error(t, r.Flush(ctx))
		r.Add("rar-b", true)
		r.repo = repo
		require.NoError(t, r.Flush(ctx))
		got, _ := repo.ListCreateOutcomes(ctx, "rar-b", now, now)
		require.Len(t, got, 1)
		assert.Equal(t, int64(2), got[0].Failed)
	})
}
