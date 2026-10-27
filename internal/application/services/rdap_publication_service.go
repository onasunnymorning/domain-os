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

// RDAPPublicationPolicyService manages RDAP publication policies and their
// assignment to TLDs (ADR 0021), and gives RDAP nodes the set they apply.
// Every change is one repository transaction with its outbox event, which
// records the actor and the object before and after.
type RDAPPublicationPolicyService struct {
	repo repositories.RDAPPublicationPolicyRepository
	tlds repositories.TLDRepository
	now  func() time.Time
}

// NewRDAPPublicationPolicyService returns the service. tlds checks that an
// assigned TLD exists and tells its type.
func NewRDAPPublicationPolicyService(repo repositories.RDAPPublicationPolicyRepository, tlds repositories.TLDRepository) *RDAPPublicationPolicyService {
	return &RDAPPublicationPolicyService{repo: repo, tlds: tlds, now: time.Now}
}

// PublicationSet returns every policy and assignment, for an RDAP node.
func (s *RDAPPublicationPolicyService) PublicationSet(ctx context.Context) (*entities.RDAPPublicationSet, error) {
	policies, err := s.repo.ListPolicies(ctx)
	if err != nil {
		return nil, fmt.Errorf("list RDAP publication policies: %w", err)
	}
	assignments, err := s.repo.ListAssignments(ctx)
	if err != nil {
		return nil, fmt.Errorf("list RDAP publication assignments: %w", err)
	}
	return entities.NewRDAPPublicationSet(policies, assignments, s.now().UTC()), nil
}

// ListPolicies returns every policy, the built-in ones included.
func (s *RDAPPublicationPolicyService) ListPolicies(ctx context.Context) ([]*entities.RDAPPublicationPolicy, error) {
	set, err := s.PublicationSet(ctx)
	if err != nil {
		return nil, err
	}
	return set.Policies(), nil
}

// GetPolicy returns a policy. A built-in not yet stored is returned with
// its starting rules.
func (s *RDAPPublicationPolicyService) GetPolicy(ctx context.Context, name string) (*entities.RDAPPublicationPolicy, error) {
	p, err := s.repo.GetPolicy(ctx, name)
	if errors.Is(err, entities.ErrRDAPPublicationPolicyNotFound) {
		for _, b := range entities.BuiltInRDAPPublicationPolicies() {
			if b.Name == name {
				return b, nil
			}
		}
	}
	return p, err
}

// SavePolicy creates or replaces the named policy.
func (s *RDAPPublicationPolicyService) SavePolicy(ctx context.Context, name string, cmd commands.SaveRDAPPublicationPolicyCommand) (*entities.RDAPPublicationPolicy, error) {
	p, err := entities.NewRDAPPublicationPolicy(name, cmd.Description, cmd.Roles, cmd.ContactFormURI)
	if err != nil {
		return nil, err
	}
	p.UpdatedAt, p.UpdatedBy = s.now().UTC(), actorOf(ctx)
	err = s.repo.SavePolicy(ctx, p, func(before *entities.RDAPPublicationPolicy) []entities.DomainEvent {
		return []entities.DomainEvent{s.event(ctx, entities.EventRDAPPublicationPolicySaved, name, fmt.Sprintf("RDAP publication policy %s saved", name), p, before, p)}
	})
	if err != nil {
		return nil, err
	}
	return p, nil
}

// DeletePolicy deletes a custom policy no TLD is assigned. The built-in
// policies cannot be deleted.
func (s *RDAPPublicationPolicyService) DeletePolicy(ctx context.Context, name string) error {
	if entities.IsBuiltInRDAPPublicationPolicy(name) {
		return fmt.Errorf("%w: %s", entities.ErrRDAPPublicationPolicyBuiltIn, name)
	}
	return s.repo.DeletePolicy(ctx, name, func(before *entities.RDAPPublicationPolicy) []entities.DomainEvent {
		return []entities.DomainEvent{s.event(ctx, entities.EventRDAPPublicationPolicyDeleted, name, fmt.Sprintf("RDAP publication policy %s deleted", name), before, before, nil)}
	})
}

// TLDPolicy is the publication policy that applies to the TLD.
func (s *RDAPPublicationPolicyService) TLDPolicy(ctx context.Context, tld string) (*entities.RDAPTLDPublication, error) {
	t, err := s.tlds.GetByName(ctx, tld, false)
	if err != nil {
		return nil, err
	}
	assignments, err := s.repo.ListAssignments(ctx)
	if err != nil {
		return nil, err
	}
	out := &entities.RDAPTLDPublication{TLD: t.Name, Policy: entities.BuiltInRDAPPublicationPolicyFor(t.Name, t.Type)}
	if name, ok := assignments[t.Name]; ok {
		out.Policy, out.Assigned = name, true
	}
	return out, nil
}

// AssignPolicy assigns the TLD a publication policy, which must exist. An
// empty policy removes the assignment.
func (s *RDAPPublicationPolicyService) AssignPolicy(ctx context.Context, tld string, cmd commands.AssignRDAPPublicationPolicyCommand) (*entities.RDAPTLDPublication, error) {
	t, err := s.tlds.GetByName(ctx, tld, false)
	if err != nil {
		return nil, err
	}
	if cmd.Policy != "" {
		if _, err := s.GetPolicy(ctx, cmd.Policy); err != nil {
			return nil, err
		}
		// A built-in not yet stored is stored first, so the assignment's
		// foreign key holds.
		if entities.IsBuiltInRDAPPublicationPolicy(cmd.Policy) {
			if _, err := s.repo.GetPolicy(ctx, cmd.Policy); errors.Is(err, entities.ErrRDAPPublicationPolicyNotFound) {
				b, _ := s.GetPolicy(ctx, cmd.Policy)
				if _, err := s.SavePolicy(ctx, b.Name, commands.SaveRDAPPublicationPolicyCommand{Description: b.Description, Roles: b.Roles, ContactFormURI: b.ContactFormURI}); err != nil {
					return nil, err
				}
			}
		}
	}
	after := entities.RDAPPublicationAssignment{TLD: t.Name, Policy: cmd.Policy}
	err = s.repo.AssignPolicy(ctx, t.Name, cmd.Policy, func(prev string) []entities.DomainEvent {
		before := entities.RDAPPublicationAssignment{TLD: t.Name, Policy: prev}
		return []entities.DomainEvent{s.event(ctx, entities.EventRDAPPublicationPolicyAssigned, string(t.Name),
			fmt.Sprintf("RDAP publication policy %q assigned to %s", cmd.Policy, t.Name), after, before, after)}
	})
	if err != nil {
		return nil, err
	}
	return s.TLDPolicy(ctx, tld)
}

func (s *RDAPPublicationPolicyService) event(ctx context.Context, eventType, subject, msg string, data, before, after interface{}) entities.DomainEvent {
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
