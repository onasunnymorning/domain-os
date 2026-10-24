package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
)

func TestEPPCreateOutcomeRepository(t *testing.T) {
	tx := getTestDB().Begin()
	defer tx.Rollback()
	repo := NewEPPCreateOutcomeRepository(tx)
	ctx := context.Background()
	day1 := time.Date(2026, 10, 22, 23, 30, 0, 0, time.FixedZone("x", -3600)) // 2026-10-23 UTC
	day2 := time.Date(2026, 10, 24, 0, 0, 0, 0, time.UTC)

	require.NoError(t, repo.AddCreateOutcomes(ctx, []entities.EPPCreateOutcomes{
		{ClID: "rar-out-a", Day: day1, Succeeded: 2, Failed: 30},
		{ClID: "rar-out-b", Day: day1, Succeeded: 1},
	}))
	require.NoError(t, repo.AddCreateOutcomes(ctx, []entities.EPPCreateOutcomes{
		{ClID: "rar-out-a", Day: day1, Succeeded: 1, Failed: 5},
		{ClID: "rar-out-a", Day: day2, Failed: 7},
	}))
	got, err := repo.ListCreateOutcomes(ctx, "rar-out-a", day1, day2)
	require.NoError(t, err)
	assert.Equal(t, []entities.EPPCreateOutcomes{
		{ClID: "rar-out-a", Day: time.Date(2026, 10, 23, 0, 0, 0, 0, time.UTC), Succeeded: 3, Failed: 35},
		{ClID: "rar-out-a", Day: day2, Failed: 7},
	}, got, "added to, by UTC day")
	got, err = repo.ListCreateOutcomes(ctx, "rar-out-a", day2, day2)
	require.NoError(t, err)
	assert.Len(t, got, 1)
}
