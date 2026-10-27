package services

import (
	"context"
	"errors"
	"fmt"

	"github.com/onasunnymorning/domain-os/internal/application/interfaces"
	"github.com/onasunnymorning/domain-os/internal/application/rdappolicy"
	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
	"github.com/onasunnymorning/domain-os/pkg/domain/repositories"
)

// RDAPService reads what the RDAP server serves (ADR 0020). It never writes.
type RDAPService struct {
	domains    repositories.DomainRepository
	hosts      repositories.HostRepository
	registrars repositories.RegistrarRepository
	tlds       repositories.TLDPhaseReader
	contacts   repositories.ContactRepository
}

// RDAPServiceDeps are the repositories an RDAPService reads.
type RDAPServiceDeps struct {
	Domains    repositories.DomainRepository
	Hosts      repositories.HostRepository
	Registrars repositories.RegistrarRepository
	TLDs       repositories.TLDPhaseReader
	Contacts   repositories.ContactRepository
}

// NewRDAPService returns an RDAPService reading from deps.
func NewRDAPService(deps RDAPServiceDeps) *RDAPService {
	return &RDAPService{domains: deps.Domains, hosts: deps.Hosts, registrars: deps.Registrars, tlds: deps.TLDs, contacts: deps.Contacts}
}

// Domain returns what a domain response is built from, or
// entities.ErrDomainNotFound.
//
// Contacts are loaded only for the roles the TLD's current GA phase allows
// and its publication policy publishes (ADR 0021): a contact in any other
// role is never read, so it cannot reach a response.
func (s *RDAPService) Domain(ctx context.Context, name string) (*interfaces.RDAPDomain, error) {
	d, err := s.domains.GetDomainByName(ctx, name, true)
	if err != nil {
		return nil, err
	}
	out := &interfaces.RDAPDomain{Domain: d, Contacts: map[rdappolicy.Role]*entities.Contact{}}

	out.Registrar, err = s.registrars.GetByClID(ctx, d.ClID.String(), false)
	if err != nil && !errors.Is(err, entities.ErrRegistrarNotFound) {
		return nil, fmt.Errorf("sponsor %s of %s: %w", d.ClID, d.Name, err)
	}

	// The TLD's type and phases only: no prices, fees or counts, so a
	// lookup's cost does not grow with the TLD.
	tld, err := s.tlds.GetWithPhases(ctx, d.TLDName.String())
	if err != nil {
		return nil, fmt.Errorf("TLD of %s: %w", d.Name, err)
	}
	out.TLDType = tld.Type
	out.Publication = rdappolicy.BuiltInFor(tld.Type)
	if phase, err := tld.GetCurrentGAPhase(); err == nil {
		cdp := phase.Policy.ContactDataPolicy
		out.ContactPolicy = &cdp
	}

	for role, id := range map[rdappolicy.Role]entities.ClIDType{
		rdappolicy.RoleRegistrant: d.RegistrantID,
		rdappolicy.RoleAdmin:      d.AdminID,
		rdappolicy.RoleTech:       d.TechID,
		rdappolicy.RoleBilling:    d.BillingID,
	} {
		if _, published := out.Publication.Roles[role]; id == "" || !published || !rdappolicy.RoleAllowed(out.ContactPolicy, role) {
			continue
		}
		c, err := s.contacts.GetContactByID(ctx, id.String())
		if errors.Is(err, entities.ErrContactNotFound) {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("%s contact of %s: %w", role, d.Name, err)
		}
		out.Contacts[role] = c
	}
	return out, nil
}

// NameserverExists reports whether any registrar holds a host named name.
func (s *RDAPService) NameserverExists(ctx context.Context, name string) (bool, error) {
	n, err := s.hosts.CountHostsByName(ctx, name)
	return n > 0, err
}

// Registrar returns the registrar with IANA ID ianaID, or
// entities.ErrRegistrarNotFound.
func (s *RDAPService) Registrar(ctx context.Context, ianaID int) (*entities.Registrar, error) {
	return s.registrars.GetByGurID(ctx, ianaID)
}
