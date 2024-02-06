package services

import (
	"github.com/onasunnymorning/domain-os/internal/domain/repositories"
)

// SyncService is a service for synchronizing data from external sources and storing it in the database
// SyncService implements the SyncService interface
type SyncService struct {
	registrarRepository repositories.IANARegistrarRepository
	Spec5Repository     repositories.Spec5LabelRepository
	IcannRepository     repositories.ICANNRepository
	IanaRepository      repositories.IANARepository
}

// NewSyncService returns a new Spec5Service
func NewSyncService(
	registrarRepository repositories.IANARegistrarRepository,
	spec5Repository repositories.Spec5LabelRepository,
	icannRepository repositories.ICANNRepository,
	ianaRepository repositories.IANARepository,
) *SyncService {
	return &SyncService{
		registrarRepository: registrarRepository,
		Spec5Repository:     spec5Repository,
		IcannRepository:     icannRepository,
		IanaRepository:      ianaRepository,
	}
}

// RefreshSpec5Labels deletes and recreates all Spec5Labels using the ICANN XML registry as a source
// This is only needed when ICANN updates their XML registry. This happens very infrequently.
// Use this when the system is initialized, after that only when ICANN notifies you of an update to the XML registry
func (s *SyncService) RefreshSpec5Labels() error {
	// Get the list of labels from the ICANN XML registry
	labels, err := s.IcannRepository.ListSpec5Labels()
	if err != nil {
		return err
	}
	// Replace the existing list of labels in the database with the new list
	err = s.Spec5Repository.UpdateAll(labels)
	if err != nil {
		return err
	}
	return nil
}

// RefreshIANARegistrars deletes and recreates all IANARegistrars using the IANA XML registry as a source
// This is only needed when IANA updates their XML registry. This happens not very frequently
// Use this when the system is initialized, after that only when IANA or ICANN notifies you of an update to the XML registry
// Or you receive a termination notice from ICANN for a registrar
func (s *SyncService) RefreshIANARegistrars() error {
	// Get the list of registrars from the IANA XML registry
	registrars, err := s.IanaRepository.ListRegistrars()
	if err != nil {
		return err
	}

	// Replace the existing list of registrars in the database with the new list
	err = s.registrarRepository.UpdateAll(registrars)
	if err != nil {
		return err
	}
	return nil
}
