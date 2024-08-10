package interfaces

import (
	"context"

	"github.com/onasunnymorning/domain-os/internal/domain/entities"
)

// IANARegistrarService is a service for managing IANA & ICANN Accredited Registrars
// IANARegistrarService defines the IANARegistrarService interface
type IANARegistrarService interface {
	List(ctx context.Context, pageSize int, pageCursor, nameSearchString, status string) ([]*entities.IANARegistrar, error)
	GetByGurID(ctx context.Context, gurID int) (*entities.IANARegistrar, error)
	Count(ctx context.Context) (int, error)
}
