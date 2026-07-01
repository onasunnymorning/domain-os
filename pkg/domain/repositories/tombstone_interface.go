package repositories

import (
	"context"

	"github.com/onasunnymorning/domain-os/internal/application/queries"
	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
)

// TombstoneRepository defines the interface for interacting with domain tombstone storage.
type TombstoneRepository interface {
	// CreateTombstone persists a new tombstone. Uses UPSERT semantics for backfill idempotency.
	CreateTombstone(ctx context.Context, tombstone *entities.DomainTombstone) (*entities.DomainTombstone, error)

	// GetTombstoneByRoID retrieves a single tombstone by its ROID (primary key).
	GetTombstoneByRoID(ctx context.Context, roid string) (*entities.DomainTombstone, error)

	// GetTombstonesByName retrieves all tombstones for a given domain name (multiple incarnations),
	// ordered by PurgedAt DESC (most recent first).
	GetTombstonesByName(ctx context.Context, name string) ([]*entities.DomainTombstone, error)

	// ListTombstones returns a paginated list of tombstones with optional filters.
	ListTombstones(ctx context.Context, params queries.ListItemsQuery) ([]*entities.DomainTombstone, string, error)

	// CountTombstones returns the number of tombstones matching the given filter.
	CountTombstones(ctx context.Context, filter queries.ListTombstonesFilter) (int64, error)
}
