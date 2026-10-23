// Package sessionstore keeps the registry of live EPP sessions in Redis
// (docs/epp/ARCHITECTURE.md §3).
//
// Each session is a key holding its JSON record, with the lease as its TTL.
// Sorted sets index the sessions by source address, registrar and login, and
// one indexes them all; each member's score is its lease expiry in Unix
// milliseconds. Counting a cap prunes the expired members first, so counts
// come from live leases, not from counters that can drift.
//
// Every lease time comes from Redis's own clock (TIME, inside the scripts),
// never a node's, so clock skew between nodes cannot expire a live session
// early or keep a dead one counted. A closed session leaves a tombstone, so a
// heartbeat that read it just before it closed cannot bring it back.
//
// The scripts touch several keys at once, some named inside the script, so
// the registry needs a single Redis, not a cluster.
package sessionstore

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
	"github.com/onasunnymorning/domain-os/pkg/domain/queries"
)

const (
	keyPrefix   = "epp:session:"
	allKey      = "epp:sessions:all"
	killChannel = "epp:sessions:kill"
)

func sessionKey(id string) string     { return keyPrefix + id }
func tombstoneKey(id string) string   { return "epp:session-closed:" + id }
func sourceKey(ip string) string      { return "epp:sessions:ip:" + ip }
func registrarKey(clid string) string { return "epp:sessions:clid:" + clid }
func loginKey(login string) string    { return "epp:sessions:login:" + login }
func familyKey(name string) string    { return "epp:sessions:family:" + name }

// Redis is a repositories.EPPSessionRegistry in Redis.
type Redis struct {
	client *redis.Client
}

// NewRedis returns a registry in client's Redis.
func NewRedis(client *redis.Client) *Redis {
	return &Redis{client: client}
}

// clock is the preamble every script starts with: now and the lease expiry,
// in milliseconds of Redis's clock. It expects the lease in ms in ARGV[2].
const clock = `
local t = redis.call('TIME')
local now = tonumber(t[1]) * 1000 + math.floor(tonumber(t[2]) / 1000)
local lease = tonumber(ARGV[2])
local exp = now + lease
`

func unavailable(err error) error {
	return fmt.Errorf("%w: %w", entities.ErrEPPSessionsUnavailable, err)
}

// open takes a slot in the source index if it is below the cap, and records
// the session. A session already in the index keeps its slot.
//
// KEYS: source index, all index, session key.
// ARGV: id, lease ms, cap, record.
var open = redis.NewScript(clock + `
redis.call('ZREMRANGEBYSCORE', KEYS[1], '-inf', now)
if not redis.call('ZSCORE', KEYS[1], ARGV[1]) and redis.call('ZCARD', KEYS[1]) >= tonumber(ARGV[3]) then
  return 0
end
redis.call('ZADD', KEYS[1], exp, ARGV[1])
redis.call('PEXPIRE', KEYS[1], lease)
redis.call('ZADD', KEYS[2], exp, ARGV[1])
redis.call('SET', KEYS[3], ARGV[4], 'PX', lease)
return 1
`)

// authenticate takes a slot in the registrar, login and family indexes if
// each is below its cap, and rewrites the record. It returns 1, or -1 at the
// registrar cap, -2 at the login cap, or -3 at the family cap. A family cap
// of 0 means the session is in no family, and its index is left alone.
//
// KEYS: registrar index, login index, family index, session key.
// ARGV: id, lease ms, registrar cap, login cap, family cap, record.
var authenticate = redis.NewScript(clock + `
local family = tonumber(ARGV[5]) > 0
local n = 2
if family then n = 3 end
for i = 1, n do redis.call('ZREMRANGEBYSCORE', KEYS[i], '-inf', now) end
if not redis.call('ZSCORE', KEYS[1], ARGV[1]) and redis.call('ZCARD', KEYS[1]) >= tonumber(ARGV[3]) then
  return -1
end
if not redis.call('ZSCORE', KEYS[2], ARGV[1]) and redis.call('ZCARD', KEYS[2]) >= tonumber(ARGV[4]) then
  return -2
end
if family and not redis.call('ZSCORE', KEYS[3], ARGV[1]) and redis.call('ZCARD', KEYS[3]) >= tonumber(ARGV[5]) then
  return -3
end
for i = 1, n do
  redis.call('ZADD', KEYS[i], exp, ARGV[1])
  redis.call('PEXPIRE', KEYS[i], lease)
end
redis.call('SET', KEYS[4], ARGV[6], 'PX', lease)
return 1
`)

// renew extends one session's lease and rewrites its record, unless the
// session has closed. A session the registry lost is put back.
//
// KEYS: session key, tombstone, all index, then the session's other indexes.
// ARGV: id, lease ms, record.
var renew = redis.NewScript(clock + `
if redis.call('EXISTS', KEYS[2]) == 1 then return 0 end
redis.call('SET', KEYS[1], ARGV[3], 'PX', lease)
redis.call('ZREMRANGEBYSCORE', KEYS[3], '-inf', now)
redis.call('ZADD', KEYS[3], exp, ARGV[1])
for i = 4, #KEYS do
  redis.call('ZADD', KEYS[i], exp, ARGV[1])
  redis.call('PEXPIRE', KEYS[i], lease)
end
return 1
`)

// closeSession removes a session from every index its stored record names
// and every index the caller names, deletes the record and leaves a
// tombstone.
//
// KEYS: session key, tombstone, all index, then the caller's indexes.
// ARGV: id, tombstone ms, source prefix, registrar prefix, login prefix,
// family prefix.
var closeSession = redis.NewScript(clock + `
local rec = redis.call('GET', KEYS[1])
if rec then
  local ok, s = pcall(cjson.decode, rec)
  if ok and type(s) == 'table' then
    if type(s.client_ip) == 'string' then redis.call('ZREM', ARGV[3] .. s.client_ip, ARGV[1]) end
    if type(s.clid) == 'string' then redis.call('ZREM', ARGV[4] .. s.clid, ARGV[1]) end
    if type(s.login_id) == 'string' then redis.call('ZREM', ARGV[5] .. s.login_id, ARGV[1]) end
    if type(s.family) == 'string' then redis.call('ZREM', ARGV[6] .. s.family, ARGV[1]) end
  end
end
for i = 3, #KEYS do redis.call('ZREM', KEYS[i], ARGV[1]) end
redis.call('DEL', KEYS[1])
redis.call('SET', KEYS[2], '1', 'PX', lease)
return 1
`)

// Open registers s under its source address's cap.
func (r *Redis) Open(ctx context.Context, s *entities.EPPSession, lease time.Duration, perSource int) error {
	rec, err := json.Marshal(s)
	if err != nil {
		return fmt.Errorf("encode EPP session %s: %w", s.ID, err)
	}
	ok, err := open.Run(ctx, r.client,
		[]string{sourceKey(s.ClientIP.String()), allKey, sessionKey(s.ID)},
		s.ID, lease.Milliseconds(), perSource, rec).Int()
	if err != nil {
		return unavailable(err)
	}
	if ok == 0 {
		return &entities.EPPSessionLimitError{Limit: entities.EPPSessionLimitSource, Cap: perSource}
	}
	return nil
}

// Authenticate records s's login under its registrar's, login's and
// family's caps.
func (r *Redis) Authenticate(ctx context.Context, s *entities.EPPSession, lease time.Duration, limits entities.EPPSessionLimits) error {
	rec, err := json.Marshal(s)
	if err != nil {
		return fmt.Errorf("encode EPP session %s: %w", s.ID, err)
	}
	perFamily := 0
	if s.Family != "" {
		perFamily = limits.PerFamily
	}
	res, err := authenticate.Run(ctx, r.client,
		[]string{registrarKey(string(s.ClID)), loginKey(string(s.LoginID)), familyKey(s.Family), sessionKey(s.ID)},
		s.ID, lease.Milliseconds(), limits.PerRegistrar, limits.PerLogin, perFamily, rec).Int()
	if err != nil {
		return unavailable(err)
	}
	switch res {
	case -1:
		return &entities.EPPSessionLimitError{Limit: entities.EPPSessionLimitRegistrar, Cap: limits.PerRegistrar}
	case -2:
		return &entities.EPPSessionLimitError{Limit: entities.EPPSessionLimitLogin, Cap: limits.PerLogin}
	case -3:
		return &entities.EPPSessionLimitError{Limit: entities.EPPSessionLimitFamily, Cap: perFamily}
	}
	return nil
}

// indexes are the indexes a session belongs to, other than the all index.
func indexes(s *entities.EPPSession) []string {
	keys := []string{sourceKey(s.ClientIP.String())}
	if s.Authenticated() {
		keys = append(keys, registrarKey(string(s.ClID)), loginKey(string(s.LoginID)))
	}
	if s.Family != "" {
		keys = append(keys, familyKey(s.Family))
	}
	return keys
}

// Renew extends every session's lease and rewrites its record, in one round
// trip, skipping sessions that have closed meanwhile.
func (r *Redis) Renew(ctx context.Context, sessions []*entities.EPPSession, lease time.Duration) error {
	if len(sessions) == 0 {
		return nil
	}
	// EvalSha in a pipeline cannot fall back to EVAL, so make sure the
	// script is loaded first. SCRIPT LOAD is idempotent and cheap.
	if err := renew.Load(ctx, r.client).Err(); err != nil {
		return unavailable(err)
	}
	pipe := r.client.Pipeline()
	for _, s := range sessions {
		rec, err := json.Marshal(s)
		if err != nil {
			return fmt.Errorf("encode EPP session %s: %w", s.ID, err)
		}
		keys := append([]string{sessionKey(s.ID), tombstoneKey(s.ID), allKey}, indexes(s)...)
		renew.EvalSha(ctx, pipe, keys, s.ID, lease.Milliseconds(), rec)
	}
	if _, err := pipe.Exec(ctx); err != nil {
		return unavailable(err)
	}
	return nil
}

// tombstoneTTL is how long a closed session cannot be renewed. A heartbeat's
// snapshot is never older than one lease, and leases are minutes.
const tombstoneTTL = 10 * time.Minute

// Close removes s from every index, whatever the caller's copy of it says,
// and keeps it from being renewed.
func (r *Redis) Close(ctx context.Context, s *entities.EPPSession) error {
	keys := append([]string{sessionKey(s.ID), tombstoneKey(s.ID), allKey}, indexes(s)...)
	err := closeSession.Run(ctx, r.client, keys, s.ID, tombstoneTTL.Milliseconds(),
		sourceKey(""), registrarKey(""), loginKey(""), familyKey("")).Err()
	if err != nil {
		return unavailable(err)
	}
	return nil
}

// Get returns a live session.
func (r *Redis) Get(ctx context.Context, id string) (*entities.EPPSession, error) {
	raw, err := r.client.Get(ctx, sessionKey(id)).Bytes()
	if errors.Is(err, redis.Nil) {
		return nil, entities.ErrEPPSessionNotFound
	}
	if err != nil {
		return nil, unavailable(err)
	}
	var s entities.EPPSession
	if err := json.Unmarshal(raw, &s); err != nil {
		return nil, fmt.Errorf("decode EPP session %s: %w", id, err)
	}
	return &s, nil
}

// listBatch is how many records List fetches per round trip.
const listBatch = 200

// List returns a page of the live sessions matching the filter, by ID.
func (r *Redis) List(ctx context.Context, q queries.ListItemsQuery) ([]*entities.EPPSession, string, error) {
	var filter queries.ListEPPSessionsFilter
	if f, ok := q.Filter.(queries.ListEPPSessionsFilter); ok {
		filter = f
	}
	// Read the narrowest index the filter allows.
	index := allKey
	switch {
	case filter.LoginID != "":
		index = loginKey(filter.LoginID)
	case filter.ClID != "":
		index = registrarKey(filter.ClID)
	case filter.ClientIP != "":
		index = sourceKey(filter.ClientIP)
	}
	// Redis's clock, as the scripts use, not this process's.
	t, err := r.client.Time(ctx).Result()
	if err != nil {
		return nil, "", unavailable(err)
	}
	now := strconv.FormatInt(t.UnixMilli(), 10)
	ids, err := r.client.ZRangeByScore(ctx, index, &redis.ZRangeBy{Min: "(" + now, Max: "+inf"}).Result()
	if err != nil {
		return nil, "", unavailable(err)
	}
	slices.Sort(ids)
	if q.PageCursor != "" {
		i, _ := slices.BinarySearch(ids, q.PageCursor)
		for i < len(ids) && ids[i] <= q.PageCursor {
			i++
		}
		ids = ids[i:]
	}
	pageSize := q.PageSize
	if pageSize <= 0 {
		pageSize = len(ids)
	}

	var page []*entities.EPPSession
	for start := 0; start < len(ids); start += listBatch {
		batch := ids[start:min(start+listBatch, len(ids))]
		keys := make([]string, len(batch))
		for i, id := range batch {
			keys[i] = sessionKey(id)
		}
		raws, err := r.client.MGet(ctx, keys...).Result()
		if err != nil {
			return nil, "", unavailable(err)
		}
		for _, raw := range raws {
			str, ok := raw.(string)
			if !ok {
				continue // expired since the index was read
			}
			var s entities.EPPSession
			if err := json.Unmarshal([]byte(str), &s); err != nil {
				return nil, "", fmt.Errorf("decode EPP session: %w", err)
			}
			if !filter.Matches(&s) {
				continue
			}
			if len(page) == pageSize {
				return page, page[len(page)-1].ID, nil
			}
			page = append(page, &s)
		}
	}
	return page, "", nil
}

// RequestKill publishes the IDs; every node receives them and ends the ones
// it holds.
func (r *Redis) RequestKill(ctx context.Context, ids ...string) error {
	pipe := r.client.Pipeline()
	for _, id := range ids {
		pipe.Publish(ctx, killChannel, id)
	}
	if _, err := pipe.Exec(ctx); err != nil {
		return unavailable(err)
	}
	return nil
}

// KillRequests subscribes to kill requests until ctx is done. The
// subscription reconnects on its own after a Redis outage; requests
// published while it was down are lost, so a kill is best effort and the
// admin API says so.
func (r *Redis) KillRequests(ctx context.Context) (<-chan string, error) {
	sub := r.client.Subscribe(ctx, killChannel)
	if _, err := sub.Receive(ctx); err != nil {
		_ = sub.Close()
		return nil, unavailable(err)
	}
	out := make(chan string)
	go func() {
		defer close(out)
		defer sub.Close()
		msgs := sub.Channel()
		for {
			select {
			case <-ctx.Done():
				return
			case m, ok := <-msgs:
				if !ok {
					return
				}
				select {
				case out <- m.Payload:
				case <-ctx.Done():
					return
				}
			}
		}
	}()
	return out, nil
}
