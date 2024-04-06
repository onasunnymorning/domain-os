package interfaces

import (
	"context"

	"github.com/onasunnymorning/domain-os/internal/application/commands"
	"github.com/onasunnymorning/domain-os/internal/domain/entities"
)

// PhaseService is the interface for the phase service
type PhaseService interface {
	CreatePhase(ctx context.Context, cmd *commands.CreatePhaseCommand) (*entities.Phase, error)
	GetPhaseByTLDAndName(ctx context.Context, tld, name string) (*entities.Phase, error)
	DeletePhaseByTLDAndName(ctx context.Context, tld, name string) error
	ListPhasesByTLD(ctx context.Context, tld string, pageSize int, pageCursor string) ([]*entities.Phase, error)
	ListActivePhasesByTLD(ctx context.Context, tld string, pageSize int, pageCursor string) ([]*entities.Phase, error)
	EndPhase(ctx context.Context, cmd *commands.EndPhaseCommand) (*entities.Phase, error)
}
