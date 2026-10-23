package memstore

import (
	"context"
	"slices"
	"sync"
	"time"

	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
	"github.com/onasunnymorning/domain-os/pkg/domain/queries"
)

// EPPSessions is an in-memory repositories.EPPSessionRegistry with the same
// lease and cap semantics as the Redis one. Several test servers can share
// one, as the nodes of a cluster share Redis.
type EPPSessions struct {
	mu       sync.Mutex
	now      func() time.Time
	sessions map[string]memSession
	// closed keeps a closed session from being renewed back, as the Redis
	// registry's tombstones do.
	closed map[string]time.Time
	subs   []chan string
	err    error
}

type memSession struct {
	s       entities.EPPSession
	expires time.Time
}

// NewEPPSessions returns an empty registry.
func NewEPPSessions() *EPPSessions {
	return &EPPSessions{now: time.Now, sessions: map[string]memSession{}, closed: map[string]time.Time{}}
}

// Fail makes every method return err wrapped in
// entities.ErrEPPSessionsUnavailable, to simulate an outage; nil ends it.
func (r *EPPSessions) Fail(err error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.err = err
}

func (r *EPPSessions) failure() error {
	if r.err == nil {
		return nil
	}
	return &unavailableError{r.err}
}

type unavailableError struct{ err error }

func (e *unavailableError) Error() string {
	return entities.ErrEPPSessionsUnavailable.Error() + ": " + e.err.Error()
}
func (e *unavailableError) Unwrap() []error {
	return []error{entities.ErrEPPSessionsUnavailable, e.err}
}

// count counts live sessions other than id that match.
func (r *EPPSessions) count(id string, now time.Time, match func(*entities.EPPSession) bool) int {
	n := 0
	for k, m := range r.sessions {
		if k != id && now.Before(m.expires) && match(&m.s) {
			n++
		}
	}
	return n
}

// Open registers s under its source address's cap.
func (r *EPPSessions) Open(_ context.Context, s *entities.EPPSession, lease time.Duration, perSource int) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if err := r.failure(); err != nil {
		return err
	}
	now := r.now()
	if r.count(s.ID, now, func(o *entities.EPPSession) bool { return o.ClientIP == s.ClientIP }) >= perSource {
		return &entities.EPPSessionLimitError{Limit: entities.EPPSessionLimitSource, Cap: perSource}
	}
	r.sessions[s.ID] = memSession{s: *s, expires: now.Add(lease)}
	return nil
}

// Authenticate records s's login under its registrar's, login's and
// family's caps.
func (r *EPPSessions) Authenticate(_ context.Context, s *entities.EPPSession, lease time.Duration, limits entities.EPPSessionLimits) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if err := r.failure(); err != nil {
		return err
	}
	now := r.now()
	if r.count(s.ID, now, func(o *entities.EPPSession) bool { return o.ClID == s.ClID }) >= limits.PerRegistrar {
		return &entities.EPPSessionLimitError{Limit: entities.EPPSessionLimitRegistrar, Cap: limits.PerRegistrar}
	}
	if r.count(s.ID, now, func(o *entities.EPPSession) bool { return o.LoginID == s.LoginID }) >= limits.PerLogin {
		return &entities.EPPSessionLimitError{Limit: entities.EPPSessionLimitLogin, Cap: limits.PerLogin}
	}
	if s.Family != "" && r.count(s.ID, now, func(o *entities.EPPSession) bool { return o.Family == s.Family }) >= limits.PerFamily {
		return &entities.EPPSessionLimitError{Limit: entities.EPPSessionLimitFamily, Cap: limits.PerFamily}
	}
	r.sessions[s.ID] = memSession{s: *s, expires: now.Add(lease)}
	return nil
}

// Renew extends the leases and rewrites the records.
func (r *EPPSessions) Renew(_ context.Context, sessions []*entities.EPPSession, lease time.Duration) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if err := r.failure(); err != nil {
		return err
	}
	now := r.now()
	for _, s := range sessions {
		if until, ok := r.closed[s.ID]; ok && now.Before(until) {
			continue
		}
		r.sessions[s.ID] = memSession{s: *s, expires: now.Add(lease)}
	}
	return nil
}

// Close removes s.
func (r *EPPSessions) Close(_ context.Context, s *entities.EPPSession) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if err := r.failure(); err != nil {
		return err
	}
	delete(r.sessions, s.ID)
	r.closed[s.ID] = r.now().Add(10 * time.Minute)
	return nil
}

// Get returns a live session.
func (r *EPPSessions) Get(_ context.Context, id string) (*entities.EPPSession, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if err := r.failure(); err != nil {
		return nil, err
	}
	m, ok := r.sessions[id]
	if !ok || !r.now().Before(m.expires) {
		return nil, entities.ErrEPPSessionNotFound
	}
	s := m.s
	return &s, nil
}

// List returns a page of the live sessions matching the filter, by ID.
func (r *EPPSessions) List(_ context.Context, q queries.ListItemsQuery) ([]*entities.EPPSession, string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if err := r.failure(); err != nil {
		return nil, "", err
	}
	var filter queries.ListEPPSessionsFilter
	if f, ok := q.Filter.(queries.ListEPPSessionsFilter); ok {
		filter = f
	}
	now := r.now()
	var all []*entities.EPPSession
	for id, m := range r.sessions {
		if id > q.PageCursor && now.Before(m.expires) && filter.Matches(&m.s) {
			s := m.s
			all = append(all, &s)
		}
	}
	slices.SortFunc(all, func(a, b *entities.EPPSession) int {
		switch {
		case a.ID < b.ID:
			return -1
		case a.ID > b.ID:
			return 1
		}
		return 0
	})
	if q.PageSize > 0 && len(all) > q.PageSize {
		return all[:q.PageSize], all[q.PageSize-1].ID, nil
	}
	return all, "", nil
}

// RequestKill delivers the IDs to every subscriber.
func (r *EPPSessions) RequestKill(_ context.Context, ids ...string) error {
	r.mu.Lock()
	if err := r.failure(); err != nil {
		r.mu.Unlock()
		return err
	}
	subs := slices.Clone(r.subs)
	r.mu.Unlock()
	for _, id := range ids {
		for _, ch := range subs {
			ch <- id
		}
	}
	return nil
}

// KillRequests subscribes until ctx is done.
func (r *EPPSessions) KillRequests(ctx context.Context) (<-chan string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if err := r.failure(); err != nil {
		return nil, err
	}
	ch := make(chan string, 64)
	r.subs = append(r.subs, ch)
	go func() {
		<-ctx.Done()
		r.mu.Lock()
		defer r.mu.Unlock()
		r.subs = slices.DeleteFunc(r.subs, func(c chan string) bool { return c == ch })
	}()
	return ch, nil
}
