package interfaces

import (
	"context"

	"github.com/onasunnymorning/domain-os/internal/domain/entities"
)

// TLDHeaderService is the interface that wraps the basic TLD header service methods
type TLDHeaderService interface {
	GetTLDHeader(ctx context.Context, name string) (*entities.TLDHeader, error)
}
