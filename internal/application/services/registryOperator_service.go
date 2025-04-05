package services

import (
	"context"
	"errors"

	"github.com/onasunnymorning/domain-os/internal/application/commands"
	"github.com/onasunnymorning/domain-os/internal/application/queries"
	"github.com/onasunnymorning/domain-os/internal/domain/entities"
	"github.com/onasunnymorning/domain-os/internal/domain/repositories"
)

// RegistryOperatorService implements the RegistryOperatorService interface
type RegistryOperatorService struct {
	ryRepo repositories.RegistryOperatorRepository
}

// NewRegistryOperatorService creates a new RegistryOperatorService instance
func NewRegistryOperatorService(ryRepo repositories.RegistryOperatorRepository) *RegistryOperatorService {
	return &RegistryOperatorService{
		ryRepo: ryRepo,
	}
}

// Create creates a new RegistryOperator
func (s *RegistryOperatorService) Create(ctx context.Context, cmd *commands.CreateRegistryOperatorCommand) (*entities.RegistryOperator, error) {
	ry, err := entities.NewRegistryOperator(cmd.RyID, cmd.Name, cmd.Email)
	if err != nil {
		return nil, errors.Join(entities.ErrInvalidRegistryOperator, err)
	}

	if cmd.Voice != "" {
		v, err := entities.NewE164Type(cmd.Voice)
		if err != nil {
			return nil, errors.Join(entities.ErrInvalidRegistryOperator, err)
		}
		ry.Voice = *v
	}

	if cmd.Fax != "" {
		f, err := entities.NewE164Type(cmd.Fax)
		if err != nil {
			return nil, errors.Join(entities.ErrInvalidRegistryOperator, err)
		}
		ry.Fax = *f
	}

	if cmd.URL != "" {
		url, err := entities.NewURL(cmd.URL)
		if err != nil {
			return nil, errors.Join(entities.ErrInvalidRegistryOperator, err)
		}
		ry.URL = *url
	}

	return s.ryRepo.Create(ctx, ry)
}

// GetByRyID gets a RegistryOperator by its RyID
func (s *RegistryOperatorService) GetByRyID(ctx context.Context, ryid string) (*entities.RegistryOperator, error) {
	return s.ryRepo.GetByRyID(ctx, ryid)
}

// Update updates a RegistryOperator
func (s *RegistryOperatorService) Update(ctx context.Context, ry *entities.RegistryOperator) (*entities.RegistryOperator, error) {
	return s.ryRepo.Update(ctx, ry)
}

// DeleteByRyID deletes a RegistryOperator by its RyID
func (s *RegistryOperatorService) DeleteByRyID(ctx context.Context, ryid string) error {
	return s.ryRepo.DeleteByRyID(ctx, ryid)
}

// List retrieves RegistryOperators
func (s *RegistryOperatorService) List(ctx context.Context, params queries.ListItemsQuery) ([]*entities.RegistryOperator, string, error) {
	return s.ryRepo.List(ctx, params)
}
