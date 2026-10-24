package lockout

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
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

func TestFailuresWhileLockedDoNotStartTheNextLock(t *testing.T) {
	ctx := context.Background()
	c := &clock{t: time.Date(2026, 10, 13, 12, 0, 0, 0, time.UTC)}
	m := NewMemory(testPolicy)
	m.now = c.now
	for i := 0; i < 3; i++ {
		_, _ = m.RecordFailure(ctx, "k")
	}
	// A burst that was already verifying when the lock started.
	for i := 0; i < 10; i++ {
		d, err := m.RecordFailure(ctx, "k")
		require.NoError(t, err)
		require.Zero(t, d, "the existing lock is neither extended nor reported again")
	}
	c.advance(11 * time.Second)
	_, _ = m.RecordFailure(ctx, "k")
	_, _ = m.RecordFailure(ctx, "k")
	d, _ := m.RecordFailure(ctx, "k")
	require.Equal(t, 20*time.Second, d, "the next lock is the second strike, not the twelfth")
}

func TestMemoryTableIsBounded(t *testing.T) {
	ctx := context.Background()
	defer func(n int) { maxMemoryKeys = n }(maxMemoryKeys)
	maxMemoryKeys = 3
	c := &clock{t: time.Date(2026, 10, 13, 12, 0, 0, 0, time.UTC)}
	m := NewMemory(testPolicy)
	m.now = c.now

	// Three keys locked out fill the table; a fourth is not tracked, and
	// no lock is released to make room.
	for _, k := range []string{"a", "b", "c"} {
		for i := 0; i < 3; i++ {
			_, _ = m.RecordFailure(ctx, k)
		}
	}
	_, _ = m.RecordFailure(ctx, "d")
	require.Len(t, m.keys, 3)
	for _, k := range []string{"a", "b", "c"} {
		d, _ := m.LockedFor(ctx, k)
		require.Greater(t, d, time.Duration(0), k)
	}

	// Once the locks expire, their entries are still counting strikes, but
	// they can be dropped to make room.
	c.advance(11 * time.Second)
	_, _ = m.RecordFailure(ctx, "d")
	require.LessOrEqual(t, len(m.keys), 3)
	require.Contains(t, m.keys, "d")
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

// TestRedis runs the production Lua script against a real Redis on
// localhost:6379. CI starts one and the test fails there without it; on a
// machine without Redis it skips.
func TestRedis(t *testing.T) {
	client := redis.NewClient(&redis.Options{Addr: "localhost:6379", DB: 15})
	ctx := context.Background()
	if err := client.Ping(ctx).Err(); err != nil {
		if os.Getenv("CI") != "" {
			t.Fatalf("Redis is required in CI for the lockout script tests: %v", err)
		}
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

	// Failures in flight while locked are not counted, and do not report
	// the lock again: only the failure that started it does.
	for i := 0; i < 5; i++ {
		d, err = r.RecordFailure(ctx, key)
		require.NoError(t, err)
		require.Zero(t, d)
	}
	strikes, err := client.Get(ctx, "epp:lockout:strikes:"+key).Int()
	require.NoError(t, err)
	require.Equal(t, 1, strikes)

	// Expire the lock, and the next three failures start the second strike.
	require.NoError(t, client.Del(ctx, "epp:lockout:lock:"+key).Err())
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

func TestRedisListAndLiftLockouts(t *testing.T) {
	client := redis.NewClient(&redis.Options{Addr: "localhost:6379", DB: 9})
	ctx := context.Background()
	if err := client.Ping(ctx).Err(); err != nil {
		if os.Getenv("CI") != "" {
			t.Fatalf("Redis is required in CI for the lockout script tests: %v", err)
		}
		t.Skipf("no Redis on localhost:6379: %v", err)
	}
	t.Cleanup(func() { _ = client.Close() })
	require.NoError(t, client.FlushDB(ctx).Err())
	r := NewRedis(client, testPolicy)
	lock := func(key string) {
		for range testPolicy.MaxFailures {
			_, err := r.RecordFailure(ctx, key)
			require.NoError(t, err)
		}
	}
	lock("login-a|192.0.2.1")
	lock("login-a|2001:db8::1")
	lock("login-b|192.0.2.1")
	_, err := r.RecordFailure(ctx, "login-a|198.51.100.7") // counted, not locked
	require.NoError(t, err)

	all, err := r.ListLockouts(ctx)
	require.NoError(t, err)
	require.Len(t, all, 3)
	assert.Equal(t, "login-a", all[0].Login)
	assert.Equal(t, "192.0.2.1", all[0].ClientIP)
	assert.True(t, all[0].EndsAt.After(time.Now()))

	lifted, err := r.LiftLockouts(ctx, "login-a")
	require.NoError(t, err)
	assert.Len(t, lifted, 2)
	for _, src := range []string{"192.0.2.1", "2001:db8::1"} {
		d, err := r.LockedFor(ctx, "login-a|"+src)
		require.NoError(t, err)
		assert.Zero(t, d, src)
	}
	keys, err := client.Keys(ctx, "epp:lockout:*login-a|*").Result()
	require.NoError(t, err)
	assert.Empty(t, keys, "failure counts and strikes go too, from every source")
	all, _ = r.ListLockouts(ctx)
	require.Len(t, all, 1)
	assert.Equal(t, "login-b", all[0].Login)

	t.Run("login IDs with | or glob characters reach only their own keys", func(t *testing.T) {
		require.NoError(t, client.FlushDB(ctx).Err())
		lock("ops|a|192.0.2.1")
		lock("ops|192.0.2.1")
		lock("a*b|192.0.2.1")
		lock("axb|192.0.2.1")
		all, err := r.ListLockouts(ctx)
		require.NoError(t, err)
		logins := map[string]string{}
		for _, l := range all {
			logins[l.Login] = l.ClientIP
		}
		assert.Equal(t, map[string]string{"ops|a": "192.0.2.1", "ops": "192.0.2.1", "a*b": "192.0.2.1", "axb": "192.0.2.1"}, logins)

		lifted, err := r.LiftLockouts(ctx, "a*b")
		require.NoError(t, err)
		require.Len(t, lifted, 1)
		_, err = r.LiftLockouts(ctx, "ops")
		require.NoError(t, err)
		for _, kept := range []string{"axb|192.0.2.1", "ops|a|192.0.2.1"} {
			d, err := r.LockedFor(ctx, kept)
			require.NoError(t, err)
			assert.Positive(t, d, kept)
		}
	})
}
