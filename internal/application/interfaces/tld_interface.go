package interfaces

import (
	"context"
	"github.com/onasunnymorning/domain-os/internal/application/commands"
	"github.com/onasunnymorning/domain-os/internal/domain/entities"
)

type TLDService interface {
	CreateTLD(ctx context.Context, cmd *commands.CreateTLDCommand) (*entities.TLD, error)
	GetTLDByName(ctx context.Context, name string, preloadAll bool) (*entities.TLD, error)
	ListTLDs(ctx context.Context, pageSize int, pageCursor string) ([]*entities.TLD, error)
	DeleteTLDByName(ctx context.Context, name string) error
	GetTLDHeader(ctx context.Context, name string) (*entities.TLDHeader, error)
	CountTLDs(ctx context.Context) (int64, error)
	SetAllowEscrowImport(ctx context.Context, name string, allowEscrowImport bool) (*entities.TLD, error)
}
