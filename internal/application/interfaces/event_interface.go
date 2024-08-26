package interfaces

import "github.com/onasunnymorning/domain-os/internal/domain/entities"

// EventService is the interface that defines the methods that the event service should implement
type EventService interface {
	SendStream(event *entities.Event) error
}
