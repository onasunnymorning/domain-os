package interfaces

import (
	"context"

	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
)

// RDAPLookups is what the RDAP server reads from the registry (ADR 0020).
// Every method is read-only.
type RDAPLookups interface {
	// Domain returns the domain named name (an A-label), or
	// entities.ErrDomainNotFound.
	Domain(ctx context.Context, name string) (*entities.Domain, error)
	// NameserverExists reports whether any registrar holds a host object
	// named name. Phase 3 replaces it with a lookup that picks one host.
	NameserverExists(ctx context.Context, name string) (bool, error)
	// Registrar returns the registrar with IANA ID ianaID, or
	// entities.ErrRegistrarNotFound.
	Registrar(ctx context.Context, ianaID int) (*entities.Registrar, error)
}
