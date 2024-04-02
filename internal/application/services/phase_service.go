package services

import (
	"github.com/onasunnymorning/domain-os/internal/application/commands"
	"github.com/onasunnymorning/domain-os/internal/domain/entities"
	"github.com/onasunnymorning/domain-os/internal/domain/repositories"
	"golang.org/x/net/context"
)

// PhaseService is the implementation of the PhaseService interface
type PhaseService struct {
	phaseRepo repositories.PhaseRepository
}

// NewPhaseService returns a new instance of PhaseService
func NewPhaseService(phaseRepo repositories.PhaseRepository) *PhaseService {
	return &PhaseService{
		phaseRepo: phaseRepo,
	}
}

// CreatePhase handles the creation of a new phase
func (svc *PhaseService) CreatePhase(ctx context.Context, cmd *commands.CreatePhaseCommand) (*entities.Phase, error) {
	newPhase, err := entities.NewPhase(cmd.Name, cmd.Type, cmd.Starts)
	if err != nil {
		return nil, err
	}
	// If and End is provided, set it
	if cmd.Ends != nil {
		newPhase.Ends = cmd.Ends
	}

	dbPhase, err := svc.phaseRepo.CreatePhase(ctx, newPhase)
	if err != nil {
		return nil, err
	}

	return dbPhase, nil
}

// GetPhaseByTLDAndName retrieves a phase by its name
func (svc *PhaseService) GetPhaseByTLDAndName(ctx context.Context, tld, name string) (*entities.Phase, error) {
	return svc.phaseRepo.GetPhaseByTLDAndName(ctx, tld, name)
}

// DeletePhaseByTLDAndName deletes a phase by its name
func (svc *PhaseService) DeletePhaseByTLDAndName(ctx context.Context, tld, name string) error {
	return svc.phaseRepo.DeletePhaseByTLDAndName(ctx, tld, name)
}
