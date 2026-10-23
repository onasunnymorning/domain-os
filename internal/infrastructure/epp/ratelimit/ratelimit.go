// Package ratelimit keeps the EPP command rate buckets and daily create
// quotas (docs/epp/ARCHITECTURE.md §5, docs/epp/POLICY_REFERENCE.md
// § Command rates).
//
// Buckets are GCRA (the generic cell rate algorithm): each bucket is one
// number, its theoretical arrival time (TAT), the time at which it would be
// full again. Taking n tokens moves the TAT n emission intervals on; the
// tokens can be had once the TAT is no more than a burst of intervals
// ahead of now. One number per bucket, updated atomically, so there is no
// refill loop and nothing to drift.
//
// Redis keeps the buckets and quotas across nodes, its scripts using
// Redis's clock, never a node's. When Redis cannot answer, FailOpen falls
// back to buckets kept on the node (ADR 0013).
package ratelimit

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
)

// interval is a rate's emission interval in microseconds: the time one
// token takes to come back.
func interval(r entities.EPPRate) int64 {
	return int64(math.Ceil(1e6 / float64(r.PerSecond)))
}

// take is GCRA over several buckets at once, at now (µs). tats are the
// buckets' TATs (0 for a bucket not yet used). It returns the new TATs and
// the answer; the caller stores the TATs only if the tokens were taken.
//
// The command runs at one time for every bucket: the earliest at which all
// of them conform, each bucket's being its TAT less what its burst allows
// ahead. Each bucket is then charged from that time, not from now, so a
// bucket that was not the one waited for cannot let another command
// through before this one runs.
func take(now int64, tats []int64, buckets []entities.EPPRateBucket, cost int, maxWait time.Duration) ([]int64, entities.EPPRateTake) {
	var res entities.EPPRateTake
	at := now
	for i, b := range buckets {
		iv := interval(b.Rate)
		if t := tats[i] + int64(cost-b.Rate.Burst)*iv; t > at {
			at, res.Limiting = t, i
		}
	}
	out := make([]int64, len(buckets))
	for i, b := range buckets {
		out[i] = max(tats[i], at) + int64(cost)*interval(b.Rate)
	}
	res.Wait = time.Duration(at-now) * time.Microsecond
	res.Taken = res.Wait <= maxWait
	return out, res
}

// ---------------------------------------------------------------------------
// Redis
// ---------------------------------------------------------------------------

// Redis keeps buckets and quotas in Redis, shared by every node. The
// scripts touch several keys at once, so it needs a single Redis, not a
// cluster, like the session registry.
type Redis struct{ client *redis.Client }

// NewRedis returns a limiter in client's Redis.
func NewRedis(client *redis.Client) *Redis { return &Redis{client: client} }

func bucketKey(k string) string { return "epp:rate:" + k }
func quotaKey(k string) string  { return "epp:quota:" + k }

// takeScript is take, in Redis. The TATs are stored as whole microseconds
// (formatted with %.0f: Lua's default formatting of a number this large
// loses digits), each expiring when its bucket would be full again.
//
// KEYS: the buckets. ARGV: cost, max wait µs, then each bucket's interval
// µs and burst. Returns {taken 0/1, wait µs, limiting bucket from 1}.
var takeScript = redis.NewScript(`
local t = redis.call('TIME')
local now = tonumber(t[1]) * 1000000 + tonumber(t[2])
local cost = tonumber(ARGV[1])
local maxwait = tonumber(ARGV[2])
local at, limiting = now, 1
local tats = {}
for i = 1, #KEYS do
  tats[i] = tonumber(redis.call('GET', KEYS[i]) or 0)
  local due = tats[i] + (cost - tonumber(ARGV[2 + 2 * i])) * tonumber(ARGV[1 + 2 * i])
  if due > at then at, limiting = due, i end
end
local wait = at - now
if wait > maxwait then return {0, wait, limiting} end
for i = 1, #KEYS do
  local tat = tats[i]
  if tat < at then tat = at end
  tat = tat + cost * tonumber(ARGV[1 + 2 * i])
  redis.call('SET', KEYS[i], string.format('%.0f', tat), 'PX', math.ceil((tat - now) / 1000) + 1)
end
return {1, wait, limiting}
`)

// Take implements interfaces.EPPRateLimiter.
func (r *Redis) Take(ctx context.Context, buckets []entities.EPPRateBucket, cost int, maxWait time.Duration) (entities.EPPRateTake, error) {
	keys := make([]string, len(buckets))
	args := []any{cost, maxWait.Microseconds()}
	for i, b := range buckets {
		keys[i] = bucketKey(b.Key)
		args = append(args, interval(b.Rate), b.Rate.Burst)
	}
	v, err := takeScript.Run(ctx, r.client, keys, args...).Int64Slice()
	if err != nil {
		return entities.EPPRateTake{}, fmt.Errorf("take rate tokens: %w", err)
	}
	if len(v) != 3 {
		return entities.EPPRateTake{}, fmt.Errorf("take rate tokens: unexpected reply %v", v)
	}
	return entities.EPPRateTake{Taken: v[0] == 1, Wait: time.Duration(v[1]) * time.Microsecond, Limiting: int(v[2]) - 1}, nil
}

// A quota is a set of reservation IDs, one per create counted, so that
// reserving and releasing are idempotent: the client retries a command
// whose reply was lost, and a retried reservation must not take a second
// slot, nor a retried release free one that is not its own.

// reserveScript adds a reservation if it is there already or the quota has
// room. KEYS: the quota. ARGV: reservation ID, limit, ttl ms. Returns 1 if
// reserved.
var reserveScript = redis.NewScript(`
if redis.call('SISMEMBER', KEYS[1], ARGV[1]) == 1 then return 1 end
if redis.call('SCARD', KEYS[1]) >= tonumber(ARGV[2]) then return 0 end
redis.call('SADD', KEYS[1], ARGV[1])
if redis.call('PTTL', KEYS[1]) < 0 then redis.call('PEXPIRE', KEYS[1], ARGV[3]) end
return 1
`)

// ReserveQuota implements interfaces.EPPRateLimiter.
func (r *Redis) ReserveQuota(ctx context.Context, key string, limit int, ttl time.Duration) (func(context.Context), bool, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return nil, false, fmt.Errorf("reserve quota: %w", err)
	}
	return r.reserve(ctx, key, hex.EncodeToString(b[:]), limit, ttl)
}

// reserve reserves id against key.
func (r *Redis) reserve(ctx context.Context, key, id string, limit int, ttl time.Duration) (func(context.Context), bool, error) {
	k := quotaKey(key)
	n, err := reserveScript.Run(ctx, r.client, []string{k}, id, limit, ttl.Milliseconds()).Int64()
	if err != nil {
		return nil, false, fmt.Errorf("reserve quota: %w", err)
	}
	if n != 1 {
		return nil, false, nil
	}
	return func(ctx context.Context) { _ = r.client.SRem(ctx, k, id).Err() }, true, nil
}

// QuotaCount is how much of a quota is used, for tests and diagnostics.
func (r *Redis) QuotaCount(ctx context.Context, key string) (int, error) {
	n, err := r.client.SCard(ctx, quotaKey(key)).Result()
	return int(n), err
}

// ---------------------------------------------------------------------------
// Memory
// ---------------------------------------------------------------------------

// maxMemoryKeys bounds the node-local tables. Buckets are per registrar or
// family and class, so a node holds far fewer; past the bound, keys whose
// time is up are dropped, and if none is, a new key is let through
// uncounted rather than the table grow without bound.
var maxMemoryKeys = 100_000

type quotaEntry struct {
	n       int
	expires time.Time
}

// Memory keeps buckets and quotas on this node only.
type Memory struct {
	now    func() time.Time
	mu     sync.Mutex
	tats   map[string]int64
	quotas map[string]*quotaEntry
}

// NewMemory returns a node-local limiter.
func NewMemory() *Memory {
	return &Memory{now: time.Now, tats: map[string]int64{}, quotas: map[string]*quotaEntry{}}
}

// Take implements interfaces.EPPRateLimiter.
func (m *Memory) Take(_ context.Context, buckets []entities.EPPRateBucket, cost int, maxWait time.Duration) (entities.EPPRateTake, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := m.now().UnixMicro()
	tats := make([]int64, len(buckets))
	for i, b := range buckets {
		tats[i] = m.tats[b.Key]
	}
	next, res := take(now, tats, buckets, cost, maxWait)
	if !res.Taken {
		return res, nil
	}
	for i, b := range buckets {
		if _, ok := m.tats[b.Key]; !ok && !m.bucketRoom(now) {
			continue
		}
		m.tats[b.Key] = next[i]
	}
	return res, nil
}

// bucketRoom reports whether a new bucket fits, dropping full buckets (TAT
// passed) when the table is at its bound.
func (m *Memory) bucketRoom(now int64) bool {
	if len(m.tats) < maxMemoryKeys {
		return true
	}
	for k, tat := range m.tats {
		if tat <= now {
			delete(m.tats, k)
		}
	}
	return len(m.tats) < maxMemoryKeys
}

// ReserveQuota implements interfaces.EPPRateLimiter.
func (m *Memory) ReserveQuota(_ context.Context, key string, limit int, ttl time.Duration) (func(context.Context), bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := m.now()
	e := m.quotas[key]
	if e != nil && !now.Before(e.expires) {
		delete(m.quotas, key)
		e = nil
	}
	if e == nil {
		if !m.quotaRoom(now) {
			return func(context.Context) {}, true, nil
		}
		e = &quotaEntry{expires: now.Add(ttl)}
		m.quotas[key] = e
	}
	if e.n >= limit {
		return nil, false, nil
	}
	e.n++
	return func(context.Context) {
		m.mu.Lock()
		defer m.mu.Unlock()
		if e.n > 0 {
			e.n--
		}
	}, true, nil
}

func (m *Memory) quotaRoom(now time.Time) bool {
	if len(m.quotas) < maxMemoryKeys {
		return true
	}
	for k, e := range m.quotas {
		if !now.Before(e.expires) {
			delete(m.quotas, k)
		}
	}
	return len(m.quotas) < maxMemoryKeys
}

// ---------------------------------------------------------------------------
// FailOpen
// ---------------------------------------------------------------------------

type limiter interface {
	Take(ctx context.Context, buckets []entities.EPPRateBucket, cost int, maxWait time.Duration) (entities.EPPRateTake, error)
	ReserveQuota(ctx context.Context, key string, limit int, ttl time.Duration) (func(context.Context), bool, error)
}

const (
	// primaryTimeout bounds each call to the primary store. Every command
	// asks it, so the bound is tighter than the session registry's.
	primaryTimeout = 200 * time.Millisecond
	// retryPrimary is how long, after the primary fails, commands go
	// straight to the fallback before the primary is tried again, so an
	// unresponsive Redis does not slow every command by primaryTimeout.
	retryPrimary = 5 * time.Second
)

// FailOpen uses primary and, when it fails, fallback, for retryPrimary
// before trying primary again. It never returns an error. degraded is
// called with true when primary fails and false when it answers again, for
// a metric and an alert.
type FailOpen struct {
	primary, fallback limiter
	degraded          func(bool)
	now               func() time.Time
	mu                sync.Mutex
	skipUntil         time.Time
}

// NewFailOpen returns a limiter that degrades from primary to fallback.
func NewFailOpen(primary, fallback limiter, degraded func(bool)) *FailOpen {
	if degraded == nil {
		degraded = func(bool) {}
	}
	return &FailOpen{primary: primary, fallback: fallback, degraded: degraded, now: time.Now}
}

// usePrimary reports whether to try the primary now.
func (f *FailOpen) usePrimary() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return !f.now().Before(f.skipUntil)
}

// result records how the primary answered.
func (f *FailOpen) result(err error) {
	f.mu.Lock()
	if err != nil {
		f.skipUntil = f.now().Add(retryPrimary)
	} else {
		f.skipUntil = time.Time{}
	}
	f.mu.Unlock()
	f.degraded(err != nil)
}

// Take implements interfaces.EPPRateLimiter.
func (f *FailOpen) Take(ctx context.Context, buckets []entities.EPPRateBucket, cost int, maxWait time.Duration) (entities.EPPRateTake, error) {
	if f.usePrimary() {
		pctx, cancel := context.WithTimeout(ctx, primaryTimeout)
		res, err := f.primary.Take(pctx, buckets, cost, maxWait)
		cancel()
		f.result(err)
		if err == nil {
			return res, nil
		}
	}
	res, _ := f.fallback.Take(ctx, buckets, cost, maxWait)
	return res, nil
}

// bounded is release with each call bounded to primaryTimeout, so a stalled
// primary cannot hold up the command that gives its create back.
func bounded(release func(context.Context)) func(context.Context) {
	if release == nil {
		return nil
	}
	return func(ctx context.Context) {
		ctx, cancel := context.WithTimeout(ctx, primaryTimeout)
		defer cancel()
		release(ctx)
	}
}

// ReserveQuota implements interfaces.EPPRateLimiter. A reservation is
// released in the store that made it.
func (f *FailOpen) ReserveQuota(ctx context.Context, key string, limit int, ttl time.Duration) (func(context.Context), bool, error) {
	if f.usePrimary() {
		pctx, cancel := context.WithTimeout(ctx, primaryTimeout)
		release, ok, err := f.primary.ReserveQuota(pctx, key, limit, ttl)
		cancel()
		f.result(err)
		if err == nil {
			return bounded(release), ok, nil
		}
	}
	release, ok, _ := f.fallback.ReserveQuota(ctx, key, limit, ttl)
	return release, ok, nil
}
