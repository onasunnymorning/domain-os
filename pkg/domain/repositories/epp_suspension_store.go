package repositories

import (
	"context"

	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
)

// EPPSuspensionStore is the admin API's view of logins suspended for
// failed creates (docs/epp/POLICY_REFERENCE.md § Failed creates), kept
// where the EPP nodes keep them.
type EPPSuspensionStore interface {
	// ListSuspensions returns every login suspended now.
	ListSuspensions(ctx context.Context) ([]entities.EPPLoginSuspension, error)
	// LiftSuspension ends a login's suspension and clears the strikes that
	// caused it, returning the suspension as it was. A login not suspended
	// is entities.ErrEPPSuspensionNotFound.
	LiftSuspension(ctx context.Context, login string) (*entities.EPPLoginSuspension, error)
}
