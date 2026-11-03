package memstore

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"sync"

	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
)

// LifecyclePolicies is an in-memory repositories.LifecyclePolicyRepository
// for tests, with the Postgres repository's rules: a policy a phase names
// is not deleted. Uses holds which phases name which policy; events are
// kept in Events.
type LifecyclePolicies struct {
	mu       sync.Mutex
	policies map[string]*entities.LifecyclePolicy
	Uses     map[string][]entities.LifecyclePolicyUse
	Events   []entities.DomainEvent
}

// NewLifecyclePolicies returns a repository holding gtld-standard.
func NewLifecyclePolicies() *LifecyclePolicies {
	r := &LifecyclePolicies{policies: map[string]*entities.LifecyclePolicy{}, Uses: map[string][]entities.LifecyclePolicyUse{}}
	r.policies[entities.LifecyclePolicyStandard] = entities.StandardLifecyclePolicy()
	return r
}

func (r *LifecyclePolicies) ListPolicies(context.Context) ([]*entities.LifecyclePolicy, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]*entities.LifecyclePolicy, 0, len(r.policies))
	for _, p := range r.policies {
		c := *p
		out = append(out, &c)
	}
	slices.SortFunc(out, func(a, b *entities.LifecyclePolicy) int { return strings.Compare(a.Name, b.Name) })
	return out, nil
}

func (r *LifecyclePolicies) GetPolicy(_ context.Context, name string) (*entities.LifecyclePolicy, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	p, ok := r.policies[name]
	if !ok {
		return nil, fmt.Errorf("%w: %s", entities.ErrLifecyclePolicyNotFound, name)
	}
	c := *p
	return &c, nil
}

func (r *LifecyclePolicies) SavePolicy(_ context.Context, p *entities.LifecyclePolicy, events func(*entities.LifecyclePolicy) ([]entities.DomainEvent, error)) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	before := r.policies[p.Name]
	evs, err := events(before)
	if err != nil {
		return err
	}
	c := *p
	c.UsedBy = nil
	r.policies[p.Name] = &c
	r.Events = append(r.Events, evs...)
	return nil
}

func (r *LifecyclePolicies) DeletePolicy(_ context.Context, name string, events func(*entities.LifecyclePolicy) []entities.DomainEvent) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	before, ok := r.policies[name]
	if !ok {
		return fmt.Errorf("%w: %s", entities.ErrLifecyclePolicyNotFound, name)
	}
	if uses := r.Uses[name]; len(uses) > 0 {
		return fmt.Errorf("%w: %s/%s", entities.ErrLifecyclePolicyInUse, uses[0].TLD, uses[0].Phase)
	}
	delete(r.policies, name)
	r.Events = append(r.Events, events(before)...)
	return nil
}

func (r *LifecyclePolicies) PolicyUses(_ context.Context, name string) ([]entities.LifecyclePolicyUse, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return slices.Clone(r.Uses[name]), nil
}
