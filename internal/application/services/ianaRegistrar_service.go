package services

import (
	"context"

	"github.com/onasunnymorning/domain-os/internal/domain/entities"
	"github.com/onasunnymorning/domain-os/internal/domain/repositories"
)

// IANARegistrarService implements the IANARegistrarService interface
type IANARegistrarService struct {
	ianaRepository repositories.IANARegistrarRepository
}

// NewIANARegistrarService returns a new IANARegistrarService
func NewIANARegistrarService(ianaRepo repositories.IANARegistrarRepository) *IANARegistrarService {
	return &IANARegistrarService{
		ianaRepository: ianaRepo,
	}
}

// ListAll lists the IANARegistrars
func (s *IANARegistrarService) List(ctx context.Context, pageSize int, pageCursor, nameSearchString, status string) ([]*entities.IANARegistrar, error) {
	return s.ianaRepository.List(ctx, pageSize, pageCursor, nameSearchString, status)
}

// GetByGurID Retrieves gets a IANARegistrar by GurID
func (s *IANARegistrarService) GetByGurID(ctx context.Context, gurID int) (*entities.IANARegistrar, error) {
	return s.ianaRepository.GetByGurID(ctx, gurID)
}

// Count returns the number of IANARegistrars
func (s *IANARegistrarService) Count(ctx context.Context) (int, error) {
	return s.ianaRepository.Count(ctx)
}
