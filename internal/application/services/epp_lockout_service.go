package services

import (
	"context"
	"errors"
	"fmt"

	"github.com/onasunnymorning/domain-os/internal/appcontext"
	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
	"github.com/onasunnymorning/domain-os/pkg/domain/repositories"
)

// EPPLockoutService lets operators see and lift the (login, source address)
// pairs locked out after failed passwords (docs/epp/POLICY_REFERENCE.md
// § Authentication).
type EPPLockoutService struct {
	store       repositories.EPPLockoutStore
	credentials repositories.EPPCredentialRepository
	events      repositories.EventPublisher
}

// NewEPPLockoutService returns the service. credentials names each login's
// registrar; events records each lift.
func NewEPPLockoutService(store repositories.EPPLockoutStore, credentials repositories.EPPCredentialRepository, events repositories.EventPublisher) *EPPLockoutService {
	return &EPPLockoutService{store: store, credentials: credentials, events: events}
}

// List returns the pairs locked now with their registrar, a registrar's
// only if clid is set. A locked login that no longer exists is left out.
func (s *EPPLockoutService) List(ctx context.Context, clid string) ([]entities.EPPLoginLockout, error) {
	all, err := s.store.ListLockouts(ctx)
	if err != nil {
		return nil, err
	}
	owner := map[string]entities.ClIDType{}
	out := make([]entities.EPPLoginLockout, 0, len(all))
	for _, l := range all {
		c, ok := owner[l.Login]
		if !ok {
			access, err := s.credentials.GetAccessByLogin(ctx, l.Login)
			switch {
			case errors.Is(err, entities.ErrEPPLoginNotFound):
				owner[l.Login] = ""
				continue
			case err != nil:
				return nil, err
			}
			c = entities.ClIDType(access.ClID)
			owner[l.Login] = c
		}
		if c == "" || (clid != "" && c.String() != clid) {
			continue
		}
		l.ClID = c
		out = append(out, l)
	}
	return out, nil
}

// Lift clears the registrar's login's lockouts, with their failure counts
// and strikes, from every source address. It is recorded first as an
// epp.login.lockout_lifted event with the operator as actor; if that
// cannot be written, nothing is lifted.
func (s *EPPLockoutService) Lift(ctx context.Context, clid, login string) ([]entities.EPPLoginLockout, error) {
	access, err := s.credentials.GetAccess(ctx, clid)
	if err != nil {
		return nil, err
	}
	if _, err := access.Login(login); err != nil {
		return nil, err
	}
	all, err := s.List(ctx, clid)
	if err != nil {
		return nil, err
	}
	var locked []entities.EPPLoginLockout
	for _, l := range all {
		if l.Login == login {
			locked = append(locked, l)
		}
	}
	if len(locked) == 0 {
		return nil, fmt.Errorf("%w: %s", entities.ErrEPPLockoutNotFound, login)
	}
	e := entities.NewDomainEvent(eppEventSource, entities.EventEPPLoginLockoutLifted, clid,
		fmt.Sprintf("an operator lifted the lockouts of EPP login %s", login), locked)
	e.Actor, _ = appcontext.UserID(ctx)
	if v, ok := appcontext.TraceID(ctx); ok {
		e.TraceID = v
	}
	e.BeforeState = locked
	if err := s.events.Publish(ctx, e); err != nil {
		return nil, fmt.Errorf("record lockout lift: %w", err)
	}
	if _, err := s.store.LiftLockouts(ctx, login); err != nil {
		return nil, err
	}
	return locked, nil
}
