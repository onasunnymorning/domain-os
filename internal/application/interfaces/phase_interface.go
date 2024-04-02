package interfaces

import (
	"context"

	"github.com/onasunnymorning/domain-os/internal/application/commands"
	"github.com/onasunnymorning/domain-os/internal/domain/entities"
)

// PhaseService is the interface for the phase service
type PhaseService interface {
	CreatePhase(ctx context.Context, cmd *commands.CreatePhaseCommand) (*entities.Phase, error)
	GetPhaseByName(ctx context.Context, name string) (*entities.Phase, error)
	DeletePhaseByName(ctx context.Context, name string) error
}
