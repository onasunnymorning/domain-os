package repositories

import "github.com/onasunnymorning/domain-os/internal/domain/entities"

// EventRepository is the interface that defines the methods that the event repository should implement
type EventRepository interface {
	SendStream(event *entities.Event) error
}
