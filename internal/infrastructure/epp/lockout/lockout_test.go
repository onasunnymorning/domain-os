package lockout

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

var testPolicy = Policy{MaxFailures: 3, Window: time.Minute, BaseLock: 10 * time.Second, MaxLock: 35 * time.Second, StrikeMemory: time.Hour}

func TestPolicyBackoff(t *testing.T) {
	for strike, want := range map[int]time.Duration{1: 10 * time.Second, 2: 20 * time.Second, 3: 35 * time.Second, 9: 35 * time.Second} {
		require.Equal(t, want, testPolicy.lockFor(strike), "strike %d", strike)
	}
	require.Equal(t, 24*time.Hour, DefaultPolicy.lockFor(10))
}

type clock struct{ t time.Time }

func (c *clock) now() time.Time          { return c.t }
func (c *clock) advance(d time.Duration) { c.t = c.t.Add(d) }

func TestMemoryLocksAndBacksOff(t *testing.T) {
	ctx := context.Background()
	c := &clock{t: time.Date(2026, 10, 13, 12, 0, 0, 0, time.UTC)}
	m := NewMemory(testPolicy)
	m.now = c.now

	fail := func() time.Duration {
		d, err := m.RecordFailure(ctx, "k")
		require.NoError(t, err)
		return d
	}
	require.Zero(t, fail())
	require.Zero(t, fail())
	require.Equal(t, 10*time.Second, fail(), "the third failure locks")
	d, _ := m.LockedFor(ctx, "k")
	require.Equal(t, 10*time.Second, d)

	c.advance(11 * time.Second)
	d, _ = m.LockedFor(ctx, "k")
	require.Zero(t, d, "the lock expires")
	fail()
	fail()
	require.Equal(t, 20*time.Second, fail(), "the second lock doubles")

	other, _ := m.LockedFor(ctx, "other")
	require.Zero(t, other, "keys are independent")

	require.NoError(t, m.Clear(ctx, "k"))
	d, _ = m.LockedFor(ctx, "k")
	require.Zero(t, d)
	fail()
	fail()
	require.Equal(t, 10*time.Second, fail(), "clearing forgets the strikes too")
}

func TestMemoryWindowExpires(t *testing.T) {
	ctx := context.Background()
	c := &clock{t: time.Date(2026, 10, 13, 12, 0, 0, 0, time.UTC)}
	m := NewMemory(testPolicy)
	m.now = c.now
	_, _ = m.RecordFailure(ctx, "k")
	_, _ = m.RecordFailure(ctx, "k")
	c.advance(testPolicy.Window)
	d, _ := m.RecordFailure(ctx, "k")
	require.Zero(t, d, "failures outside the window do not add up")
}

type brokenStore struct{}

var errDown = errors.New("redis down")

func (brokenStore) LockedFor(context.Context, string) (time.Duration, error)     { return 0, errDown }
func (brokenStore) RecordFailure(context.Context, string) (time.Duration, error) { return 0, errDown }
func (brokenStore) Clear(context.Context, string) error                          { return errDown }

func TestFailOpenFallsBackToTheNode(t *testing.T) {
	ctx := context.Background()
	var degraded []bool
	f := NewFailOpen(brokenStore{}, NewMemory(testPolicy), func(d bool) { degraded = append(degraded, d) })
	for i := 0; i < 2; i++ {
		d, err := f.RecordFailure(ctx, "k")
		require.NoError(t, err)
		require.Zero(t, d)
	}
	d, err := f.RecordFailure(ctx, "k")
	require.NoError(t, err)
	require.Equal(t, 10*time.Second, d, "the node still locks while Redis is down")
	d, err = f.LockedFor(ctx, "k")
	require.NoError(t, err)
	require.Greater(t, d, time.Duration(0))
	require.NoError(t, f.Clear(ctx, "k"))
	require.NotContains(t, degraded, false)
	require.Contains(t, degraded, true)
}

func TestFailOpenKeepsALocalLockAfterRecovery(t *testing.T) {
	ctx := context.Background()
	local := NewMemory(testPolicy)
	for i := 0; i < 3; i++ {
		_, _ = local.RecordFailure(ctx, "k")
	}
	f := NewFailOpen(NewMemory(testPolicy), local, nil)
	d, err := f.LockedFor(ctx, "k")
	require.NoError(t, err)
	require.Greater(t, d, time.Duration(0))
}

// TestRedis runs against a local Redis when there is one, as the limiter's
// integration tests do; CI has none and skips it.
func TestRedis(t *testing.T) {
	client := redis.NewClient(&redis.Options{Addr: "localhost:6379", DB: 15})
	ctx := context.Background()
	if err := client.Ping(ctx).Err(); err != nil {
		t.Skipf("no Redis on localhost:6379: %v", err)
	}
	t.Cleanup(func() { _ = client.Close() })
	r := NewRedis(client, testPolicy)
	key := "test-" + time.Now().Format(time.RFC3339Nano)
	t.Cleanup(func() { _ = r.Clear(ctx, key) })

	for i := 0; i < 2; i++ {
		d, err := r.RecordFailure(ctx, key)
		require.NoError(t, err)
		require.Zero(t, d)
	}
	d, err := r.RecordFailure(ctx, key)
	require.NoError(t, err)
	require.Equal(t, 10*time.Second, d)
	left, err := r.LockedFor(ctx, key)
	require.NoError(t, err)
	require.InDelta(t, float64(10*time.Second), float64(left), float64(time.Second))

	for i := 0; i < 3; i++ {
		d, err = r.RecordFailure(ctx, key)
		require.NoError(t, err)
	}
	require.Equal(t, 20*time.Second, d, "the second lock doubles")

	require.NoError(t, r.Clear(ctx, key))
	left, err = r.LockedFor(ctx, key)
	require.NoError(t, err)
	require.Zero(t, left)
}
