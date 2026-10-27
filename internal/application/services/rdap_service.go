package services

import (
	"context"
	"errors"
	"fmt"
	"strings"

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
	out.Publication = rdappolicy.BuiltInFor(tld.Name, tld.Type)
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

// Nameserver returns the host served for name, or entities.ErrHostNotFound.
//
// Host names are unique per registrar, not across the registry, so a name
// can match several hosts. RFC 9082 has no "several" answer for an exact
// lookup, so one is picked (ADR 0020 decision 5): the host sponsored by the
// sponsor of its superordinate domain, which is the host RFC 5732 says the
// name belongs to and the one whose glue is published; otherwise the oldest.
func (s *RDAPService) Nameserver(ctx context.Context, name string) (*interfaces.RDAPNameserver, error) {
	hosts, err := s.hosts.ListHostsByName(ctx, name)
	if err != nil {
		return nil, err
	}
	if len(hosts) == 0 {
		return nil, entities.ErrHostNotFound
	}
	out := &interfaces.RDAPNameserver{Host: hosts[0], Ambiguous: len(hosts) > 1}
	if out.Ambiguous {
		sponsor, err := s.superordinateSponsor(ctx, name)
		if err != nil {
			return nil, err
		}
		for _, h := range hosts {
			if sponsor != "" && h.ClID == sponsor {
				out.Host = h
				break
			}
		}
	}
	// The stored linked flag can outlive the host's last association, so
	// it is counted now, as EPP's <host:info> does.
	id, err := out.Host.RoID.Int64()
	if err != nil {
		return nil, fmt.Errorf("nameserver %s: %w", name, err)
	}
	links, err := s.hosts.GetHostAssociationCount(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("count domains using nameserver %s: %w", name, err)
	}
	out.Host.Status.Linked = links > 0

	out.Registrar, err = s.registrars.GetByClID(ctx, out.Host.ClID.String(), false)
	if err != nil && !errors.Is(err, entities.ErrRegistrarNotFound) {
		return nil, fmt.Errorf("sponsor %s of nameserver %s: %w", out.Host.ClID, name, err)
	}
	return out, nil
}

// superordinateSponsor is the sponsor of the registered domain that host
// name sits under, or empty if it sits under none: an external host. A host
// may be named like the domain itself, so the full name is tried first.
func (s *RDAPService) superordinateSponsor(ctx context.Context, name string) (entities.ClIDType, error) {
	labels := strings.Split(name, ".")
	// A registered domain has at least two labels: try the name and every
	// ancestor of it that does.
	for i := 0; i+2 <= len(labels); i++ {
		d, err := s.domains.GetDomainByName(ctx, strings.Join(labels[i:], "."), false)
		if errors.Is(err, entities.ErrDomainNotFound) {
			continue
		}
		if err != nil {
			return "", err
		}
		return d.ClID, nil
	}
	return "", nil
}

// Registrar returns the registrar with IANA ID ianaID, or
// entities.ErrRegistrarNotFound.
func (s *RDAPService) Registrar(ctx context.Context, ianaID int) (*entities.Registrar, error) {
	return s.registrars.GetByGurID(ctx, ianaID)
}
