package services

import (
	"errors"

	"github.com/onasunnymorning/domain-os/internal/application/commands"
	"github.com/onasunnymorning/domain-os/internal/domain/entities"
	"github.com/onasunnymorning/domain-os/internal/domain/repositories"
	"golang.org/x/net/context"
)

// PhaseService is the implementation of the PhaseService interface
type PhaseService struct {
	tldRepo   repositories.TLDRepository
	phaseRepo repositories.PhaseRepository
}

// NewPhaseService returns a new instance of PhaseService
func NewPhaseService(phaseRepo repositories.PhaseRepository, tldRepo repositories.TLDRepository) *PhaseService {
	return &PhaseService{
		tldRepo:   tldRepo,
		phaseRepo: phaseRepo,
	}
}

// CreatePhase handles the creation of a new phase
func (svc *PhaseService) CreatePhase(ctx context.Context, cmd *commands.CreatePhaseCommand) (*entities.Phase, error) {
	newPhase, err := entities.NewPhase(cmd.Name, cmd.Type, cmd.Starts)
	if err != nil {
		return nil, errors.Join(entities.ErrInvalidPhase, err)
	}
	// If and End is provided, set it
	if cmd.Ends != nil {
		newPhase.Ends = cmd.Ends
	}
	// Set the TLDName on the phase
	newPhase.TLDName = entities.DomainName(cmd.TLDName)

	// Pass through our entity for validation

	// Get the TLD
	tld, err := svc.tldRepo.GetByName(ctx, cmd.TLDName, false)
	if err != nil {
		return nil, err
	}
	// See if we can add the phase to the TLD
	err = tld.AddPhase(newPhase)
	if err != nil {
		return nil, errors.Join(entities.ErrInvalidPhase, err)
	}

	// If we were able to add the phase to the TLD, save the Phase to the repository
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
func (svc *PhaseService) DeletePhaseByTLDAndName(ctx context.Context, tldName, name string) error {
	tld, err := svc.tldRepo.GetByName(ctx, tldName, false)
	if err != nil {
		// If the TLD is not found, there aren't any phases, so we return nil to stay idempotent
		if errors.Is(err, entities.ErrTLDNotFound) {
			return nil
		}
		return err
	}

	// Use our Entity functions to delete the phase
	err = tld.DeletePhase(entities.ClIDType(name))
	if err != nil {
		return err
	}

	// If there were no errors, remove the phase from the repository
	return svc.phaseRepo.DeletePhaseByTLDAndName(ctx, tldName, name)
}

// ListPhasesByTLD retrieves all phases for a TLD
func (svc *PhaseService) ListPhasesByTLD(ctx context.Context, tld string, pageSize int, pageCursor string) ([]*entities.Phase, error) {
	return svc.phaseRepo.ListPhasesByTLD(ctx, tld, pageSize, pageCursor)
}

// ListActivePhasesByTLD retrieves all active phases for a TLD
func (svc *PhaseService) ListActivePhasesByTLD(ctx context.Context, tld string, pageSize int, pageCursor string) ([]*entities.Phase, error) {
	phases, err := svc.phaseRepo.ListPhasesByTLD(ctx, tld, pageSize, pageCursor)
	if err != nil {
		return nil, err
	}

	activePhases := make([]*entities.Phase, 0)
	for _, phase := range phases {
		if phase.IsCurrentlyActive() {
			activePhases = append(activePhases, phase)
		}
	}

	return activePhases, nil
}

// ListActiveGAPhases retrieves all active General Availability phases
func (svc *PhaseService) ListActiveGAPhases(ctx context.Context, pageSize int, pageCursor string) ([]*entities.Phase, error) {
	return svc.phaseRepo.ListActiveGAPhases(ctx, pageSize, pageCursor)
}

// EndPhase Sets or updates the enddate on a phase
func (svc *PhaseService) EndPhase(ctx context.Context, cmd *commands.EndPhaseCommand) (*entities.Phase, error) {
	// Get the TLD
	tld, err := svc.tldRepo.GetByName(ctx, cmd.TLDName, false)
	if err != nil {
		return nil, err
	}

	// Use our domain functions to set the end and catch any errors
	endedPhase, err := tld.EndPhase(entities.ClIDType(cmd.PhaseName), cmd.Ends)
	if err != nil {
		return nil, err
	}

	// If there are no conflicts, save to the repository
	updatedPhase, err := svc.phaseRepo.UpdatePhase(ctx, endedPhase)
	if err != nil {
		return nil, err
	}

	// if all is fine, return the updated phase

	return updatedPhase, nil
}

// UpdatePhase updates a phase
func (svc *PhaseService) UpdatePhase(ctx context.Context, phase *entities.Phase) (*entities.Phase, error) {
	return svc.phaseRepo.UpdatePhase(ctx, phase)
}
