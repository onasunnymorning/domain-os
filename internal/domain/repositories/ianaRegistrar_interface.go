package repositories

import (
	"context"

	"github.com/onasunnymorning/domain-os/internal/domain/entities"
)

// IANARegistrarRepository is the interface for the IANARegistrarRepository
type IANARegistrarRepository interface {
	UpdateAll(ctx context.Context, registrars []*entities.IANARegistrar) error
	List(ctx context.Context, pageSize int, pageCursor, nameSearchString, status string) ([]*entities.IANARegistrar, error)
	GetByGurID(ctx context.Context, gurID int) (*entities.IANARegistrar, error)
	Count(ctx context.Context) (int, error)
}
