package interfaces

import (
	"context"

	"github.com/onasunnymorning/domain-os/internal/application/commands"
	"github.com/onasunnymorning/domain-os/internal/domain/entities"
)

// FeeService is the interface for the FeeService
type FeeService interface {
	CreateFee(ctx context.Context, cmd *commands.CreateFeeCommand) (*entities.Fee, error)
	ListFees(ctx context.Context, phaseName, TLDName string) ([]entities.Fee, error)
	DeleteFee(ctx context.Context, phaseName, TLDName, feeName, currency string) error
}
