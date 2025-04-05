package services

import (
	"context"

	"github.com/onasunnymorning/domain-os/internal/application/queries"
	"github.com/onasunnymorning/domain-os/internal/domain/entities"
	"github.com/onasunnymorning/domain-os/internal/domain/repositories"
)

// Spec5Service implements the Spec5Service interface
type Spec5Service struct {
	spec5Repository repositories.Spec5LabelRepository
}

// NewSpec5Service returns a new Spec5Service
func NewSpec5Service(spec5Repo repositories.Spec5LabelRepository) *Spec5Service {
	return &Spec5Service{
		spec5Repository: spec5Repo,
	}
}

// ListAll lists all the Spec5Labels
func (s *Spec5Service) List(ctx context.Context, params queries.ListItemsQuery) ([]*entities.Spec5Label, string, error) {
	return s.spec5Repository.List(ctx, params)
}
