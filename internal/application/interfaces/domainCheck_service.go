package interfaces

import (
	"context"

	"github.com/onasunnymorning/domain-os/internal/application/queries"
)

// DomainCheckService is the interface for the DomainCheckService
type DomainCheckService interface {
	CheckDomain(ctx context.Context, q *queries.DomainCheckQuery) (*queries.DomainCheckResult, error)
}
