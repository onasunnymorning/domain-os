package interfaces

import (
	"context"

	"github.com/google/uuid"
	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
)

// EscrowKeyProbeStarter starts the workflow that proves, on a worker, that a
// key version's material can be fetched and used (issue #429, ADR-0009). The
// caller has already authorized the principal for the owner.
type EscrowKeyProbeStarter interface {
	StartEscrowKeyProbe(ctx context.Context, owner entities.EscrowKeyOwner, versionID uuid.UUID, requestedBy string) (workflowID string, err error)
}
