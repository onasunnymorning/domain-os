package repositories

import (
	"context"

	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
)

// LifecyclePolicyRepository stores lifecycle policies
// (entities.LifecyclePolicy, ADR 0023). As the EPP policy repository, every
// change reads what it replaces and writes its outbox events in the same
// transaction, so each event's before and after are exact. The events
// functions are given the stored policy before the change, nil if there
// was none.
type LifecyclePolicyRepository interface {
	// ListPolicies returns every stored policy, by name.
	ListPolicies(ctx context.Context) ([]*entities.LifecyclePolicy, error)
	// GetPolicy returns a policy, or entities.ErrLifecyclePolicyNotFound.
	GetPolicy(ctx context.Context, name string) (*entities.LifecyclePolicy, error)
	// SavePolicy creates or replaces p. events is called inside the
	// transaction with the policy being replaced (nil when p is new) and
	// returns the change's events; an error from it saves nothing.
	SavePolicy(ctx context.Context, p *entities.LifecyclePolicy, events func(before *entities.LifecyclePolicy) ([]entities.DomainEvent, error)) error
	// DeletePolicy deletes a policy no phase names
	// (entities.ErrLifecyclePolicyInUse otherwise;
	// entities.ErrLifecyclePolicyNotFound if there is none).
	DeletePolicy(ctx context.Context, name string, events func(before *entities.LifecyclePolicy) []entities.DomainEvent) error
	// PolicyUses lists the phases that name the policy, by TLD and phase.
	PolicyUses(ctx context.Context, name string) ([]entities.LifecyclePolicyUse, error)
}
