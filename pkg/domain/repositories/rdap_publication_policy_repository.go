package repositories

import (
	"context"

	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
)

// RDAPPublicationPolicyRepository stores RDAP publication policies and their
// assignment to TLDs (ADR 0021). As for EPP policies (ADR 0019), every change
// is serialised with every other and writes its outbox events, built from
// what it replaces, in the same transaction. The events functions are given
// the stored object before the change, nil (or "") if there was none.
type RDAPPublicationPolicyRepository interface {
	// ListPolicies returns every stored policy, by name.
	ListPolicies(ctx context.Context) ([]*entities.RDAPPublicationPolicy, error)
	// GetPolicy returns a policy, or entities.ErrRDAPPublicationPolicyNotFound.
	GetPolicy(ctx context.Context, name string) (*entities.RDAPPublicationPolicy, error)
	// SavePolicy creates or replaces p.
	SavePolicy(ctx context.Context, p *entities.RDAPPublicationPolicy, events func(before *entities.RDAPPublicationPolicy) []entities.DomainEvent) error
	// DeletePolicy deletes a policy no TLD is assigned
	// (entities.ErrRDAPPublicationPolicyInUse otherwise;
	// entities.ErrRDAPPublicationPolicyNotFound if there is none).
	DeletePolicy(ctx context.Context, name string, events func(before *entities.RDAPPublicationPolicy) []entities.DomainEvent) error

	// Snapshot returns every stored policy and every assignment, read from
	// one consistent view of the database, so no assignment names a policy
	// created after the policies were read.
	Snapshot(ctx context.Context) ([]*entities.RDAPPublicationPolicy, map[entities.DomainName]string, error)

	// ListAssignments returns every TLD's assigned policy.
	ListAssignments(ctx context.Context) (map[entities.DomainName]string, error)
	// AssignPolicy assigns policy, which must exist, to tld, which must be
	// a TLD; an empty policy removes the assignment, so the TLD has the
	// built-in for its type. A TLD's deletion deletes its assignment.
	AssignPolicy(ctx context.Context, tld entities.DomainName, policy string, events func(before string) []entities.DomainEvent) error
}
