package services

import (
	"github.com/onasunnymorning/domain-os/internal/domain/entities"
	"github.com/onasunnymorning/domain-os/internal/domain/repositories"
	"golang.org/x/net/context"
)

// DomainService immplements the DomainService interface
type DomainService struct {
	domainRepository repositories.DomainRepository
	roidService      RoidService
}

// NewDomainService returns a new instance of a DomainService
func NewDomainService(repo repositories.DomainRepository, roidService RoidService) *DomainService {
	return &DomainService{
		domainRepository: repo,
		roidService:      roidService,
	}
}

// ListDomains returns a list of domains
func (s *DomainService) ListDomains(ctx context.Context, pageSize int, cursor string) ([]*entities.Contact, error) {
	return s.domainRepository.ListDomains(ctx, pageSize, cursor)
}
