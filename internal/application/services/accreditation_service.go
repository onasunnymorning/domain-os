package services

import (
	"errors"

	"github.com/onasunnymorning/domain-os/internal/domain/entities"
	"github.com/onasunnymorning/domain-os/internal/domain/repositories"
	"golang.org/x/net/context"
)

var ErrInvalidAccreditation = errors.New("invalid accreditation")

// AccreditationService implements the AccreditationService interface
type AccreditationService struct {
	accRepo repositories.AccreditationRepository
	rarRepo repositories.RegistrarRepository
	tldRepo repositories.TLDRepository
}

// NewAccreditationService returns a new AccreditationService
func NewAccreditationService(accRepo repositories.AccreditationRepository, rarRepo repositories.RegistrarRepository, tldRepo repositories.TLDRepository) *AccreditationService {
	return &AccreditationService{
		accRepo: accRepo,
		rarRepo: rarRepo,
		tldRepo: tldRepo,
	}
}

// CreateAccreditation creates an accreditation
func (s *AccreditationService) CreateAccreditation(ctx context.Context, tldName, rarClID string) error {
	// Get the TLD
	tld, err := s.tldRepo.GetByName(ctx, tldName)
	if err != nil {
		return errors.Join(ErrInvalidAccreditation, err)
	}

	// Get the Registrar, preloading TLDs
	rar, err := s.rarRepo.GetByClID(ctx, rarClID, true)
	if err != nil {
		return errors.Join(ErrInvalidAccreditation, err)
	}

	// Accredit the Registrar using domain functions
	err = rar.AccreditFor(tld)
	if err != nil {
		return errors.Join(ErrInvalidAccreditation, err)
	}

	// Save the accreditation and return the result
	return s.accRepo.CreateAccreditation(ctx, tldName, rarClID)
}

// DeleteAccreditation deletes an accreditation
func (s *AccreditationService) DeleteAccreditation(ctx context.Context, tldName, rarClID string) error {
	// Get the TLD
	tld, err := s.tldRepo.GetByName(ctx, tldName)
	if err != nil {
		return errors.Join(ErrInvalidAccreditation, err)
	}

	// Get the Registrar, preloading TLDs
	rar, err := s.rarRepo.GetByClID(ctx, rarClID, true)
	if err != nil {
		return errors.Join(ErrInvalidAccreditation, err)
	}

	// Deaccredit the Registrar using domain functions
	err = rar.DeAccreditFor(tld)
	if err != nil {
		return errors.Join(ErrInvalidAccreditation, err)
	}

	// Delete the accreditation and return the result
	return s.accRepo.DeleteAccreditation(ctx, tldName, rarClID)
}

// ListTLDRegistrars lists the registrars that are accredited for a TLD
func (s *AccreditationService) ListTLDRegistrars(ctx context.Context, pageSize int, pageCursor, tldName string) ([]*entities.Registrar, error) {
	return s.accRepo.ListTLDRegistrars(ctx, pageSize, pageCursor, tldName)
}

// ListRegistrarTLDs lists the TLDs that a registrar is accredited for
func (s *AccreditationService) ListRegistrarTLDs(ctx context.Context, pageSize int, pageCursor, rarClID string) ([]*entities.TLD, error) {
	return s.accRepo.ListRegistrarTLDs(ctx, pageSize, pageCursor, rarClID)
}
