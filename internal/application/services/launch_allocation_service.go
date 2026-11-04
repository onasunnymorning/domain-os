package services

import (
	"context"
	"errors"
	"fmt"
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
	contacts     repositories.ContactRepository
	hosts        repositories.HostRepository
	registrar    ApplicationRegistrar
	tx           repositories.Transactor
	events       repositories.EventPublisher
	now          func() time.Time
}

// NewLaunchAllocationService returns the service. contacts and hosts check,
// at allocation, that the application's contacts and nameservers are still
// the applicant's; registrar registers an allocated name (DomainService);
// tx runs an allocation, its rejections and their poll messages in one
// transaction; events may be nil.
func NewLaunchAllocationService(
	applications repositories.LaunchApplicationRepository,
	phases repositories.PhaseRepository,
	tlds repositories.TLDRepository,
	contacts repositories.ContactRepository,
	hosts repositories.HostRepository,
	registrar ApplicationRegistrar,
	tx repositories.Transactor,
	events repositories.EventPublisher,
) *LaunchAllocationService {
	return &LaunchAllocationService{
		applications: applications, phases: phases, tlds: tlds, contacts: contacts, hosts: hosts, registrar: registrar, tx: tx, events: events, now: time.Now,
	}
}

// GetApplication implements interfaces.LaunchApplicationService.
func (s *LaunchAllocationService) GetApplication(ctx context.Context, scope entities.RegistryScope, id string) (*entities.LaunchApplication, error) {
	return s.inScope(ctx, scope, id)
}

// ListApplications implements interfaces.LaunchApplicationService.
func (s *LaunchAllocationService) ListApplications(ctx context.Context, scope entities.RegistryScope, tld, phase string, status entities.ApplicationStatus, pageSize int, pageCursor string) ([]*entities.LaunchApplication, error) {
	if err := s.tldInScope(ctx, scope, tld); err != nil {
		return nil, err
	}
	if status != "" && !status.IsValid() {
		return nil, fmt.Errorf("%w: status %q", entities.ErrInvalidApplication, status)
	}
	return s.applications.ListApplicationsByPhase(ctx, tld, phase, status, pageSize, pageCursor)
}

// Validate implements interfaces.LaunchApplicationService.
func (s *LaunchAllocationService) Validate(ctx context.Context, scope entities.RegistryScope, cmd *commands.ValidateApplicationCommand) (*entities.LaunchApplication, error) {
	app, err := s.inScope(ctx, scope, cmd.ApplicationID)
	if err != nil {
		return nil, err
	}
	if cmd.Valid == nil {
		return nil, entities.ErrDecisionRequired
	}
	before := app.DeepCopy()
	event := EventApplicationValidated
	if *cmd.Valid {
		err = app.Validated()
	} else {
		if cmd.Reason == "" {
			return nil, entities.ErrReasonRequired
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
		return nil, entities.ErrReasonRequired
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

// begin opens the decision's transaction, as DomainService.begin does: done
// commits it when the decision returned no error, rolls it back on an error
// or a panic (which it re-raises), and reports the commit's error.
func (s *LaunchAllocationService) begin(ctx context.Context) (context.Context, func(*error)) {
	ctx, finish := s.tx.Begin(ctx)
	return ctx, func(errp *error) {
		if r := recover(); r != nil {
			_ = finish(fmt.Errorf("panic: %v", r))
			panic(r)
		}
		*errp = finish(*errp)
	}
}

// write stores app, a decided application, queues its applicant's message
// and publishes the event, in one transaction.
func (s *LaunchAllocationService) write(ctx context.Context, event string, cmd, before interface{}, app *entities.LaunchApplication) (_ *entities.LaunchApplication, err error) {
	ctx, done := s.begin(ctx)
	defer done(&err)
	stored, err := s.applications.UpdateApplicationIfUnchanged(ctx, app)
	if err != nil {
		return nil, err
	}
	if err := s.applications.QueuePollMessages(ctx, []*entities.PollMessage{entities.NewApplicationPollMessage(stored, s.now().UTC())}); err != nil {
		return nil, err
	}
	if err := s.publish(ctx, event, cmd, before, stored); err != nil {
		return nil, err
	}
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
	// The contacts were the applicant's when the application was filed; a
	// contact deleted since (open applications count as links, so only an
	// administrator's delete), or re-created by another registrar, must
	// not reach the domain.
	if err := s.contactsStillTheApplicants(ctx, app); err != nil {
		return nil, err
	}
	if err := s.hostsStillTheApplicants(ctx, app); err != nil {
		return nil, err
	}
	before := app.DeepCopy()
	ctx, done := s.begin(ctx)
	defer done(&err)
	// The name's lock (LaunchApplicationRepository.LockName): no filing for
	// the name runs between the registration below and the rejections.
	if err := s.applications.LockName(ctx, app.Name.String()); err != nil {
		return nil, err
	}

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
		if err := s.publish(ctx, EventApplicationRejected, cmd, rejectedBefore, rejected); err != nil {
			return nil, err
		}
	}
	if err := s.applications.QueuePollMessages(ctx, msgs); err != nil {
		return nil, err
	}
	if err := s.publish(ctx, EventApplicationAllocated, cmd, before, winner); err != nil {
		return nil, err
	}
	return winner, nil
}

// contactsStillTheApplicants checks that every contact the application
// names exists and is the applicant's (entities.ErrApplicationContactGone
// otherwise). The registration write then holds them by foreign key.
func (s *LaunchAllocationService) contactsStillTheApplicants(ctx context.Context, app *entities.LaunchApplication) error {
	seen := map[entities.ClIDType]bool{}
	for _, id := range []entities.ClIDType{app.RegistrantID, app.AdminID, app.TechID, app.BillingID} {
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		c, err := s.contacts.GetContactByID(ctx, id.String())
		switch {
		case errors.Is(err, entities.ErrContactNotFound):
			return fmt.Errorf("%w: %s", entities.ErrApplicationContactGone, id)
		case err != nil:
			return fmt.Errorf("look up contact %s: %w", id, err)
		case c.ClID != app.ClID:
			return fmt.Errorf("%w: %s is %s's", entities.ErrApplicationContactGone, id, c.ClID)
		}
	}
	return nil
}

// hostsStillTheApplicants checks that every nameserver the application
// names is still one of the applicant's host objects
// (entities.ErrApplicationHostGone otherwise): an application names hosts
// by name, so a host deleted (by an administrator: open applications count
// as links) or renamed since filing is gone from it.
func (s *LaunchAllocationService) hostsStillTheApplicants(ctx context.Context, app *entities.LaunchApplication) error {
	for _, name := range app.HostNames {
		if _, err := s.hosts.GetHostByNameAndClID(ctx, name, app.ClID.String()); err != nil {
			if errors.Is(err, entities.ErrHostNotFound) {
				return fmt.Errorf("%w: %s", entities.ErrApplicationHostGone, name)
			}
			return fmt.Errorf("look up host %s: %w", name, err)
		}
	}
	return nil
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

// publish records the decision's event in the transaction ctx carries. A
// failure is returned, so the decision rolls back with it (INV-01: the
// change and its outbox event commit together or not at all).
func (s *LaunchAllocationService) publish(ctx context.Context, eventType string, command, before, after interface{}) error {
	if s.events == nil {
		return nil
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
		return fmt.Errorf("publish %s: %w", eventType, err)
	}
	return nil
}
