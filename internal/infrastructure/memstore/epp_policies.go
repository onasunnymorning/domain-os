package memstore

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"sync"

	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
)

// EPPPolicies is an in-memory repositories.EPPPolicyRepository for tests,
// with the Postgres repository's rules: a policy in use is not deleted, and
// a registrar is in at most one family. Events are kept in Events.
type EPPPolicies struct {
	mu          sync.Mutex
	policies    map[string]*entities.EPPPolicy
	assignments map[entities.ClIDType]string
	families    map[string]*entities.RegistrarFamily
	Events      []entities.DomainEvent
}

// NewEPPPolicies returns an empty repository.
func NewEPPPolicies() *EPPPolicies {
	return &EPPPolicies{
		policies: map[string]*entities.EPPPolicy{}, assignments: map[entities.ClIDType]string{},
		families: map[string]*entities.RegistrarFamily{},
	}
}

func (r *EPPPolicies) ListPolicies(context.Context) ([]*entities.EPPPolicy, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]*entities.EPPPolicy, 0, len(r.policies))
	for _, p := range r.policies {
		c := *p
		out = append(out, &c)
	}
	slices.SortFunc(out, func(a, b *entities.EPPPolicy) int { return strings.Compare(a.Name, b.Name) })
	return out, nil
}

func (r *EPPPolicies) GetPolicy(_ context.Context, name string) (*entities.EPPPolicy, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	p, ok := r.policies[name]
	if !ok {
		return nil, fmt.Errorf("%w: %s", entities.ErrEPPPolicyNotFound, name)
	}
	c := *p
	return &c, nil
}

func (r *EPPPolicies) SavePolicy(_ context.Context, p *entities.EPPPolicy, events []entities.DomainEvent) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	c := *p
	r.policies[p.Name] = &c
	r.Events = append(r.Events, events...)
	return nil
}

func (r *EPPPolicies) DeletePolicy(_ context.Context, name string, events []entities.DomainEvent) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.policies[name]; !ok {
		return fmt.Errorf("%w: %s", entities.ErrEPPPolicyNotFound, name)
	}
	for clid, p := range r.assignments {
		if p == name {
			return fmt.Errorf("%w: assigned to %s", entities.ErrEPPPolicyInUse, clid)
		}
	}
	for _, f := range r.families {
		if f.Policy == name {
			return fmt.Errorf("%w: assigned to %s", entities.ErrEPPPolicyInUse, f.Name)
		}
	}
	delete(r.policies, name)
	r.Events = append(r.Events, events...)
	return nil
}

func (r *EPPPolicies) ListAssignments(context.Context) (map[entities.ClIDType]string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make(map[entities.ClIDType]string, len(r.assignments))
	for k, v := range r.assignments {
		out[k] = v
	}
	return out, nil
}

func (r *EPPPolicies) AssignPolicy(_ context.Context, clid entities.ClIDType, policy string, events []entities.DomainEvent) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if policy == "" {
		delete(r.assignments, clid)
	} else {
		r.assignments[clid] = policy
	}
	r.Events = append(r.Events, events...)
	return nil
}

func (r *EPPPolicies) ListFamilies(context.Context) ([]*entities.RegistrarFamily, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]*entities.RegistrarFamily, 0, len(r.families))
	for _, f := range r.families {
		c := *f
		out = append(out, &c)
	}
	slices.SortFunc(out, func(a, b *entities.RegistrarFamily) int { return strings.Compare(a.Name, b.Name) })
	return out, nil
}

func (r *EPPPolicies) GetFamily(_ context.Context, name string) (*entities.RegistrarFamily, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	f, ok := r.families[name]
	if !ok {
		return nil, fmt.Errorf("%w: %s", entities.ErrRegistrarFamilyNotFound, name)
	}
	c := *f
	return &c, nil
}

func (r *EPPPolicies) SaveFamily(_ context.Context, f *entities.RegistrarFamily, events []entities.DomainEvent) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, other := range r.families {
		if other.Name == f.Name {
			continue
		}
		for _, m := range f.Members {
			if slices.Contains(other.Members, m) {
				return fmt.Errorf("%w: %s is in %s", entities.ErrRegistrarInFamily, m, other.Name)
			}
		}
	}
	c := *f
	r.families[f.Name] = &c
	r.Events = append(r.Events, events...)
	return nil
}

func (r *EPPPolicies) DeleteFamily(_ context.Context, name string, events []entities.DomainEvent) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.families[name]; !ok {
		return fmt.Errorf("%w: %s", entities.ErrRegistrarFamilyNotFound, name)
	}
	delete(r.families, name)
	r.Events = append(r.Events, events...)
	return nil
}
