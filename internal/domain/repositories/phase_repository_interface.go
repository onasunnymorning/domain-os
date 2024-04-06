package repositories

import (
	"context"

	"github.com/onasunnymorning/domain-os/internal/domain/entities"
)

// PhaseRepository is the interface that wraps the basic Phase repository methods
type PhaseRepository interface {
	CreatePhase(ctx context.Context, phase *entities.Phase) (*entities.Phase, error)
	GetPhaseByTLDAndName(ctx context.Context, tld, name string) (*entities.Phase, error)
	DeletePhaseByTLDAndName(ctx context.Context, tld, name string) error
	UpdatePhase(ctx context.Context, phase *entities.Phase) (*entities.Phase, error)
	ListPhasesByTLD(ctx context.Context, tld string, pageSize int, pageCursor string) ([]*entities.Phase, error)
}
