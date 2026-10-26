package interfaces

import (
	"context"

	"github.com/onasunnymorning/domain-os/internal/application/rdappolicy"
	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
)

// RDAPLookups is what the RDAP server reads from the registry (ADR 0020).
// Every method is read-only.
type RDAPLookups interface {
	// Domain returns what a domain response is built from, or
	// entities.ErrDomainNotFound.
	Domain(ctx context.Context, name string) (*RDAPDomain, error)
	// Nameserver returns the host object served for name, or
	// entities.ErrHostNotFound. Host names are unique per registrar, not
	// across the registry, so it picks one (ADR 0020 decision 5).
	Nameserver(ctx context.Context, name string) (*RDAPNameserver, error)
	// Registrar returns the registrar with IANA ID ianaID, or
	// entities.ErrRegistrarNotFound.
	Registrar(ctx context.Context, ianaID int) (*entities.Registrar, error)
	// RefreshPublicationPolicies reloads the TLDs' publication policies. A
	// failed load keeps the set last loaded.
	RefreshPublicationPolicies(ctx context.Context) error
}

// RDAPDomain is everything a domain response is built from.
type RDAPDomain struct {
	// Domain carries its nameservers and DS records.
	Domain *entities.Domain
	// Registrar is the sponsor, nil if it could not be found.
	Registrar *entities.Registrar
	// TLDType decides which notices apply and, until Phase 3, the built-in
	// publication policy.
	TLDType entities.TLDType
	// ContactPolicy is the contact policy of the TLD's current GA phase,
	// nil when there is none: then no contact is served.
	ContactPolicy *entities.ContactDataPolicy
	// Publication is the TLD's publication policy.
	Publication rdappolicy.Policy
	// Contacts are the domain's contacts in the roles ContactPolicy allows
	// and Publication publishes. Contacts in other roles are not loaded.
	Contacts map[rdappolicy.Role]*entities.Contact
}

// RDAPNameserver is everything a nameserver response is built from.
type RDAPNameserver struct {
	// Host carries its addresses.
	Host *entities.Host
	// Registrar is the host's sponsor, nil if it could not be found.
	Registrar *entities.Registrar
	// Ambiguous is true when more than one registrar holds a host with
	// this name, and the rule picked one.
	Ambiguous bool
}
