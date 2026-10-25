package activities

import (
	"context"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReleaseOffset(t *testing.T) {
	seen := map[time.Duration]bool{}
	for range 200 {
		d, err := ReleaseOffset(context.Background(), 45*time.Minute)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, d, time.Duration(0))
		assert.Less(t, d, 45*time.Minute)
		assert.Zero(t, d%time.Second)
		seen[d] = true
	}
	assert.Greater(t, len(seen), 100, "the offsets vary")
	d, err := ReleaseOffset(context.Background(), 0)
	require.NoError(t, err)
	assert.Zero(t, d, "no window, no wait")
}

func TestShuffleForRelease(t *testing.T) {
	names := []string{"a.test", "b.test", "c.test", "d.test", "e.test", "f.test", "g.test", "h.test"}
	orders := map[string]bool{}
	for range 50 {
		got, err := ShuffleForRelease(context.Background(), names)
		require.NoError(t, err)
		sorted := slices.Clone(got)
		slices.Sort(sorted)
		assert.Equal(t, names, sorted, "the same names")
		orders[strings.Join(got, ",")] = true
	}
	assert.Greater(t, len(orders), 1, "in more than one order")
	assert.Equal(t, []string{"a.test", "b.test", "c.test", "d.test", "e.test", "f.test", "g.test", "h.test"}, names, "the input is left alone")
	got, err := ShuffleForRelease(context.Background(), nil)
	require.NoError(t, err)
	assert.Empty(t, got)
}
