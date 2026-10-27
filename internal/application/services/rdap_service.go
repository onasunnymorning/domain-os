package services

import (
	"context"

	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
	"github.com/onasunnymorning/domain-os/pkg/domain/repositories"
)

// RDAPService reads what the RDAP server serves (ADR 0020). It never writes.
type RDAPService struct {
	domains    repositories.DomainRepository
	hosts      repositories.HostRepository
	registrars repositories.RegistrarRepository
}

// NewRDAPService returns an RDAPService reading from the given repositories.
func NewRDAPService(domains repositories.DomainRepository, hosts repositories.HostRepository, registrars repositories.RegistrarRepository) *RDAPService {
	return &RDAPService{domains: domains, hosts: hosts, registrars: registrars}
}

// Domain returns the domain named name, or entities.ErrDomainNotFound.
// Nameservers and DS records are loaded with it.
func (s *RDAPService) Domain(ctx context.Context, name string) (*entities.Domain, error) {
	return s.domains.GetDomainByName(ctx, name, true)
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
