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

func check(budget entities.EPPFailedCreateBudget, family *entities.EPPFailedCreateBudget) entities.EPPFailedCreateCheck {
	c := entities.EPPFailedCreateCheck{
		Login: "login-a", SuspendAfter: 3, SuspendFor: 15 * time.Minute,
		Scopes: []entities.EPPFailedCreateScope{{Key: "reg:rar-a", Budget: budget}},
	}
	if family != nil {
		c.Scopes = append(c.Scopes, entities.EPPFailedCreateScope{Key: "fam:acme", Budget: *family})
	}
	return c
}

func record(t *testing.T, tr failedCreateTracker, scopes []string, failed, succeeded int) {
	t.Helper()
	for range failed {
		require.NoError(t, tr.Record(context.Background(), scopes, true))
	}
	for range succeeded {
		require.NoError(t, tr.Record(context.Background(), scopes, false))
	}
}

// failedCreatesContract is what every tracker does.
func failedCreatesContract(t *testing.T, tr failedCreateTracker) {
	ctx := context.Background()
	budget := entities.EPPFailedCreateBudget{Failures: 4, RatioPercent: 80}

	st, err := tr.Assess(ctx, check(budget, nil))
	require.NoError(t, err)
	assert.Zero(t, st, "nothing counted, nothing to say")

	record(t, tr, []string{"reg:rar-a"}, 4, 2)
	st, _ = tr.Assess(ctx, check(budget, nil))
	assert.Zero(t, st.Strikes, "4 of 6 is under 80%")

	record(t, tr, []string{"reg:rar-a"}, 4, 0)
	for i := 1; i <= 3; i++ {
		st, err = tr.Assess(ctx, check(budget, nil))
		require.NoError(t, err)
		assert.Equal(t, i, st.Strikes, "8 of 10 is over: each create sent is a strike")
		assert.Zero(t, st.Suspended)
	}
	st, _ = tr.Assess(ctx, check(budget, nil))
	assert.True(t, st.NewlySuspended, "the strike past SuspendAfter suspends the login")
	assert.Equal(t, 15*time.Minute, st.Suspended)
	d, err := tr.Suspended(ctx, "login-a")
	require.NoError(t, err)
	assert.InDelta(t, 15*time.Minute, d, float64(time.Second))
	st, _ = tr.Assess(ctx, check(budget, nil))
	assert.False(t, st.NewlySuspended)
	assert.Positive(t, st.Suspended, "and stays suspended")

	t.Run("a family over its budget is struck as one, whichever member sends the create", func(t *testing.T) {
		family := &entities.EPPFailedCreateBudget{Failures: 5, RatioPercent: 50}
		member := func(clid, login string) entities.EPPFailedCreateCheck {
			c := check(budget, family)
			c.Login, c.Scopes[0].Key = login, "reg:"+clid
			return c
		}
		record(t, tr, []string{"reg:rar-b", "fam:acme"}, 1, 0)
		record(t, tr, []string{"reg:rar-c", "fam:acme"}, 4, 4)
		st, err := tr.Assess(ctx, member("rar-b", "login-b"))
		require.NoError(t, err)
		assert.Equal(t, 1, st.Strikes)
		assert.Equal(t, 1, st.Over, "the family's scope")
		for i := 2; i <= 3; i++ {
			st, _ = tr.Assess(ctx, member("rar-c", "login-c"))
			assert.Equal(t, i, st.Strikes, "the family's strikes, not rar-c's own")
		}
		st, _ = tr.Assess(ctx, member("rar-d", "login-d"))
		assert.True(t, st.NewlySuspended, "past SuspendAfter for the family")
		st, _ = tr.Assess(ctx, member("rar-d", "login-d"))
		assert.Positive(t, st.Suspended)
		assert.Equal(t, 1, st.Over, "the suspension remembers it was the family's")
	})
}

func TestMemoryFailedCreates(t *testing.T) {
	m := NewMemoryFailedCreates()
	failedCreatesContract(t, m)

	t.Run("counts leave the window, strikes expire, suspensions end", func(t *testing.T) {
		m := NewMemoryFailedCreates()
		now := time.Unix(1_700_000_000, 0)
		m.now = func() time.Time { return now }
		ctx := context.Background()
		budget := entities.EPPFailedCreateBudget{Failures: 2, RatioPercent: 50}
		record(t, m, []string{"reg:rar-a"}, 2, 0)
		st, _ := m.Assess(ctx, check(budget, nil))
		assert.Equal(t, 1, st.Strikes)

		now = now.Add(entities.EPPFailedCreateWindow)
		st, _ = m.Assess(ctx, check(budget, nil))
		assert.Zero(t, st.Strikes, "the failures have left the window")
		record(t, m, []string{"reg:rar-a"}, 2, 0)
		st, _ = m.Assess(ctx, check(budget, nil))
		assert.Equal(t, 1, st.Strikes, "the earlier strike expired with the window")
		assert.Len(t, m.slots["reg:rar-a"], 1, "old slots are dropped")

		c := check(budget, nil)
		c.SuspendAfter = 1
		st, _ = m.Assess(ctx, c)
		require.True(t, st.NewlySuspended)
		now = now.Add(c.SuspendFor)
		d, _ := m.Suspended(ctx, "login-a")
		assert.Zero(t, d)
	})
}

func TestRedisFailedCreates(t *testing.T) {
	client := redis.NewClient(&redis.Options{Addr: "localhost:6379", DB: 11})
	t.Cleanup(func() { _ = client.Close() })
	ctx := context.Background()
	if err := client.Ping(ctx).Err(); err != nil {
		t.Skipf("no Redis on localhost:6379: %v", err)
	}
	require.NoError(t, client.FlushDB(ctx).Err())
	failedCreatesContract(t, NewRedis(client))

	ttl, err := client.PTTL(ctx, strikeKey("reg:rar-a")).Result()
	require.NoError(t, err)
	assert.InDelta(t, entities.EPPFailedCreateWindow, ttl, float64(time.Second), "strikes expire a window after the last")
}

type failingTracker struct{ calls int }

func (f *failingTracker) Assess(context.Context, entities.EPPFailedCreateCheck) (entities.EPPFailedCreateStanding, error) {
	f.calls++
	return entities.EPPFailedCreateStanding{}, errors.New("down")
}

func (f *failingTracker) Record(context.Context, []string, bool) error {
	f.calls++
	return errors.New("down")
}

func (f *failingTracker) Suspended(context.Context, string) (time.Duration, error) {
	f.calls++
	return 0, errors.New("down")
}

func TestFailOpenFailedCreates(t *testing.T) {
	primary := &failingTracker{}
	var degraded []bool
	f := NewFailOpenFailedCreates(primary, NewMemoryFailedCreates(), func(d bool) { degraded = append(degraded, d) })
	ctx := context.Background()
	budget := entities.EPPFailedCreateBudget{Failures: 1, RatioPercent: 100}

	require.NoError(t, f.Record(ctx, []string{"reg:rar-a"}, true))
	c := check(budget, nil)
	c.SuspendAfter = 0
	st, err := f.Assess(ctx, c)
	require.NoError(t, err)
	assert.True(t, st.NewlySuspended, "counted and decided on the node")
	assert.Equal(t, 1, primary.calls, "the primary is left alone after it fails")
	assert.Equal(t, []bool{true}, degraded)

	f.skipUntil = time.Time{}
	d, err := f.Suspended(ctx, "login-a")
	require.NoError(t, err)
	assert.Equal(t, 15*time.Minute, d.Round(time.Minute), "a suspension made on the node holds")
}
