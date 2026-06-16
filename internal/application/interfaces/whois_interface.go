package interfaces

import (
	"context"

	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
)

type WhoisService interface {
	GetDomainWhois(ctx context.Context, dn string) (*entities.WhoisResponse, error)
}
