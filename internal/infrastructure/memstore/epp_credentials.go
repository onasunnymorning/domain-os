// Package memstore holds in-memory repositories for tests that exercise the
// application and interface layers without a database.
package memstore

import (
	"context"
	"net/netip"
	"slices"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
)

// EPPCredentials is an in-memory repositories.EPPCredentialRepository. Like
// the Postgres one, a change is all or nothing: it is applied to a copy and
// kept only if it succeeds.
type EPPCredentials struct {
	mu     sync.Mutex
	access map[string]*entities.EPPRegistrarAccess
	events []entities.DomainEvent
	err    error
}

// Fail makes every method return err, to simulate an outage; nil ends it.
func (s *EPPCredentials) Fail(err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.err = err
}

// NewEPPCredentials returns a store with an empty access for each registrar.
func NewEPPCredentials(clids ...string) *EPPCredentials {
	s := &EPPCredentials{access: map[string]*entities.EPPRegistrarAccess{}}
	for _, c := range clids {
		s.access[c] = &entities.EPPRegistrarAccess{ClID: entities.ClIDType(c)}
	}
	return s
}

// Events returns the events written so far.
func (s *EPPCredentials) Events() []entities.DomainEvent {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Clone(s.events)
}

// GetAccess returns a copy of a registrar's access.
func (s *EPPCredentials) GetAccess(_ context.Context, clid string) (*entities.EPPRegistrarAccess, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.err != nil {
		return nil, s.err
	}
	a, ok := s.access[clid]
	if !ok {
		return nil, entities.ErrRegistrarNotFound
	}
	return a.Clone(), nil
}

// GetAccessByLogin returns a copy of the access that holds loginID.
func (s *EPPCredentials) GetAccessByLogin(_ context.Context, loginID string) (*entities.EPPRegistrarAccess, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.err != nil {
		return nil, s.err
	}
	for _, a := range s.access {
		if _, err := a.Login(loginID); err == nil {
			return a.Clone(), nil
		}
	}
	return nil, entities.ErrEPPLoginNotFound
}

// UpdateAccess applies fn to a copy and keeps it if fn succeeds.
func (s *EPPCredentials) UpdateAccess(_ context.Context, clid string, fn func(*entities.EPPRegistrarAccess) ([]entities.DomainEvent, error)) (*entities.EPPRegistrarAccess, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.err != nil {
		return nil, s.err
	}
	a, ok := s.access[clid]
	if !ok {
		return nil, entities.ErrRegistrarNotFound
	}
	c := a.Clone()
	events, err := fn(c)
	if err != nil {
		return nil, err
	}
	for _, l := range c.Logins {
		for other, oa := range s.access {
			if other == clid {
				continue
			}
			if _, err := oa.Login(string(l.LoginID)); err == nil {
				return nil, entities.ErrEPPLoginExists
			}
		}
	}
	s.access[clid] = c
	s.events = append(s.events, events...)
	return c.Clone(), nil
}

// ListUsableAllowedNetworks returns every network usable at now.
func (s *EPPCredentials) ListUsableAllowedNetworks(_ context.Context, now time.Time) ([]netip.Prefix, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.err != nil {
		return nil, s.err
	}
	var out []netip.Prefix
	for _, a := range s.access {
		for _, n := range a.AllowedNetworks {
			if n.Usable(now) && !slices.Contains(out, n.Prefix) {
				out = append(out, n.Prefix)
			}
		}
	}
	return out, nil
}

// ListCompromisedSince returns credentials retired as compromised since.
func (s *EPPCredentials) ListCompromisedSince(_ context.Context, since time.Time) ([]uuid.UUID, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.err != nil {
		return nil, s.err
	}
	var out []uuid.UUID
	add := func(lc *entities.EPPCredentialLifecycle) {
		if lc.Compromised() && lc.RetireAt != nil && !lc.RetireAt.Before(since) {
			out = append(out, lc.ID)
		}
	}
	for _, a := range s.access {
		for _, l := range a.Logins {
			for _, p := range l.Passwords {
				add(&p.EPPCredentialLifecycle)
			}
		}
		for _, p := range a.CertificatePins {
			add(&p.EPPCredentialLifecycle)
		}
		for _, n := range a.AllowedNetworks {
			add(&n.EPPCredentialLifecycle)
		}
	}
	return out, nil
}
