package repositories

import (
	"context"

	"github.com/onasunnymorning/domain-os/internal/domain/entities"
)

type AccreditationRepository interface {
	CreateAccreditation(ctx context.Context, tld *entities.TLD, rar *entities.Registrar) error
	DeleteAccreditation(ctx context.Context, tld *entities.TLD, rar *entities.Registrar) error
	ListTLDRegistrars(ctx context.Context, pageSize int, cursor string, tld *entities.TLD) ([]*entities.Registrar, error)
	ListRegistrarTLDs(ctx context.Context, pageSize int, cursor string, rar *entities.Registrar) ([]*entities.TLD, error)
}
