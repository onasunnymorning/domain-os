package services

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/onasunnymorning/domain-os/internal/appcontext"
	"github.com/onasunnymorning/domain-os/internal/application/commands"
	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
	"github.com/onasunnymorning/domain-os/pkg/domain/repositories"
)

// RDAPAccessPolicyService manages RDAP access policies and the networks
// they are assigned to (ADR 0022), and gives RDAP nodes the set they
// enforce.
type RDAPAccessPolicyService struct {
	repo repositories.RDAPAccessPolicyRepository
	now  func() time.Time
}

// NewRDAPAccessPolicyService returns the service.
func NewRDAPAccessPolicyService(repo repositories.RDAPAccessPolicyRepository) *RDAPAccessPolicyService {
	return &RDAPAccessPolicyService{repo: repo, now: time.Now}
}

// AccessSet returns every policy and network, for an RDAP node. Both are
// read in one snapshot, and a set in which a network names a policy it does
// not hold is refused, so a node keeps the set in force rather than give a
// network anonymous limits by mistake.
func (s *RDAPAccessPolicyService) AccessSet(ctx context.Context) (*entities.RDAPAccessSet, error) {
	policies, networks, err := s.repo.Snapshot(ctx)
	if err != nil {
		return nil, fmt.Errorf("read the RDAP access policies: %w", err)
	}
	set := entities.NewRDAPAccessSet(policies, networks, s.now().UTC())
	if missing := set.UnresolvedNetworks(); len(missing) > 0 {
		return nil, fmt.Errorf("%w: %s", entities.ErrRDAPAccessPolicyNotFound, strings.Join(missing, ", "))
	}
	return set, nil
}

// ListPolicies returns every policy, the built-in ones included.
func (s *RDAPAccessPolicyService) ListPolicies(ctx context.Context) ([]*entities.RDAPAccessPolicy, error) {
	set, err := s.AccessSet(ctx)
	if err != nil {
		return nil, err
	}
	return set.Policies(), nil
}

// GetPolicy returns a policy; a built-in not yet stored with its starting
// limits.
func (s *RDAPAccessPolicyService) GetPolicy(ctx context.Context, name string) (*entities.RDAPAccessPolicy, error) {
	p, err := s.repo.GetPolicy(ctx, name)
	if errors.Is(err, entities.ErrRDAPAccessPolicyNotFound) {
		for _, b := range entities.BuiltInRDAPAccessPolicies() {
			if b.Name == name {
				return b, nil
			}
		}
	}
	return p, err
}

// SavePolicy creates or replaces the named policy.
func (s *RDAPAccessPolicyService) SavePolicy(ctx context.Context, name string, cmd commands.SaveRDAPAccessPolicyCommand) (*entities.RDAPAccessPolicy, error) {
	p, err := entities.NewRDAPAccessPolicy(name, cmd.Description, cmd.Limits)
	if err != nil {
		return nil, err
	}
	p.UpdatedAt, p.UpdatedBy = s.now().UTC(), actorOf(ctx)
	err = s.repo.SavePolicy(ctx, p, func(before *entities.RDAPAccessPolicy) []entities.DomainEvent {
		return []entities.DomainEvent{rdapPolicyEvent(ctx, entities.EventRDAPAccessPolicySaved, name, fmt.Sprintf("RDAP access policy %s saved", name), p, before, p)}
	})
	if err != nil {
		return nil, err
	}
	return p, nil
}

// DeletePolicy deletes a custom policy no network is assigned.
func (s *RDAPAccessPolicyService) DeletePolicy(ctx context.Context, name string) error {
	if entities.IsBuiltInRDAPAccessPolicy(name) {
		return fmt.Errorf("%w: %s", entities.ErrRDAPAccessPolicyBuiltIn, name)
	}
	return s.repo.DeletePolicy(ctx, name, func(before *entities.RDAPAccessPolicy) []entities.DomainEvent {
		return []entities.DomainEvent{rdapPolicyEvent(ctx, entities.EventRDAPAccessPolicyDeleted, name, fmt.Sprintf("RDAP access policy %s deleted", name), before, before, nil)}
	})
}

// ListNetworks returns every assigned network, most specific first.
func (s *RDAPAccessPolicyService) ListNetworks(ctx context.Context) ([]entities.RDAPAccessNetwork, error) {
	set, err := s.AccessSet(ctx)
	if err != nil {
		return nil, err
	}
	out := set.Networks()
	if out == nil {
		out = []entities.RDAPAccessNetwork{}
	}
	return out, nil
}

// AssignNetwork assigns the network a policy, which must exist; an empty
// policy removes the assignment.
func (s *RDAPAccessPolicyService) AssignNetwork(ctx context.Context, cmd commands.AssignRDAPAccessNetworkCommand) (*entities.RDAPAccessNetwork, error) {
	network, err := entities.ParseRDAPAccessNetwork(cmd.Network)
	if err != nil {
		return nil, err
	}
	if cmd.Policy != "" {
		if _, err := s.GetPolicy(ctx, cmd.Policy); err != nil {
			return nil, err
		}
		// A built-in not yet stored is stored first, so the foreign key holds.
		if _, err := s.repo.GetPolicy(ctx, cmd.Policy); errors.Is(err, entities.ErrRDAPAccessPolicyNotFound) {
			b, err := s.GetPolicy(ctx, cmd.Policy)
			if err != nil {
				return nil, err
			}
			if _, err := s.SavePolicy(ctx, b.Name, commands.SaveRDAPAccessPolicyCommand{Description: b.Description, Limits: b.Limits}); err != nil {
				return nil, err
			}
		}
	}
	after := entities.RDAPAccessNetwork{Network: network, Policy: cmd.Policy}
	err = s.repo.AssignNetwork(ctx, network, cmd.Policy, func(prev string) []entities.DomainEvent {
		before := entities.RDAPAccessNetwork{Network: network, Policy: prev}
		return []entities.DomainEvent{rdapPolicyEvent(ctx, entities.EventRDAPAccessNetworkAssigned, network.String(),
			fmt.Sprintf("RDAP access policy %q assigned to %s", cmd.Policy, network), after, before, after)}
	})
	if err != nil {
		return nil, err
	}
	return &after, nil
}

// rdapPolicyEvent is an outbox event for an RDAP policy change, with its
// actor and the object before and after.
func rdapPolicyEvent(ctx context.Context, eventType, subject, msg string, data, before, after interface{}) entities.DomainEvent {
	e := entities.NewDomainEvent(eppEventSource, eventType, subject, msg, data)
	if v, ok := appcontext.TraceID(ctx); ok {
		e.TraceID = v
	}
	if v, ok := appcontext.CorrelationID(ctx); ok {
		e.CorrelationID = v
	}
	e.Actor = actorOf(ctx)
	e.BeforeState, e.AfterState = before, after
	return e
}
