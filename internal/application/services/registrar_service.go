package services

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/onasunnymorning/domain-os/internal/appcontext"
	"github.com/onasunnymorning/domain-os/internal/application/commands"
	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
	"github.com/onasunnymorning/domain-os/pkg/domain/queries"
	"github.com/onasunnymorning/domain-os/pkg/domain/repositories"
)

// RegistrarService implements the RegistrarService interface
type RegistrarService struct {
	registrarRepository repositories.RegistrarRepository
	eventPublisher      repositories.EventPublisher
}

// NewRegistrarService creates a new RegistrarService
func NewRegistrarService(registrarRepository repositories.RegistrarRepository, ep repositories.EventPublisher) *RegistrarService {
	return &RegistrarService{
		registrarRepository: registrarRepository,
		eventPublisher:      ep,
	}
}

// Create creates a new registrar
func (s *RegistrarService) Create(ctx context.Context, cmd *commands.CreateRegistrarCommand) (*entities.Registrar, error) {
	newRar, err := rarFromCmd(cmd)
	if err != nil {
		return nil, err
	}

	createdRar, err := s.registrarRepository.Create(ctx, newRar)
	if err != nil {
		return nil, err
	}

	// Log the registrar lifecycle event
	event := entities.NewRegistrarLifecycleEvent(createdRar.ClID.String(), entities.RegistrarEventTypeCreate)
	s.publishRegistrarEvent(ctx, "registrar.created", fmt.Sprintf("registrar %s created", cmd.ClID), event, cmd, createdRar, nil)

	return createdRar, nil
}

// Bulk Create new registrars
func (s *RegistrarService) BulkCreate(ctx context.Context, cmds []*commands.CreateRegistrarCommand) error {
	rars, err := bulkRarFromCmd(cmds)
	if err != nil {
		return err
	}

	err = s.registrarRepository.BulkCreate(ctx, rars)
	if err != nil {
		return err
	}

	// Log the registrar lifecycle events, recording which registrars were
	// created so the batch is auditable from the event payload alone.
	clids := make([]string, 0, len(cmds))
	for _, cmd := range cmds {
		clids = append(clids, cmd.ClID)
	}
	event := entities.NewRegistrarLifecycleEvent("", entities.RegistrarEventTypeCreate)
	event.ClientIDs = clids
	s.publishRegistrarEvent(ctx, "registrar.bulk_created", fmt.Sprintf("bulk created %d registrars", len(cmds)), event, cmds, rars, nil)

	return nil
}

// GetByClID returns a registrar by its ClID
func (s *RegistrarService) GetByClID(ctx context.Context, clid string, preloadTLDs bool) (*entities.Registrar, error) {
	return s.registrarRepository.GetByClID(ctx, clid, preloadTLDs)
}

// GetByGurID returns a registrar by its GurID
func (s *RegistrarService) GetByGurID(ctx context.Context, gurID int) (*entities.Registrar, error) {
	return s.registrarRepository.GetByGurID(ctx, gurID)
}

// List returns a list of registrars
func (s *RegistrarService) List(ctx context.Context, params queries.ListItemsQuery) ([]*entities.RegistrarListItem, string, error) {
	return s.registrarRepository.List(ctx, params)
}

// Update updates a registrar
func (s *RegistrarService) Update(ctx context.Context, rar *entities.Registrar) (*entities.Registrar, error) {
	// get the registrar
	registrar, err := s.registrarRepository.GetByClID(ctx, rar.ClID.String(), false)
	if err != nil {
		return nil, err
	}

	// make a copy of the original
	previousRar := registrar.DeepCopy()

	// preserve read-only metadata fields from the previous state
	rar.CreatedAt = previousRar.CreatedAt

	// update the registrar
	updatedRar, err := s.registrarRepository.Update(ctx, rar)
	if err != nil {
		return nil, err
	}

	// Log the registrar lifecycle event
	event := entities.NewRegistrarLifecycleEvent(rar.ClID.String(), entities.RegistrarEventTypeUpdate)
	s.publishRegistrarEvent(ctx, "registrar.updated", fmt.Sprintf("registrar %s updated", rar.ClID), event, rar, updatedRar, previousRar)

	return updatedRar, nil
}

// Delete deletes a registrar by its ClID
func (s *RegistrarService) Delete(ctx context.Context, clid string) error {
	// get the registrar
	previousRar, err := s.registrarRepository.GetByClID(ctx, clid, false)
	if err != nil {
		return err
	}

	// delete the registrar
	err = s.registrarRepository.Delete(ctx, clid)
	if err != nil {
		return err
	}

	// Log the registrar lifecycle event
	event := entities.NewRegistrarLifecycleEvent(clid, entities.RegistrarEventTypeDelete)
	s.publishRegistrarEvent(ctx, "registrar.deleted", fmt.Sprintf("registrar %s deleted", clid), event, nil, nil, previousRar)

	return nil
}

// Count returns the number of registrars
func (s *RegistrarService) Count(ctx context.Context) (int64, error) {
	return s.registrarRepository.Count(ctx)
}

// SetStatus sets the status of a registrar
func (s *RegistrarService) SetStatus(ctx context.Context, clid string, status entities.RegistrarStatus) error {
	// get the registrar
	registrar, err := s.registrarRepository.GetByClID(ctx, clid, false)
	if err != nil {
		return err
	}

	// No-op guard: if the status already matches, skip the write and the
	// lifecycle event. Avoids redundant "status set to X" events on every
	// idempotent sync run (e.g. special reserved registrars re-forced to ok).
	if strings.EqualFold(registrar.Status.String(), string(status)) {
		return nil
	}

	// make a copy of the original
	previousRar := registrar.DeepCopy()

	// set the status using domain logic
	err = registrar.SetStatus(status)
	if err != nil {
		return err
	}

	// save the registrar
	updatedRar, err := s.registrarRepository.Update(ctx, registrar)
	if err != nil {
		return err
	}

	// Log the registrar lifecycle event
	event := entities.NewRegistrarLifecycleEvent(clid, entities.RegistrarEventTypeUpdate)
	s.publishRegistrarEvent(ctx, "registrar.status_updated", fmt.Sprintf("registrar %s status set to %s", clid, status), event, registrar, updatedRar, previousRar)

	return nil
}

// SetIANAStatus sets the IANA status of a registrar
func (s *RegistrarService) SetIANAStatus(ctx context.Context, clid string, status entities.IANARegistrarStatus) error {
	// get the registrar
	registrar, err := s.registrarRepository.GetByClID(ctx, clid, false)
	if err != nil {
		return err
	}

	// make a copy of the original
	previousRar := registrar.DeepCopy()

	// set the IANA status if valid
	if !status.IsValid() {
		return entities.ErrInvalidRegistrarIANAStatus
	}
	registrar.IANAStatus = status

	// save the registrar
	updatedRar, err := s.registrarRepository.Update(ctx, registrar)
	if err != nil {
		return err
	}

	// Log the registrar lifecycle event
	event := entities.NewRegistrarLifecycleEvent(clid, entities.RegistrarEventTypeUpdate)
	s.publishRegistrarEvent(ctx, "registrar.iana_status_updated", fmt.Sprintf("registrar %s IANA status set to %s", clid, status), event, registrar, updatedRar, previousRar)

	return nil
}

// bulkRarFromCmd creates a slice of registrars from a slice of Create Registrar Commands
func bulkRarFromCmd(cmds []*commands.CreateRegistrarCommand) ([]*entities.Registrar, error) {
	var rars []*entities.Registrar
	for _, cmd := range cmds {
		newRar, err := rarFromCmd(cmd)
		if err != nil {
			return nil, err
		}
		rars = append(rars, newRar)
	}
	return rars, nil
}

// registrarFromCommand creates a registrar from a Create Registrar Command
func rarFromCmd(cmd *commands.CreateRegistrarCommand) (*entities.Registrar, error) {
	newRar, err := entities.NewRegistrar(cmd.ClID, cmd.Name, cmd.Email, cmd.GurID, cmd.PostalInfo)
	if err != nil {
		return nil, errors.Join(entities.ErrInvalidRegistrar, err)
	}

	// Add the optional fields
	if cmd.Voice != "" {
		v, err := entities.NewE164Type(cmd.Voice)
		if err != nil {
			return nil, errors.Join(entities.ErrInvalidRegistrar, err)
		}
		newRar.Voice = *v
	}
	if cmd.Fax != "" {
		f, err := entities.NewE164Type(cmd.Fax)
		if err != nil {
			return nil, errors.Join(entities.ErrInvalidRegistrar, err)
		}
		newRar.Fax = *f
	}
	if cmd.URL != "" {
		url, err := entities.NewURL(cmd.URL)
		if err != nil {
			return nil, errors.Join(entities.ErrInvalidRegistrar, err)
		}
		newRar.URL = *url
	}
	if cmd.RdapBaseURL != "" {
		rdapBaseURL, err := entities.NewURL(cmd.RdapBaseURL)
		if err != nil {
			return nil, errors.Join(entities.ErrInvalidRegistrar, err)
		}
		newRar.RdapBaseURL = *rdapBaseURL
	}
	if cmd.WhoisInfo != nil {
		wi, err := entities.NewWhoisInfo(cmd.WhoisInfo.Name.String(), cmd.WhoisInfo.URL.String())
		if err != nil {
			return nil, errors.Join(entities.ErrInvalidRegistrar, err)
		}
		newRar.WhoisInfo = *wi
	}

	// Optional: set initial statuses if provided
	if cmd.Status != "" {
		// normalize and validate
		st := entities.RegistrarStatus(strings.ToLower(cmd.Status))
		if !(&st).IsValid() {
			return nil, errors.Join(entities.ErrInvalidRegistrar, entities.ErrInvalidRegistrarStatus)
		}
		newRar.Status = st
	}
	if cmd.IANAStatus != "" {
		if !cmd.IANAStatus.IsValid() {
			return nil, errors.Join(entities.ErrInvalidRegistrar, entities.ErrInvalidRegistrarIANAStatus)
		}
		newRar.IANAStatus = cmd.IANAStatus
	}

	// Check if the registrar is valid
	if err := newRar.Validate(); err != nil {
		return nil, errors.Join(entities.ErrInvalidRegistrar, err)
	}

	return newRar, nil
}

func (s *RegistrarService) publishRegistrarEvent(
	ctx context.Context,
	eventType string,
	msg string,
	event *entities.RegistrarLifecycleEvent,
	command interface{},
	newState interface{},
	previousState interface{},
) {
	if event == nil {
		return
	}
	// Populate trace_id and correlation_id if they exist
	if traceID, ok := appcontext.TraceID(ctx); ok {
		event.TraceID = traceID
	}
	if correlationID, ok := appcontext.CorrelationID(ctx); ok {
		event.CorrelationID = correlationID
	}

	domainEvent := entities.NewDomainEvent(
		"domain-os/api",
		eventType,
		event.ClientID,
		msg,
		event,
	)
	domainEvent.TraceID = event.TraceID
	domainEvent.CorrelationID = event.CorrelationID
	domainEvent.Command = command
	domainEvent.BeforeState = previousState
	domainEvent.AfterState = newState
	if actor, ok := appcontext.UserID(ctx); ok {
		domainEvent.Actor = actor
	}

	if domainEvent.Subject == "" {
		domainEvent.Subject = "bulk"
	}

	if err := s.eventPublisher.Publish(ctx, domainEvent); err != nil {
		fmt.Printf("failed to publish registrar event %s: %v\n", eventType, err)
	}
}
