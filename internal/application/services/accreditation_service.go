package services

import (
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
	"github.com/onasunnymorning/domain-os/pkg/domain/repositories"
	"golang.org/x/net/context"
)

var ErrInvalidAccreditation = errors.New("invalid accreditation")

// AccreditationService implements the AccreditationService interface
type AccreditationService struct {
	accRepo        repositories.AccreditationRepository
	rarRepo        repositories.RegistrarRepository
	tldRepo        repositories.TLDRepository
	eventPublisher repositories.EventPublisher
}

// NewAccreditationService returns a new AccreditationService
func NewAccreditationService(
	accRepo repositories.AccreditationRepository,
	rarRepo repositories.RegistrarRepository,
	tldRepo repositories.TLDRepository,
	eventPublisher repositories.EventPublisher,
) *AccreditationService {
	return &AccreditationService{
		accRepo:        accRepo,
		rarRepo:        rarRepo,
		tldRepo:        tldRepo,
		eventPublisher: eventPublisher,
	}
}

// CreateAccreditation creates an accreditation
func (s *AccreditationService) CreateAccreditation(ctx context.Context, tldName, rarClID string) error {
	// Get the TLD
	tld, err := s.tldRepo.GetByName(ctx, tldName, false)
	if err != nil {
		return errors.Join(ErrInvalidAccreditation, err)
	}

	// Get the Registrar, preloading TLDs
	rar, err := s.rarRepo.GetByClID(ctx, rarClID, true)
	if err != nil {
		return errors.Join(ErrInvalidAccreditation, err)
	}

	// Accredit the Registrar using domain functions
	err = rar.AccreditFor(tld)
	if err != nil {
		return errors.Join(ErrInvalidAccreditation, err)
	}

	// Save the accreditation and return the result
	err = s.accRepo.CreateAccreditation(ctx, tldName, rarClID)
	if err != nil {
		return err
	}

	s.publishAccreditationEvent(ctx, "accreditation.created", fmt.Sprintf("%s/%s", tldName, rarClID), fmt.Sprintf("Registrar %s accredited for TLD %s", rarClID, tldName), map[string]string{"tld": tldName, "registrar": rarClID}, rar, nil)

	return nil
}

// DeleteAccreditation deletes an accreditation
func (s *AccreditationService) DeleteAccreditation(ctx context.Context, tldName, rarClID string) error {
	// Get the TLD
	tld, err := s.tldRepo.GetByName(ctx, tldName, false)
	if err != nil {
		return errors.Join(ErrInvalidAccreditation, err)
	}

	// Get the Registrar, preloading TLDs
	rar, err := s.rarRepo.GetByClID(ctx, rarClID, true)
	if err != nil {
		return errors.Join(ErrInvalidAccreditation, err)
	}

	// Deaccredit the Registrar using domain functions
	err = rar.DeAccreditFor(tld)
	if err != nil {
		return errors.Join(ErrInvalidAccreditation, err)
	}

	// Delete the accreditation and return the result
	err = s.accRepo.DeleteAccreditation(ctx, tldName, rarClID)
	if err != nil {
		return err
	}

	s.publishAccreditationEvent(ctx, "accreditation.deleted", fmt.Sprintf("%s/%s", tldName, rarClID), fmt.Sprintf("Registrar %s de-accredited for TLD %s", rarClID, tldName), map[string]string{"tld": tldName, "registrar": rarClID}, nil, rar)

	return nil
}

// ListTLDRegistrars lists the registrars that are accredited for a TLD
func (s *AccreditationService) ListTLDRegistrars(ctx context.Context, pageSize int, pageCursor, tldName string) ([]*entities.Registrar, error) {
	return s.accRepo.ListTLDRegistrars(ctx, pageSize, pageCursor, tldName)
}

// ListRegistrarTLDs lists the TLDs that a registrar is accredited for
func (s *AccreditationService) ListRegistrarTLDs(ctx context.Context, pageSize int, pageCursor, rarClID string) ([]*entities.TLD, error) {
	return s.accRepo.ListRegistrarTLDs(ctx, pageSize, pageCursor, rarClID)
}

// IsRegistrarAccreditedForTLD checks if a registrar is accredited for a TLD. It returns an error if the registrar or TLD is not found or the accredtitiation cannot be determined
func (s *AccreditationService) IsRegistrarAccreditedForTLD(ctx context.Context, tldName, rarClID string) (bool, error) {
	return s.rarRepo.IsRegistrarAccreditedForTLD(ctx, tldName, rarClID)
}

func (s *AccreditationService) publishAccreditationEvent(
	ctx context.Context,
	eventType string,
	subject string,
	msg string,
	command interface{},
	newState interface{},
	previousState interface{},
) {
	if s.eventPublisher == nil {
		return
	}
	data := map[string]interface{}{
		"accreditation_id": subject,
		"timestamp":        time.Now().UTC(),
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

	if err := s.eventPublisher.Publish(ctx, domainEvent); err != nil {
		log.Printf("failed to publish accreditation event %s: %v", eventType, err)
	}
}
