package repositories

import (
	"context"

	"github.com/onasunnymorning/domain-os/internal/domain/entities"
)

// Spec5LabelRepository is the interface for the Spec5LabelRepository
// This is our own internal repository
type Spec5LabelRepository interface {
	UpdateAll([]*entities.Spec5Label) error
	List(ctx context.Context, pageSize int, pageCursor string) ([]*entities.Spec5Label, error)
}
