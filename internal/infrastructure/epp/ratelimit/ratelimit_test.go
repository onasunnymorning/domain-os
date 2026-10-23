package ratelimit

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
)

func bucket(key string, perSecond, burst int) entities.EPPRateBucket {
	return entities.EPPRateBucket{Key: key, Rate: entities.EPPRate{PerSecond: perSecond, Burst: burst}}
}

// GCRA with a clock that only moves when told to.
func TestTakeIsGCRA(t *testing.T) {
	m := NewMemory()
	now := time.Unix(1_700_000_000, 0)
	m.now = func() time.Time { return now }
	ctx := context.Background()
	b := []entities.EPPRateBucket{bucket("r", 10, 5)} // a token every 100ms, 5 at once

	for i := range 5 {
		res, _ := m.Take(ctx, b, 1, 0)
		require.True(t, res.Taken, "burst token %d", i)
		assert.Zero(t, res.Wait)
	}
	res, _ := m.Take(ctx, b, 1, 0)
	assert.False(t, res.Taken, "the burst is spent")
	assert.Equal(t, 100*time.Millisecond, res.Wait, "the next token is one interval away")

	res, _ = m.Take(ctx, b, 1, 150*time.Millisecond)
	assert.True(t, res.Taken, "a wait the caller accepts takes the token, to be used after it")
	assert.Equal(t, 100*time.Millisecond, res.Wait)
	res, _ = m.Take(ctx, b, 1, 150*time.Millisecond)
	assert.False(t, res.Taken, "the next is 200ms away: not taken, and nothing taken")
	assert.Equal(t, 200*time.Millisecond, res.Wait)

	now = now.Add(time.Second)
	res, _ = m.Take(ctx, b, 3, 0)
	assert.True(t, res.Taken, "tokens come back at the rate, up to the burst")
	res, _ = m.Take(ctx, b, 3, 0)
	assert.False(t, res.Taken)
	assert.Equal(t, 100*time.Millisecond, res.Wait, "cost counts in tokens")
}

func TestTakeFromSeveralBucketsAtOnce(t *testing.T) {
	m := NewMemory()
	now := time.Unix(1_700_000_000, 0)
	m.now = func() time.Time { return now }
	ctx := context.Background()
	reg := bucket("reg", 10, 10)
	fam := bucket("fam", 10, 3)

	res, _ := m.Take(ctx, []entities.EPPRateBucket{reg, fam}, 3, 0)
	require.True(t, res.Taken)
	res, _ = m.Take(ctx, []entities.EPPRateBucket{reg, fam}, 1, 0)
	assert.False(t, res.Taken)
	assert.Equal(t, 1, res.Limiting, "the family bucket is the one that waits")
	res, _ = m.Take(ctx, []entities.EPPRateBucket{reg}, 7, 0)
	assert.True(t, res.Taken, "a refused take took nothing from the registrar's bucket")
}

func TestMemoryQuota(t *testing.T) {
	m := NewMemory()
	now := time.Unix(1_700_000_000, 0)
	m.now = func() time.Time { return now }
	ctx := context.Background()

	r1, ok, _ := m.ReserveQuota(ctx, "q", 2, time.Hour)
	require.True(t, ok)
	_, ok, _ = m.ReserveQuota(ctx, "q", 2, time.Hour)
	require.True(t, ok)
	release, ok, _ := m.ReserveQuota(ctx, "q", 2, time.Hour)
	assert.False(t, ok)
	assert.Nil(t, release)
	r1(ctx)
	_, ok, _ = m.ReserveQuota(ctx, "q", 2, time.Hour)
	assert.True(t, ok, "a released reservation is free again")
	now = now.Add(time.Hour)
	_, ok, _ = m.ReserveQuota(ctx, "q", 2, time.Hour)
	assert.True(t, ok, "the count expires")
}

func TestMemoryTablesAreBounded(t *testing.T) {
	defer func(n int) { maxMemoryKeys = n }(maxMemoryKeys)
	maxMemoryKeys = 2
	m := NewMemory()
	now := time.Unix(1_700_000_000, 0)
	m.now = func() time.Time { return now }
	ctx := context.Background()
	for _, k := range []string{"a", "b", "c"} {
		res, _ := m.Take(ctx, []entities.EPPRateBucket{bucket(k, 1, 1)}, 1, 0)
		assert.True(t, res.Taken, k)
	}
	assert.Len(t, m.tats, 2, "past the bound a new bucket goes uncounted")
	now = now.Add(2 * time.Second)
	_, _ = m.Take(ctx, []entities.EPPRateBucket{bucket("c", 1, 1)}, 1, 0)
	assert.Contains(t, m.tats, "c", "full buckets make room")
}

type failing struct{ calls int }

func (f *failing) Take(context.Context, []entities.EPPRateBucket, int, time.Duration) (entities.EPPRateTake, error) {
	f.calls++
	return entities.EPPRateTake{}, errors.New("down")
}

func (f *failing) ReserveQuota(context.Context, string, int, time.Duration) (func(context.Context), bool, error) {
	f.calls++
	return nil, false, errors.New("down")
}

func TestFailOpenFallsBackToTheNode(t *testing.T) {
	primary := &failing{}
	var degraded []bool
	f := NewFailOpen(primary, NewMemory(), func(d bool) { degraded = append(degraded, d) })
	now := time.Unix(1_700_000_000, 0)
	f.now = func() time.Time { return now }
	ctx := context.Background()
	b := []entities.EPPRateBucket{bucket("r", 1, 1)}

	res, err := f.Take(ctx, b, 1, 0)
	require.NoError(t, err)
	assert.True(t, res.Taken)
	res, _ = f.Take(ctx, b, 1, 0)
	assert.False(t, res.Taken, "the node's buckets apply")
	_, ok, err := f.ReserveQuota(ctx, "q", 1, time.Hour)
	require.NoError(t, err)
	assert.True(t, ok)
	assert.Equal(t, 1, primary.calls, "after a failure the primary is left alone for a while")
	assert.Equal(t, []bool{true}, degraded)

	now = now.Add(retryPrimary)
	_, _ = f.Take(ctx, b, 1, 0)
	assert.Equal(t, 2, primary.calls, "then tried again")
}

func TestRedis(t *testing.T) {
	client := redis.NewClient(&redis.Options{Addr: "localhost:6379", DB: 12})
	t.Cleanup(func() { _ = client.Close() })
	ctx := context.Background()
	if err := client.Ping(ctx).Err(); err != nil {
		t.Skipf("no Redis on localhost:6379: %v", err)
	}
	require.NoError(t, client.FlushDB(ctx).Err())
	r := NewRedis(client)

	t.Run("buckets", func(t *testing.T) {
		b := []entities.EPPRateBucket{bucket("t:reg", 1, 3), bucket("t:fam", 1, 10)}
		for i := range 3 {
			res, err := r.Take(ctx, b, 1, 0)
			require.NoError(t, err)
			require.True(t, res.Taken, "burst token %d", i)
		}
		res, err := r.Take(ctx, b, 1, 0)
		require.NoError(t, err)
		assert.False(t, res.Taken)
		assert.Equal(t, 0, res.Limiting)
		assert.InDelta(t, time.Second, res.Wait, float64(100*time.Millisecond))

		res, err = r.Take(ctx, b, 1, 2*time.Second)
		require.NoError(t, err)
		assert.True(t, res.Taken, "within the wait accepted")
		assert.InDelta(t, time.Second, res.Wait, float64(100*time.Millisecond))

		ttl, err := client.PTTL(ctx, bucketKey("t:reg")).Result()
		require.NoError(t, err)
		assert.InDelta(t, 4*time.Second, ttl, float64(200*time.Millisecond), "a bucket's key expires when it is full again")
		tat, err := client.Get(ctx, bucketKey("t:reg")).Int64()
		require.NoError(t, err, "the TAT is stored as whole microseconds")
		assert.Greater(t, tat, time.Now().UnixMicro())
	})

	t.Run("quota", func(t *testing.T) {
		release, ok, err := r.ReserveQuota(ctx, "q", 2, time.Hour)
		require.NoError(t, err)
		require.True(t, ok)
		_, ok, _ = r.ReserveQuota(ctx, "q", 2, time.Hour)
		require.True(t, ok)
		_, ok, err = r.ReserveQuota(ctx, "q", 2, time.Hour)
		require.NoError(t, err)
		assert.False(t, ok)
		n, _ := r.QuotaCount(ctx, "q")
		assert.Equal(t, 2, n, "a refused reservation counts nothing")

		release(ctx)
		n, _ = r.QuotaCount(ctx, "q")
		assert.Equal(t, 1, n)
		ttl, _ := client.PTTL(ctx, quotaKey("q")).Result()
		assert.InDelta(t, time.Hour, ttl, float64(time.Minute))
	})
}
