package entities

import (
	"errors"
	"fmt"
	"net/netip"
	"slices"
	"time"

	"github.com/google/uuid"
)

// Live EPP sessions across every node (docs/epp/ARCHITECTURE.md §3). Each is
// a lease in a shared registry, renewed by its node's heartbeat, so counts
// come from live leases and a crashed node's sessions expire with them.

var (
	ErrEPPSessionNotFound      = errors.New("EPP session not found")
	ErrEPPSessionLimit         = errors.New("EPP session limit reached")
	ErrEPPSessionsUnavailable  = errors.New("EPP session registry unavailable")
	ErrInvalidEPPSessionFilter = errors.New("invalid EPP session filter: name at least one of clid, login, client_ip or credential_id")
)

// EPPSessionLimit names the cap a session hit.
type EPPSessionLimit string

const (
	EPPSessionLimitSource    EPPSessionLimit = "source_ip"
	EPPSessionLimitRegistrar EPPSessionLimit = "registrar"
	EPPSessionLimitLogin     EPPSessionLimit = "login"
)

// EPPSessionLimitError says which cap a session hit. It wraps
// ErrEPPSessionLimit.
type EPPSessionLimitError struct {
	Limit EPPSessionLimit
	Cap   int
}

func (e *EPPSessionLimitError) Error() string {
	return fmt.Sprintf("%s: %d sessions per %s", ErrEPPSessionLimit, e.Cap, e.Limit)
}

func (e *EPPSessionLimitError) Unwrap() error { return ErrEPPSessionLimit }

// EPPSessionCaps are the session caps. They are server settings until the
// policy engine makes them per tier (Phase 5).
type EPPSessionCaps struct {
	PerSource    int
	PerRegistrar int
	PerLogin     int
}

// ForLogin is the cap for a login: its own session cap when set, otherwise
// the default.
func (c EPPSessionCaps) ForLogin(loginCap int) int {
	if loginCap > 0 {
		return loginCap
	}
	return c.PerLogin
}

// EPPSession is one live session as the registry records it.
type EPPSession struct {
	ID       string     `json:"id"`
	Node     string     `json:"node"`
	ClientIP netip.Addr `json:"client_ip"`
	// Set once the session has logged in.
	ClID          ClIDType     `json:"clid,omitempty"`
	LoginID       ClIDType     `json:"login_id,omitempty"`
	Role          EPPLoginRole `json:"role,omitempty"`
	LoginCap      int          `json:"-"`
	CredentialIDs []uuid.UUID  `json:"credential_ids,omitempty"`

	ConnectedAt     time.Time  `json:"connected_at"`
	AuthenticatedAt *time.Time `json:"authenticated_at,omitempty"`
	LastCommandAt   *time.Time `json:"last_command_at,omitempty"`
	Commands        int64      `json:"commands"`
}

// Authenticated reports whether the session has logged in.
func (s *EPPSession) Authenticated() bool { return s.ClID != "" }

// Authenticate records who the session logged in as.
func (s *EPPSession) Authenticate(l *EPPAuthenticatedLogin, at time.Time) {
	s.ClID, s.LoginID, s.Role, s.LoginCap = l.ClID, l.LoginID, l.Role, l.SessionCap
	s.CredentialIDs = l.CredentialIDs()
	t := at.UTC()
	s.AuthenticatedAt = &t
}

// UsedCredential reports whether the session authenticated with id.
func (s *EPPSession) UsedCredential(id uuid.UUID) bool {
	return slices.Contains(s.CredentialIDs, id)
}

// EventEPPSessionKillRequested is the outbox event for an operator asking
// for a session to be ended. It is written before the request is sent, so
// every request has its record; a request that could not be sent leaves the
// record of an attempt, and the operator sees the failure.
const EventEPPSessionKillRequested = "epp.session.kill_requested"

// EPPSessionKillRequestedData is the payload of an
// epp.session.kill_requested event.
type EPPSessionKillRequestedData struct {
	SessionID string   `json:"session_id"`
	ClID      ClIDType `json:"clid,omitempty"`
	LoginID   ClIDType `json:"login_id,omitempty"`
	ClientIP  string   `json:"client_ip"`
	Node      string   `json:"node"`
	// Filter is set when the session was one of several ended by a filter.
	Filter string `json:"filter,omitempty"`
}
