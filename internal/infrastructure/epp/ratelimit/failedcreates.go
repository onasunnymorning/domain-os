package ratelimit

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
)

// The failed-create budget's counts (interfaces.EPPFailedCreateTracker).
//
// Creates are counted per scope (a registrar or a family) in one-minute
// slots, each holding how many creates failed (2302) and succeeded; the
// window is the current slot and the four before it. A registrar's strikes
// count its creates sent while over budget and expire a window after the
// last; a login's suspension is a key that expires when it ends.

// windowSlots is the number of one-minute slots in the window.
const windowSlots = int64(entities.EPPFailedCreateWindow / time.Minute)

func slotOf(t time.Time) int64 { return t.Unix() / 60 }

func suspensionKey(login string) string { return "epp:suspended:" + login }
func strikeKey(k string) string         { return "epp:fc-strikes:" + k }

// ---------------------------------------------------------------------------
// Redis
// ---------------------------------------------------------------------------

// assessScript is Assess. Slot keys (epp:fc:<scope>:<slot>, hashes of f
// and s) are named inside the script, so, like the rest of this package,
// it needs a single Redis, not a cluster.
//
// KEYS: the suspension, the strikes. ARGV: suspend after, suspend for ms,
// window ms, slots, scope count, then per scope its key, failures and
// ratio. Returns {suspended ms, newly suspended 0/1, strikes, over scope
// from 1}.
var assessScript = redis.NewScript(`
local suspended = redis.call('PTTL', KEYS[1])
if suspended > 0 then return {suspended, 0, 0, 0} end
local t = redis.call('TIME')
local slot = math.floor(tonumber(t[1]) / 60)
local slots = tonumber(ARGV[4])
local over = 0
for i = 1, tonumber(ARGV[5]) do
  local base = 5 + (i - 1) * 3
  local f, ok = 0, 0
  for m = slot - slots + 1, slot do
    local v = redis.call('HMGET', 'epp:fc:' .. ARGV[base + 1] .. ':' .. m, 'f', 's')
    f = f + tonumber(v[1] or 0)
    ok = ok + tonumber(v[2] or 0)
  end
  if over == 0 and f >= tonumber(ARGV[base + 2]) and f * 100 >= tonumber(ARGV[base + 3]) * (f + ok) then over = i end
end
if over == 0 then return {0, 0, 0, 0} end
local strikes = redis.call('INCR', KEYS[2])
redis.call('PEXPIRE', KEYS[2], ARGV[3])
if strikes > tonumber(ARGV[1]) then
  redis.call('SET', KEYS[1], '1', 'PX', ARGV[2])
  return {tonumber(ARGV[2]), 1, strikes, over}
end
return {0, 0, strikes, over}
`)

// recordScript counts one outcome in each scope's current slot.
// ARGV: field (f or s), slot ttl ms, then the scopes.
var recordScript = redis.NewScript(`
local t = redis.call('TIME')
local slot = math.floor(tonumber(t[1]) / 60)
for i = 3, #ARGV do
  local key = 'epp:fc:' .. ARGV[i] .. ':' .. slot
  redis.call('HINCRBY', key, ARGV[1], 1)
  redis.call('PEXPIRE', key, ARGV[2])
end
return 0
`)

// slotTTL keeps a slot until it has left the window.
var slotTTL = entities.EPPFailedCreateWindow + time.Minute

// Assess implements interfaces.EPPFailedCreateTracker.
func (r *Redis) Assess(ctx context.Context, c entities.EPPFailedCreateCheck) (entities.EPPFailedCreateStanding, error) {
	args := []any{c.SuspendAfter, c.SuspendFor.Milliseconds(), entities.EPPFailedCreateWindow.Milliseconds(), windowSlots, len(c.Scopes)}
	for _, s := range c.Scopes {
		args = append(args, s.Key, s.Budget.Failures, s.Budget.RatioPercent)
	}
	v, err := assessScript.Run(ctx, r.client, []string{suspensionKey(c.Login), strikeKey(c.Strikes)}, args...).Int64Slice()
	if err != nil {
		return entities.EPPFailedCreateStanding{}, fmt.Errorf("assess failed creates: %w", err)
	}
	if len(v) != 4 {
		return entities.EPPFailedCreateStanding{}, fmt.Errorf("assess failed creates: unexpected reply %v", v)
	}
	return entities.EPPFailedCreateStanding{
		Suspended: time.Duration(v[0]) * time.Millisecond, NewlySuspended: v[1] == 1,
		Strikes: int(v[2]), Over: max(int(v[3])-1, 0),
	}, nil
}

// Record implements interfaces.EPPFailedCreateTracker.
func (r *Redis) Record(ctx context.Context, scopes []string, failed bool) error {
	field := "s"
	if failed {
		field = "f"
	}
	args := []any{field, slotTTL.Milliseconds()}
	for _, s := range scopes {
		args = append(args, s)
	}
	if err := recordScript.Run(ctx, r.client, nil, args...).Err(); err != nil {
		return fmt.Errorf("record create outcome: %w", err)
	}
	return nil
}

// Suspended implements interfaces.EPPFailedCreateTracker.
func (r *Redis) Suspended(ctx context.Context, login string) (time.Duration, error) {
	d, err := r.client.PTTL(ctx, suspensionKey(login)).Result()
	if err != nil {
		return 0, fmt.Errorf("login suspension: %w", err)
	}
	return max(d, 0), nil
}

// ---------------------------------------------------------------------------
// Memory
// ---------------------------------------------------------------------------

type outcomes struct{ failed, succeeded int }

type expiring struct {
	n     int
	until time.Time
}

// MemoryFailedCreates keeps the counts on this node only.
type MemoryFailedCreates struct {
	now        func() time.Time
	mu         sync.Mutex
	slots      map[string]map[int64]*outcomes
	strikes    map[string]*expiring
	suspension map[string]time.Time
}

// NewMemoryFailedCreates returns node-local counts.
func NewMemoryFailedCreates() *MemoryFailedCreates {
	return &MemoryFailedCreates{
		now: time.Now, slots: map[string]map[int64]*outcomes{},
		strikes: map[string]*expiring{}, suspension: map[string]time.Time{},
	}
}

// Assess implements interfaces.EPPFailedCreateTracker.
func (m *MemoryFailedCreates) Assess(_ context.Context, c entities.EPPFailedCreateCheck) (entities.EPPFailedCreateStanding, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := m.now()
	if until, ok := m.suspension[c.Login]; ok {
		if now.Before(until) {
			return entities.EPPFailedCreateStanding{Suspended: until.Sub(now)}, nil
		}
		delete(m.suspension, c.Login)
	}
	over := -1
	for i, s := range c.Scopes {
		var f, ok int
		for slot := slotOf(now) - windowSlots + 1; slot <= slotOf(now); slot++ {
			if o := m.slots[s.Key][slot]; o != nil {
				f, ok = f+o.failed, ok+o.succeeded
			}
		}
		if s.Budget.Over(f, ok) {
			over = i
			break
		}
	}
	if over < 0 {
		return entities.EPPFailedCreateStanding{}, nil
	}
	e := m.strikes[c.Strikes]
	if e == nil || !now.Before(e.until) {
		e = &expiring{}
		m.strikes[c.Strikes] = e
	}
	e.n++
	e.until = now.Add(entities.EPPFailedCreateWindow)
	st := entities.EPPFailedCreateStanding{Strikes: e.n, Over: over}
	if e.n > c.SuspendAfter {
		m.suspension[c.Login] = now.Add(c.SuspendFor)
		st.Suspended, st.NewlySuspended = c.SuspendFor, true
	}
	return st, nil
}

// Record implements interfaces.EPPFailedCreateTracker. Slots that have
// left the window are dropped as new ones are written, so the table holds
// at most a window per scope.
func (m *MemoryFailedCreates) Record(_ context.Context, scopes []string, failed bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	slot := slotOf(m.now())
	for _, s := range scopes {
		bySlot := m.slots[s]
		if bySlot == nil {
			bySlot = map[int64]*outcomes{}
			m.slots[s] = bySlot
		}
		for old := range bySlot {
			if old <= slot-windowSlots {
				delete(bySlot, old)
			}
		}
		o := bySlot[slot]
		if o == nil {
			o = &outcomes{}
			bySlot[slot] = o
		}
		if failed {
			o.failed++
		} else {
			o.succeeded++
		}
	}
	return nil
}

// Suspended implements interfaces.EPPFailedCreateTracker.
func (m *MemoryFailedCreates) Suspended(_ context.Context, login string) (time.Duration, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return max(m.suspension[login].Sub(m.now()), 0), nil
}

// ---------------------------------------------------------------------------
// FailOpen
// ---------------------------------------------------------------------------

type failedCreateTracker interface {
	Assess(ctx context.Context, c entities.EPPFailedCreateCheck) (entities.EPPFailedCreateStanding, error)
	Record(ctx context.Context, scopes []string, failed bool) error
	Suspended(ctx context.Context, login string) (time.Duration, error)
}

// FailOpenFailedCreates uses primary and, when it fails, fallback, like
// FailOpen. It never returns an error.
type FailOpenFailedCreates struct {
	breaker
	primary, fallback failedCreateTracker
}

// NewFailOpenFailedCreates returns counts that degrade from primary to
// fallback.
func NewFailOpenFailedCreates(primary, fallback failedCreateTracker, degraded func(bool)) *FailOpenFailedCreates {
	return &FailOpenFailedCreates{breaker: newBreaker(degraded), primary: primary, fallback: fallback}
}

// Assess implements interfaces.EPPFailedCreateTracker. A suspension made
// on the node while Redis was down still holds after it recovers.
func (f *FailOpenFailedCreates) Assess(ctx context.Context, c entities.EPPFailedCreateCheck) (entities.EPPFailedCreateStanding, error) {
	if local, _ := f.fallback.Suspended(ctx, c.Login); local > 0 {
		return entities.EPPFailedCreateStanding{Suspended: local}, nil
	}
	if f.usePrimary() {
		pctx, cancel := context.WithTimeout(ctx, primaryTimeout)
		st, err := f.primary.Assess(pctx, c)
		cancel()
		f.result(err)
		if err == nil {
			return st, nil
		}
	}
	st, _ := f.fallback.Assess(ctx, c)
	return st, nil
}

// Record implements interfaces.EPPFailedCreateTracker.
func (f *FailOpenFailedCreates) Record(ctx context.Context, scopes []string, failed bool) error {
	if f.usePrimary() {
		pctx, cancel := context.WithTimeout(ctx, primaryTimeout)
		err := f.primary.Record(pctx, scopes, failed)
		cancel()
		f.result(err)
		if err == nil {
			return nil
		}
	}
	_ = f.fallback.Record(ctx, scopes, failed)
	return nil
}

// Suspended implements interfaces.EPPFailedCreateTracker: the longer of
// the two stores' suspensions.
func (f *FailOpenFailedCreates) Suspended(ctx context.Context, login string) (time.Duration, error) {
	local, _ := f.fallback.Suspended(ctx, login)
	if f.usePrimary() {
		pctx, cancel := context.WithTimeout(ctx, primaryTimeout)
		d, err := f.primary.Suspended(pctx, login)
		cancel()
		f.result(err)
		if err == nil {
			return max(d, local), nil
		}
	}
	return local, nil
}
