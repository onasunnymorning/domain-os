package services

import (
	"github.com/onasunnymorning/domain-os/internal/domain/entities"
	"github.com/onasunnymorning/domain-os/internal/domain/repositories"
)

// IANAXMLService implements the IANAXMLService interface
type IANAXMLService struct {
	ianaRegistrarRepository repositories.IANARegistrarRepository
}

// NewIANAXMLService creates a new IANAXMLRegistrarService
func NewIANAXMLService(ianaRegistrarRepo repositories.IANARegistrarRepository) *IANAXMLService {
	return &IANAXMLService{
		ianaRegistrarRepository: ianaRegistrarRepo,
	}
}

// ListIANARegistrars returns a list of IANARegistrars
func (svc *IANAXMLService) ListIANARegistrars() ([]*entities.IANARegistrar, error) {
	ianaRegistrars, err := svc.ianaRegistrarRepository.ListIANARegistrars()
	if err != nil {
		return nil, err
	}

	return ianaRegistrars, nil
}
