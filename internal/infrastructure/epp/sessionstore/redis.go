// Package sessionstore keeps the registry of live EPP sessions in Redis
// (docs/epp/ARCHITECTURE.md §3).
//
// Each session is a key holding its JSON record, with the lease as its TTL.
// Sorted sets index the sessions by source address, registrar and login, and
// one indexes them all; each member's score is its lease expiry in Unix
// milliseconds. Counting a cap prunes the expired members first, so counts
// come from live leases, not from counters that can drift. The scripts touch
// several keys at once, so the registry needs a single Redis (or one hash
// slot), not a cluster.
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
func sourceKey(ip string) string      { return "epp:sessions:ip:" + ip }
func registrarKey(clid string) string { return "epp:sessions:clid:" + clid }
func loginKey(login string) string    { return "epp:sessions:login:" + login }

// Redis is a repositories.EPPSessionRegistry in Redis.
type Redis struct {
	client *redis.Client
	now    func() time.Time
}

// NewRedis returns a registry in client's Redis.
func NewRedis(client *redis.Client) *Redis {
	return &Redis{client: client, now: time.Now}
}

func unavailable(err error) error {
	return fmt.Errorf("%w: %w", entities.ErrEPPSessionsUnavailable, err)
}

// open takes a slot in the source index if it is below the cap, and records
// the session. A session already in the index keeps its slot.
//
// KEYS: source index, all index, session key.
// ARGV: id, now ms, expiry ms, cap, record, lease ms.
var open = redis.NewScript(`
redis.call('ZREMRANGEBYSCORE', KEYS[1], '-inf', ARGV[2])
if not redis.call('ZSCORE', KEYS[1], ARGV[1]) and redis.call('ZCARD', KEYS[1]) >= tonumber(ARGV[4]) then
  return 0
end
redis.call('ZADD', KEYS[1], ARGV[3], ARGV[1])
redis.call('PEXPIRE', KEYS[1], ARGV[6])
redis.call('ZADD', KEYS[2], ARGV[3], ARGV[1])
redis.call('SET', KEYS[3], ARGV[5], 'PX', ARGV[6])
return 1
`)

// authenticate takes a slot in the registrar and login indexes if both are
// below their caps, and rewrites the record. It returns 1, or -1 at the
// registrar cap, or -2 at the login cap.
//
// KEYS: registrar index, login index, session key.
// ARGV: id, now ms, expiry ms, registrar cap, login cap, record, lease ms.
var authenticate = redis.NewScript(`
redis.call('ZREMRANGEBYSCORE', KEYS[1], '-inf', ARGV[2])
redis.call('ZREMRANGEBYSCORE', KEYS[2], '-inf', ARGV[2])
if not redis.call('ZSCORE', KEYS[1], ARGV[1]) and redis.call('ZCARD', KEYS[1]) >= tonumber(ARGV[4]) then
  return -1
end
if not redis.call('ZSCORE', KEYS[2], ARGV[1]) and redis.call('ZCARD', KEYS[2]) >= tonumber(ARGV[5]) then
  return -2
end
redis.call('ZADD', KEYS[1], ARGV[3], ARGV[1])
redis.call('PEXPIRE', KEYS[1], ARGV[7])
redis.call('ZADD', KEYS[2], ARGV[3], ARGV[1])
redis.call('PEXPIRE', KEYS[2], ARGV[7])
redis.call('SET', KEYS[3], ARGV[6], 'PX', ARGV[7])
return 1
`)

func (r *Redis) times(lease time.Duration) (now, expiry int64) {
	t := r.now()
	return t.UnixMilli(), t.Add(lease).UnixMilli()
}

// Open registers s under its source address's cap.
func (r *Redis) Open(ctx context.Context, s *entities.EPPSession, lease time.Duration, perSource int) error {
	rec, err := json.Marshal(s)
	if err != nil {
		return fmt.Errorf("encode EPP session %s: %w", s.ID, err)
	}
	now, expiry := r.times(lease)
	ok, err := open.Run(ctx, r.client,
		[]string{sourceKey(s.ClientIP.String()), allKey, sessionKey(s.ID)},
		s.ID, now, expiry, perSource, rec, lease.Milliseconds()).Int()
	if err != nil {
		return unavailable(err)
	}
	if ok == 0 {
		return &entities.EPPSessionLimitError{Limit: entities.EPPSessionLimitSource, Cap: perSource}
	}
	return nil
}

// Authenticate records s's login under its registrar's and login's caps.
func (r *Redis) Authenticate(ctx context.Context, s *entities.EPPSession, lease time.Duration, perRegistrar, perLogin int) error {
	rec, err := json.Marshal(s)
	if err != nil {
		return fmt.Errorf("encode EPP session %s: %w", s.ID, err)
	}
	now, expiry := r.times(lease)
	res, err := authenticate.Run(ctx, r.client,
		[]string{registrarKey(string(s.ClID)), loginKey(string(s.LoginID)), sessionKey(s.ID)},
		s.ID, now, expiry, perRegistrar, perLogin, rec, lease.Milliseconds()).Int()
	if err != nil {
		return unavailable(err)
	}
	switch res {
	case -1:
		return &entities.EPPSessionLimitError{Limit: entities.EPPSessionLimitRegistrar, Cap: perRegistrar}
	case -2:
		return &entities.EPPSessionLimitError{Limit: entities.EPPSessionLimitLogin, Cap: perLogin}
	}
	return nil
}

// indexes are the index keys a session belongs to.
func indexes(s *entities.EPPSession) []string {
	keys := []string{allKey, sourceKey(s.ClientIP.String())}
	if s.Authenticated() {
		keys = append(keys, registrarKey(string(s.ClID)), loginKey(string(s.LoginID)))
	}
	return keys
}

// Renew extends every session's lease and rewrites its record, in one
// round trip. It also prunes expired sessions from the all index, which
// nothing else counts.
func (r *Redis) Renew(ctx context.Context, sessions []*entities.EPPSession, lease time.Duration) error {
	now, expiry := r.times(lease)
	pipe := r.client.Pipeline()
	pipe.ZRemRangeByScore(ctx, allKey, "-inf", strconv.FormatInt(now, 10))
	for _, s := range sessions {
		rec, err := json.Marshal(s)
		if err != nil {
			return fmt.Errorf("encode EPP session %s: %w", s.ID, err)
		}
		pipe.Set(ctx, sessionKey(s.ID), rec, lease)
		for _, k := range indexes(s) {
			pipe.ZAdd(ctx, k, redis.Z{Score: float64(expiry), Member: s.ID})
			if k != allKey {
				pipe.PExpire(ctx, k, lease)
			}
		}
	}
	if _, err := pipe.Exec(ctx); err != nil {
		return unavailable(err)
	}
	return nil
}

// Close removes s.
func (r *Redis) Close(ctx context.Context, s *entities.EPPSession) error {
	pipe := r.client.Pipeline()
	pipe.Del(ctx, sessionKey(s.ID))
	for _, k := range indexes(s) {
		pipe.ZRem(ctx, k, s.ID)
	}
	if _, err := pipe.Exec(ctx); err != nil {
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
	now := strconv.FormatInt(r.now().UnixMilli(), 10)
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
