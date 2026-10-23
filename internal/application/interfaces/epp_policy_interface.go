package interfaces

import (
	"context"
	"time"

	"github.com/onasunnymorning/domain-os/internal/application/commands"
	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
)

// EPPPolicySource is what an EPP node reads its policies from, every
// refresh interval (docs/epp/ARCHITECTURE.md §5).
type EPPPolicySource interface {
	// PolicySet returns every policy, assignment and family.
	PolicySet(ctx context.Context) (*entities.EPPPolicySet, error)
}

// EPPPolicyService is the admin API's view of EPP policies and registrar
// families (ADR 0019).
type EPPPolicyService interface {
	EPPPolicySource
	ListPolicies(ctx context.Context) ([]*entities.EPPPolicy, error)
	GetPolicy(ctx context.Context, name string) (*entities.EPPPolicy, error)
	SavePolicy(ctx context.Context, name string, cmd commands.SaveEPPPolicyCommand) (*entities.EPPPolicy, error)
	DeletePolicy(ctx context.Context, name string) error
	// RegistrarPolicy is what applies to a registrar.
	RegistrarPolicy(ctx context.Context, clid string) (*entities.EPPRegistrarPolicy, error)
	AssignPolicy(ctx context.Context, clid string, cmd commands.AssignEPPPolicyCommand) (*entities.EPPRegistrarPolicy, error)
	ListFamilies(ctx context.Context) ([]*entities.RegistrarFamily, error)
	GetFamily(ctx context.Context, name string) (*entities.RegistrarFamily, error)
	SaveFamily(ctx context.Context, name string, cmd commands.SaveRegistrarFamilyCommand) (*entities.RegistrarFamily, error)
	DeleteFamily(ctx context.Context, name string) error
}

// EPPRateLimiter keeps the registrars' and families' command rate buckets
// and daily create quotas, across every node (docs/epp/ARCHITECTURE.md §5).
type EPPRateLimiter interface {
	// Take takes cost tokens from every bucket at once, if each can give
	// them within maxWait; otherwise it takes none.
	Take(ctx context.Context, buckets []entities.EPPRateBucket, cost int, maxWait time.Duration) (entities.EPPRateTake, error)
	// ReserveQuota counts one against the quota key, if it is below limit.
	// The count lasts ttl. release gives the reservation back, for a create
	// that did not succeed; it is nil when ok is false.
	ReserveQuota(ctx context.Context, key string, limit int, ttl time.Duration) (release func(context.Context), ok bool, err error)
}
