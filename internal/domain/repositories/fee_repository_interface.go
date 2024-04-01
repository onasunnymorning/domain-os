package repositories

import (
	"context"

	"github.com/onasunnymorning/domain-os/internal/domain/entities"
)

// FeeRepository is the interface for the FeeRepository
type FeeRepository interface {
	CreateFee(ctx context.Context, fee *entities.Fee) (*entities.Fee, error)
	GetFee(ctx context.Context, phaseID int64, name, currency string) (*entities.Fee, error)
	DeleteFee(ctx context.Context, phaseID int64, name, currency string) error
}
