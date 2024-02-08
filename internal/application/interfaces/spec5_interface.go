package interfaces

import (
	"context"

	"github.com/onasunnymorning/domain-os/internal/domain/entities"
)

// Spec5Service is a service for managing RA Specification 5 labels
// Spec5Service defines the Spec5Service interface
type Spec5Service interface {
	List(ctx context.Context, pageSize int, pageCursor string) ([]*entities.Spec5Label, error)
}
