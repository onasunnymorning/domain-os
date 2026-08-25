package repositories

import (
	"context"

	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
	"github.com/onasunnymorning/domain-os/pkg/domain/queries"
)

// Spec5LabelRepository is the interface for the Spec5LabelRepository
// This is our own internal repository
type Spec5LabelRepository interface {
	UpdateAll(ctx context.Context, labels []*entities.Spec5Label) error
	List(ctx context.Context, params queries.ListItemsQuery) ([]*entities.Spec5Label, string, error)
}
