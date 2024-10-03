package services

import (
	"github.com/onasunnymorning/domain-os/internal/domain/entities"
	"github.com/onasunnymorning/domain-os/internal/domain/repositories"
	"golang.org/x/net/context"
)

// WhoisService implements the whois service interface
type WhoisService struct {
	domRepo repositories.DomainRepository
	rarRepo repositories.RegistrarRepository
}

// NewWhoisService creates a new instance of WhoisService
func NewWhoisService(domRepo repositories.DomainRepository, rarRepo repositories.RegistrarRepository) *WhoisService {
	return &WhoisService{
		domRepo: domRepo,
		rarRepo: rarRepo,
	}
}

// GetDomainWhois returns the whois information of a domain
func (s *WhoisService) GetDomainWhois(ctx context.Context, dn string) (*entities.WhoisResponse, error) {
	// First retrieve the domain
	dom, err := s.domRepo.GetDomainByName(ctx, dn, true)
	if err != nil {
		return nil, err
	}

	// Then look up the registrar
	rar, err := s.rarRepo.GetByClID(ctx, dom.ClID.String(), false)
	if err != nil {
		return nil, err
	}

	// Populate the whois response
	wr, err := entities.NewWhoisResponse(dom, rar)
	if err != nil {
		return nil, err
	}

	return wr, nil
}
