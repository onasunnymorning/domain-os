package interfaces

import (
	"context"

	"github.com/onasunnymorning/domain-os/internal/application/commands"
	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
)

// LifecyclePolicyService is the admin API's view of lifecycle policies
// (ADR 0023): the named timing a TLD phase follows.
type LifecyclePolicyService interface {
	// ListPolicies returns every policy, gtld-standard included, each with
	// the phases that name it.
	ListPolicies(ctx context.Context) ([]*entities.LifecyclePolicy, error)
	// GetPolicy returns a policy with the phases that name it.
	GetPolicy(ctx context.Context, name string) (*entities.LifecyclePolicy, error)
	SavePolicy(ctx context.Context, name string, cmd commands.SaveLifecyclePolicyCommand) (*entities.LifecyclePolicy, error)
	DeletePolicy(ctx context.Context, name string) error
}
