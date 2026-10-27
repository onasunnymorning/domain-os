package repositories

import (
	"context"
	"net/netip"

	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
)

// RDAPAccessPolicyRepository stores RDAP access policies and the networks
// they are assigned to (ADR 0022). As for the publication policies, every
// change is serialised and writes its outbox events, built from what it
// replaces, in the same transaction.
type RDAPAccessPolicyRepository interface {
	// ListPolicies returns every stored policy, by name.
	ListPolicies(ctx context.Context) ([]*entities.RDAPAccessPolicy, error)
	// GetPolicy returns a policy, or entities.ErrRDAPAccessPolicyNotFound.
	GetPolicy(ctx context.Context, name string) (*entities.RDAPAccessPolicy, error)
	// SavePolicy creates or replaces p.
	SavePolicy(ctx context.Context, p *entities.RDAPAccessPolicy, events func(before *entities.RDAPAccessPolicy) []entities.DomainEvent) error
	// DeletePolicy deletes a policy no network is assigned
	// (entities.ErrRDAPAccessPolicyInUse otherwise).
	DeletePolicy(ctx context.Context, name string, events func(before *entities.RDAPAccessPolicy) []entities.DomainEvent) error

	// Snapshot returns every stored policy and every assigned network, read
	// together so that each network's policy is among the policies.
	Snapshot(ctx context.Context) ([]*entities.RDAPAccessPolicy, []entities.RDAPAccessNetwork, error)
	// ListNetworks returns every assigned network.
	ListNetworks(ctx context.Context) ([]entities.RDAPAccessNetwork, error)
	// AssignNetwork assigns policy, which must exist, to network; an empty
	// policy removes the assignment.
	AssignNetwork(ctx context.Context, network netip.Prefix, policy string, events func(before string) []entities.DomainEvent) error
}
