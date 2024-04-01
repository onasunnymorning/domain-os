package repositories

import (
	"context"

	"github.com/onasunnymorning/domain-os/internal/domain/entities"
)

// PhaseRepository is the interface that wraps the basic Phase repository methods
type PhaseRepository interface {
	CreatePhase(ctx context.Context, phase *entities.Phase) (*entities.Phase, error)
	GetPhaseByName(ctx context.Context, name string) (*entities.Phase, error)
	DeletePhaseByName(ctx context.Context, name string) error
	UpdatePhase(ctx context.Context, phase *entities.Phase) (*entities.Phase, error)
}
