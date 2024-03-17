package services

import "github.com/onasunnymorning/domain-os/internal/domain/repositories"

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
