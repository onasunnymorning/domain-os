package services

import (
	"context"
	"errors"

	"github.com/onasunnymorning/domain-os/internal/application/commands"
	"github.com/onasunnymorning/domain-os/internal/domain/entities"
	"github.com/onasunnymorning/domain-os/internal/domain/repositories"
)

// RegistrarService implements the RegistrarService interface
type RegistrarService struct {
	registrarRepository repositories.RegistrarRepository
}

// NewRegistrarService creates a new RegistrarService
func NewRegistrarService(registrarRepository repositories.RegistrarRepository) *RegistrarService {
	return &RegistrarService{
		registrarRepository: registrarRepository,
	}
}

// Create creates a new registrar
func (s *RegistrarService) Create(ctx context.Context, cmd *commands.CreateRegistrarCommand) (*commands.CreateRegistrarCommandResult, error) {
	newRar, err := entities.NewRegistrar(cmd.ClID, cmd.Name, cmd.Email, cmd.GurID, cmd.PostalInfo)
	if err != nil {
		return nil, errors.Join(entities.ErrInvalidRegistrar, err)
	}

	// Add the optional fields
	if cmd.Voice != "" {
		v, err := entities.NewE164Type(cmd.Voice)
		if err != nil {
			return nil, errors.Join(entities.ErrInvalidRegistrar, err)
		}
		newRar.Voice = *v
	}
	if cmd.Fax != "" {
		f, err := entities.NewE164Type(cmd.Fax)
		if err != nil {
			return nil, errors.Join(entities.ErrInvalidRegistrar, err)
		}
		newRar.Fax = *f
	}
	if cmd.URL != "" {
		url, err := entities.NewURL(cmd.URL)
		if err != nil {
			return nil, errors.Join(entities.ErrInvalidRegistrar, err)
		}
		newRar.URL = *url
	}
	if cmd.RdapBaseURL != "" {
		rdapBaseURL, err := entities.NewURL(cmd.RdapBaseURL)
		if err != nil {
			return nil, errors.Join(entities.ErrInvalidRegistrar, err)
		}
		newRar.RdapBaseURL = *rdapBaseURL
	}
	if cmd.WhoisInfo != nil {
		wi, err := entities.NewWhoisInfo(cmd.WhoisInfo.Name.String(), cmd.WhoisInfo.URL.String())
		if err != nil {
			return nil, errors.Join(entities.ErrInvalidRegistrar, err)
		}
		newRar.WhoisInfo = *wi
	}

	// Check if the registrar is valid
	if err := newRar.Validate(); err != nil {
		return nil, errors.Join(entities.ErrInvalidRegistrar, err)
	}

	createdRegistrar, err := s.registrarRepository.Create(ctx, newRar)
	if err != nil {
		return nil, err
	}

	var result commands.CreateRegistrarCommandResult
	result.Result = *createdRegistrar

	return &result, nil
}

// GetByClID returns a registrar by its ClID
func (s *RegistrarService) GetByClID(ctx context.Context, clid string, preloadTLDs bool) (*entities.Registrar, error) {
	return s.registrarRepository.GetByClID(ctx, clid, preloadTLDs)
}

// GetByGurID returns a registrar by its GurID
func (s *RegistrarService) GetByGurID(ctx context.Context, gurID int) (*entities.Registrar, error) {
	return s.registrarRepository.GetByGurID(ctx, gurID)
}

// List returns a list of registrars
func (s *RegistrarService) List(ctx context.Context, pagesize int, pagecursor string) ([]*entities.Registrar, error) {
	return s.registrarRepository.List(ctx, pagesize, pagecursor)
}

// Update updates a registrar
func (s *RegistrarService) Update(ctx context.Context, rar *entities.Registrar) (*entities.Registrar, error) {
	return s.registrarRepository.Update(ctx, rar)
}

// Delete deletes a registrar by its ClID
func (s *RegistrarService) Delete(ctx context.Context, clid string) error {
	return s.registrarRepository.Delete(ctx, clid)
}

// Count returns the number of registrars
func (s *RegistrarService) Count(ctx context.Context) (int64, error) {
	return s.registrarRepository.Count(ctx)
}

// SetStatus sets the status of a registrar
func (s *RegistrarService) SetStatus(ctx context.Context, clid string, status entities.RegistrarStatus) error {
	// get the registrar
	registrar, err := s.registrarRepository.GetByClID(ctx, clid, false)
	if err != nil {
		return err
	}

	// set the status using domain logic
	err = registrar.SetStatus(status)
	if err != nil {
		return err
	}

	// save the registrar
	_, err = s.registrarRepository.Update(ctx, registrar)
	if err != nil {
		return err
	}

	return nil
}
