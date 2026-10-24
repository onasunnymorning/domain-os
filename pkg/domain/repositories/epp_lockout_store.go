package repositories

import (
	"context"

	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
)

// EPPLockoutStore is the admin API's view of (login, source address) pairs
// locked out after failed passwords, kept where the EPP nodes keep them.
type EPPLockoutStore interface {
	// ListLockouts returns every pair locked now; ClID is not set.
	ListLockouts(ctx context.Context) ([]entities.EPPLoginLockout, error)
	// LiftLockouts clears a login's lockouts, failure counts and strikes,
	// from every source address, and returns the lockouts that were in
	// force.
	LiftLockouts(ctx context.Context, login string) ([]entities.EPPLoginLockout, error)
}
