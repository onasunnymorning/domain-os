package repositories

import (
	"context"

	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
)

type EventPublisher interface {
	Publish(ctx context.Context, events ...entities.DomainEvent) error
}
