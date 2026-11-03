package services

import (
	"context"
	"fmt"
	"time"

	"github.com/onasunnymorning/domain-os/internal/appcontext"
	"github.com/onasunnymorning/domain-os/internal/application/commands"
	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
	"github.com/onasunnymorning/domain-os/pkg/domain/repositories"
)

// LifecyclePolicyService manages lifecycle policies
// (entities.LifecyclePolicy, ADR 0023). Every change is one repository
// transaction with its outbox event, which records the actor and the
// policy before and after, read in the same transaction as the change.
type LifecyclePolicyService struct {
	repo repositories.LifecyclePolicyRepository
	now  func() time.Time
}

// NewLifecyclePolicyService returns the service.
func NewLifecyclePolicyService(repo repositories.LifecyclePolicyRepository) *LifecyclePolicyService {
	return &LifecyclePolicyService{repo: repo, now: time.Now}
}

// ListPolicies returns every policy with the phases that name it.
func (s *LifecyclePolicyService) ListPolicies(ctx context.Context) ([]*entities.LifecyclePolicy, error) {
	ps, err := s.repo.ListPolicies(ctx)
	if err != nil {
		return nil, fmt.Errorf("list lifecycle policies: %w", err)
	}
	uses, err := s.repo.ListPolicyUses(ctx)
	if err != nil {
		return nil, fmt.Errorf("phases naming the lifecycle policies: %w", err)
	}
	for _, p := range ps {
		p.UsedBy = uses[p.Name]
	}
	return ps, nil
}

// GetPolicy returns a policy with the phases that name it.
func (s *LifecyclePolicyService) GetPolicy(ctx context.Context, name string) (*entities.LifecyclePolicy, error) {
	p, err := s.repo.GetPolicy(ctx, name)
	if err != nil {
		return nil, err
	}
	if p.UsedBy, err = s.repo.PolicyUses(ctx, name); err != nil {
		return nil, fmt.Errorf("phases naming lifecycle policy %s: %w", name, err)
	}
	return p, nil
}

// SavePolicy creates or replaces the named policy. A change reaches every
// phase that names the policy on its next lifecycle decision.
func (s *LifecyclePolicyService) SavePolicy(ctx context.Context, name string, cmd commands.SaveLifecyclePolicyCommand) (*entities.LifecyclePolicy, error) {
	p, err := entities.NewLifecyclePolicy(name, cmd.Description, cmd.Settings)
	if err != nil {
		return nil, err
	}
	p.UpdatedAt, p.UpdatedBy = saveTime(s.now()), actorOf(ctx)
	err = s.repo.SavePolicy(ctx, p, func(before *entities.LifecyclePolicy) ([]entities.DomainEvent, error) {
		if cmd.CreateOnly && before != nil {
			return nil, fmt.Errorf("%w: %s", entities.ErrLifecyclePolicyExists, name)
		}
		if cmd.ExpectedUpdatedAt != nil && (before == nil || !before.UpdatedAt.Equal(*cmd.ExpectedUpdatedAt)) {
			return nil, fmt.Errorf("%w: %s", entities.ErrLifecyclePolicyChanged, name)
		}
		eventType, msg := entities.EventLifecyclePolicyUpdated, fmt.Sprintf("Lifecycle policy %s updated", name)
		if before == nil {
			eventType, msg = entities.EventLifecyclePolicyCreated, fmt.Sprintf("Lifecycle policy %s created", name)
		}
		return []entities.DomainEvent{s.event(ctx, eventType, name, msg, p, before, p)}, nil
	})
	if err != nil {
		return nil, err
	}
	return s.GetPolicy(ctx, name)
}

// DeletePolicy deletes a policy no phase names. gtld-standard cannot be
// deleted.
func (s *LifecyclePolicyService) DeletePolicy(ctx context.Context, name string) error {
	if name == entities.LifecyclePolicyStandard {
		return fmt.Errorf("%w: %s", entities.ErrLifecyclePolicyBuiltIn, name)
	}
	return s.repo.DeletePolicy(ctx, name, func(before *entities.LifecyclePolicy) []entities.DomainEvent {
		return []entities.DomainEvent{s.event(ctx, entities.EventLifecyclePolicyDeleted, name, fmt.Sprintf("Lifecycle policy %s deleted", name), before, before, nil)}
	})
}

func (s *LifecyclePolicyService) event(ctx context.Context, eventType, subject, msg string, data, before, after interface{}) entities.DomainEvent {
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
