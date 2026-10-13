// Package lockout locks a (login, source address) out of EPP after repeated
// failed logins, with exponential backoff (docs/epp/POLICY_REFERENCE.md).
//
// Redis keeps the counts across nodes. When Redis cannot answer, FailOpen
// falls back to a count kept on the node, so an outage neither locks every
// registrar out nor removes the lockout altogether (ADR 0013).
package lockout

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

// Policy is when a key is locked out and for how long.
type Policy struct {
	// MaxFailures within Window locks the key.
	MaxFailures int
	Window      time.Duration
	// The first lockout lasts BaseLock; each further one within StrikeMemory
	// doubles it, up to MaxLock.
	BaseLock     time.Duration
	MaxLock      time.Duration
	StrikeMemory time.Duration
}

// DefaultPolicy is 5 failures in 15 minutes: locked for 15 minutes, doubling
// on each repeat within a day, up to 24 hours.
var DefaultPolicy = Policy{MaxFailures: 5, Window: 15 * time.Minute, BaseLock: 15 * time.Minute, MaxLock: 24 * time.Hour, StrikeMemory: 24 * time.Hour}

// lockFor is the lockout for the strike-th lockout of a key, counting from 1.
func (p Policy) lockFor(strike int) time.Duration {
	d := p.BaseLock
	for i := 1; i < strike && d < p.MaxLock; i++ {
		d *= 2
	}
	return min(d, p.MaxLock)
}

// Memory keeps lockouts on this node only.
type Memory struct {
	policy Policy
	now    func() time.Time
	mu     sync.Mutex
	keys   map[string]*memEntry
}

type memEntry struct {
	failures    int
	windowEnds  time.Time
	lockedUntil time.Time
	strikes     int
	strikesEnd  time.Time
}

// NewMemory returns a node-local lockout.
func NewMemory(p Policy) *Memory {
	return &Memory{policy: p, now: time.Now, keys: map[string]*memEntry{}}
}

// LockedFor returns how much longer key is locked.
func (m *Memory) LockedFor(_ context.Context, key string) (time.Duration, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	e := m.keys[key]
	if e == nil {
		return 0, nil
	}
	return max(e.lockedUntil.Sub(m.now()), 0), nil
}

// RecordFailure counts a failure and returns the lockout now in force, if
// any. A failure while the key is already locked is not counted: it was in
// flight when the lock started, and counting it would start the next, doubled
// lock from the same burst.
func (m *Memory) RecordFailure(_ context.Context, key string) (time.Duration, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := m.now()
	e := m.keys[key]
	if e != nil && now.Before(e.lockedUntil) {
		return e.lockedUntil.Sub(now), nil
	}
	if e == nil {
		if !m.makeRoom(now) {
			// Every tracked key is locked out and the table is full. Leave
			// this one uncounted rather than grow without bound or release
			// a lock early.
			return 0, nil
		}
		e = &memEntry{}
		m.keys[key] = e
	}
	if !now.Before(e.windowEnds) {
		e.failures, e.windowEnds = 0, now.Add(m.policy.Window)
	}
	if !now.Before(e.strikesEnd) {
		e.strikes = 0
	}
	e.failures++
	if e.failures < m.policy.MaxFailures {
		return 0, nil
	}
	e.failures, e.windowEnds = 0, time.Time{}
	e.strikes++
	e.strikesEnd = now.Add(m.policy.StrikeMemory)
	d := m.policy.lockFor(e.strikes)
	e.lockedUntil = now.Add(d)
	return d, nil
}

// Clear forgets key.
func (m *Memory) Clear(_ context.Context, key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.keys, key)
	return nil
}

// maxMemoryKeys is a hard bound on the node-local table: about 10 MB.
var maxMemoryKeys = 100_000

// makeRoom reports whether a new key fits, freeing space if the table is
// full. It drops, in order of preference, entries with nothing left to
// remember, then entries that are counting failures or strikes but are not
// locked. A lock in force is never dropped.
func (m *Memory) makeRoom(now time.Time) bool {
	if len(m.keys) < maxMemoryKeys {
		return true
	}
	for k, e := range m.keys {
		if !now.Before(e.windowEnds) && !now.Before(e.lockedUntil) && !now.Before(e.strikesEnd) {
			delete(m.keys, k)
		}
	}
	for k, e := range m.keys {
		if len(m.keys) < maxMemoryKeys {
			break
		}
		if !now.Before(e.lockedUntil) {
			delete(m.keys, k)
		}
	}
	return len(m.keys) < maxMemoryKeys
}

// Redis keeps lockouts in Redis, shared by every node.
type Redis struct {
	client *redis.Client
	policy Policy
}

// NewRedis returns a Redis-backed lockout.
func NewRedis(client *redis.Client, p Policy) *Redis {
	return &Redis{client: client, policy: p}
}

func keys(key string) (fail, lock, strikes string) {
	return "epp:lockout:fail:" + key, "epp:lockout:lock:" + key, "epp:lockout:strikes:" + key
}

// recordFailure counts a failure atomically and, at the threshold, starts a
// lockout whose length doubles with each strike. It returns the lockout in
// force in milliseconds, or 0. A failure while the key is locked is not
// counted, so failures in flight when the lock started cannot start the next
// one.
var recordFailure = redis.NewScript(`
local locked = redis.call('PTTL', KEYS[2])
if locked > 0 then return locked end
local n = redis.call('INCR', KEYS[1])
if n == 1 then redis.call('PEXPIRE', KEYS[1], ARGV[2]) end
if n < tonumber(ARGV[1]) then return 0 end
redis.call('DEL', KEYS[1])
local s = redis.call('INCR', KEYS[3])
redis.call('PEXPIRE', KEYS[3], ARGV[5])
local d = tonumber(ARGV[3])
for i = 2, s do
  d = d * 2
  if d >= tonumber(ARGV[4]) then break end
end
if d > tonumber(ARGV[4]) then d = tonumber(ARGV[4]) end
redis.call('SET', KEYS[2], '1', 'PX', d)
return d
`)

// LockedFor returns how much longer key is locked.
func (r *Redis) LockedFor(ctx context.Context, key string) (time.Duration, error) {
	_, lock, _ := keys(key)
	d, err := r.client.PTTL(ctx, lock).Result()
	if err != nil {
		return 0, fmt.Errorf("lockout status: %w", err)
	}
	return max(d, 0), nil // -2 (no key) and -1 (no TTL) are not locked
}

// RecordFailure counts a failure and returns the lockout in force, if any.
func (r *Redis) RecordFailure(ctx context.Context, key string) (time.Duration, error) {
	fail, lock, strikes := keys(key)
	ms, err := recordFailure.Run(ctx, r.client, []string{fail, lock, strikes},
		r.policy.MaxFailures, r.policy.Window.Milliseconds(), r.policy.BaseLock.Milliseconds(),
		r.policy.MaxLock.Milliseconds(), r.policy.StrikeMemory.Milliseconds()).Int64()
	if err != nil {
		return 0, fmt.Errorf("record failed login: %w", err)
	}
	return time.Duration(ms) * time.Millisecond, nil
}

// Clear forgets key.
func (r *Redis) Clear(ctx context.Context, key string) error {
	fail, lock, strikes := keys(key)
	if err := r.client.Del(ctx, fail, lock, strikes).Err(); err != nil {
		return fmt.Errorf("clear lockout: %w", err)
	}
	return nil
}

// store is what FailOpen wraps.
type store interface {
	LockedFor(ctx context.Context, key string) (time.Duration, error)
	RecordFailure(ctx context.Context, key string) (time.Duration, error)
	Clear(ctx context.Context, key string) error
}

// primaryTimeout bounds each call to the primary store, so an unresponsive
// Redis delays a login by this much at most before falling back.
const primaryTimeout = 500 * time.Millisecond

// FailOpen uses primary and, when it fails, fallback. It never returns an
// error. degraded is called with true when primary fails and false when it
// answers again, for a metric and an alert.
type FailOpen struct {
	primary, fallback store
	degraded          func(bool)
}

func bounded(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(ctx, primaryTimeout)
}

// NewFailOpen returns a lockout that degrades from primary to fallback.
func NewFailOpen(primary, fallback store, degraded func(bool)) *FailOpen {
	if degraded == nil {
		degraded = func(bool) {}
	}
	return &FailOpen{primary: primary, fallback: fallback, degraded: degraded}
}

// LockedFor is the longer of the two stores' lockouts, so a lockout started
// on this node while Redis was down still holds after it recovers.
func (f *FailOpen) LockedFor(ctx context.Context, key string) (time.Duration, error) {
	local, _ := f.fallback.LockedFor(ctx, key)
	pctx, cancel := bounded(ctx)
	defer cancel()
	d, err := f.primary.LockedFor(pctx, key)
	f.degraded(err != nil)
	return max(d, local), nil
}

// RecordFailure counts in primary, or in fallback when primary fails.
func (f *FailOpen) RecordFailure(ctx context.Context, key string) (time.Duration, error) {
	pctx, cancel := bounded(ctx)
	defer cancel()
	d, err := f.primary.RecordFailure(pctx, key)
	f.degraded(err != nil)
	if err != nil {
		return f.fallback.RecordFailure(ctx, key)
	}
	return d, nil
}

// Clear clears both stores.
func (f *FailOpen) Clear(ctx context.Context, key string) error {
	_ = f.fallback.Clear(ctx, key)
	pctx, cancel := bounded(ctx)
	defer cancel()
	err := f.primary.Clear(pctx, key)
	f.degraded(err != nil)
	return nil
}
