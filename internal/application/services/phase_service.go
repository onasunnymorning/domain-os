package services

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/onasunnymorning/domain-os/internal/appcontext"
	"github.com/onasunnymorning/domain-os/internal/application/commands"
	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
	"github.com/onasunnymorning/domain-os/pkg/domain/repositories"
)

// PhaseService is the implementation of the PhaseService interface
type PhaseService struct {
	tldRepo        repositories.TLDRepository
	phaseRepo      repositories.PhaseRepository
	lifecycleRepo  repositories.LifecyclePolicyRepository
	applications   repositories.LaunchApplicationRepository
	eventPublisher repositories.EventPublisher
}

// NewPhaseService returns a new instance of PhaseService. lifecycleRepo
// answers which lifecycle policies a phase may name (ADR 0023);
// applications answers whether a launch phase has applications filed (ADR
// 0024), which freezes its registration model; nil counts none.
func NewPhaseService(
	phaseRepo repositories.PhaseRepository,
	tldRepo repositories.TLDRepository,
	lifecycleRepo repositories.LifecyclePolicyRepository,
	applications repositories.LaunchApplicationRepository,
	eventPublisher repositories.EventPublisher,
) *PhaseService {
	return &PhaseService{
		tldRepo:        tldRepo,
		phaseRepo:      phaseRepo,
		lifecycleRepo:  lifecycleRepo,
		applications:   applications,
		eventPublisher: eventPublisher,
	}
}

// SetLifecyclePolicy names the lifecycle policy a phase follows, within
// scope (INV-02): an operator scope reaches only its own TLDs, read with
// the operator in the query, and another operator's TLD is
// entities.ErrTLDNotFound. The policy must exist
// (entities.ErrLifecyclePolicyNotFound) and the phase must not have ended
// (entities.ErrUpdateHistoricPhase). The change applies to lifecycle
// decisions made after it; dates already written to domains stay.
func (svc *PhaseService) SetLifecyclePolicy(ctx context.Context, scope entities.RegistryScope, cmd *commands.SetPhaseLifecyclePolicyCommand) (*entities.Phase, error) {
	if err := scope.Validate(); err != nil {
		return nil, err
	}
	if !scope.IsPlatform() {
		if _, err := svc.tldRepo.GetByNameForOperator(ctx, scope.Operator(), cmd.TLDName); err != nil {
			return nil, err
		}
	}
	phase, err := svc.phaseRepo.GetPhaseByTLDAndName(ctx, cmd.TLDName, cmd.PhaseName)
	if err != nil {
		return nil, err
	}
	if _, err := phase.CanUpdate(); err != nil {
		return nil, err
	}
	policy, err := svc.lifecycleRepo.GetPolicy(ctx, cmd.LifecyclePolicy)
	if err != nil {
		return nil, err
	}
	phase.SetLifecyclePolicy(policy)
	return svc.UpdatePhase(ctx, phase)
}

// SetLaunchSettings replaces a launch phase's launch settings (ADR 0024)
// within scope, as SetLifecyclePolicy does. The phase decides what may
// change when (entities.Phase.SetLaunchSettings): nothing on a GA phase or
// an ended one, the mode only before the phase starts, the model only
// before it starts or has applications filed.
func (svc *PhaseService) SetLaunchSettings(ctx context.Context, scope entities.RegistryScope, cmd *commands.SetLaunchSettingsCommand) (*entities.Phase, error) {
	if err := scope.Validate(); err != nil {
		return nil, err
	}
	if !scope.IsPlatform() {
		if _, err := svc.tldRepo.GetByNameForOperator(ctx, scope.Operator(), cmd.TLDName); err != nil {
			return nil, err
		}
	}
	phase, err := svc.phaseRepo.GetPhaseByTLDAndName(ctx, cmd.TLDName, cmd.PhaseName)
	if err != nil {
		return nil, err
	}
	hasApplications := false
	if svc.applications != nil {
		n, err := svc.applications.CountApplicationsByPhase(ctx, cmd.TLDName, cmd.PhaseName)
		if err != nil {
			return nil, fmt.Errorf("count applications in %s/%s: %w", cmd.TLDName, cmd.PhaseName, err)
		}
		hasApplications = n > 0
	}
	if err := phase.SetLaunchSettings(cmd.LaunchSettings, time.Now().UTC(), hasApplications); err != nil {
		return nil, err
	}
	return svc.UpdatePhase(ctx, phase)
}

// CreatePhase handles the creation of a new phase
func (svc *PhaseService) CreatePhase(ctx context.Context, cmd *commands.CreatePhaseCommand) (*entities.Phase, error) {
	newPhase, err := entities.NewPhase(cmd.Name, cmd.Type, cmd.Starts)
	if err != nil {
		return nil, errors.Join(entities.ErrInvalidPhase, err)
	}
	if cmd.Launch != nil {
		// A phase being created has no history to protect (nothing has
		// started under it, no applications exist), so any valid settings
		// go, on a launch phase.
		if newPhase.Type != entities.PhaseTypeLaunch {
			return nil, errors.Join(entities.ErrInvalidPhase, entities.ErrLaunchSettingsOnGA)
		}
		if err := cmd.Launch.Validate(); err != nil {
			return nil, errors.Join(entities.ErrInvalidPhase, err)
		}
		newPhase.Launch = *cmd.Launch
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
func (svc *PhaseService) DeletePhaseByTLDAndName(ctx context.Context, scope entities.RegistryScope, tldName, name string) error {
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
	err = svc.phaseRepo.DeletePhaseByTLDAndName(ctx, scope, tldName, name)
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
	if traceID, ok := appcontext.TraceID(ctx); ok {
		domainEvent.TraceID = traceID
	}
	if correlationID, ok := appcontext.CorrelationID(ctx); ok {
		domainEvent.CorrelationID = correlationID
	}
	domainEvent.Command = command
	domainEvent.BeforeState = previousState
	domainEvent.AfterState = newState
	if actor, ok := appcontext.UserID(ctx); ok {
		domainEvent.Actor = actor
	}

	if err := svc.eventPublisher.Publish(ctx, domainEvent); err != nil {
		log.Printf("failed to publish phase event %s: %v", eventType, err)
	}
}
