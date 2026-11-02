package repositories

import (
	"context"

	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
)

// EPPPolicyRepository stores EPP policies, their assignment to registrars,
// and registrar families (entities.EPPPolicy). Every change is serialised
// with every other: it reads what it replaces and writes its outbox events,
// built from that, in the same transaction, so each event's before and
// after are exact even under concurrent changes. The events functions are
// given the stored object before the change, nil (or "") if there was none.
type EPPPolicyRepository interface {
	// ListPolicies returns every stored policy, by name.
	ListPolicies(ctx context.Context) ([]*entities.EPPPolicy, error)
	// GetPolicy returns a policy, or entities.ErrEPPPolicyNotFound.
	GetPolicy(ctx context.Context, name string) (*entities.EPPPolicy, error)
	// SavePolicy creates or replaces p. events is called inside the
	// transaction with the policy being replaced (nil when p is new) and
	// returns the change's events; an error from it saves nothing, so a
	// caller can refuse to replace what it finds.
	SavePolicy(ctx context.Context, p *entities.EPPPolicy, events func(before *entities.EPPPolicy) ([]entities.DomainEvent, error)) error
	// DeletePolicy deletes a policy no registrar or family is assigned
	// (entities.ErrEPPPolicyInUse otherwise; entities.ErrEPPPolicyNotFound
	// if there is none).
	DeletePolicy(ctx context.Context, name string, events func(before *entities.EPPPolicy) []entities.DomainEvent) error

	// ListAssignments returns every registrar's assigned policy.
	ListAssignments(ctx context.Context) (map[entities.ClIDType]string, error)
	// AssignPolicy assigns policy, which must exist, to clid, which must be
	// a registrar; an empty policy removes the assignment, so the registrar
	// has "default". A registrar's deletion deletes its assignment.
	AssignPolicy(ctx context.Context, clid entities.ClIDType, policy string, events func(before string) []entities.DomainEvent) error

	// ListFamilies returns every family with its members, by name.
	ListFamilies(ctx context.Context) ([]*entities.RegistrarFamily, error)
	// GetFamily returns a family, or entities.ErrRegistrarFamilyNotFound.
	GetFamily(ctx context.Context, name string) (*entities.RegistrarFamily, error)
	// SaveFamily creates or replaces f, its members included. A member of
	// another family is entities.ErrRegistrarInFamily. A registrar's
	// deletion removes it from its family.
	// events is as for SavePolicy: an error from it saves nothing.
	SaveFamily(ctx context.Context, f *entities.RegistrarFamily, events func(before *entities.RegistrarFamily) ([]entities.DomainEvent, error)) error
	// DeleteFamily deletes a family; its members belong to none.
	DeleteFamily(ctx context.Context, name string, events func(before *entities.RegistrarFamily) []entities.DomainEvent) error
}
