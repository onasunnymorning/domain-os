package interfaces

import (
	"context"

	"github.com/onasunnymorning/domain-os/internal/domain/entities"
)

// AccreditationService is the interface for the AccreditationService
type AccreditationService interface {
	CreateAccreditation(ctx context.Context, tldName, rarClID string) error
	DeleteAccreditation(ctx context.Context, tldName, rarClID string) error
	ListTLDRegistrars(ctx context.Context, pageSize int, cursor string, tldName string) ([]*entities.Registrar, error)
	ListRegistrarTLDs(ctx context.Context, pageSize int, cursor string, rarClID string) ([]*entities.TLD, error)
}
