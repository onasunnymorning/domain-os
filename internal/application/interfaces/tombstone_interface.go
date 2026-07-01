package interfaces

import (
	"context"

	"github.com/onasunnymorning/domain-os/internal/application/queries"
	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
)

// TombstoneService defines the application-level interface for domain tombstone operations.
type TombstoneService interface {
	// GetTombstoneByRoID retrieves a single tombstone by its ROID.
	GetTombstoneByRoID(ctx context.Context, roid string) (*entities.DomainTombstone, error)

	// GetTombstonesByName retrieves all incarnations of a domain name, ordered by PurgedAt DESC.
	GetTombstonesByName(ctx context.Context, name string) ([]*entities.DomainTombstone, error)

	// ListTombstones returns a paginated list of tombstones with optional filters.
	ListTombstones(ctx context.Context, params queries.ListItemsQuery) ([]*entities.DomainTombstone, string, error)

	// CountTombstones returns the number of tombstones matching the given filter.
	CountTombstones(ctx context.Context, filter queries.ListTombstonesFilter) (int64, error)
}
