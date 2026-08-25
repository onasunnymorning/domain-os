package interfaces

import (
	"context"

	"github.com/onasunnymorning/domain-os/internal/application/commands"
	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
	"github.com/onasunnymorning/domain-os/pkg/domain/queries"
)

type TLDService interface {
	CreateTLD(ctx context.Context, cmd *commands.CreateTLDCommand) (*entities.TLD, error)
	GetTLDByName(ctx context.Context, name string, preloadAll bool) (*entities.TLD, error)
	ListTLDs(ctx context.Context, params queries.ListItemsQuery) ([]*entities.TLD, string, error)
	DeleteTLDByName(ctx context.Context, name string) error
	GetTLDHeader(ctx context.Context, name string) (*entities.TLDHeader, error)
	CountTLDs(ctx context.Context, filter queries.ListTldsFilter) (int64, error)
	SetAllowEscrowImport(ctx context.Context, name string, allowEscrowImport bool) (*entities.TLD, error)
}
