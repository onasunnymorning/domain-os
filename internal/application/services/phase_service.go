package services

import (
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/onasunnymorning/domain-os/internal/application/commands"
	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
	"github.com/onasunnymorning/domain-os/pkg/domain/repositories"
	"golang.org/x/net/context"
)

// PhaseService is the implementation of the PhaseService interface
type PhaseService struct {
	tldRepo        repositories.TLDRepository
	phaseRepo      repositories.PhaseRepository
	eventPublisher repositories.EventPublisher
}

// NewPhaseService returns a new instance of PhaseService
func NewPhaseService(
	phaseRepo repositories.PhaseRepository,
	tldRepo repositories.TLDRepository,
	eventPublisher repositories.EventPublisher,
) *PhaseService {
	return &PhaseService{
		tldRepo:        tldRepo,
		phaseRepo:      phaseRepo,
		eventPublisher: eventPublisher,
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

	svc.publishPhaseEvent(ctx, "phase.created", fmt.Sprintf("%s/%s", dbPhase.TLDName.String(), dbPhase.Name.String()), fmt.Sprintf("Phase %s for TLD %s created", dbPhase.Name.String(), dbPhase.TLDName.String()), cmd, dbPhase, nil)

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

	prevPhase, err := svc.phaseRepo.GetPhaseByTLDAndName(ctx, tldName, name)
	if err != nil {
		return err
	}

	// Use our Entity functions to delete the phase
	err = tld.DeletePhase(entities.ClIDType(name))
	if err != nil {
		return err
	}

	// If there were no errors, remove the phase from the repository
	err = svc.phaseRepo.DeletePhaseByTLDAndName(ctx, tldName, name)
	if err != nil {
		return err
	}

	svc.publishPhaseEvent(ctx, "phase.deleted", fmt.Sprintf("%s/%s", tldName, name), fmt.Sprintf("Phase %s for TLD %s deleted", name, tldName), nil, nil, prevPhase)
	return nil
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

	prevPhase, err := svc.phaseRepo.GetPhaseByTLDAndName(ctx, cmd.TLDName, cmd.PhaseName)
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

	svc.publishPhaseEvent(ctx, "phase.ended", fmt.Sprintf("%s/%s", updatedPhase.TLDName.String(), updatedPhase.Name.String()), fmt.Sprintf("Phase %s for TLD %s ended", updatedPhase.Name.String(), updatedPhase.TLDName.String()), cmd, updatedPhase, prevPhase)

	return updatedPhase, nil
}

// UpdatePhase updates a phase
func (svc *PhaseService) UpdatePhase(ctx context.Context, phase *entities.Phase) (*entities.Phase, error) {
	prevPhase, err := svc.phaseRepo.GetPhaseByTLDAndName(ctx, phase.TLDName.String(), phase.Name.String())
	if err != nil {
		return nil, err
	}

	updated, err := svc.phaseRepo.UpdatePhase(ctx, phase)
	if err != nil {
		return nil, err
	}

	svc.publishPhaseEvent(ctx, "phase.updated", fmt.Sprintf("%s/%s", updated.TLDName.String(), updated.Name.String()), fmt.Sprintf("Phase %s for TLD %s updated", updated.Name.String(), updated.TLDName.String()), nil, updated, prevPhase)

	return updated, nil
}

func (svc *PhaseService) publishPhaseEvent(
	ctx context.Context,
	eventType string,
	subject string,
	msg string,
	command interface{},
	newState interface{},
	previousState interface{},
) {
	if svc.eventPublisher == nil {
		return
	}
	data := map[string]interface{}{
		"phase_id":  subject,
		"timestamp": time.Now().UTC(),
	}

	domainEvent := entities.NewDomainEvent(
		"domain-os/api",
		eventType,
		subject,
		msg,
		data,
	)
	if traceID, ok := ctx.Value("trace_id").(string); ok {
		domainEvent.TraceID = traceID
	}
	if correlationID, ok := ctx.Value("correlation_id").(string); ok {
		domainEvent.CorrelationID = correlationID
	}
	domainEvent.Command = command
	domainEvent.BeforeState = previousState
	domainEvent.AfterState = newState
	if actor, ok := ctx.Value("userid").(string); ok {
		domainEvent.Actor = actor
	}

	if err := svc.eventPublisher.Publish(ctx, domainEvent); err != nil {
		log.Printf("failed to publish phase event %s: %v", eventType, err)
	}
}
