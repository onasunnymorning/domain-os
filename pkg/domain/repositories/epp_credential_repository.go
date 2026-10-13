package repositories

import (
	"context"

	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
)

// EPPCredentialRepository persists each registrar's EPP access: logins,
// passwords, certificate pins and allowed networks (ADR 0014).
//
// The access is one aggregate. UpdateAccess loads it under a lock on the
// registrar, applies a change, and saves the result together with the change's
// outbox events in one transaction, so the caps hold under concurrent changes
// and a committed change always has its event (INV-01).
//
// Credentials are never deleted; retired ones stay as the record of what was
// used, and when.
type EPPCredentialRepository interface {
	// GetAccess returns a registrar's access, empty if it has no credentials
	// yet. An unknown registrar is entities.ErrRegistrarNotFound.
	GetAccess(ctx context.Context, clid string) (*entities.EPPRegistrarAccess, error)
	// GetAccessByLogin returns the access of the registrar that owns loginID,
	// or entities.ErrEPPLoginNotFound.
	GetAccessByLogin(ctx context.Context, loginID string) (*entities.EPPRegistrarAccess, error)
	// UpdateAccess applies fn to a registrar's access and saves the result
	// with the events fn returns. An error from fn saves nothing. A new login
	// whose ID another registrar already uses fails with
	// entities.ErrEPPLoginExists.
	UpdateAccess(ctx context.Context, clid string, fn func(*entities.EPPRegistrarAccess) ([]entities.DomainEvent, error)) (*entities.EPPRegistrarAccess, error)
}
