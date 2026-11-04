package interfaces

import (
	"context"

	"github.com/onasunnymorning/domain-os/internal/application/commands"
	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
)

// LaunchApplicationService is the operator's side of launch applications
// (ADR 0024 §3): reading the applications filed in a phase and deciding
// them. Every method is within scope (INV-02): an operator scope reaches
// only applications in its own TLDs, and another operator's is
// entities.ErrTLDNotFound.
type LaunchApplicationService interface {
	// GetApplication returns one application by ID.
	GetApplication(ctx context.Context, scope entities.RegistryScope, id string) (*entities.LaunchApplication, error)
	// ListApplications lists a page of the applications filed in a phase,
	// oldest first, those of one status when status is not empty; the
	// cursor is the one a previous page gave (INV-11).
	ListApplications(ctx context.Context, scope entities.RegistryScope, tld, phase string, status entities.ApplicationStatus, pageSize int, pageCursor string) ([]*entities.LaunchApplication, error)
	// Validate records the outcome of validating a pendingValidation
	// application, and tells the applicant.
	Validate(ctx context.Context, scope entities.RegistryScope, cmd *commands.ValidateApplicationCommand) (*entities.LaunchApplication, error)
	// Allocate registers the name to a pendingAllocation application in its
	// phase, which may have ended, rejects every other open application for
	// the name, and tells each applicant; all in one transaction.
	Allocate(ctx context.Context, scope entities.RegistryScope, cmd *commands.AllocateApplicationCommand) (*entities.LaunchApplication, error)
	// Reject ends a non-final application with the reason, and tells the
	// applicant.
	Reject(ctx context.Context, scope entities.RegistryScope, cmd *commands.RejectApplicationCommand) (*entities.LaunchApplication, error)
}
