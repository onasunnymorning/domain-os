package repositories

import (
	"context"

	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
)

// EventRepository provides filtered, paginated event queries.
type EventRepository interface {
	SearchEvents(ctx context.Context, filter entities.EventSearchFilter) (*entities.EventSearchResult, error)
}
