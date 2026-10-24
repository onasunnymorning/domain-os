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
	// CreateOutcomes is a registrar's daily counts of domain creates that
	// created a domain and that failed for a registered name.
	CreateOutcomes(ctx context.Context, clid string, from, to time.Time) ([]entities.EPPCreateOutcomes, error)
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

// EPPFailedCreateTracker counts domain creates refused because the name is
// registered (2302) against those that created a domain, per registrar and
// family over entities.EPPFailedCreateWindow, across every node, and keeps
// the strikes and login suspensions of the failed-create budget
// (docs/epp/ARCHITECTURE.md §5).
type EPPFailedCreateTracker interface {
	// Assess is asked before a domain create: whether its login is
	// suspended and, if any scope is over budget, the create's strike,
	// suspending the login past SuspendAfter strikes.
	Assess(ctx context.Context, c entities.EPPFailedCreateCheck) (entities.EPPFailedCreateStanding, error)
	// Record counts a create's outcome in each scope.
	Record(ctx context.Context, scopes []string, failed bool) error
	// Suspended is how much longer a login is suspended, or 0.
	Suspended(ctx context.Context, login string) (time.Duration, error)
}
