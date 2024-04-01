package repositories

import (
	"context"

	"github.com/onasunnymorning/domain-os/internal/domain/entities"
)

// PhaseFeeRepository is the interface for the PhaseFeeRepository
type PhaseFeeRepository interface {
	CreateFee(ctx context.Context, fee *entities.Fee) (*entities.Fee, error)
	GetFee(ctx context.Context, phase, name, currency string) (*entities.Fee, error)
	DeleteFee(ctx context.Context, phase, name, currency string) error
}
