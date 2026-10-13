package epp

import (
	"crypto/tls"
	"sync"
	"sync/atomic"
	"time"
)

// sessionState is where a session is in RFC 5730's lifecycle. A session is
// created once the TLS handshake completes and the greeting is about to be
// sent, so CONNECTED and GREETED collapse into awaitingLogin.
type sessionState int32

const (
	stateAwaitingLogin sessionState = iota
	stateAuthenticated
	stateClosing
)

func (s sessionState) String() string {
	switch s {
	case stateAwaitingLogin:
		return "awaiting_login"
	case stateAuthenticated:
		return "authenticated"
	case stateClosing:
		return "closing"
	default:
		return "unknown"
	}
}

// session is one EPP connection past its TLS handshake. Commands on a session
// run one at a time on the connection's own goroutine; the fields other
// goroutines may read are atomic.
type session struct {
	id          string
	clientIP    string
	conn        *tls.Conn
	connectedAt time.Time
	loginTimer  *time.Timer

	state    atomic.Int32
	clID     atomic.Pointer[string] // the clID presented at login; not yet authenticated (Phase 2)
	commands atomic.Int64
}

func (s *session) currentState() sessionState { return sessionState(s.state.Load()) }

func (s *session) setState(st sessionState) { s.state.Store(int32(st)) }

func (s *session) loggedInAs() string {
	if p := s.clID.Load(); p != nil {
		return *p
	}
	return ""
}

// sessionRegistry is this node's view of its live sessions. It is what drain
// closes and what the login timer looks sessions up in. The cluster-wide
// registry is Phase 3 of docs/epp/ARCHITECTURE.md.
type sessionRegistry struct {
	mu sync.Mutex
	m  map[string]*session
}

func newSessionRegistry() *sessionRegistry {
	return &sessionRegistry{m: make(map[string]*session)}
}

func (r *sessionRegistry) add(s *session) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.m[s.id] = s
}

func (r *sessionRegistry) get(id string) *session {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.m[id]
}

// remove deletes the session and returns it, or nil if it was not present.
func (r *sessionRegistry) remove(id string) *session {
	r.mu.Lock()
	defer r.mu.Unlock()
	s := r.m[id]
	delete(r.m, id)
	return s
}

func (r *sessionRegistry) all() []*session {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]*session, 0, len(r.m))
	for _, s := range r.m {
		out = append(out, s)
	}
	return out
}

func (r *sessionRegistry) len() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.m)
}
