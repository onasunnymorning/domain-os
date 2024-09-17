package interfaces

import (
	"context"

	"github.com/onasunnymorning/domain-os/internal/domain/entities"
)

type WhoisService interface {
	GetDomainWhois(ctx context.Context, dn string) (*entities.WhoisResponse, error)
}
