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

// Events the operator's decisions about applications publish.
const (
	EventApplicationValidated = "launch.application.validated"
	EventApplicationInvalid   = "launch.application.invalid"
	EventApplicationAllocated = "launch.application.allocated"
	EventApplicationRejected  = "launch.application.rejected"
)

// ErrReasonRequired: an application is made invalid or rejected without a
// reason.
var ErrReasonRequired = errors.New("a reason is required")

// ApplicationRegistrar registers an allocated application's name:
// DomainService.RegisterApplication.
type ApplicationRegistrar interface {
	RegisterApplication(ctx context.Context, app *entities.LaunchApplication, phase *entities.Phase) (*entities.Domain, error)
}

// LaunchAllocationService implements interfaces.LaunchApplicationService:
// the operator reads the applications filed in an application phase and
// decides them (ADR 0024 §3). There is no allocation workflow: the decision
// (auction, lottery, manual) is the operator's, made through the admin API.
type LaunchAllocationService struct {
	applications repositories.LaunchApplicationRepository
	phases       repositories.PhaseRepository
	tlds         repositories.TLDRepository
	registrar    ApplicationRegistrar
	tx           repositories.Transactor
	events       repositories.EventPublisher
	now          func() time.Time
}

// NewLaunchAllocationService returns the service. registrar registers an
// allocated name (DomainService); tx runs an allocation, its rejections and
// their poll messages in one transaction; events may be nil.
func NewLaunchAllocationService(
	applications repositories.LaunchApplicationRepository,
	phases repositories.PhaseRepository,
	tlds repositories.TLDRepository,
	registrar ApplicationRegistrar,
	tx repositories.Transactor,
	events repositories.EventPublisher,
) *LaunchAllocationService {
	return &LaunchAllocationService{
		applications: applications, phases: phases, tlds: tlds, registrar: registrar, tx: tx, events: events, now: time.Now,
	}
}

// GetApplication implements interfaces.LaunchApplicationService.
func (s *LaunchAllocationService) GetApplication(ctx context.Context, scope entities.RegistryScope, id string) (*entities.LaunchApplication, error) {
	return s.inScope(ctx, scope, id)
}

// ListApplications implements interfaces.LaunchApplicationService.
func (s *LaunchAllocationService) ListApplications(ctx context.Context, scope entities.RegistryScope, tld, phase string, status entities.ApplicationStatus) ([]*entities.LaunchApplication, error) {
	if err := s.tldInScope(ctx, scope, tld); err != nil {
		return nil, err
	}
	if status != "" && !status.IsValid() {
		return nil, fmt.Errorf("%w: status %q", entities.ErrInvalidApplication, status)
	}
	return s.applications.ListApplicationsByPhase(ctx, tld, phase, status)
}

// Validate implements interfaces.LaunchApplicationService.
func (s *LaunchAllocationService) Validate(ctx context.Context, scope entities.RegistryScope, cmd *commands.ValidateApplicationCommand) (*entities.LaunchApplication, error) {
	app, err := s.inScope(ctx, scope, cmd.ApplicationID)
	if err != nil {
		return nil, err
	}
	before := app.DeepCopy()
	event := EventApplicationValidated
	if cmd.Valid {
		err = app.Validated()
	} else {
		if cmd.Reason == "" {
			return nil, ErrReasonRequired
		}
		event = EventApplicationInvalid
		err = app.Invalidate(cmd.Reason)
	}
	if err != nil {
		return nil, err
	}
	return s.write(ctx, event, cmd, before, app)
}

// Reject implements interfaces.LaunchApplicationService.
func (s *LaunchAllocationService) Reject(ctx context.Context, scope entities.RegistryScope, cmd *commands.RejectApplicationCommand) (*entities.LaunchApplication, error) {
	if cmd.Reason == "" {
		return nil, ErrReasonRequired
	}
	app, err := s.inScope(ctx, scope, cmd.ApplicationID)
	if err != nil {
		return nil, err
	}
	before := app.DeepCopy()
	if err := app.Reject(cmd.Reason); err != nil {
		return nil, err
	}
	return s.write(ctx, EventApplicationRejected, cmd, before, app)
}

// write stores app, a decided application, queues its applicant's message
// and publishes the event, in one transaction.
func (s *LaunchAllocationService) write(ctx context.Context, event string, cmd, before interface{}, app *entities.LaunchApplication) (_ *entities.LaunchApplication, err error) {
	ctx, finish := s.tx.Begin(ctx)
	defer func() { err = finish(err) }()
	stored, err := s.applications.UpdateApplicationIfUnchanged(ctx, app)
	if err != nil {
		return nil, err
	}
	if err := s.applications.QueuePollMessages(ctx, []*entities.PollMessage{entities.NewApplicationPollMessage(stored, s.now().UTC())}); err != nil {
		return nil, err
	}
	s.publish(ctx, event, cmd, before, stored)
	return stored, nil
}

// Allocate implements interfaces.LaunchApplicationService. The domain is
// registered through the registration path (DomainService), which prices
// nothing again: the quote from filing is billed and recorded. The other
// open applications for the name, of every registrar, are rejected, and
// every applicant is told, in the same transaction.
func (s *LaunchAllocationService) Allocate(ctx context.Context, scope entities.RegistryScope, cmd *commands.AllocateApplicationCommand) (_ *entities.LaunchApplication, err error) {
	app, err := s.inScope(ctx, scope, cmd.ApplicationID)
	if err != nil {
		return nil, err
	}
	if app.Status != entities.ApplicationStatusPendingAllocation {
		return nil, fmt.Errorf("%w: %s is %s, not pendingAllocation", entities.ErrApplicationTransition, app.ID, app.Status)
	}
	phase, err := s.phases.GetPhaseByTLDAndName(ctx, app.TLDName.String(), app.PhaseName.String())
	if err != nil {
		return nil, fmt.Errorf("read phase %s of %s: %w", app.PhaseName, app.TLDName, err)
	}
	before := app.DeepCopy()
	ctx, finish := s.tx.Begin(ctx)
	defer func() { err = finish(err) }()

	dom, err := s.registrar.RegisterApplication(ctx, app, phase)
	if err != nil {
		return nil, err
	}
	if err := app.Allocate(dom.RoID); err != nil {
		return nil, err
	}
	winner, err := s.applications.UpdateApplicationIfUnchanged(ctx, app)
	if err != nil {
		return nil, err
	}
	now := s.now().UTC()
	msgs := []*entities.PollMessage{entities.NewApplicationPollMessage(winner, now)}
	others, err := s.applications.ListApplicationsByName(ctx, app.Name.String())
	if err != nil {
		return nil, err
	}
	reason := fmt.Sprintf("%s was allocated to another application", app.Name)
	for _, o := range others {
		if o.ID == winner.ID || o.Status.IsFinal() {
			continue
		}
		rejectedBefore := o.DeepCopy()
		if err := o.Reject(reason); err != nil {
			return nil, err
		}
		rejected, err := s.applications.UpdateApplicationIfUnchanged(ctx, o)
		if err != nil {
			return nil, err
		}
		msgs = append(msgs, entities.NewApplicationPollMessage(rejected, now))
		s.publish(ctx, EventApplicationRejected, cmd, rejectedBefore, rejected)
	}
	if err := s.applications.QueuePollMessages(ctx, msgs); err != nil {
		return nil, err
	}
	s.publish(ctx, EventApplicationAllocated, cmd, before, winner)
	return winner, nil
}

// inScope returns the application with ID id if its TLD is within scope:
// an operator scope reaches only its own TLDs (INV-02), and another
// operator's TLD is entities.ErrTLDNotFound.
func (s *LaunchAllocationService) inScope(ctx context.Context, scope entities.RegistryScope, id string) (*entities.LaunchApplication, error) {
	if err := scope.Validate(); err != nil {
		return nil, err
	}
	app, err := s.applications.GetApplicationByID(ctx, entities.RoidType(id))
	if err != nil {
		return nil, err
	}
	if err := s.tldInScope(ctx, scope, app.TLDName.String()); err != nil {
		return nil, err
	}
	return app, nil
}

func (s *LaunchAllocationService) tldInScope(ctx context.Context, scope entities.RegistryScope, tld string) error {
	if err := scope.Validate(); err != nil {
		return err
	}
	if scope.IsPlatform() {
		return nil
	}
	_, err := s.tlds.GetByNameForOperator(ctx, scope.Operator(), tld)
	return err
}

func (s *LaunchAllocationService) publish(ctx context.Context, eventType string, command, before, after interface{}) {
	if s.events == nil {
		return
	}
	app := after.(*entities.LaunchApplication)
	ev := entities.NewDomainEvent("domain-os/api", eventType, app.ID.String(),
		fmt.Sprintf("Application %s for %s by %s is %s", app.ID, app.Name, app.ClID, app.Status),
		map[string]interface{}{"application_id": app.ID.String(), "domain_name": app.Name.String(), "phase": app.PhaseName.String(), "clid": app.ClID.String(), "timestamp": s.now().UTC()})
	if traceID, ok := appcontext.TraceID(ctx); ok {
		ev.TraceID = traceID
	}
	if correlationID, ok := appcontext.CorrelationID(ctx); ok {
		ev.CorrelationID = correlationID
	}
	if actor, ok := appcontext.UserID(ctx); ok {
		ev.Actor = actor
	}
	ev.Command, ev.BeforeState, ev.AfterState = command, before, after
	if err := s.events.Publish(ctx, ev); err != nil {
		log.Printf("failed to publish %s: %v", eventType, err)
	}
}
