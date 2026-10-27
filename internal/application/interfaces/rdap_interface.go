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
	// NameserverExists reports whether any registrar holds a host object
	// named name. Phase 3 replaces it with a lookup that picks one host.
	NameserverExists(ctx context.Context, name string) (bool, error)
	// Registrar returns the registrar with IANA ID ianaID, or
	// entities.ErrRegistrarNotFound.
	Registrar(ctx context.Context, ianaID int) (*entities.Registrar, error)
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
