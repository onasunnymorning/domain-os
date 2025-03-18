package services

import (
	"context"
	"errors"
	"fmt"

	"github.com/onasunnymorning/domain-os/internal/application/commands"
	"github.com/onasunnymorning/domain-os/internal/application/queries"
	"github.com/onasunnymorning/domain-os/internal/domain/entities"
	"github.com/onasunnymorning/domain-os/internal/domain/repositories"
	"go.uber.org/zap"
)

// RegistrarService implements the RegistrarService interface
type RegistrarService struct {
	registrarRepository repositories.RegistrarRepository
	logger              *zap.Logger
}

// NewRegistrarService creates a new RegistrarService
func NewRegistrarService(registrarRepository repositories.RegistrarRepository) *RegistrarService {
	logger, _ := zap.NewProduction()
	return &RegistrarService{
		registrarRepository: registrarRepository,
		logger:              logger,
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
	s.logLifecycleEvent(ctx, fmt.Sprintf("registrar %s created", cmd.ClID), event, cmd, createdRar, nil)

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

	// Log the registrar lifecycle events
	event := entities.NewRegistrarLifecycleEvent("", entities.RegistrarEventTypeCreate)
	s.logLifecycleEvent(ctx, fmt.Sprintf("bulk created %d registrars", len(cmds)), event, cmds, rars, nil)

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

	// update the registrar
	updatedRar, err := s.registrarRepository.Update(ctx, rar)
	if err != nil {
		return nil, err
	}

	// Log the registrar lifecycle event
	event := entities.NewRegistrarLifecycleEvent(rar.ClID.String(), entities.RegistrarEventTypeUpdate)
	s.logLifecycleEvent(ctx, fmt.Sprintf("registrar %s updated", rar.ClID), event, rar, updatedRar, previousRar)

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
	s.logLifecycleEvent(ctx, fmt.Sprintf("registrar %s deleted", clid), event, nil, nil, previousRar)

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
	s.logLifecycleEvent(ctx, fmt.Sprintf("registrar %s status set to %s", clid, status), event, registrar, updatedRar, previousRar)

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

	// Check if the registrar is valid
	if err := newRar.Validate(); err != nil {
		return nil, errors.Join(entities.ErrInvalidRegistrar, err)
	}

	return newRar, nil
}

func (s *RegistrarService) logLifecycleEvent(
	ctx context.Context,
	msg string,
	event *entities.RegistrarLifecycleEvent,
	command interface{},
	newState interface{},
	previousState interface{},
) {
	// Populate trace_id and correlation_id if they exist
	if trace_id, ok := ctx.Value("trace_id").(string); ok {
		event.TraceID = trace_id
	}
	if correlation_id, ok := ctx.Value("correlation_id").(string); ok {
		event.CorrelationID = correlation_id
	}
	// Log the domain lifecycle event
	s.logger.Info(
		msg,
		zap.String("event_type", "registrar_lifecycle_event"),
		zap.Any("registrar_lifecycle_event", event),
		zap.Any("command", command),
		zap.Any("new_state", newState),
		zap.Any("previous_state", previousState),
	)
}
