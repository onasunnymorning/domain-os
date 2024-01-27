package interfaces

import (
	"context"
	"github.com/onasunnymorning/domain-os/internal/application/commands"
	"github.com/onasunnymorning/domain-os/internal/domain/entities"
)

type NNDNService interface {
	CreateNNDN(ctx context.Context, cmd *commands.CreateNNDNCommand) (*entities.NNDN, error)
	GetNNDNByName(ctx context.Context, name string) (*entities.NNDN, error)
	ListNNDNs(ctx context.Context, pageSize int, pageCursor string) ([]*entities.NNDN, error)
	DeleteNNDNByName(ctx context.Context, name string) error
}
