package services

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/onasunnymorning/domain-os/internal/appcontext"
	"github.com/onasunnymorning/domain-os/internal/application/commands"
	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
	"github.com/onasunnymorning/domain-os/pkg/domain/repositories"
)

// EPPPolicyService manages EPP policies, their assignment to registrars, and
// registrar families (entities.EPPPolicy, ADR 0019), and gives EPP nodes the
// set they enforce. Every change is one repository transaction with its
// outbox event, which records the actor and the object before and after,
// read in the same transaction as the change (the repository serialises
// changes), so they are exact under concurrent changes.
type EPPPolicyService struct {
	repo       repositories.EPPPolicyRepository
	registrars repositories.RegistrarRepository
	now        func() time.Time
}

// NewEPPPolicyService returns the service. registrars checks that an
// assigned registrar or a family member exists.
func NewEPPPolicyService(repo repositories.EPPPolicyRepository, registrars repositories.RegistrarRepository) *EPPPolicyService {
	return &EPPPolicyService{repo: repo, registrars: registrars, now: time.Now}
}

// PolicySet returns every policy, assignment and family, for an EPP node.
func (s *EPPPolicyService) PolicySet(ctx context.Context) (*entities.EPPPolicySet, error) {
	policies, err := s.repo.ListPolicies(ctx)
	if err != nil {
		return nil, fmt.Errorf("list EPP policies: %w", err)
	}
	assignments, err := s.repo.ListAssignments(ctx)
	if err != nil {
		return nil, fmt.Errorf("list EPP policy assignments: %w", err)
	}
	families, err := s.repo.ListFamilies(ctx)
	if err != nil {
		return nil, fmt.Errorf("list registrar families: %w", err)
	}
	return entities.NewEPPPolicySet(policies, assignments, families, s.now().UTC()), nil
}

// ListPolicies returns every policy, the built-in ones included.
func (s *EPPPolicyService) ListPolicies(ctx context.Context) ([]*entities.EPPPolicy, error) {
	set, err := s.PolicySet(ctx)
	if err != nil {
		return nil, err
	}
	return set.Policies(), nil
}

// GetPolicy returns a policy. A built-in policy not yet stored is returned
// with its starting limits.
func (s *EPPPolicyService) GetPolicy(ctx context.Context, name string) (*entities.EPPPolicy, error) {
	p, err := s.repo.GetPolicy(ctx, name)
	if errors.Is(err, entities.ErrEPPPolicyNotFound) {
		for _, b := range entities.BuiltInEPPPolicies() {
			if b.Name == name {
				return b, nil
			}
		}
	}
	return p, err
}

// SavePolicy creates or replaces the named policy.
func (s *EPPPolicyService) SavePolicy(ctx context.Context, name string, cmd commands.SaveEPPPolicyCommand) (*entities.EPPPolicy, error) {
	p, err := entities.NewEPPPolicy(name, cmd.Description, cmd.Limits)
	if err != nil {
		return nil, err
	}
	p.UpdatedAt, p.UpdatedBy = s.now().UTC(), actorOf(ctx)
	err = s.repo.SavePolicy(ctx, p, func(before *entities.EPPPolicy) []entities.DomainEvent {
		return []entities.DomainEvent{s.event(ctx, entities.EventEPPPolicySaved, name, fmt.Sprintf("EPP policy %s saved", name), p, before, p)}
	})
	if err != nil {
		return nil, err
	}
	return p, nil
}

// DeletePolicy deletes a custom policy that nothing is assigned. The
// built-in policies cannot be deleted.
func (s *EPPPolicyService) DeletePolicy(ctx context.Context, name string) error {
	if entities.IsBuiltInEPPPolicy(name) {
		return fmt.Errorf("%w: %s", entities.ErrEPPPolicyBuiltIn, name)
	}
	return s.repo.DeletePolicy(ctx, name, func(before *entities.EPPPolicy) []entities.DomainEvent {
		return []entities.DomainEvent{s.event(ctx, entities.EventEPPPolicyDeleted, name, fmt.Sprintf("EPP policy %s deleted", name), before, before, nil)}
	})
}

// RegistrarPolicy is what applies to the registrar: its policy, and its
// family with the family's policy.
func (s *EPPPolicyService) RegistrarPolicy(ctx context.Context, clid string) (*entities.EPPRegistrarPolicy, error) {
	if _, err := s.registrars.GetByClID(ctx, clid, false); err != nil {
		return nil, err
	}
	set, err := s.PolicySet(ctx)
	if err != nil {
		return nil, err
	}
	rp := set.ForRegistrar(entities.ClIDType(clid))
	return &rp, nil
}

// AssignPolicy assigns the registrar a policy, which must exist. "default"
// or none removes the assignment.
func (s *EPPPolicyService) AssignPolicy(ctx context.Context, clid string, cmd commands.AssignEPPPolicyCommand) (*entities.EPPRegistrarPolicy, error) {
	if _, err := s.registrars.GetByClID(ctx, clid, false); err != nil {
		return nil, err
	}
	policy := cmd.Policy
	if policy == entities.EPPPolicyDefault {
		policy = ""
	}
	if policy != "" {
		if _, err := s.GetPolicy(ctx, policy); err != nil {
			return nil, err
		}
	}
	after := entities.EPPPolicyAssignment{ClID: entities.ClIDType(clid), Policy: policy}
	err := s.repo.AssignPolicy(ctx, entities.ClIDType(clid), policy, func(prev string) []entities.DomainEvent {
		before := entities.EPPPolicyAssignment{ClID: entities.ClIDType(clid), Policy: prev}
		return []entities.DomainEvent{s.event(ctx, entities.EventEPPPolicyAssigned, clid,
			fmt.Sprintf("EPP policy %s assigned to %s", nameOrDefault(policy), clid), after, before, after)}
	})
	if err != nil {
		return nil, err
	}
	return s.RegistrarPolicy(ctx, clid)
}

// ListFamilies returns every registrar family.
func (s *EPPPolicyService) ListFamilies(ctx context.Context) ([]*entities.RegistrarFamily, error) {
	return s.repo.ListFamilies(ctx)
}

// GetFamily returns a registrar family.
func (s *EPPPolicyService) GetFamily(ctx context.Context, name string) (*entities.RegistrarFamily, error) {
	return s.repo.GetFamily(ctx, name)
}

// SaveFamily creates or replaces the named family. Its members must be
// registrars, in no other family; its policy, if given, must exist.
func (s *EPPPolicyService) SaveFamily(ctx context.Context, name string, cmd commands.SaveRegistrarFamilyCommand) (*entities.RegistrarFamily, error) {
	policy := cmd.Policy
	if policy == entities.EPPPolicyDefault {
		policy = ""
	}
	f, err := entities.NewRegistrarFamily(name, cmd.Description, policy, cmd.Members)
	if err != nil {
		return nil, err
	}
	if policy != "" {
		if _, err := s.GetPolicy(ctx, policy); err != nil {
			return nil, err
		}
	}
	for _, m := range f.Members {
		if _, err := s.registrars.GetByClID(ctx, m.String(), false); err != nil {
			return nil, fmt.Errorf("family member %s: %w", m, err)
		}
	}
	f.UpdatedAt, f.UpdatedBy = s.now().UTC(), actorOf(ctx)
	err = s.repo.SaveFamily(ctx, f, func(before *entities.RegistrarFamily) []entities.DomainEvent {
		return []entities.DomainEvent{s.event(ctx, entities.EventRegistrarFamilySaved, name, fmt.Sprintf("Registrar family %s saved", name), f, before, f)}
	})
	if err != nil {
		return nil, err
	}
	return f, nil
}

// DeleteFamily deletes a family; its members belong to none.
func (s *EPPPolicyService) DeleteFamily(ctx context.Context, name string) error {
	return s.repo.DeleteFamily(ctx, name, func(before *entities.RegistrarFamily) []entities.DomainEvent {
		return []entities.DomainEvent{s.event(ctx, entities.EventRegistrarFamilyDeleted, name, fmt.Sprintf("Registrar family %s deleted", name), before, before, nil)}
	})
}

func (s *EPPPolicyService) event(ctx context.Context, eventType, subject, msg string, data, before, after interface{}) entities.DomainEvent {
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

func nameOrDefault(policy string) string {
	if policy == "" {
		return entities.EPPPolicyDefault
	}
	return policy
}
