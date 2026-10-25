package repositories

import (
	"context"
	"time"

	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
)

// EPPCreateOutcomeRepository keeps each registrar's daily counts of domain
// creates that created a domain and that were refused because the name is
// registered: the basis for a fee on excessive failed creates
// (docs/epp/POLICY_REFERENCE.md § Failed creates).
type EPPCreateOutcomeRepository interface {
	// AddCreateOutcomes adds the counts to what is stored for each
	// registrar and day.
	AddCreateOutcomes(ctx context.Context, counts []entities.EPPCreateOutcomes) error
	// ListCreateOutcomes returns a registrar's days from from to to,
	// inclusive, oldest first.
	ListCreateOutcomes(ctx context.Context, clid entities.ClIDType, from, to time.Time) ([]entities.EPPCreateOutcomes, error)
}
