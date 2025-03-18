package repositories

import (
	"context"

	"github.com/onasunnymorning/domain-os/internal/application/queries"
	"github.com/onasunnymorning/domain-os/internal/domain/entities"
)

// NNDNRepository defines the interface for interacting with NNDN data storage.
type NNDNRepository interface {
	// CreateNNDN persists a new NNDN object in the repository.
	CreateNNDN(ctx context.Context, nndn *entities.NNDN) (*entities.NNDN, error)

	// GetNNDN retrieves an NNDN object by its ID/Name from the repository.
	GetNNDN(ctx context.Context, name string) (*entities.NNDN, error)

	// UpdateNNDN updates an existing NNDN object in the repository.
	UpdateNNDN(ctx context.Context, nndn *entities.NNDN) (*entities.NNDN, error)

	// DeleteNNDN removes an NNDN object from the repository by its ID/Name.
	DeleteNNDN(ctx context.Context, name string) error

	// ListNNDNs returns a list of NNDN objects, with pagination support.
	ListNNDNs(ctx context.Context, params queries.ListItemsQuery) ([]*entities.NNDN, string, error)

	// Count returns the number of NNDN objects in the repository optionally filtered by the provided query.
	Count(ctx context.Context, filter queries.ListNndnsFilter) (int, error)
}
