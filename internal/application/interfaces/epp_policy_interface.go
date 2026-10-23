package interfaces

import (
	"context"

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
