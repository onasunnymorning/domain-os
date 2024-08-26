package services

import (
	"github.com/onasunnymorning/domain-os/internal/domain/entities"
	"github.com/onasunnymorning/domain-os/internal/domain/repositories"
)

// EventService implements the EventService interface
type EventService struct {
	eventRepo repositories.EventRepository
}

// NewEventService creates a new EventService
func NewEventService(eventRepo repositories.EventRepository) *EventService {
	return &EventService{
		eventRepo: eventRepo,
	}
}

// Send sends an event
func (svc *EventService) SendStream(event *entities.Event) error {
	return svc.eventRepo.SendStream(event)
}
