package services

import (
	"context"
	"fmt"

	"github.com/onasunnymorning/domain-os/internal/appcontext"
	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
	"github.com/onasunnymorning/domain-os/pkg/domain/repositories"
)

// EPPSuspensionService lets operators see and lift the logins suspended
// for failed creates (docs/epp/POLICY_REFERENCE.md § Failed creates).
type EPPSuspensionService struct {
	store       repositories.EPPSuspensionStore
	credentials repositories.EPPCredentialRepository
	events      repositories.EventPublisher
}

// NewEPPSuspensionService returns the service. credentials checks that a
// login is the registrar's; events records each lift.
func NewEPPSuspensionService(store repositories.EPPSuspensionStore, credentials repositories.EPPCredentialRepository, events repositories.EventPublisher) *EPPSuspensionService {
	return &EPPSuspensionService{store: store, credentials: credentials, events: events}
}

// List returns the logins suspended now, a registrar's only if clid is
// set.
func (s *EPPSuspensionService) List(ctx context.Context, clid string) ([]entities.EPPLoginSuspension, error) {
	all, err := s.store.ListSuspensions(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]entities.EPPLoginSuspension, 0, len(all))
	for _, sus := range all {
		if clid == "" || sus.ClID.String() == clid {
			out = append(out, sus)
		}
	}
	return out, nil
}

// Lift ends the registrar's login's suspension now, clearing the strikes
// that caused it, so its next create over budget starts escalation afresh.
// The lift is recorded as an epp.login.suspension_lifted event with the
// operator as actor, written first: if it cannot be, nothing is lifted.
func (s *EPPSuspensionService) Lift(ctx context.Context, clid, login string) (*entities.EPPLoginSuspension, error) {
	access, err := s.credentials.GetAccess(ctx, clid)
	if err != nil {
		return nil, err
	}
	if _, err := access.Login(login); err != nil {
		return nil, err
	}
	current, err := s.List(ctx, clid)
	if err != nil {
		return nil, err
	}
	var found *entities.EPPLoginSuspension
	for i := range current {
		if current[i].Login == login {
			found = &current[i]
		}
	}
	if found == nil {
		return nil, fmt.Errorf("%w: %s", entities.ErrEPPSuspensionNotFound, login)
	}
	e := entities.NewDomainEvent(eppEventSource, entities.EventEPPLoginSuspensionLifted, clid,
		fmt.Sprintf("an operator lifted the failed-create suspension of EPP login %s", login), found)
	e.Actor, _ = appcontext.UserID(ctx)
	if v, ok := appcontext.TraceID(ctx); ok {
		e.TraceID = v
	}
	e.BeforeState = found
	if err := s.events.Publish(ctx, e); err != nil {
		return nil, fmt.Errorf("record suspension lift: %w", err)
	}
	return s.store.LiftSuspension(ctx, login)
}
