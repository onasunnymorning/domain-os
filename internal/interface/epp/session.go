package epp

import (
	"crypto/tls"
	"net/netip"
	"sync"
	"sync/atomic"
	"time"

	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
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
	source      netip.Addr
	conn        *tls.Conn
	connectedAt time.Time
	loginTimer  *time.Timer
	// failedLogins is only touched by the session's own goroutine.
	failedLogins int

	state         atomic.Int32
	auth          atomic.Pointer[entities.EPPAuthenticatedLogin] // set by a successful <login>
	authAt        atomic.Int64                                   // Unix nanoseconds of the login
	commands      atomic.Int64
	lastCommandAt atomic.Int64 // Unix nanoseconds; 0 before the first command
}

// snapshot is the session as the cluster-wide registry records it. It reads
// only atomics, so the heartbeat can take it while a command runs.
func (s *session) snapshot(node string) *entities.EPPSession {
	rec := &entities.EPPSession{
		ID: s.id, Node: node, ClientIP: s.source,
		ConnectedAt: s.connectedAt.UTC(), Commands: s.commands.Load(),
	}
	if a := s.auth.Load(); a != nil {
		rec.Authenticate(a, time.Unix(0, s.authAt.Load()))
	}
	if ns := s.lastCommandAt.Load(); ns != 0 {
		t := time.Unix(0, ns).UTC()
		rec.LastCommandAt = &t
	}
	return rec
}

func (s *session) currentState() sessionState { return sessionState(s.state.Load()) }

func (s *session) setState(st sessionState) { s.state.Store(int32(st)) }

// loggedInAs is the login ID the session authenticated as, or "".
func (s *session) loggedInAs() string {
	if a := s.auth.Load(); a != nil {
		return string(a.LoginID)
	}
	return ""
}

// registrar is the ClID of the registrar the session acts for, or "".
func (s *session) registrar() string {
	if a := s.auth.Load(); a != nil {
		return string(a.ClID)
	}
	return ""
}

// sessionRegistry is this node's view of its live sessions: what drain
// closes, what the heartbeat renews in the cluster-wide registry, and what
// the caps fall back to when that registry cannot answer.
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

// count counts the sessions match selects.
func (r *sessionRegistry) count(match func(*session) bool) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	n := 0
	for _, s := range r.m {
		if match(s) {
			n++
		}
	}
	return n
}

func (r *sessionRegistry) len() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.m)
}
