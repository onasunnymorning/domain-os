package interfaces

import (
	"github.com/onasunnymorning/domain-os/internal/application/commands"
	"github.com/onasunnymorning/domain-os/internal/domain/entities"
	"golang.org/x/net/context"
)

type TLDService interface {
	CreateTLD(ctx context.Context, cmd *commands.CreateTLDCommand) (*commands.CreateTLDCommandResult, error)
	GetTLDByName(ctx context.Context, name string, preloadAll bool) (*entities.TLD, error)
	ListTLDs(ctx context.Context, pageSize int, pageCursor string) ([]*entities.TLD, error)
	DeleteTLDByName(ctx context.Context, name string) error
	GetTLDHeader(ctx context.Context, name string) (*entities.TLDHeader, error)
	CountTLDs(ctx context.Context) (int64, error)
}
